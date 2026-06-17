package service

import (
	"context"
	"encoding/json"

	"YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/pkg/cache"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

// conversationService 对话管理服务实现
type conversationService struct {
	conversationRepo repository.ConversationRepository
	messageRepo      repository.MessageRepository
	cache            *cache.ChatCache
}

// 确保实现了接口
var _ ConversationService = (*conversationService)(nil)

// NewConversationService 创建对话管理服务
func NewConversationService(
	conversationRepo repository.ConversationRepository,
	messageRepo repository.MessageRepository,
	chatCache *cache.ChatCache,
) ConversationService {
	return &conversationService{
		conversationRepo: conversationRepo,
		messageRepo:      messageRepo,
		cache:            chatCache,
	}
}

// CreateConversation 创建对话
func (s *conversationService) CreateConversation(ctx context.Context, userID, notebookID uint, title string) (uint, error) {
	conv := &entity.Conversation{
		NotebookID: notebookID,
		UserID:     userID,
		Title:      title,
	}
	if conv.Title == "" {
		conv.Title = "新对话"
	}

	if err := s.conversationRepo.Create(conv); err != nil {
		return 0, bizerrors.NewWithErr(bizerrors.CodeInternalError, "创建对话失败", err)
	}
	return conv.ID, nil
}

// GetConversation 获取对话详情
func (s *conversationService) GetConversation(ctx context.Context, conversationID uint) (*response.ConversationResponse, error) {
	conv, err := s.conversationRepo.FindByID(conversationID)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询对话失败", err)
	}
	if conv == nil {
		return nil, bizerrors.ErrNotFound
	}

	return &response.ConversationResponse{
		ID:         conv.ID,
		Title:      conv.Title,
		NotebookID: conv.NotebookID,
		CreatedAt:  conv.CreatedAt,
		UpdatedAt:  conv.UpdatedAt,
	}, nil
}

// ListConversations 获取对话列表
func (s *conversationService) ListConversations(ctx context.Context, notebookID uint) ([]*response.ConversationResponse, error) {
	convs, err := s.conversationRepo.FindByNotebookID(notebookID)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询对话列表失败", err)
	}

	result := make([]*response.ConversationResponse, 0, len(convs))
	for _, conv := range convs {
		result = append(result, &response.ConversationResponse{
			ID:         conv.ID,
			Title:      conv.Title,
			NotebookID: conv.NotebookID,
			CreatedAt:  conv.CreatedAt,
			UpdatedAt:  conv.UpdatedAt,
		})
	}
	return result, nil
}

// UpdateConversation 更新对话标题
func (s *conversationService) UpdateConversation(ctx context.Context, conversationID uint, title string) error {
	conv, err := s.conversationRepo.FindByID(conversationID)
	if err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询对话失败", err)
	}
	if conv == nil {
		return bizerrors.ErrNotFound
	}

	conv.Title = title
	if err := s.conversationRepo.Update(conv); err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "更新对话失败", err)
	}
	return nil
}

// DeleteConversation 删除对话
func (s *conversationService) DeleteConversation(ctx context.Context, conversationID uint) error {
	conv, err := s.conversationRepo.FindByID(conversationID)
	if err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询对话失败", err)
	}
	if conv == nil {
		return bizerrors.ErrNotFound
	}

	// 先删除关联的消息
	if err := s.messageRepo.DeleteByConversationID(conversationID); err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "删除对话消息失败", err)
	}

	// 再删除对话
	if err := s.conversationRepo.Delete(conversationID); err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "删除对话失败", err)
	}

	// 清除 Redis 缓存
	if err := s.cache.DeleteConversationCache(ctx, conversationID); err != nil {
		logger.Warn("[Agent] 清除对话缓存失败", zap.Error(err))
	}

	return nil
}

// GetMessages 获取消息历史
func (s *conversationService) GetMessages(ctx context.Context, conversationID uint) ([]*response.MessageResponse, error) {
	msgs, err := s.messageRepo.FindByConversationID(conversationID)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询消息失败", err)
	}

	result := make([]*response.MessageResponse, 0, len(msgs))
	for _, msg := range msgs {
		resp := &response.MessageResponse{
			ID:        msg.ID,
			Role:      msg.Role,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt,
		}

		if msg.Metadata != "" {
			var metadata response.MessageMetadata
			if err := json.Unmarshal([]byte(msg.Metadata), &metadata); err == nil {
				resp.Metadata = &metadata
			}
		}

		result = append(result, resp)
	}
	return result, nil
}
