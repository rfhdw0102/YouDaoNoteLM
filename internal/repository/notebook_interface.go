package repository

import "YoudaoNoteLm/internal/model/entity"

// NotebookRepository 笔记本仓储接口
type NotebookRepository interface {
	// Create 创建笔记本
	Create(notebook *entity.Notebook) error
	// FindByID 根据 ID 查找笔记本
	FindByID(id uint) (*entity.Notebook, error)
	// ExistsByName 检查用户下是否存在同名笔记本
	ExistsByName(userID uint, name string) (bool, error)
	// ListByUserID 查询用户的所有笔记本（按创建时间降序）
	ListByUserID(userID uint) ([]*entity.Notebook, error)
	// Update 更新笔记本
	Update(notebook *entity.Notebook) error
	// Delete 删除笔记本（软删除）
	Delete(id uint) error
	// CountByUserID 统计用户笔记本数量
	CountByUserID(userID uint) (int64, error)
}
