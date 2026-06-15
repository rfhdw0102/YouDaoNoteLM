package request

import "encoding/json"

// UserStatusRequest 启用/禁用用户请求
type UserStatusRequest struct {
	Enabled bool `json:"enabled"`
}

// ConfigUpdateRequest 更新配置请求
type ConfigUpdateRequest struct {
	ConfigValue json.RawMessage `json:"config_value" binding:"required"`
	Enabled     bool            `json:"enabled"`
}

// ConfigAddRequest 新增配置请求
type ConfigAddRequest struct {
	ConfigKey   string          `json:"config_key" binding:"required"`
	ConfigValue json.RawMessage `json:"config_value" binding:"required"`
	Description string          `json:"description"`
}
