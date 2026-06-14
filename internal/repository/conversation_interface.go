package repository

import "YoudaoNoteLm/internal/model/entity"

// ConversationRepository 对话仓储接口
type ConversationRepository interface {
	// Create 创建对话
	Create(conv *entity.Conversation) error
	// FindByID 根据 ID 查找对话
	FindByID(id uint) (*entity.Conversation, error)
	// FindByNotebookID 查找笔记本下的所有对话
	FindByNotebookID(notebookID uint) ([]*entity.Conversation, error)
	// Update 更新对话
	Update(conv *entity.Conversation) error
	// Delete 删除对话（软删除）
	Delete(id uint) error
}
