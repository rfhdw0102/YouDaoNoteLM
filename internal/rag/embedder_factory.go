package rag

import (
	"context"
	"fmt"

	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/service/external"
	embeddingExt "YoudaoNoteLm/internal/service/external/embedding"

	"github.com/cloudwego/eino/components/embedding"
)

// embedderAdapter 将外部 EmbeddingService 适配为 eino embedding.Embedder
type embedderAdapter struct {
	inner embeddingExt.EmbeddingService
}

func (a *embedderAdapter) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	return a.inner.EmbedBatch(texts)
}

// NewEmbedderFromRegistry 通过 Registry 创建 eino Embedder
// 支持所有已注册的 embedding provider（volcengine、openai 等）
func NewEmbedderFromRegistry(ctx context.Context, registry *external.Registry, cfg *entity.UserConfig) (embedding.Embedder, error) {
	if cfg == nil {
		return nil, fmt.Errorf("embedding 配置为空")
	}

	sc := external.NewServiceConfigFromEntity(
		cfg.Provider, cfg.APIURL, cfg.APIKey, cfg.Model, cfg.ExtraConfig)

	svc, err := registry.Create("embedding", cfg.Provider, sc)
	if err != nil {
		return nil, fmt.Errorf("创建 embedding provider 失败: %w", err)
	}

	// 优先尝试直接断言为 eino Embedder
	if embedder, ok := svc.(embedding.Embedder); ok {
		return embedder, nil
	}

	// 降级：检查是否有 GetEmbedder() 方法（如 ArkEmbeddingService）
	type embedderProvider interface {
		GetEmbedder() embedding.Embedder
	}
	if ep, ok := svc.(embedderProvider); ok {
		return ep.GetEmbedder(), nil
	}

	// 最后降级：尝试断言为外部 EmbeddingService 并适配
	if embedSvc, ok := svc.(embeddingExt.EmbeddingService); ok {
		return &embedderAdapter{inner: embedSvc}, nil
	}

	return nil, fmt.Errorf("embedding provider %s 未实现 eino Embedder 或 EmbeddingService 接口", cfg.Provider)
}

// NewEmbedder 兼容入口（已废弃，请使用 NewEmbedderFromRegistry）
// 保留以避免破坏现有调用方
func NewEmbedder(ctx context.Context, cfg *entity.UserConfig) (embedding.Embedder, error) {
	if cfg == nil {
		return nil, fmt.Errorf("embedding config is nil")
	}

	// 委托到全局 Registry
	registry := external.GetGlobalRegistry()
	return NewEmbedderFromRegistry(ctx, registry, cfg)
}
