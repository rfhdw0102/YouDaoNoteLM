package service

import (
	"YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/model/entity"
	"encoding/json"
)

// AdminService 后台管理服务接口
type AdminService interface {
	ListUsers(page, size int, keyword string) ([]*response.AdminUserResponse, int64, error)
	// UpdateUserStatus 启用/禁用用户。operatorID 为操作者 ID，用于校验不能禁用同等级用户。
	UpdateUserStatus(operatorID, targetID uint, enabled bool) error
	GetConfigs(group string) ([]*entity.SysConfig, error)
	UpdateConfig(group, key string, value json.RawMessage, enabled bool) error
	AddConfig(group, key string, value json.RawMessage, description string) error
	DeleteConfig(group, key string) error
	GetConfigStatus() ([]response.ConfigStatusGroupResponse, error)
}
