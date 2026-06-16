package rag

import (
	"context"
	"fmt"
	"strings"

	"YoudaoNoteLm/internal/model/entity"

	einoArk "github.com/cloudwego/eino-ext/components/embedding/ark"
	einoOpenai "github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino/components/embedding"
)

// EmbeddingProvider 定义支持的 embedding 提供商类型
type EmbeddingProvider string

const (
	ProviderArk        EmbeddingProvider = "ark"        // 火山引擎（豆包）
	ProviderVolcengine EmbeddingProvider = "volcengine" // 火山引擎（豆包）- 另一个名称
	ProviderOpenAI     EmbeddingProvider = "openai"     // OpenAI
)

// EmbeddingConfig embedding 模型配置
type EmbeddingConfig struct {
	Provider   EmbeddingProvider `json:"provider"`             // 提供商
	APIKey     string            `json:"api_key"`              // API Key
	Model      string            `json:"model"`                // 模型名称或接入点 ID
	BaseURL    string            `json:"base_url,omitempty"`   // 自定义 API 地址（可选）
	Dimensions *int              `json:"dimensions,omitempty"` // 向量维度（可选，部分模型支持）
}

// NewEmbedder 根据配置创建 eino Embedder
// 支持所有 eino-ext 集成的 embedding 模型
func NewEmbedder(ctx context.Context, cfg *EmbeddingConfig) (embedding.Embedder, error) {
	if cfg == nil {
		return nil, fmt.Errorf("embedding 配置不能为空")
	}

	switch cfg.Provider {
	case ProviderArk, ProviderVolcengine:
		return createArkEmbedder(ctx, cfg)
	case ProviderOpenAI:
		return createOpenAIEmbedder(ctx, cfg)
	default:
		return nil, fmt.Errorf("不支持的 embedding 提供商: %s", cfg.Provider)
	}
}

// createArkEmbedder 创建火山引擎 Ark Embedder
// 火山引擎统一使用 multi_modal_api 类型
func createArkEmbedder(ctx context.Context, cfg *EmbeddingConfig) (embedding.Embedder, error) {
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
		return nil, fmt.Errorf("创建 Ark Embedder 失败: %w", err)
	}
	return embedder, nil
}

// createOpenAIEmbedder 创建 OpenAI Embedder
func createOpenAIEmbedder(ctx context.Context, cfg *EmbeddingConfig) (embedding.Embedder, error) {
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
		return nil, fmt.Errorf("创建 OpenAI Embedder 失败: %w", err)
	}
	return embedder, nil
}

// NewEmbedderFromConfig 从 entity.UserConfig 创建 eino Embedder
// 这是一个便捷函数，自动将 UserConfig 转换为 EmbeddingConfig
func NewEmbedderFromConfig(ctx context.Context, cfg *entity.UserConfig) (embedding.Embedder, error) {
	if cfg == nil {
		return nil, fmt.Errorf("embedding 配置不能为空")
	}

	embeddingCfg := &EmbeddingConfig{
		Provider:   EmbeddingProvider(cfg.Provider),
		APIKey:     cfg.APIKey,
		Model:      cfg.Model,
		BaseURL:    cfg.APIURL,
		Dimensions: cfg.Dimensions,
	}

	return NewEmbedder(ctx, embeddingCfg)
}

// GetBatchSizeByAPIKey 根据 API Key 前缀判断模型商，返回对应的批量限制
// 火山引擎/豆包: ark- 前缀，限制 256
// OpenAI 兼容: sk- 前缀，限制 2048
// 其他未知: 默认 25（保守值），比如通义，智谱
func GetBatchSizeByAPIKey(apiKey string) int {
	switch {
	case strings.HasPrefix(apiKey, "ark-"):
		// 火山引擎/豆包
		return 256
	case strings.HasPrefix(apiKey, "sk-"):
		// OpenAI 兼容接口
		return 2048
	default:
		// 未知厂商，使用保守值
		return 25
	}
}
