package repository

import (
	"YoudaoNoteLm/internal/model/entity"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// Upsert 创建或更新绑定（原子操作，使用 ON DUPLICATE KEY UPDATE 避免并发冲突）
func (r *youdaoBindingRepository) Upsert(binding *entity.YoudaoBinding) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"api_key", "status", "updated_at", "deleted_at"}),
	}).Create(binding).Error
}

func (r *youdaoBindingRepository) Update(binding *entity.YoudaoBinding) error {
	return r.db.Save(binding).Error
}

func (r *youdaoBindingRepository) Delete(userID uint) error {
	return r.db.Unscoped().Where("user_id = ?", userID).Delete(&entity.YoudaoBinding{}).Error
}
