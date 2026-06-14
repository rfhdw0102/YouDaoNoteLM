package repository

import (
	"YoudaoNoteLm/internal/model/entity"
	"errors"

	"gorm.io/gorm"
)

type youdaoBindingRepository struct {
	db *gorm.DB
}

func NewYoudaoBindingRepository(db *gorm.DB) YoudaoBindingRepository {
	return &youdaoBindingRepository{db: db}
}

func (r *youdaoBindingRepository) FindByUserID(userID uint) (*entity.YoudaoBinding, error) {
	var binding entity.YoudaoBinding
	err := r.db.Where("user_id = ?", userID).First(&binding).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &binding, nil
}

func (r *youdaoBindingRepository) Create(binding *entity.YoudaoBinding) error {
	return r.db.Create(binding).Error
}

func (r *youdaoBindingRepository) Update(binding *entity.YoudaoBinding) error {
	return r.db.Save(binding).Error
}

func (r *youdaoBindingRepository) Delete(userID uint) error {
	return r.db.Where("user_id = ?", userID).Delete(&entity.YoudaoBinding{}).Error
}
