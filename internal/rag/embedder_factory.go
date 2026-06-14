package rag

import (
	"context"
	"fmt"

	"YoudaoNoteLm/internal/model/entity"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino/components/embedding"
)

// NewEmbedder 根据用户配置创建 eino Embedder
// 强制使用 Doubao-embedding-vision，维度 2048
func NewEmbedder(ctx context.Context, cfg *entity.UserConfig) (embedding.Embedder, error) {
	if cfg == nil {
		return nil, fmt.Errorf("embedding config is nil")
	}

	// 强制使用 Doubao provider
	if cfg.Provider != "doubao" {
		return nil, fmt.Errorf("仅支持 doubao embedding provider，当前: %s", cfg.Provider)
	}

	return newDoubaoEmbedder(ctx, cfg)
}

// newDoubaoEmbedder 创建豆包(火山引擎 Ark) Embedder
// 豆包使用火山引擎 Ark 平台，endpoint ID 作为 Model
func newDoubaoEmbedder(ctx context.Context, cfg *entity.UserConfig) (embedding.Embedder, error) {
	embCfg := &ark.EmbeddingConfig{
		APIKey: cfg.APIKey,
		Model:  cfg.Model, // Ark endpoint ID，如 "ep-xxx"
	}
	if cfg.APIURL != "" {
		embCfg.BaseURL = cfg.APIURL
	}
	// Doubao-embedding-vision 是多模态模型
	apiType := ark.APITypeMultiModal
	embCfg.APIType = &apiType
	return ark.NewEmbedder(ctx, embCfg)
}
