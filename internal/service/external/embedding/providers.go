// internal/service/external/embedding/providers.go
package embedding

import (
	"fmt"
	"strconv"

	"YoudaoNoteLm/internal/service/external"
)

const ServiceType = "embedding"

// arkEmbeddingFactory 创建火山引擎 Ark Embedding 客户端（基于 Eino 框架）
func arkEmbeddingFactory(cfg *external.ServiceConfig) (interface{}, error) {
	model := cfg.Model
	if model == "" {
		model = cfg.GetExtraString("model")
	}
	if model == "" {
		return nil, fmt.Errorf("Embedding 接入点 ID 未配置")
	}

	// API 地址：必填，默认 https://ark.cn-beijing.volces.com/api/v3
	apiURL := cfg.APIURL
	if apiURL == "" {
		apiURL = "https://ark.cn-beijing.volces.com/api/v3"
	}

	// 向量维度：必填，默认 2048
	dimensionsStr := cfg.GetExtraString("dimensions")
	dimensions := 2048
	if dimensionsStr != "" {
		if d, err := strconv.Atoi(dimensionsStr); err == nil && d > 0 {
			dimensions = d
		}
	}

	return NewArkEmbeddingService(cfg.APIKey, model, "multi_modal_api", apiURL, dimensions)
}

func init() {
	r := external.GetGlobalRegistry()

	// 火山引擎（豆包）Embedding — 基于 Eino ark 组件，支持文本和多模态
	r.Register(ServiceType, "volcengine", "火山引擎（豆包）Embedding",
		[]string{"api_key", "model", "dimensions", "api_url"}, []string{},
		external.FactoryFunc(arkEmbeddingFactory), map[string]string{
			"api_key":    "API Key",
			"model":      "接入点 ID（如 ep-20260505091808-xxxx）",
			"dimensions": "向量维度（默认 2048）",
			"api_url":    "API 地址（默认 https://ark.cn-beijing.volces.com/api/v3）",
		})

	// OpenAI 兼容 Embedding — 支持 OpenAI、千问、智谱等
	r.Register(ServiceType, "openai", "OpenAI 兼容 Embedding",
		[]string{"api_key", "model", "dimensions", "api_url"}, []string{},
		external.FactoryFunc(openaiEmbeddingFactory), map[string]string{
			"api_key":    "API Key",
			"model":      "模型名称（如 text-embedding-3-small）",
			"dimensions": "向量维度",
			"api_url":    "API 地址（如 https://dashscope.aliyuncs.com/compatible-mode/v1）",
		})
}

// openaiEmbeddingFactory 创建 OpenAI 兼容 Embedding 客户端
func openaiEmbeddingFactory(cfg *external.ServiceConfig) (interface{}, error) {
	model := cfg.Model
	if model == "" {
		return nil, fmt.Errorf("Embedding 模型名称未配置")
	}

	apiURL := cfg.APIURL
	if apiURL == "" {
		return nil, fmt.Errorf("API 地址未配置")
	}

	// 向量维度：必填
	dimensionsStr := cfg.GetExtraString("dimensions")
	if dimensionsStr == "" {
		return nil, fmt.Errorf("向量维度未配置")
	}
	dimensions, err := strconv.Atoi(dimensionsStr)
	if err != nil || dimensions <= 0 {
		return nil, fmt.Errorf("向量维度必须是正整数")
	}

	return NewOpenAIEmbeddingService(cfg.APIKey, model, apiURL, dimensions)
}
