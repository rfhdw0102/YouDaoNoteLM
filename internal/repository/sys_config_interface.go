package repository

import "YoudaoNoteLm/internal/model/entity"

type SysConfigRepository interface {
	FindByGroup(group string) ([]*entity.SysConfig, error)
	FindByGroupAndKey(group, key string) (*entity.SysConfig, error)
	Create(config *entity.SysConfig) error
	Update(config *entity.SysConfig) error
	GetConfigStatusSummary() ([]map[string]interface{}, error)
}
