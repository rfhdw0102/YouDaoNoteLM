package reranker

import (
	"YoudaoNoteLm/internal/service/external"
	"fmt"
)

const ServiceType = "reranker"

func init() {
	r := external.GetGlobalRegistry()

	// 注册 Cohere
	r.Register(ServiceType, "cohere", "Cohere Rerank",
		[]string{"api_key"}, []string{"api_url", "model"},
		external.FactoryFunc(createCohereReranker), map[string]string{
			"api_key": "API Key",
			"api_url": "API 地址（默认 https://api.cohere.com）",
			"model":   "模型名称（默认 rerank-multilingual-v3.0）",
		})

	// 注册 Jina
	r.Register(ServiceType, "jina", "Jina Reranker",
		[]string{"api_key"}, []string{"api_url", "model"},
		external.FactoryFunc(createJinaReranker), map[string]string{
			"api_key": "API Key",
			"api_url": "API 地址（默认 https://api.jina.ai）",
			"model":   "模型名称（默认 jina-reranker-v2-base-multilingual）",
		})

	// 注册 SiliconFlow
	r.Register(ServiceType, "siliconflow", "SiliconFlow Reranker",
		[]string{"api_key"}, []string{"api_url", "model"},
		external.FactoryFunc(createSiliconFlowReranker), map[string]string{
			"api_key": "API Key",
			"api_url": "API 地址（默认 https://api.siliconflow.cn）",
			"model":   "模型名称（默认 BAAI/bge-reranker-v2-m3）",
		})
}

// createCohereReranker 创建 Cohere Reranker
func createCohereReranker(cfg *external.ServiceConfig) (interface{}, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("Cohere API Key 未配置")
	}

	baseURL := cfg.APIURL
	if baseURL == "" {
		baseURL = defaultCohereBaseURL
	}

	return NewCohereReranker(cfg.APIKey, baseURL, cfg.Model)
}

// createJinaReranker 创建 Jina Reranker
func createJinaReranker(cfg *external.ServiceConfig) (interface{}, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("Jina API Key 未配置")
	}

	baseURL := cfg.APIURL
	if baseURL == "" {
		baseURL = defaultJinaBaseURL
	}

	return NewJinaReranker(cfg.APIKey, baseURL, cfg.Model)
}

// createSiliconFlowReranker 创建 SiliconFlow Reranker
func createSiliconFlowReranker(cfg *external.ServiceConfig) (interface{}, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("SiliconFlow API Key 未配置")
	}

	baseURL := cfg.APIURL
	if baseURL == "" {
		baseURL = defaultSiliconFlowBaseURL
	}

	return NewSiliconFlowReranker(cfg.APIKey, baseURL, cfg.Model)
}
