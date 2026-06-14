package repository

import (
	"YoudaoNoteLm/internal/model/entity"
	"errors"

	"gorm.io/gorm"
)

type sourceRepository struct {
	db *gorm.DB
}

func NewSourceRepository(db *gorm.DB) SourceRepository {
	return &sourceRepository{db: db}
}

func (r *sourceRepository) FindByID(id uint) (*entity.Source, error) {
	var source entity.Source
	err := r.db.First(&source, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &source, nil
}

func (r *sourceRepository) Create(source *entity.Source) error {
	return r.db.Create(source).Error
}

func (r *sourceRepository) Update(source *entity.Source) error {
	return r.db.Save(source).Error
}

func (r *sourceRepository) Delete(id uint) error {
	return r.db.Delete(&entity.Source{}, id).Error
}

func (r *sourceRepository) BatchDelete(ids []uint) error {
	return r.db.Delete(&entity.Source{}, "id IN ?", ids).Error
}

func (r *sourceRepository) ListByNotebook(userID, notebookID uint, keyword string, offset, limit int) ([]*entity.Source, int64, error) {
	var sources []*entity.Source
	var total int64

	query := r.db.Where("user_id = ? AND notebook_id = ?", userID, notebookID)
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}

	if err := query.Model(&entity.Source{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&sources).Error
	if err != nil {
		return nil, 0, err
	}

	return sources, total, nil
}

func (r *sourceRepository) UpdateStatus(id uint, status string, errMsg string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if errMsg != "" {
		updates["error_message"] = errMsg
	}
	return r.db.Model(&entity.Source{}).Where("id = ?", id).Updates(updates).Error
}

func (r *sourceRepository) SetVectorized(id uint) error {
	return r.db.Model(&entity.Source{}).Where("id = ?", id).Update("vectorized", true).Error
}
