package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/rag"
	"YoudaoNoteLm/internal/repository"
	externalYoudao "YoudaoNoteLm/internal/service/external/youdao"
	"YoudaoNoteLm/pkg/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type youdaoService struct {
	cli          externalYoudao.CLI
	bindingRepo  repository.YoudaoBindingRepository
	sourceRepo   repository.SourceRepository
	ingestionSvc rag.IngestionService
	cancelFuncs  sync.Map // taskID -> context.CancelFunc
	cookiesPath  string   // youdaonote cookies 文件路径（用于 .note 格式转换）
}

// NewYoudaoService 创建有道云笔记服务
func NewYoudaoService(
	cli externalYoudao.CLI,
	bindingRepo repository.YoudaoBindingRepository,
	sourceRepo repository.SourceRepository,
	ingestionSvc rag.IngestionService,
	cookiesPath string,
) YoudaoService {
	return &youdaoService{
		cli:          cli,
		bindingRepo:  bindingRepo,
		sourceRepo:   sourceRepo,
		ingestionSvc: ingestionSvc,
		cookiesPath:  cookiesPath,
	}
}

// getAPIKey 获取用户的有道 API Key（内部辅助方法）
func (s *youdaoService) getAPIKey(userID uint) (string, error) {
	binding, err := s.bindingRepo.FindByUserID(userID)
	if err != nil {
		return "", fmt.Errorf("查询绑定信息失败: %w", err)
	}
	if binding == nil || binding.Status != "active" {
		return "", fmt.Errorf("请先绑定有道云笔记账号")
	}
	return binding.APIKey, nil
}

// Bind 绑定有道 API Key
func (s *youdaoService) Bind(userID uint, apiKey string) error {
	// 1. 检查 CLI 是否可用
	if err := s.cli.CheckAvailable(); err != nil {
		return fmt.Errorf("youdaonote CLI 不可用: %w", err)
	}

	// 2. 验证 Key 有效性（调用 list 测试）
	_, err := s.cli.List(apiKey, "")
	if err != nil {
		return fmt.Errorf("API Key 无效，请检查后重试")
	}

	// 3. 使用 Upsert 原子操作，避免并发冲突
	binding := &entity.YoudaoBinding{
		UserID: userID,
		APIKey: apiKey,
		Status: "active",
	}
	return s.bindingRepo.Upsert(binding)
}

// Unbind 解绑有道账号
func (s *youdaoService) Unbind(userID uint) error {
	return s.bindingRepo.Delete(userID)
}

// GetBinding 获取绑定信息
func (s *youdaoService) GetBinding(userID uint) (*entity.YoudaoBinding, error) {
	return s.bindingRepo.FindByUserID(userID)
}

// ListNotes 浏览有道云笔记目录
func (s *youdaoService) ListNotes(userID uint, folderID string) ([]externalYoudao.NoteItem, error) {
	apiKey, err := s.getAPIKey(userID)
	if err != nil {
		return nil, err
	}

	items, err := s.cli.List(apiKey, folderID)
	if err != nil {
		return nil, fmt.Errorf("获取笔记列表失败: %w", err)
	}

	return items, nil
}

// ImportNote 导入单篇有道云笔记到本系统
func (s *youdaoService) ImportNote(userID uint, notebookID uint, fileID string) (*entity.Source, error) {
	apiKey, err := s.getAPIKey(userID)
	if err != nil {
		return nil, err
	}

	// 1. 读取笔记内容
	readResult, err := s.cli.Read(apiKey, fileID)
	if err != nil {
		return nil, fmt.Errorf("读取笔记内容失败: %w", err)
	}

	content := strings.TrimSpace(readResult.Content)

	// .note 格式必须转换为 Markdown（向量化要求 Markdown 格式）
	if readResult.RawFormat == "note" {
		// 空笔记无需转换，直接返回空内容，由调用方处理
		if content == "" && s.cookiesPath == "" {
			return nil, fmt.Errorf("笔记内容为空")
		}
		if s.cookiesPath == "" {
			return nil, fmt.Errorf("笔记为 .note 格式，但未配置 cookies 文件路径，无法转换")
		}
		logger.Info("笔记为 .note 格式，开始转换为 Markdown", zap.String("file_id", fileID))
		convertedContent, convertErr := s.cli.ConvertNote(fileID, s.cookiesPath)
		if convertErr != nil {
			return nil, fmt.Errorf(".note 格式转换失败: %w", convertErr)
		}
		if strings.TrimSpace(convertedContent) == "" {
			return nil, fmt.Errorf(".note 格式转换后内容为空")
		}
		content = convertedContent
		logger.Info(".note 格式转换成功", zap.String("file_id", fileID))
	} else if content == "" && s.cookiesPath != "" {
		// 非 .note 格式但内容为空，尝试转换（可能是格式识别错误）
		logger.Info("内容为空，尝试使用 youdaonote-pull 转换", zap.String("file_id", fileID))
		convertedContent, convertErr := s.cli.ConvertNote(fileID, s.cookiesPath)
		if convertErr != nil {
			logger.Warn("youdaonote-pull 转换失败", zap.String("file_id", fileID), zap.Error(convertErr))
		} else if strings.TrimSpace(convertedContent) != "" {
			content = convertedContent
			logger.Info("youdaonote-pull 转换成功", zap.String("file_id", fileID))
		}
	}

	// 检查内容是否为空
	if content == "" {
		return nil, fmt.Errorf("笔记内容为空或格式不支持")
	}

	// 2. 通过 list 获取笔记名称
	noteName := fileID // 降级使用 fileID
	items, listErr := s.cli.List(apiKey, "")
	if listErr == nil {
		for _, item := range items {
			if item.ID == fileID {
				noteName = item.Name
				break
			}
		}
	}

	// 3. 创建 Source 记录
	source := &entity.Source{
		UserID:          userID,
		NotebookID:      notebookID,
		Name:            noteName,
		Type:            "youdao",
		ExternalID:      fileID,
		MarkdownContent: content,
		Status:          "ready",
	}

	if err := s.sourceRepo.Create(source); err != nil {
		return nil, fmt.Errorf("创建 Source 记录失败: %w", err)
	}

	// 4. 同步触发 RAG 入库
	if s.ingestionSvc != nil {
		if err := s.ingestionSvc.IngestSingle(context.Background(), source.ID); err != nil {
			return nil, fmt.Errorf("RAG 入库失败: %w", err)
		}
	}

	logger.Info("有道笔记导入成功",
		zap.Uint("user_id", userID),
		zap.String("file_id", fileID),
		zap.String("name", noteName),
	)

	return source, nil
}

// ImportNotesBatch 批量导入有道云笔记
func (s *youdaoService) ImportNotesBatch(userID uint, notebookID uint, fileIDs []string, fileNames map[string]string) (string, []uint, error) {
	apiKey, err := s.getAPIKey(userID)
	if err != nil {
		return "", nil, err
	}

	// 去重
	seen := make(map[string]struct{}, len(fileIDs))
	uniqueIDs := make([]string, 0, len(fileIDs))
	for _, id := range fileIDs {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}

	sourceIDs := make([]uint, 0, len(uniqueIDs))

	// 为每个 fileID 创建 pending 状态的 Source
	for _, fileID := range uniqueIDs {
		// 优先使用前端传递的笔记标题，降级使用 fileID
		noteName := fileID
		if name, ok := fileNames[fileID]; ok && name != "" {
			noteName = name
		}

		source := &entity.Source{
			UserID:     userID,
			NotebookID: notebookID,
			Name:       noteName,
			Type:       "youdao",
			ExternalID: fileID,
			Status:     "pending",
		}
		if err := s.sourceRepo.Create(source); err != nil {
			logger.Error("创建待导入有道笔记Source失败", zap.String("file_id", fileID), zap.Error(err))
			continue
		}
		sourceIDs = append(sourceIDs, source.ID)
	}

	if len(sourceIDs) == 0 {
		return "", nil, fmt.Errorf("创建导入记录失败")
	}

	// 创建可取消的 context
	taskID := uuid.New().String()
	taskCtx, cancel := context.WithCancel(context.Background())
	s.cancelFuncs.Store(taskID, cancel)

	// 异步处理
	go s.processBatch(taskCtx, taskID, apiKey, sourceIDs, uniqueIDs)

	return taskID, sourceIDs, nil
}

// processBatch 批量处理有道笔记导入
func (s *youdaoService) processBatch(taskCtx context.Context, taskID string, apiKey string, sourceIDs []uint, fileIDs []string) {
	defer s.cancelFuncs.Delete(taskID)

	concurrency := 3
	if len(fileIDs) < concurrency {
		concurrency = len(fileIDs)
	}

	type task struct {
		sourceID uint
		fileID   string
	}

	taskCh := make(chan task, concurrency)
	doneCh := make(chan struct{}, len(fileIDs))

	// 启动 worker
	for i := 0; i < concurrency; i++ {
		go func() {
			for t := range taskCh {
				if taskCtx.Err() != nil {
					doneCh <- struct{}{}
					continue
				}
				s.processSingleNote(taskCtx, apiKey, t.sourceID, t.fileID)
				doneCh <- struct{}{}
			}
		}()
	}

	// 分发任务
	go func() {
		for i, fileID := range fileIDs {
			if taskCtx.Err() != nil {
				break
			}
			taskCh <- task{sourceID: sourceIDs[i], fileID: fileID}
		}
		close(taskCh)
	}()

	// 等待完成
	for i := 0; i < len(fileIDs); i++ {
		<-doneCh
	}

	// 处理被取消的 pending 任务
	if taskCtx.Err() != nil {
		for _, sourceID := range sourceIDs {
			src, err := s.sourceRepo.FindByID(sourceID)
			if err != nil || src == nil {
				continue
			}
			if src.Status == "pending" {
				if err := s.sourceRepo.UpdateStatus(sourceID, "cancelled", "任务已取消"); err != nil {
					logger.Warn("更新Source状态为cancelled失败", zap.Uint("source_id", sourceID), zap.Error(err))
				}
			}
		}
	}
}

// processSingleNote 处理单篇有道笔记导入
func (s *youdaoService) processSingleNote(taskCtx context.Context, apiKey string, sourceID uint, fileID string) {
	if taskCtx.Err() != nil {
		return
	}

	// 更新状态为 processing
	if err := s.sourceRepo.UpdateStatus(sourceID, "processing", ""); err != nil {
		logger.Warn("更新Source状态为processing失败", zap.Uint("source_id", sourceID), zap.Error(err))
	}

	// 读取笔记内容
	readResult, err := s.cli.Read(apiKey, fileID)
	if err != nil {
		if taskCtx.Err() != nil {
			return
		}
		if updateErr := s.sourceRepo.UpdateStatus(sourceID, "failed", fmt.Sprintf("读取失败: %v", err)); updateErr != nil {
			logger.Warn("更新Source状态为failed失败", zap.Uint("source_id", sourceID), zap.Error(updateErr))
		}
		return
	}

	content := strings.TrimSpace(readResult.Content)

	// .note 格式必须转换为 Markdown（向量化要求 Markdown 格式）
	if readResult.RawFormat == "note" {
		// 空笔记无需转换，跳过入库
		if content == "" && s.cookiesPath == "" {
			logger.Info("笔记内容为空，跳过入库", zap.String("file_id", fileID))
			if updateErr := s.sourceRepo.UpdateStatus(sourceID, "ready", ""); updateErr != nil {
				logger.Warn("更新Source状态失败", zap.Uint("source_id", sourceID), zap.Error(updateErr))
			}
			return
		}
		if s.cookiesPath == "" {
			if updateErr := s.sourceRepo.UpdateStatus(sourceID, "failed", "笔记为 .note 格式，但未配置 cookies 文件路径"); updateErr != nil {
				logger.Warn("更新Source状态为failed失败", zap.Uint("source_id", sourceID), zap.Error(updateErr))
			}
			return
		}
		logger.Info("笔记为 .note 格式，开始转换为 Markdown", zap.String("file_id", fileID))
		convertedContent, convertErr := s.cli.ConvertNote(fileID, s.cookiesPath)
		if convertErr != nil {
			if updateErr := s.sourceRepo.UpdateStatus(sourceID, "failed", fmt.Sprintf(".note 格式转换失败: %v", convertErr)); updateErr != nil {
				logger.Warn("更新Source状态为failed失败", zap.Uint("source_id", sourceID), zap.Error(updateErr))
			}
			return
		}
		if strings.TrimSpace(convertedContent) == "" {
			if updateErr := s.sourceRepo.UpdateStatus(sourceID, "failed", ".note 格式转换后内容为空"); updateErr != nil {
				logger.Warn("更新Source状态为failed失败", zap.Uint("source_id", sourceID), zap.Error(updateErr))
			}
			return
		}
		content = convertedContent
		logger.Info(".note 格式转换成功", zap.String("file_id", fileID))
	} else if content == "" && s.cookiesPath != "" {
		// 非 .note 格式但内容为空，尝试转换（可能是格式识别错误）
		logger.Info("内容为空，尝试使用 youdaonote-pull 转换", zap.String("file_id", fileID))
		convertedContent, convertErr := s.cli.ConvertNote(fileID, s.cookiesPath)
		if convertErr != nil {
			logger.Warn("youdaonote-pull 转换失败", zap.String("file_id", fileID), zap.Error(convertErr))
		} else if strings.TrimSpace(convertedContent) != "" {
			content = convertedContent
			logger.Info("youdaonote-pull 转换成功", zap.String("file_id", fileID))
		}
	}

	// 检查内容是否为空
	if content == "" {
		if taskCtx.Err() != nil {
			return
		}
		if updateErr := s.sourceRepo.UpdateStatus(sourceID, "failed", "笔记内容为空或格式不支持"); updateErr != nil {
			logger.Warn("更新Source状态为failed失败", zap.Uint("source_id", sourceID), zap.Error(updateErr))
		}
		return
	}

	// 检查 Source 是否还存在
	existing, err := s.sourceRepo.FindByID(sourceID)
	if err != nil {
		logger.Warn("查询Source失败", zap.Uint("source_id", sourceID), zap.Error(err))
		return
	}
	if existing == nil {
		return
	}

	// 更新内容和状态
	existing.MarkdownContent = content
	existing.Status = "ready"
	if err := s.sourceRepo.Update(existing); err != nil {
		if updateErr := s.sourceRepo.UpdateStatus(sourceID, "failed", fmt.Sprintf("保存失败: %v", err)); updateErr != nil {
			logger.Warn("更新Source状态为failed失败", zap.Uint("source_id", sourceID), zap.Error(updateErr))
		}
		return
	}

	// 同步触发 RAG 入库
	if s.ingestionSvc != nil {
		if err := s.ingestionSvc.IngestSingle(context.Background(), sourceID); err != nil {
			logger.Error("有道笔记批量导入 RAG 入库失败", zap.Uint("source_id", sourceID), zap.Error(err))
			if updateErr := s.sourceRepo.UpdateStatus(sourceID, "failed", fmt.Sprintf("RAG 入库失败: %v", err)); updateErr != nil {
				logger.Warn("更新Source状态为failed失败", zap.Uint("source_id", sourceID), zap.Error(updateErr))
			}
			return
		}
	}
}
