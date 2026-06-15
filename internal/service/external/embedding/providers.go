// internal/service/external/embedding/providers.go
package embedding

import (
	"fmt"

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

	// 默认使用 multi_modal_api（兼容所有火山引擎向量模型）
	apiType := cfg.GetExtraString("api_type")
	if apiType == "" {
		apiType = "multi_modal_api"
	}

	return NewArkEmbeddingService(cfg.APIKey, model, apiType, cfg.APIURL)
}

func init() {
	r := external.GetGlobalRegistry()

	// 火山引擎（豆包）Embedding — 基于 Eino ark 组件，支持文本和多模态
	r.Register(ServiceType, "volcengine", "火山引擎（豆包）Embedding",
		[]string{"api_key", "model"}, []string{},
		external.FactoryFunc(arkEmbeddingFactory), map[string]string{
			"api_key": "API Key",
			"model":   "接入点 ID（如 ep-20260505091808-xxxx）",
		})
}
