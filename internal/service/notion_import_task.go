package service

import (
	"YoudaoNoteLm/internal/model/entity"
	notion "YoudaoNoteLm/internal/service/external/notion"
	"YoudaoNoteLm/pkg/cache"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	notionImportTimeout    = 10 * time.Minute
	notionPageImportTimout = 2 * time.Minute
	notionWorkerLimit      = 2
)

func (s *notionService) ImportPagesBatch(ctx context.Context, userID, notebookID uint, pageIDs []string) (string, []uint, error) {
	if !s.config.Configured() {
		return "", nil, bizerrors.ErrNotionNotConfigured
	}
	notebook, err := s.notebookRepo.FindByID(notebookID)
	if err != nil {
		return "", nil, fmt.Errorf("find notebook: %w", err)
	}
	if notebook == nil {
		return "", nil, bizerrors.ErrNotFound
	}
	if notebook.UserID != userID {
		return "", nil, bizerrors.ErrForbidden
	}
	accessToken, err := s.activeAccessToken(userID)
	if err != nil {
		return "", nil, err
	}
	uniqueIDs, err := normalizeNotionPageIDs(pageIDs)
	if err != nil {
		return "", nil, err
	}
	sourceIDs := make([]uint, 0, len(uniqueIDs))
	for _, pageID := range uniqueIDs {
		source := &entity.Source{UserID: userID, NotebookID: notebookID, Name: pageID, Type: "notion", ExternalID: pageID, Status: "pending"}
		if err := s.sourceRepo.Create(source); err != nil {
			return "", sourceIDs, fmt.Errorf("create pending notion source: %w", err)
		}
		sourceIDs = append(sourceIDs, source.ID)
	}
	taskID := uuid.NewString()
	task := &cache.ImportTask{TaskID: taskID, UserID: userID, NotebookID: notebookID, TaskType: "notion", TotalCount: len(sourceIDs), Status: "pending", CreatedAt: time.Now().Unix()}
	if err := s.taskStore.Save(ctx, task); err != nil {
		for _, sourceID := range sourceIDs {
			_ = s.sourceRepo.UpdateStatus(sourceID, "failed", "导入任务创建失败")
		}
		return "", nil, fmt.Errorf("save notion import task: %w", err)
	}
	taskCtx, cancel := context.WithTimeout(context.Background(), notionImportTimeout)
	s.taskStore.RegisterCancel(taskID, cancel)
	go s.processImportTask(taskCtx, task, accessToken, sourceIDs, uniqueIDs)
	return taskID, sourceIDs, nil
}

func normalizeNotionPageIDs(pageIDs []string) ([]string, error) {
	unique := make([]string, 0, len(pageIDs))
	seen := make(map[string]struct{}, len(pageIDs))
	for _, rawID := range pageIDs {
		pageID := strings.TrimSpace(rawID)
		if pageID == "" {
			return nil, bizerrors.ErrInvalidParam
		}
		if _, exists := seen[pageID]; exists {
			continue
		}
		seen[pageID] = struct{}{}
		unique = append(unique, pageID)
	}
	if len(unique) == 0 || len(unique) > notionBatchSizeMax {
		return nil, bizerrors.ErrInvalidParam
	}
	return unique, nil
}

type notionImportJob struct {
	sourceID uint
	pageID   string
}
type notionImportResult struct {
	sourceID uint
	err      error
}

// processImportTask owns all progress counters, preventing concurrent workers from overwriting Redis task state.
func (s *notionService) processImportTask(taskCtx context.Context, task *cache.ImportTask, accessToken string, sourceIDs []uint, pageIDs []string) {
	defer s.taskStore.ClearCancel(task.TaskID)
	task.Status = "running"
	_ = s.taskStore.Save(context.Background(), task)

	workerCount := notionWorkerLimit
	if len(pageIDs) < workerCount {
		workerCount = len(pageIDs)
	}
	jobs := make(chan notionImportJob, workerCount)
	results := make(chan notionImportResult, len(pageIDs))
	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range jobs {
				if err := taskCtx.Err(); err != nil {
					results <- notionImportResult{sourceID: job.sourceID, err: err}
					continue
				}
				results <- notionImportResult{sourceID: job.sourceID, err: s.processPage(taskCtx, accessToken, job.sourceID, job.pageID)}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for index, pageID := range pageIDs {
			select {
			case jobs <- notionImportJob{sourceID: sourceIDs[index], pageID: pageID}:
			case <-taskCtx.Done():
				return
			}
		}
	}()
	go func() { workers.Wait(); close(results) }()

	for result := range results {
		task.ProcessedCount++
		if result.err == nil {
			task.SuccessCount++
		} else if !errorsIsCancelled(result.err) {
			task.FailCount++
		}
		_ = s.taskStore.Save(context.Background(), task)
	}
	if errorsIsCancelled(taskCtx.Err()) {
		for _, sourceID := range sourceIDs {
			source, err := s.sourceRepo.FindByID(sourceID)
			if err == nil && source != nil && source.Status == "pending" {
				_ = s.sourceRepo.UpdateStatus(sourceID, "cancelled", bizerrors.ErrNotionImportCancelled.Message)
			}
		}
		task.Status = "cancelled"
	} else if taskCtx.Err() != nil {
		for _, sourceID := range sourceIDs {
			source, err := s.sourceRepo.FindByID(sourceID)
			if err == nil && source != nil && source.Status == "pending" {
				_ = s.sourceRepo.UpdateStatus(sourceID, "failed", bizerrors.ErrNotionContentFailed.Message)
			}
		}
		task.Status = "failed"
	} else if task.FailCount == 0 {
		task.Status = "completed"
	} else if task.SuccessCount == 0 {
		task.Status = "failed"
	} else {
		task.Status = "partial_failed"
	}
	_ = s.taskStore.Save(context.Background(), task)
}

func errorsIsCancelled(err error) bool {
	return err == bizerrors.ErrNotionImportCancelled || err == context.Canceled
}

func (s *notionService) processPage(ctx context.Context, accessToken string, sourceID uint, pageID string) error {
	if err := ctx.Err(); err != nil {
		if errorsIsCancelled(err) {
			return s.cancelNotionSource(sourceID)
		}
		return s.failNotionSource(sourceID, err)
	}
	if err := s.sourceRepo.UpdateStatus(sourceID, "processing", ""); err != nil {
		return fmt.Errorf("mark notion source processing: %w", err)
	}
	pageCtx, cancel := context.WithTimeout(ctx, notionPageImportTimout)
	defer cancel()
	page, err := s.client.GetPage(pageCtx, accessToken, pageID)
	if err != nil {
		return s.failNotionSource(sourceID, err)
	}
	markdown, err := notion.RenderPageMarkdown(pageCtx, s.client, accessToken, pageID, notion.DefaultRenderLimits())
	if err != nil {
		return s.failNotionSource(sourceID, err)
	}
	if err := pageCtx.Err(); err != nil {
		if errorsIsCancelled(err) {
			return s.cancelNotionSource(sourceID)
		}
		return s.failNotionSource(sourceID, err)
	}
	source, err := s.sourceRepo.FindByID(sourceID)
	if err != nil || source == nil {
		return fmt.Errorf("find notion source: %w", err)
	}
	if s.structurer != nil {
		result, structureErr := s.structurer.Structure(pageCtx, source.UserID, markdown, StructureMeta{Title: page.Title, SourceType: "notion"})
		if structureErr == nil && strings.TrimSpace(result.Content) != "" {
			markdown = result.Content
		}
	}
	if err := pageCtx.Err(); err != nil {
		if errorsIsCancelled(err) {
			return s.cancelNotionSource(sourceID)
		}
		return s.failNotionSource(sourceID, err)
	}
	title := strings.TrimSpace(page.Title)
	if title == "" {
		title = "未命名 Notion 页面"
	}
	source.Name, source.OriginalURL, source.MarkdownContent, source.Status, source.ErrorMessage = title, page.URL, markdown, "processing", ""
	if err := s.sourceRepo.Update(source); err != nil {
		return s.failNotionSource(sourceID, err)
	}
	if s.ingestionSvc != nil {
		if err := s.ingestionSvc.IngestSingle(pageCtx, sourceID); err != nil {
			return s.failNotionSource(sourceID, err)
		}
	}
	if err := s.sourceRepo.UpdateStatus(sourceID, "ready", ""); err != nil {
		return fmt.Errorf("mark notion source ready: %w", err)
	}
	if s.configSvc != nil {
		go doGenerateAndSaveSummary(s.sourceRepo, s.configSvc, s.summaryCache, sourceID, source.UserID, markdown)
	}
	return nil
}

func (s *notionService) failNotionSource(sourceID uint, cause error) error {
	if errorsIsCancelled(cause) {
		return s.cancelNotionSource(sourceID)
	}
	friendly := mapNotionError(cause)
	_ = s.sourceRepo.UpdateStatus(sourceID, "failed", friendly.(*bizerrors.BizError).Message)
	return friendly
}

func (s *notionService) cancelNotionSource(sourceID uint) error {
	_ = s.sourceRepo.UpdateStatus(sourceID, "cancelled", bizerrors.ErrNotionImportCancelled.Message)
	return bizerrors.ErrNotionImportCancelled
}

func (s *notionService) GetImportTask(ctx context.Context, userID uint, taskID string) (*cache.ImportTask, error) {
	return s.taskStore.GetForUser(ctx, userID, taskID)
}
func (s *notionService) CancelImportTask(ctx context.Context, userID uint, taskID string) error {
	_, err := s.taskStore.CancelForUser(ctx, userID, taskID)
	return err
}
