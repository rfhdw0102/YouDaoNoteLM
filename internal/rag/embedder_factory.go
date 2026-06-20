package rag

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/pkg/logger"

	einoArk "github.com/cloudwego/eino-ext/components/embedding/ark"
	einoOpenai "github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino/components/embedding"
	"go.uber.org/zap"
)

// EmbeddingProvider 定义支持的 embedding 提供商类型
type EmbeddingProvider string

const (
	ProviderArk        EmbeddingProvider = "ark"        // 火山引擎（豆包）
	ProviderVolcengine EmbeddingProvider = "volcengine" // 火山引擎（豆包）- 另一个名称
	ProviderDoubao     EmbeddingProvider = "doubao"
	ProviderOpenAI     EmbeddingProvider = "openai" // OpenAI
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
		logger.Error("[EmbedderFactory] embedding 配置为空")
		return nil, fmt.Errorf("embedding 配置不能为空")
	}

	logger.Info("[EmbedderFactory] 创建 Embedder",
		zap.String("provider", string(cfg.Provider)),
		zap.String("model", cfg.Model),
		zap.String("baseURL", cfg.BaseURL),
	)

	switch cfg.Provider {
	case ProviderArk, ProviderVolcengine, ProviderDoubao:
		return createArkEmbedder(ctx, cfg)
	case ProviderOpenAI:
		return createOpenAIEmbedder(ctx, cfg)
	default:
		logger.Error("[EmbedderFactory] 不支持的 embedding 提供商", zap.String("provider", string(cfg.Provider)))
		return nil, fmt.Errorf("不支持的 embedding 提供商: %s", cfg.Provider)
	}
}

// createArkEmbedder 创建火山引擎 Ark Embedder
// 火山引擎统一使用 multi_modal_api 类型
func createArkEmbedder(ctx context.Context, cfg *EmbeddingConfig) (embedding.Embedder, error) {
	logger.Debug("[EmbedderFactory] 创建 Ark Embedder", zap.String("model", cfg.Model))

	if cfg.Model == "" {
		logger.Error("[EmbedderFactory] Ark embedding 模型名称或接入点 ID 未配置")
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

	logger.Info("[EmbedderFactory] Ark Embedder 创建成功", zap.String("model", cfg.Model))
	return embedder, nil
}

// createOpenAIEmbedder 创建 OpenAI Embedder
func createOpenAIEmbedder(ctx context.Context, cfg *EmbeddingConfig) (embedding.Embedder, error) {
	logger.Debug("[EmbedderFactory] 创建 OpenAI Embedder", zap.String("model", cfg.Model))

	if cfg.Model == "" {
		logger.Error("[EmbedderFactory] OpenAI embedding 模型名称未配置")
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
			zap.String("baseURL", cfg.BaseURL),
			zap.Error(err),
		)
		return nil, fmt.Errorf("创建 OpenAI Embedder 失败: %w", err)
	}

	logger.Info("[EmbedderFactory] OpenAI Embedder 创建成功",
		zap.String("model", cfg.Model),
		zap.String("baseURL", cfg.BaseURL),
	)
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

// hostBatchEntry base_url 主机名 → 单次 EmbedStrings 最大输入条数。
// OpenAI 兼容厂商；OpenAI 官方协议上限较高，单独列出。
type hostBatchEntry struct {
	suffix string // 主机名后缀，支持精确匹配或子域匹配（a.b.com 匹配 b.com）
	batch  int
}

// hostBatchTable 按 base_url 主机名查 batch size 的表。
// 维护说明：新增厂商时按 host 后缀追加即可，匹配逻辑见 batchSizeByHost。
var hostBatchTable = []hostBatchEntry{
	{"api.openai.com", 2048},           // OpenAI 官方
	{"dashscope.aliyuncs.com", 10},     // 通义千问
	{"open.bigmodel.cn", 64},           // 智谱
	{"api.siliconflow.cn", 32},         // 硅基流动
	{"api.together.xyz", 100},          // Together
	{"api.baichuan-ai.com", 16},        // 百川
	{"api.moonshot.cn", 16},            // Moonshot
	{"ark.cn-beijing.volces.com", 256}, // 火山方舟 OpenAI 兼容入口
}

const (
	// arkBatchSize 火山方舟原生协议（非 OpenAI 兼容入口）的默认批量。
	arkBatchSize = 256
	// defaultOpenAICompatibleBatchSize 未识别 host 的 OpenAI 兼容厂商兜底，
	defaultOpenAICompatibleBatchSize = 16
)

// batchSizeByHost 按 base_url 的主机名在 hostBatchTable 中查 batch size。
// 用 url.Parse + 后缀匹配，避免 strings.Contains 的子串误匹配。
func batchSizeByHost(baseURL string) (int, bool) {
	if baseURL == "" {
		return 0, false
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return 0, false
	}
	host := strings.ToLower(u.Host)
	for _, e := range hostBatchTable {
		suffix := strings.ToLower(e.suffix)
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return e.batch, true
		}
	}
	return 0, false
}

// GetBatchSize 决定一次 EmbedStrings 调用的最大输入条数。
// 判定优先级：
//  1. base_url 主机名查表（hostBatchTable）
//  2. 协议默认：火山方舟原生 256，其余 OpenAI 兼容厂商兜底 16
func GetBatchSize(provider, baseURL string) int {
	// 1. 按 base_url 主机名查表
	if size, ok := batchSizeByHost(baseURL); ok {
		return size
	}

	// 2. 协议默认
	switch EmbeddingProvider(provider) {
	case ProviderArk, ProviderVolcengine:
		return arkBatchSize
	default:
		// ProviderOpenAI 及其它未识别 host 的 OpenAI 兼容厂商统一兜底
		return defaultOpenAICompatibleBatchSize
	}
}
