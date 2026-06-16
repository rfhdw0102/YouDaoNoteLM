// internal/service/external/embedding/ark_embedding.go
package embedding

import (
	"context"
	"fmt"

	einoArk "github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino/components/embedding"
)

// ArkEmbeddingService 基于 Eino ark 组件的 Embedding 服务
// 直接实现 EmbeddingService 接口，内部委托给 eino ark Embedder
type ArkEmbeddingService struct {
	embedder embedding.Embedder
}

// NewArkEmbeddingService 创建火山引擎 Embedding 服务
// apiKey: API Key
// model: 接入点 ID（如 ep-20260505091808-5tmzc）
// apiType: "text_api" 或 "multi_modal_api"，为空默认 "text_api"
// baseURL: API 地址，为空默认 https://ark.cn-beijing.volces.com/api/v3
func NewArkEmbeddingService(apiKey, model, apiType, baseURL string) (*ArkEmbeddingService, error) {
	if model == "" {
		return nil, fmt.Errorf("Embedding 模型名称或接入点 ID 未配置")
	}

	conf := &einoArk.EmbeddingConfig{
		APIKey: apiKey,
		Model:  model,
	}
	if baseURL != "" {
		conf.BaseURL = baseURL
	}
	if apiType != "" {
		t := einoArk.APIType(apiType)
		conf.APIType = &t
	}

	embedder, err := einoArk.NewEmbedder(context.Background(), conf)
	if err != nil {
		return nil, fmt.Errorf("创建 Ark Embedder 失败: %w", err)
	}

	return &ArkEmbeddingService{embedder: embedder}, nil
}

func (s *ArkEmbeddingService) Name() string {
	return "ark"
}

func (s *ArkEmbeddingService) Embed(text string) ([]float64, error) {
	results, err := s.embedder.EmbedStrings(context.Background(), []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("Embedding 返回空结果")
	}
	return results[0], nil
}

func (s *ArkEmbeddingService) EmbedBatch(texts []string) ([][]float64, error) {
	return s.embedder.EmbedStrings(context.Background(), texts)
}

// GetEmbedder 获取底层的 eino Embedder（供 RAG 流程直接使用）
func (s *ArkEmbeddingService) GetEmbedder() embedding.Embedder {
	return s.embedder
}
