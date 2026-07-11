package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/rag"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/pkg/cache"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"
)

// notebookService 笔记本服务实现
type notebookService struct {
	notebookRepo     repository.NotebookRepository
	sourceRepo       repository.SourceRepository
	conversationRepo repository.ConversationRepository
	messageRepo      repository.MessageRepository
	ingestionSvc     rag.IngestionService
	chatCache        *cache.ChatCache
	summaryCache     *cache.SourceSummaryCache
}

// NewNotebookService 创建笔记本服务
func NewNotebookService(
	notebookRepo repository.NotebookRepository,
	sourceRepo repository.SourceRepository,
	conversationRepo repository.ConversationRepository,
	messageRepo repository.MessageRepository,
	ingestionSvc rag.IngestionService,
	chatCache *cache.ChatCache,
	summaryCache *cache.SourceSummaryCache,
) NotebookService {
	return &notebookService{
		notebookRepo:     notebookRepo,
		sourceRepo:       sourceRepo,
		conversationRepo: conversationRepo,
		messageRepo:      messageRepo,
		ingestionSvc:     ingestionSvc,
		chatCache:        chatCache,
		summaryCache:     summaryCache,
	}
}

// Create 创建笔记本
func (s *notebookService) Create(userID uint, req *request.CreateNotebookRequest) (*response.NotebookResponse, error) {
	// 检查是否存在同名笔记本
	exists, err := s.notebookRepo.ExistsByName(userID, req.Name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, bizerrors.New(bizerrors.CodeConflict, "已存在同名笔记本")
	}

	notebook := &entity.Notebook{
		UserID: userID,
		Name:   req.Name,
	}

	if err := s.notebookRepo.Create(notebook); err != nil {
		return nil, err
	}

	return s.toResponse(notebook), nil
}

// List 查询用户的所有笔记本
func (s *notebookService) List(userID uint) ([]*response.NotebookResponse, error) {
	notebooks, err := s.notebookRepo.ListByUserID(userID)
	if err != nil {
		return nil, err
	}

	result := make([]*response.NotebookResponse, 0, len(notebooks))
	for _, nb := range notebooks {
		result = append(result, s.toResponse(nb))
	}
	return result, nil
}

// Rename 重命名笔记本
func (s *notebookService) Rename(userID, notebookID uint, req *request.RenameNotebookRequest) error {
	notebook, err := s.notebookRepo.FindByID(notebookID)
	if err != nil {
		return err
	}
	if notebook == nil {
		return bizerrors.ErrNotFound
	}

	// 检查权限
	if notebook.UserID != userID {
		return bizerrors.ErrForbidden
	}

	// 检查是否存在同名笔记本（排除自身）
	if notebook.Name != req.Name {
		exists, err := s.notebookRepo.ExistsByName(userID, req.Name)
		if err != nil {
			return err
		}
		if exists {
			return bizerrors.New(bizerrors.CodeConflict, "已存在同名笔记本")
		}
	}

	notebook.Name = req.Name
	return s.notebookRepo.Update(notebook)
}

// Delete 删除笔记本
func (s *notebookService) Delete(userID, notebookID uint) error {
	notebook, err := s.notebookRepo.FindByID(notebookID)
	if err != nil {
		return err
	}
	if notebook == nil {
		return bizerrors.ErrNotFound
	}

	// 检查权限
	if notebook.UserID != userID {
		return bizerrors.ErrForbidden
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 删除该笔记本下所有 source 的向量和 parent_blocks
	if s.ingestionSvc != nil && s.sourceRepo != nil {
		s.deleteNotebookVectors(ctx, userID, notebookID)
	}

	// 清理关联会话的 Redis 缓存和消息
	if s.conversationRepo != nil && s.chatCache != nil {
		s.cleanupNotebookConversations(ctx, notebookID)
	}

	// 清理该笔记本下所有 source 的摘要缓存
	if s.summaryCache != nil {
		s.cleanupNotebookSourceSummaries(ctx, userID, notebookID)
	}

	// 软删除该笔记本下所有 source（软删除不触发 CASCADE）
	if err := s.sourceRepo.DeleteByNotebookID(notebookID); err != nil {
		logger.Error("删除笔记本关联的 source 失败",
			zap.Uint("notebook_id", notebookID),
			zap.Error(err),
		)
	}

	return s.notebookRepo.Delete(notebookID)
}

// deleteNotebookVectors 删除笔记本下所有 source 的向量数据和 parent_blocks
func (s *notebookService) deleteNotebookVectors(ctx context.Context, userID, notebookID uint) {
	// 查询该笔记本下所有 source
	sources, _, err := s.sourceRepo.ListByNotebook(userID, notebookID, "", 0, 10000)
	if err != nil {
		logger.Error("查询笔记本关联的 source 失败",
			zap.Uint("notebook_id", notebookID),
			zap.Error(err),
		)
		return
	}

	// 逐个删除向量数据和 parent_blocks
	for _, source := range sources {
		if err := s.ingestionSvc.DeleteSource(ctx, userID, source.ID); err != nil {
			logger.Error("删除笔记本关联的源数据失败",
				zap.Uint("notebook_id", notebookID),
				zap.Uint("source_id", source.ID),
				zap.Error(err),
			)
		}
	}
}

// cleanupNotebookConversations 清理笔记本下所有会话的 Redis 缓存和消息
func (s *notebookService) cleanupNotebookConversations(ctx context.Context, notebookID uint) {
	convs, err := s.conversationRepo.FindByNotebookID(notebookID)
	if err != nil {
		logger.Error("查询笔记本关联的会话失败",
			zap.Uint("notebook_id", notebookID),
			zap.Error(err),
		)
		return
	}

	for _, conv := range convs {
		// 删除消息
		if s.messageRepo != nil {
			if err := s.messageRepo.DeleteByConversationID(conv.ID); err != nil {
				logger.Warn("删除会话消息失败",
					zap.Uint("conversation_id", conv.ID),
					zap.Error(err),
				)
			}
		}
		// 清除 Redis 缓存（消息历史+摘要）
		if err := s.chatCache.DeleteConversationCache(ctx, conv.ID); err != nil {
			logger.Warn("清除会话缓存失败",
				zap.Uint("conversation_id", conv.ID),
				zap.Error(err),
			)
		}
	}

	// 删除会话本身
	if err := s.conversationRepo.DeleteByNotebookID(notebookID); err != nil {
		logger.Error("删除笔记本关联的会话失败",
			zap.Uint("notebook_id", notebookID),
			zap.Error(err),
		)
	}
}

// cleanupNotebookSourceSummaries 清理笔记本下所有 source 的摘要缓存
func (s *notebookService) cleanupNotebookSourceSummaries(ctx context.Context, userID, notebookID uint) {
	// 查询该笔记本下所有 source
	sources, _, err := s.sourceRepo.ListByNotebook(userID, notebookID, "", 0, 10000)
	if err != nil {
		logger.Error("查询笔记本关联的 source 失败（用于清理摘要缓存）",
			zap.Uint("notebook_id", notebookID),
			zap.Error(err),
		)
		return
	}

	if len(sources) == 0 {
		return
	}

	// 收集所有 sourceID
	sourceIDs := make([]uint, len(sources))
	for i, source := range sources {
		sourceIDs[i] = source.ID
	}

	// 批量删除摘要缓存
	if err := s.summaryCache.BatchDelete(ctx, sourceIDs); err != nil {
		logger.Warn("批量删除摘要缓存失败",
			zap.Uint("notebook_id", notebookID),
			zap.Int("count", len(sourceIDs)),
			zap.Error(err),
		)
	} else {
		logger.Info("已清理笔记本关联的摘要缓存",
			zap.Uint("notebook_id", notebookID),
			zap.Int("count", len(sourceIDs)),
		)
	}
}

// toResponse 转换为响应 DTO
func (s *notebookService) toResponse(notebook *entity.Notebook) *response.NotebookResponse {
	return &response.NotebookResponse{
		ID:        notebook.ID,
		Name:      notebook.Name,
		CreatedAt: notebook.CreatedAt,
		UpdatedAt: notebook.UpdatedAt,
	}
}
