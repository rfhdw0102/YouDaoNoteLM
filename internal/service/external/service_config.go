// internal/service/external/service_config.go
package external

import "encoding/json"

// ServiceConfig 通用服务配置
// 从 user_config / sys_config 映射而来，所有 provider 工厂函数统一接收
type ServiceConfig struct {
	Provider    string                 // 服务商名称（如 "searxng"、"openai"）
	APIURL      string                 // API 地址
	APIKey      string                 // API 密钥
	Model       string                 // 模型名称（LLM / Embedding 用）
	ExtraConfig map[string]interface{} // 服务商特有参数（从 JSON 解析）
}

// GetExtraString 从 ExtraConfig 获取字符串值
func (c *ServiceConfig) GetExtraString(key string) string {
	if c.ExtraConfig == nil {
		return ""
	}
	if v, ok := c.ExtraConfig[key].(string); ok {
		return v
	}
	return ""
}

// ParseExtraConfig 从 JSON 字符串解析 ExtraConfig
func (c *ServiceConfig) ParseExtraConfig(jsonStr string) {
	if jsonStr == "" {
		return
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &m); err == nil {
		c.ExtraConfig = m
	}
}

// NewServiceConfigFromEntity 从 UserConfig/SysConfig 字段构建 ServiceConfig
func NewServiceConfigFromEntity(provider, apiURL, apiKey, model, extraConfig string) *ServiceConfig {
	sc := &ServiceConfig{
		Provider: provider,
		APIURL:   apiURL,
		APIKey:   apiKey,
		Model:    model,
	}
	sc.ParseExtraConfig(extraConfig)
	return sc
}
