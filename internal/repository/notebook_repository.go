package repository

import (
	"YoudaoNoteLm/internal/model/entity"
	"errors"

	"gorm.io/gorm"
)

// notebookRepository 笔记本仓储实现
type notebookRepository struct {
	db *gorm.DB
}

// NewNotebookRepository 创建笔记本仓储
func NewNotebookRepository(db *gorm.DB) NotebookRepository {
	return &notebookRepository{db: db}
}

// Create 创建笔记本
func (r *notebookRepository) Create(notebook *entity.Notebook) error {
	return r.db.Create(notebook).Error
}

// FindByID 根据 ID 查找笔记本
func (r *notebookRepository) FindByID(id uint) (*entity.Notebook, error) {
	var notebook entity.Notebook
	err := r.db.First(&notebook, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &notebook, nil
}

// ExistsByName 检查用户下是否存在同名笔记本
func (r *notebookRepository) ExistsByName(userID uint, name string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.Notebook{}).Where("user_id = ? AND name = ?", userID, name).Count(&count).Error
	return count > 0, err
}

// ListByUserID 查询用户的所有笔记本（按创建时间降序）
func (r *notebookRepository) ListByUserID(userID uint) ([]*entity.Notebook, error) {
	var notebooks []*entity.Notebook
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&notebooks).Error
	return notebooks, err
}

// Update 更新笔记本
func (r *notebookRepository) Update(notebook *entity.Notebook) error {
	return r.db.Save(notebook).Error
}

// Delete 删除笔记本（软删除）
func (r *notebookRepository) Delete(id uint) error {
	return r.db.Delete(&entity.Notebook{}, id).Error
}

// CountByUserID 统计用户笔记本数量
func (r *notebookRepository) CountByUserID(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&entity.Notebook{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}
