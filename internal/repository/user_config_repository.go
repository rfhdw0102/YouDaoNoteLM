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

func (r *userConfigRepository) FindByUserAndTypeIncludingDeleted(userID uint, configType string) (*entity.UserConfig, error) {
	var config entity.UserConfig
	err := r.db.Unscoped().Where("user_id = ? AND config_type = ?", userID, configType).First(&config).Error
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
	// 使用 Updates 方法只更新指定字段，避免更新 created_at 等自动管理的字段
	err := r.db.Unscoped().Model(config).Updates(map[string]interface{}{
		"name":         config.Name,
		"provider":     config.Provider,
		"api_key":      config.APIKey,
		"api_url":      config.APIURL,
		"model":        config.Model,
		"dimensions":   config.Dimensions,
		"daily_quota":  config.DailyQuota,
		"extra_config": config.ExtraConfig,
		"enabled":      config.Enabled,
		"deleted_at":   nil, // 恢复为未删除状态
	}).Error
	if err != nil {
		// 记录详细错误信息
		return err
	}
	return nil
}

func (r *userConfigRepository) Delete(id uint) error {
	return r.db.Delete(&entity.UserConfig{}, id).Error
}
