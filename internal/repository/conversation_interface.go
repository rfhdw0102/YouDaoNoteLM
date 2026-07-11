package repository

import "YoudaoNoteLm/internal/model/entity"

// ConversationRepository 对话仓储接口
type ConversationRepository interface {
	// Create 创建对话
	Create(conv *entity.Conversation) error
	// FindByID 根据 ID 查找对话
	FindByID(id uint) (*entity.Conversation, error)
	// FindByIDAndUserID 根据 ID + UserID 查找对话（用于权限校验）
	FindByIDAndUserID(id, userID uint) (*entity.Conversation, error)
	// FindByNotebookID 查找笔记本下的所有对话
	FindByNotebookID(notebookID uint) ([]*entity.Conversation, error)
	// FindByNotebookIDAndUserID 查找笔记本下属于指定用户的对话
	FindByNotebookIDAndUserID(notebookID, userID uint) ([]*entity.Conversation, error)
	// Update 更新对话（注意：使用 Save，会覆盖所有字段，零值会清空数据库字段；
	// 若仅更新部分字段，请使用 UpdateTitle / UpdateSummary 等专用方法）
	Update(conv *entity.Conversation) error
	// UpdateTitle 仅更新标题字段
	UpdateTitle(id uint, title string) error
	// UpdateSummary 仅更新摘要字段
	UpdateSummary(id uint, summary string) error
	// Delete 删除对话（软删除）
	Delete(id uint) error
	// DeleteByNotebookID 删除笔记本下的所有对话（软删除）
	DeleteByNotebookID(notebookID uint) error
}
