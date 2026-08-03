package memory

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type mysqlStore struct {
	db *gorm.DB
}

func NewMySQLStore(db *gorm.DB) Store {
	return &mysqlStore{db: db}
}

func (s *mysqlStore) ListByUserID(ctx context.Context, userID uint) ([]UserMemory, error) {
	var items []UserMemory
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).Find(&items).Error
	return items, err
}

func (s *mysqlStore) Upsert(ctx context.Context, item UserMemory) (UserMemory, error) {
	err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "memory_type"}},
		DoUpdates: clause.AssignmentColumns([]string{"content", "updated_at"}),
	}).Create(&item).Error
	if err != nil {
		return UserMemory{}, err
	}

	var current UserMemory
	err = s.db.WithContext(ctx).
		Where("user_id = ? AND memory_type = ?", item.UserID, item.MemoryType).
		First(&current).Error
	if err != nil {
		return UserMemory{}, err
	}
	return current, nil
}

func (s *mysqlStore) DeleteByUserIDAndType(ctx context.Context, userID uint, typ Type) error {
	result := s.db.WithContext(ctx).
		Where("user_id = ? AND memory_type = ?", userID, string(typ)).
		Delete(&UserMemory{})
	if result.Error != nil {
		return result.Error
	}
	_ = result.RowsAffected // deleting a missing slot is intentionally idempotent
	return nil
}
