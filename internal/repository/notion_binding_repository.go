package repository

import (
	"YoudaoNoteLm/internal/model/entity"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type notionBindingRepository struct {
	db *gorm.DB
}

func NewNotionBindingRepository(db *gorm.DB) NotionBindingRepository {
	return &notionBindingRepository{db: db}
}

func (r *notionBindingRepository) FindByUserID(userID uint) (*entity.NotionBinding, error) {
	var binding entity.NotionBinding
	err := r.db.Where("user_id = ?", userID).First(&binding).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &binding, nil
}

// Upsert 创建或替换用户的绑定，恢复先前软删除的记录。
func (r *notionBindingRepository) Upsert(binding *entity.NotionBinding) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"workspace_id", "workspace_name", "workspace_icon", "bot_id",
			"access_token_encrypted", "status", "updated_at", "deleted_at",
		}),
	}).Create(binding).Error
}

// Delete 硬删除本地凭据，确保重新授权不会被旧唯一索引阻塞。
func (r *notionBindingRepository) Delete(userID uint) error {
	return r.db.Unscoped().Where("user_id = ?", userID).Delete(&entity.NotionBinding{}).Error
}
