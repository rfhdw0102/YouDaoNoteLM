// internal/service/external/embedding/openai_embedding.go
package embedding

import (
	"context"
	"fmt"

	einoOpenai "github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino/components/embedding"
)

// OpenAIEmbeddingService 基于 Eino openai 组件的 Embedding 服务
// 支持 OpenAI 及其兼容接口（千问、智谱等）
type OpenAIEmbeddingService struct {
	embedder embedding.Embedder
}

// NewOpenAIEmbeddingService 创建 OpenAI 兼容 Embedding 服务
// apiKey: API Key
// model: 模型名称（如 text-embedding-3-small、text-embedding-v3）
// baseURL: API 地址（如 https://dashscope.aliyuncs.com/compatible-mode/v1）
// dimensions: 向量维度
func NewOpenAIEmbeddingService(apiKey, model, baseURL string, dimensions int) (*OpenAIEmbeddingService, error) {
	if model == "" {
		return nil, fmt.Errorf("Embedding 模型名称未配置")
	}
	if baseURL == "" {
		return nil, fmt.Errorf("API 地址未配置")
	}
	if dimensions <= 0 {
		return nil, fmt.Errorf("向量维度必须是正整数")
	}

	conf := &einoOpenai.EmbeddingConfig{
		APIKey:     apiKey,
		Model:      model,
		BaseURL:    baseURL,
		Dimensions: &dimensions,
	}

	embedder, err := einoOpenai.NewEmbedder(context.Background(), conf)
	if err != nil {
		return nil, fmt.Errorf("创建 OpenAI Embedder 失败: %w", err)
	}

	return &OpenAIEmbeddingService{embedder: embedder}, nil
}

func (s *OpenAIEmbeddingService) Name() string {
	return "openai"
}

func (s *OpenAIEmbeddingService) Embed(text string) ([]float64, error) {
	results, err := s.embedder.EmbedStrings(context.Background(), []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("Embedding 返回空结果")
	}
	return results[0], nil
}

func (s *OpenAIEmbeddingService) EmbedBatch(texts []string) ([][]float64, error) {
	return s.embedder.EmbedStrings(context.Background(), texts)
}

// GetEmbedder 获取底层的 eino Embedder（供 RAG 流程直接使用）
func (s *OpenAIEmbeddingService) GetEmbedder() embedding.Embedder {
	return s.embedder
}
