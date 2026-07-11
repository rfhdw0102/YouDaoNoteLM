package rag

import (
	"context"
	"fmt"

	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/pkg/logger"

	einoArk "github.com/cloudwego/eino-ext/components/embedding/ark"
	einoOpenai "github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino/components/embedding"
	"go.uber.org/zap"
)

// embeddingProvider 定义支持的 embedding 提供商类型
type embeddingProvider string

const (
	providerArk        embeddingProvider = "ark"        // 火山引擎（豆包）
	providerVolcengine embeddingProvider = "volcengine" // 火山引擎 - 另一个名称
	providerDoubao     embeddingProvider = "doubao"
	providerOpenAI     embeddingProvider = "openai" // OpenAI 兼容
)

// embeddingConfig embedding 模型配置
type embeddingConfig struct {
	Provider   embeddingProvider `json:"provider"`
	APIKey     string            `json:"api_key"`
	Model      string            `json:"model"`
	BaseURL    string            `json:"base_url,omitempty"`
	Dimensions *int              `json:"dimensions,omitempty"`
}

// newEmbedder 根据配置创建 eino Embedder
func newEmbedder(ctx context.Context, cfg *embeddingConfig) (embedding.Embedder, error) {
	if cfg == nil {
		return nil, fmt.Errorf("embedding 配置不能为空")
	}

	switch cfg.Provider {
	case providerArk, providerVolcengine, providerDoubao:
		return createArkEmbedder(ctx, cfg)
	case providerOpenAI:
		return createOpenAIEmbedder(ctx, cfg)
	default:
		return nil, fmt.Errorf("不支持的 embedding 提供商: %s", cfg.Provider)
	}
}

// createArkEmbedder 创建火山引擎 Ark Embedder
func createArkEmbedder(ctx context.Context, cfg *embeddingConfig) (embedding.Embedder, error) {
	if cfg.Model == "" {
		return nil, fmt.Errorf("Ark embedding 模型名称或接入点 ID 未配置")
	}

	apiType := einoArk.APITypeMultiModal
	conf := &einoArk.EmbeddingConfig{
		APIKey:  cfg.APIKey,
		Model:   cfg.Model,
		APIType: &apiType,
	}
	if cfg.BaseURL != "" {
		conf.BaseURL = cfg.BaseURL
	}
	if cfg.Dimensions != nil {
		conf.Dimensions = cfg.Dimensions
	}

	embedder, err := einoArk.NewEmbedder(ctx, conf)
	if err != nil {
		logger.Error("[EmbedderFactory] 创建 Ark Embedder 失败",
			zap.String("model", cfg.Model),
			zap.Error(err),
		)
		return nil, fmt.Errorf("创建 Ark Embedder 失败: %w", err)
	}
	return embedder, nil
}

// createOpenAIEmbedder 创建 OpenAI 兼容 Embedder
func createOpenAIEmbedder(ctx context.Context, cfg *embeddingConfig) (embedding.Embedder, error) {
	if cfg.Model == "" {
		return nil, fmt.Errorf("OpenAI embedding 模型名称未配置")
	}

	conf := &einoOpenai.EmbeddingConfig{
		APIKey: cfg.APIKey,
		Model:  cfg.Model,
	}
	if cfg.BaseURL != "" {
		conf.BaseURL = cfg.BaseURL
	}
	if cfg.Dimensions != nil {
		conf.Dimensions = cfg.Dimensions
	}

	embedder, err := einoOpenai.NewEmbedder(ctx, conf)
	if err != nil {
		logger.Error("[EmbedderFactory] 创建 OpenAI Embedder 失败",
			zap.String("model", cfg.Model),
			zap.Error(err),
		)
		return nil, fmt.Errorf("创建 OpenAI Embedder 失败: %w", err)
	}
	return embedder, nil
}

// NewEmbedderFromConfig 从 entity.UserConfig 创建 eino Embedder
func NewEmbedderFromConfig(ctx context.Context, cfg *entity.UserConfig) (embedding.Embedder, error) {
	if cfg == nil {
		return nil, fmt.Errorf("embedding 配置不能为空")
	}

	embeddingCfg := &embeddingConfig{
		Provider:   embeddingProvider(cfg.Provider),
		APIKey:     cfg.APIKey,
		Model:      cfg.Model,
		BaseURL:    cfg.APIURL,
		Dimensions: cfg.Dimensions,
	}

	return newEmbedder(ctx, embeddingCfg)
}
