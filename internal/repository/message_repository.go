package repository

import (
	"YoudaoNoteLm/internal/model/entity"

	"gorm.io/gorm"
)

// messageRepository 消息仓储实现
type messageRepository struct {
	db *gorm.DB
}

// NewMessageRepository 创建消息仓储
func NewMessageRepository(db *gorm.DB) MessageRepository {
	return &messageRepository{db: db}
}

// Create 创建消息
func (r *messageRepository) Create(msg *entity.Message) error {
	return r.db.Create(msg).Error
}

// CreateBatch 批量创建消息
func (r *messageRepository) CreateBatch(msgs []*entity.Message) error {
	return r.db.Create(msgs).Error
}

// FindByConversationID 查找对话的所有消息
func (r *messageRepository) FindByConversationID(conversationID uint) ([]*entity.Message, error) {
	var msgs []*entity.Message
	err := r.db.Where("conversation_id = ?", conversationID).Order("created_at ASC").Find(&msgs).Error
	return msgs, err
}

// FindRecentByConversationID 查找对话的最近 N 条消息
func (r *messageRepository) FindRecentByConversationID(conversationID uint, limit int) ([]*entity.Message, error) {
	var msgs []*entity.Message
	err := r.db.Where("conversation_id = ?", conversationID).
		Order("created_at DESC").
		Limit(limit).
		Find(&msgs).Error
	// 反转为时间正序
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, err
}

// CountByConversationID 统计对话消息数
func (r *messageRepository) CountByConversationID(conversationID uint) (int64, error) {
	var count int64
	err := r.db.Model(&entity.Message{}).Where("conversation_id = ?", conversationID).Count(&count).Error
	return count, err
}

// DeleteByConversationID 软删除对话下的所有消息
func (r *messageRepository) DeleteByConversationID(conversationID uint) error {
	return r.db.Where("conversation_id = ?", conversationID).Delete(&entity.Message{}).Error
}
