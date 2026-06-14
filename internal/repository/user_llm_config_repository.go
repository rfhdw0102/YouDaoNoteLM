package repository

import (
	"errors"

	"YoudaoNoteLm/internal/model/entity"

	"gorm.io/gorm"
)

type userLLMConfigRepository struct {
	db *gorm.DB
}

func NewUserLLMConfigRepository(db *gorm.DB) UserLLMConfigRepository {
	return &userLLMConfigRepository{db: db}
}

// FindByID 根据 ID 查找配置
func (r *userLLMConfigRepository) FindByID(id uint) (*entity.UserLLMConfig, error) {
	var config entity.UserLLMConfig
	err := r.db.First(&config, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

// FindByUserID 查找用户的所有 LLM 配置
func (r *userLLMConfigRepository) FindByUserID(userID uint) ([]*entity.UserLLMConfig, error) {
	var configs []*entity.UserLLMConfig
	err := r.db.Where("user_id = ?", userID).Order("id ASC").Find(&configs).Error
	return configs, err
}

// FindEnabledByUserID 查找用户的启用的 LLM 配置
func (r *userLLMConfigRepository) FindEnabledByUserID(userID uint) ([]*entity.UserLLMConfig, error) {
	var configs []*entity.UserLLMConfig
	err := r.db.Where("user_id = ? AND enabled = ?", userID, true).Order("id ASC").Find(&configs).Error
	return configs, err
}

// FindDefaultByUserID 查找用户的默认 LLM 配置（第一个启用的）
func (r *userLLMConfigRepository) FindDefaultByUserID(userID uint) (*entity.UserLLMConfig, error) {
	var config entity.UserLLMConfig
	err := r.db.Where("user_id = ? AND enabled = ?", userID, true).Order("id ASC").First(&config).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

// Create 创建配置
func (r *userLLMConfigRepository) Create(config *entity.UserLLMConfig) error {
	return r.db.Create(config).Error
}

// Update 更新配置
func (r *userLLMConfigRepository) Update(config *entity.UserLLMConfig) error {
	return r.db.Save(config).Error
}

// Delete 删除配置
func (r *userLLMConfigRepository) Delete(id uint) error {
	return r.db.Delete(&entity.UserLLMConfig{}, id).Error
}
