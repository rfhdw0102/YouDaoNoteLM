package repository

import (
	"YoudaoNoteLm/internal/model/entity"
	"errors"

	"gorm.io/gorm"
)

type sysConfigRepository struct {
	db *gorm.DB
}

func NewSysConfigRepository(db *gorm.DB) SysConfigRepository {
	return &sysConfigRepository{db: db}
}

func (r *sysConfigRepository) FindByGroup(group string) ([]*entity.SysConfig, error) {
	var configs []*entity.SysConfig
	err := r.db.Where("config_group = ?", group).Order("config_key ASC").Find(&configs).Error
	return configs, err
}

func (r *sysConfigRepository) FindByGroupAndKey(group, key string) (*entity.SysConfig, error) {
	var config entity.SysConfig
	err := r.db.Where("config_group = ? AND config_key = ?", group, key).First(&config).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

func (r *sysConfigRepository) Create(config *entity.SysConfig) error {
	return r.db.Create(config).Error
}

func (r *sysConfigRepository) Update(config *entity.SysConfig) error {
	return r.db.Save(config).Error
}

func (r *sysConfigRepository) GetConfigStatusSummary() ([]map[string]interface{}, error) {
	type groupStatus struct {
		ConfigGroup string `gorm:"column:config_group"`
		Total       int64  `gorm:"column:total"`
		Enabled     int64  `gorm:"column:enabled_count"`
		Description string `gorm:"column:description"`
	}

	var results []groupStatus
	err := r.db.Model(&entity.SysConfig{}).
		Select("config_group, COUNT(*) as total, SUM(CASE WHEN enabled = 1 THEN 1 ELSE 0 END) as enabled_count, MIN(description) as description").
		Group("config_group").
		Find(&results).Error
	if err != nil {
		return nil, err
	}

	summary := make([]map[string]interface{}, 0, len(results))
	for _, r := range results {
		summary = append(summary, map[string]interface{}{
			"group":       r.ConfigGroup,
			"total":       r.Total,
			"enabled":     r.Enabled,
			"description": r.Description,
		})
	}
	return summary, nil
}
