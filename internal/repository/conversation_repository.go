package repository

import (
	"YoudaoNoteLm/internal/model/entity"
	"errors"

	"gorm.io/gorm"
)

// conversationRepository 对话仓储实现
type conversationRepository struct {
	db *gorm.DB
}

// NewConversationRepository 创建对话仓储
func NewConversationRepository(db *gorm.DB) ConversationRepository {
	return &conversationRepository{db: db}
}

// Create 创建对话
func (r *conversationRepository) Create(conv *entity.Conversation) error {
	return r.db.Create(conv).Error
}

// FindByID 根据 ID 查找对话
func (r *conversationRepository) FindByID(id uint) (*entity.Conversation, error) {
	var conv entity.Conversation
	err := r.db.First(&conv, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &conv, nil
}

// FindByNotebookID 查找笔记本下的所有对话
func (r *conversationRepository) FindByNotebookID(notebookID uint) ([]*entity.Conversation, error) {
	var convs []*entity.Conversation
	err := r.db.Where("notebook_id = ?", notebookID).Order("updated_at DESC").Find(&convs).Error
	return convs, err
}

// Update 更新对话
func (r *conversationRepository) Update(conv *entity.Conversation) error {
	return r.db.Save(conv).Error
}

// Delete 删除对话（软删除）
func (r *conversationRepository) Delete(id uint) error {
	return r.db.Delete(&entity.Conversation{}, id).Error
}
