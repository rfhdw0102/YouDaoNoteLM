package repository

import "YoudaoNoteLm/internal/model/entity"

type SysConfigRepository interface {
	FindByGroup(group string) ([]*entity.SysConfig, error)
	FindByGroupAndKey(group, key string) (*entity.SysConfig, error)
	Create(config *entity.SysConfig) error
	Upsert(config *entity.SysConfig) error // 创建或更新（存在则更新）
	Update(config *entity.SysConfig) error
	Delete(id uint) error
	GetConfigStatusSummary() ([]map[string]interface{}, error)
}
