// internal/service/external/llm/providers.go
package llm

import (
	"fmt"

	"YoudaoNoteLm/internal/service/external"
)

const ServiceType = "llm"

// openaiCompatibleFactory 创建 OpenAI 兼容的 LLM 客户端
func openaiCompatibleFactory(_ string) external.FactoryFunc {
	return func(cfg *external.ServiceConfig) (interface{}, error) {
		model := cfg.Model
		if model == "" {
			model = cfg.GetExtraString("model")
		}
		if model == "" {
			return nil, fmt.Errorf("LLM 模型名称未配置")
		}
		return NewOpenAIClient(cfg.Provider, cfg.APIURL, cfg.APIKey, model), nil
	}
}

func init() {
	r := external.GetGlobalRegistry()

	// OpenAI（兼容所有 OpenAI API 格式的服务商，如 DeepSeek、智谱、通义、Kimi 等）
	// 使用时修改 api_url 和 model 即可接入不同服务商
	r.Register(ServiceType, "openai", "OpenAI（兼容）",
		[]string{"api_key", "model"}, []string{"api_url"},
		openaiCompatibleFactory("OpenAI"), map[string]string{
			"api_key": "API Key",
			"model":   "模型名称（如 gpt-4o、deepseek-chat、glm-4）",
			"api_url": "API 地址（可选，默认 https://api.openai.com/v1）",
		})

	// Anthropic（Claude）
	r.Register(ServiceType, "anthropic", "Anthropic（Claude）",
		[]string{"api_key", "model"}, []string{"api_url"},
		func(cfg *external.ServiceConfig) (interface{}, error) {
			model := cfg.Model
			if model == "" {
				model = cfg.GetExtraString("model")
			}
			if model == "" {
				return nil, fmt.Errorf("Claude 模型名称未配置")
			}
			return NewAnthropicClient(cfg.APIURL, cfg.APIKey, model), nil
		}, map[string]string{
			"api_key": "API Key",
			"model":   "模型名称（如 claude-sonnet-4-20250514）",
			"api_url": "API 地址（可选，默认 https://api.anthropic.com）",
		})
}
