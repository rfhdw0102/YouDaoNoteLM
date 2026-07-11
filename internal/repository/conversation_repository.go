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

// FindByIDAndUserID 根据 ID + UserID 查找对话（权限校验场景使用）
func (r *conversationRepository) FindByIDAndUserID(id, userID uint) (*entity.Conversation, error) {
	var conv entity.Conversation
	err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&conv).Error
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

// FindByNotebookIDAndUserID 查找笔记本下属于指定用户的对话
func (r *conversationRepository) FindByNotebookIDAndUserID(notebookID, userID uint) ([]*entity.Conversation, error) {
	var convs []*entity.Conversation
	err := r.db.Where("notebook_id = ? AND user_id = ?", notebookID, userID).
		Order("updated_at DESC").Find(&convs).Error
	return convs, err
}

// Update 更新对话（使用 Save，会覆盖所有字段；只在结构体字段全部加载完成时使用）
func (r *conversationRepository) Update(conv *entity.Conversation) error {
	return r.db.Save(conv).Error
}

// UpdateTitle 仅更新标题字段
func (r *conversationRepository) UpdateTitle(id uint, title string) error {
	return r.db.Model(&entity.Conversation{}).
		Where("id = ?", id).
		Update("title", title).Error
}

// UpdateSummary 仅更新摘要字段
func (r *conversationRepository) UpdateSummary(id uint, summary string) error {
	return r.db.Model(&entity.Conversation{}).
		Where("id = ?", id).
		Update("summary", summary).Error
}

// Delete 删除对话（软删除）
func (r *conversationRepository) Delete(id uint) error {
	return r.db.Delete(&entity.Conversation{}, id).Error
}

// DeleteByNotebookID 删除笔记本下的所有对话（软删除）
func (r *conversationRepository) DeleteByNotebookID(notebookID uint) error {
	return r.db.Where("notebook_id = ?", notebookID).Delete(&entity.Conversation{}).Error
}

// DeleteWithMessages 在事务中删除对话及其所有消息
// 先软删消息，再软删对话，保证原子性
func (r *conversationRepository) DeleteWithMessages(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 删消息
		if err := tx.Where("conversation_id = ?", id).Delete(&entity.Message{}).Error; err != nil {
			return err
		}
		// 删对话
		if err := tx.Delete(&entity.Conversation{}, id).Error; err != nil {
			return err
		}
		return nil
	})
}
