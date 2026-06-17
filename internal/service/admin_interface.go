package service

import (
	"YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/model/entity"
	"encoding/json"
)

// AdminService 后台管理服务接口
type AdminService interface {
	ListUsers(page, size int, keyword string) ([]*response.AdminUserResponse, int64, error)
	UpdateUserStatus(userID uint, enabled bool) error
	GetConfigs(group string) ([]*entity.SysConfig, error)
	UpdateConfig(group, key string, value json.RawMessage, enabled bool) error
	AddConfig(group, key string, value json.RawMessage, description string) error
	DeleteConfig(group, key string) error
	GetConfigStatus() ([]response.ConfigStatusGroupResponse, error)
}
