package service

import (
	"YoudaoNoteLm/internal/rag"
	"YoudaoNoteLm/internal/service/external/storage"
	"context"
	"time"

	"YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/pkg/cache"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"
	"go.uber.org/zap"
)

type sourceService struct {
	sourceRepo   repository.SourceRepository
	storage      storage.FileStorage
	ingestionSvc rag.IngestionService
	summaryCache *cache.SourceSummaryCache
}

func NewSourceService(sourceRepo repository.SourceRepository, storage storage.FileStorage, ingestionSvc rag.IngestionService, summaryCache *cache.SourceSummaryCache) SourceService {
	return &sourceService{sourceRepo: sourceRepo, storage: storage, ingestionSvc: ingestionSvc, summaryCache: summaryCache}
}

func (s *sourceService) List(userID, notebookID uint, keyword string, page, size int) ([]*response.SourceResponse, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	if size > 100 {
		size = 100
	}

	offset := (page - 1) * size
	sources, total, err := s.sourceRepo.ListByNotebook(userID, notebookID, keyword, offset, size)
	if err != nil {
		return nil, 0, err
	}

	list := make([]*response.SourceResponse, 0, len(sources))
	for _, src := range sources {
		list = append(list, toSourceResponse(src))
	}

	return list, total, nil
}

func (s *sourceService) GetByID(id uint) (*entity.Source, error) {
	source, err := s.sourceRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, bizerrors.ErrNotFound
	}
	return source, nil
}

func (s *sourceService) Rename(id uint, name string) error {
	source, err := s.GetByID(id)
	if err != nil {
		return err
	}
	source.Name = name
	return s.sourceRepo.Update(source)
}

func (s *sourceService) Delete(id uint) error {
	source, err := s.GetByID(id)
	if err != nil {
		return err
	}

	// 删除 Milvus 中的向量数据
	if s.ingestionSvc != nil && source.Vectorized {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.ingestionSvc.DeleteSource(ctx, source.UserID, id); err != nil {
			logger.Error("删除源向量数据失败",
				zap.Uint("source_id", id),
				zap.Error(err),
			)
			// 向量删除失败不阻塞主流程，记录日志继续
		}
	}

	// 删除摘要缓存
	if s.summaryCache != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.summaryCache.Delete(ctx, id); err != nil {
			logger.Warn("删除摘要缓存失败",
				zap.Uint("source_id", id),
				zap.Error(err),
			)
		}
	}

	return s.sourceRepo.Delete(id)
}

func (s *sourceService) BatchDelete(ids []uint) error {
	// 批量删除前，先删除每个 source 的向量数据
	if s.ingestionSvc != nil && len(ids) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		for _, id := range ids {
			source, err := s.sourceRepo.FindByID(id)
			if err != nil || source == nil {
				continue
			}
			if source.Vectorized {
				if err := s.ingestionSvc.DeleteSource(ctx, source.UserID, id); err != nil {
					logger.Error("批量删除时删除源向量数据失败",
						zap.Uint("source_id", id),
						zap.Error(err),
					)
				}
			}
		}
	}

	// 批量删除摘要缓存
	if s.summaryCache != nil && len(ids) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.summaryCache.BatchDelete(ctx, ids); err != nil {
			logger.Warn("批量删除摘要缓存失败",
				zap.Uints("source_ids", ids),
				zap.Error(err),
			)
		}
	}

	return s.sourceRepo.BatchDelete(ids)
}

func (s *sourceService) DeleteFailed(userID, notebookID uint) (int64, error) {
	return s.sourceRepo.DeleteFailedByNotebook(userID, notebookID)
}

func (s *sourceService) GetContent(id uint) (string, error) {
	source, err := s.GetByID(id)
	if err != nil {
		return "", err
	}
	return source.MarkdownContent, nil
}

func (s *sourceService) GetOriginalContent(id uint) (string, string, error) {
	source, err := s.GetByID(id)
	if err != nil {
		return "", "", err
	}

	switch source.Type {
	case "file":
		// 对于文件类型，返回 Markdown 内容作为原内容展示
		// 原始文件通过 GetDownloadURL 提供下载
		return source.MarkdownContent, source.MimeType, nil
	case "url":
		return source.OriginalURL, "url", nil
	case "audio":
		return source.MarkdownContent, "audio_transcript", nil
	case "note", "youdao":
		return source.MarkdownContent, "raw_markdown", nil
	default:
		return "", "", bizerrors.New(bizerrors.CodeBadRequest, "该类型不支持查看原格式")
	}
}

func (s *sourceService) GetDownloadURL(id uint) (string, error) {
	source, err := s.GetByID(id)
	if err != nil {
		return "", err
	}
	if source.FilePath == "" {
		return "", bizerrors.New(bizerrors.CodeBadRequest, "该来源没有可下载的文件")
	}

	// 如果 storage 是 MinIO，生成预签名 URL
	if minioStorage, ok := s.storage.(interface {
		GetPresignedURL(filePath string, expiry time.Duration) (string, error)
	}); ok {
		url, err := minioStorage.GetPresignedURL(source.FilePath, 10*time.Minute)
		if err != nil {
			return "", bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "生成下载链接失败", err)
		}
		return url, nil
	}

	return "", bizerrors.New(bizerrors.CodeInternalServiceError, "存储服务不支持生成下载链接")
}

func toSourceResponse(src *entity.Source) *response.SourceResponse {
	return &response.SourceResponse{
		ID:           src.ID,
		NotebookID:   src.NotebookID,
		Name:         src.Name,
		Type:         src.Type,
		OriginalURL:  src.OriginalURL,
		FilePath:     src.FilePath,
		FileSize:     src.FileSize,
		MimeType:     src.MimeType,
		Status:       src.Status,
		ErrorMessage: src.ErrorMessage,
		Vectorized:   src.Vectorized,
		CreatedAt:    src.CreatedAt,
		UpdatedAt:    src.UpdatedAt,
	}
}

// ReimportAll 重新导入用户所有未向量化的资料
func (s *sourceService) ReimportAll(userID uint) (int, error) {
	if s.ingestionSvc == nil {
		return 0, bizerrors.New(bizerrors.CodeInternalServiceError, "向量入库服务未初始化")
	}

	// 获取用户所有未向量化的资料
	sources, err := s.sourceRepo.FindUnvectorizedByUserID(userID)
	if err != nil {
		return 0, err
	}

	if len(sources) == 0 {
		return 0, nil
	}

	// 收集所有 source ID
	sourceIDs := make([]uint, 0, len(sources))
	for _, src := range sources {
		sourceIDs = append(sourceIDs, src.ID)
	}

	// 批量入库
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 即使部分失败也继续，返回成功导入的数量
	ingestErr := s.ingestionSvc.Ingest(ctx, sourceIDs)

	// 统计实际成功导入的数量（vectorized=true 的数量）
	successCount := 0
	for _, src := range sources {
		updated, err := s.sourceRepo.FindByID(src.ID)
		if err == nil && updated != nil && updated.Vectorized {
			successCount++
		}
	}

	if ingestErr != nil {
		logger.Warn("批量重新入库部分失败",
			zap.Uint("user_id", userID),
			zap.Int("total", len(sourceIDs)),
			zap.Int("success", successCount),
			zap.Error(ingestErr),
		)
	}

	logger.Info("批量重新入库完成",
		zap.Uint("user_id", userID),
		zap.Int("total", len(sourceIDs)),
		zap.Int("success", successCount),
	)

	return successCount, nil
}

// ReimportSelected 重新导入指定的未向量化资料
func (s *sourceService) ReimportSelected(sourceIDs []uint) (int, error) {
	if s.ingestionSvc == nil {
		return 0, bizerrors.New(bizerrors.CodeInternalServiceError, "向量入库服务未初始化")
	}

	if len(sourceIDs) == 0 {
		return 0, nil
	}

	// 验证所有 source 都存在且未向量化
	for _, id := range sourceIDs {
		source, err := s.sourceRepo.FindByID(id)
		if err != nil {
			return 0, err
		}
		if source == nil {
			return 0, bizerrors.New(bizerrors.CodeNotFound, "资料不存在")
		}
		if source.Vectorized {
			return 0, bizerrors.New(bizerrors.CodeBadRequest, "资料已入库，无需重复导入")
		}
	}

	// 批量入库
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	ingestErr := s.ingestionSvc.Ingest(ctx, sourceIDs)

	// 统计实际成功导入的数量
	successCount := 0
	for _, id := range sourceIDs {
		updated, err := s.sourceRepo.FindByID(id)
		if err == nil && updated != nil && updated.Vectorized {
			successCount++
		}
	}

	if ingestErr != nil {
		logger.Warn("批量重新入库部分失败",
			zap.Int("total", len(sourceIDs)),
			zap.Int("success", successCount),
			zap.Error(ingestErr),
		)
	}

	logger.Info("批量重新入库完成",
		zap.Int("total", len(sourceIDs)),
		zap.Int("success", successCount),
	)

	return successCount, nil
}

// CreateFromNote 将笔记内容保存为来源
func (s *sourceService) CreateFromNote(userID, notebookID uint, title, content string) (*response.SourceResponse, error) {
	if title == "" {
		return nil, bizerrors.New(bizerrors.CodeBadRequest, "标题不能为空")
	}
	if content == "" {
		return nil, bizerrors.New(bizerrors.CodeBadRequest, "内容不能为空")
	}

	source := &entity.Source{
		UserID:          userID,
		NotebookID:      notebookID,
		Name:            title,
		Type:            "note",
		MarkdownContent: content,
		Status:          "pending",
		Vectorized:      false,
	}

	if err := s.sourceRepo.Create(source); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "创建来源失败", err)
	}

	// 异步进行向量化入库
	if s.ingestionSvc != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if err := s.ingestionSvc.Ingest(ctx, []uint{source.ID}); err != nil {
				logger.Warn("笔记来源向量化入库失败",
					zap.Uint("source_id", source.ID),
					zap.Error(err),
				)
			}
		}()
	}

	return toSourceResponse(source), nil
}

// DeleteByNoteAndNotebook 根据笔记标题和笔记本ID删除来源
func (s *sourceService) DeleteByNoteAndNotebook(userID, notebookID uint, title string) error {
	sources, _, err := s.sourceRepo.ListByNotebook(userID, notebookID, title, 0, 100)
	if err != nil {
		return err
	}

	for _, src := range sources {
		if src.Name == title && src.Type == "note" {
			return s.Delete(src.ID)
		}
	}

	return bizerrors.New(bizerrors.CodeNotFound, "未找到对应的笔记来源")
}
