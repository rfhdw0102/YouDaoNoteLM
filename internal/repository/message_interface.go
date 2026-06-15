package repository

import "YoudaoNoteLm/internal/model/entity"

// MessageRepository 消息仓储接口
type MessageRepository interface {
	// Create 创建消息
	Create(msg *entity.Message) error
	// CreateBatch 批量创建消息
	CreateBatch(msgs []*entity.Message) error
	// FindByConversationID 查找对话的所有消息
	FindByConversationID(conversationID uint) ([]*entity.Message, error)
	// FindRecentByConversationID 查找对话的最近 N 条消息
	FindRecentByConversationID(conversationID uint, limit int) ([]*entity.Message, error)
	// CountByConversationID 统计对话消息数
	CountByConversationID(conversationID uint) (int64, error)
	// DeleteByConversationID 软删除对话下的所有消息
	DeleteByConversationID(conversationID uint) error
}
