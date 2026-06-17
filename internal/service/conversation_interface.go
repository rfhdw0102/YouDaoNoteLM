package service

import (
	"context"

	"YoudaoNoteLm/internal/model/dto/response"
)

// ConversationService 对话管理服务接口
type ConversationService interface {
	// CreateConversation 创建对话
	CreateConversation(ctx context.Context, userID, notebookID uint, title string) (uint, error)

	// GetConversation 获取对话详情
	GetConversation(ctx context.Context, conversationID uint) (*response.ConversationResponse, error)

	// ListConversations 获取对话列表
	ListConversations(ctx context.Context, notebookID uint) ([]*response.ConversationResponse, error)

	// UpdateConversation 更新对话标题
	UpdateConversation(ctx context.Context, conversationID uint, title string) error

	// DeleteConversation 删除对话
	DeleteConversation(ctx context.Context, conversationID uint) error

	// GetMessages 获取消息历史
	GetMessages(ctx context.Context, conversationID uint) ([]*response.MessageResponse, error)
}
