package embedding

import (
    "fmt"

    "YoudaoNoteLm/internal/service/external"
)

const ServiceType = "embedding"

func arkEmbeddingFactory(cfg *external.ServiceConfig) (interface{}, error) {
    model := cfg.Model
    if model == "" {
        model = cfg.GetExtraString("model")
    }
    if model == "" {
        return nil, fmt.Errorf("embedding model is required")
    }

    apiType := cfg.GetExtraString("api_type")
    if apiType == "" {
        apiType = "multi_modal_api"
    }

    return NewArkEmbeddingService(cfg.APIKey, model, apiType, cfg.APIURL)
}

func init() {
    r := external.GetGlobalRegistry()
    r.Register(ServiceType, "volcengine", "Volcengine Embedding",
        []string{"api_key", "model"}, []string{},
        external.FactoryFunc(arkEmbeddingFactory), map[string]string{
            "api_key": "API Key",
            "model":   "Endpoint ID",
        })
    r.RegisterAlias(ServiceType, "doubao", "volcengine")
}
