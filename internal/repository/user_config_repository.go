package repository

import (
	"YoudaoNoteLm/internal/model/entity"
	"errors"

	"gorm.io/gorm"
)

type userConfigRepository struct {
	db *gorm.DB
}

func NewUserConfigRepository(db *gorm.DB) UserConfigRepository {
	return &userConfigRepository{db: db}
}

func (r *userConfigRepository) FindByUserAndType(userID uint, configType string) (*entity.UserConfig, error) {
	var config entity.UserConfig
	err := r.db.Where("user_id = ? AND config_type = ?", userID, configType).First(&config).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

func (r *userConfigRepository) FindByID(id uint) (*entity.UserConfig, error) {
	var config entity.UserConfig
	err := r.db.First(&config, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

func (r *userConfigRepository) Create(config *entity.UserConfig) error {
	return r.db.Create(config).Error
}

func (r *userConfigRepository) Update(config *entity.UserConfig) error {
	return r.db.Save(config).Error
}

func (r *userConfigRepository) Delete(id uint) error {
	return r.db.Delete(&entity.UserConfig{}, id).Error
}
