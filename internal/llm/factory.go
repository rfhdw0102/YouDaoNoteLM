package llm

import (
	"context"
	"fmt"

	"YoudaoNoteLm/internal/model/entity"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

// NewChatModel 根据用户 LLM 配置创建 ChatModel
func NewChatModel(ctx context.Context, cfg *entity.UserLLMConfig) (model.ChatModel, error) {
	if cfg == nil {
		return nil, fmt.Errorf("LLM 配置不能为空")
	}

	switch cfg.Provider {
	case "ark", "doubao":
		return newArkChatModel(ctx, cfg)
	case "deepseek":
		return newDeepSeekChatModel(ctx, cfg)
	case "openai":
		return newOpenAIChatModel(ctx, cfg)
	case "qianwen", "qwen":
		return newQianwenChatModel(ctx, cfg)
	case "anthropic", "claude":
		return newAnthropicChatModel(ctx, cfg)
	default:
		return nil, fmt.Errorf("不支持的 LLM 提供商: %s", cfg.Provider)
	}
}

// NewToolCallingChatModel 创建支持 Tool Calling 的 ChatModel
// 返回的 ToolCallingChatModel 并发安全，推荐在 Agent 场景使用
func NewToolCallingChatModel(ctx context.Context, cfg *entity.UserLLMConfig) (model.ToolCallingChatModel, error) {
	chatModel, err := NewChatModel(ctx, cfg)
	if err != nil {
		return nil, err
	}

	tccm, ok := chatModel.(model.ToolCallingChatModel)
	if !ok {
		return nil, fmt.Errorf("模型提供商 '%s' 不支持 ToolCallingChatModel 接口", cfg.Provider)
	}
	return tccm, nil
}

// newArkChatModel 创建火山引擎 Ark ChatModel
func newArkChatModel(ctx context.Context, cfg *entity.UserLLMConfig) (model.ChatModel, error) {
	arkCfg := &ark.ChatModelConfig{
		APIKey: cfg.APIKey,
		Model:  cfg.Model,
	}
	if cfg.APIURL != "" {
		arkCfg.BaseURL = cfg.APIURL
	}
	return ark.NewChatModel(ctx, arkCfg)
}

// newDeepSeekChatModel 创建 DeepSeek ChatModel
func newDeepSeekChatModel(ctx context.Context, cfg *entity.UserLLMConfig) (model.ChatModel, error) {
	dsCfg := &deepseek.ChatModelConfig{
		APIKey: cfg.APIKey,
		Model:  cfg.Model,
	}
	if cfg.APIURL != "" {
		dsCfg.BaseURL = cfg.APIURL
	}
	return deepseek.NewChatModel(ctx, dsCfg)
}

// newOpenAIChatModel 创建 OpenAI ChatModel
func newOpenAIChatModel(ctx context.Context, cfg *entity.UserLLMConfig) (model.ChatModel, error) {
	oaiCfg := &openai.ChatModelConfig{
		APIKey: cfg.APIKey,
		Model:  cfg.Model,
	}
	if cfg.APIURL != "" {
		oaiCfg.BaseURL = cfg.APIURL
	}
	return openai.NewChatModel(ctx, oaiCfg)
}

// newQianwenChatModel 创建千问 ChatModel（使用 OpenAI 兼容接口）
func newQianwenChatModel(ctx context.Context, cfg *entity.UserLLMConfig) (model.ChatModel, error) {
	qwCfg := &openai.ChatModelConfig{
		APIKey:  cfg.APIKey,
		Model:   cfg.Model,
		BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
	}
	if cfg.APIURL != "" {
		qwCfg.BaseURL = cfg.APIURL
	}
	return openai.NewChatModel(ctx, qwCfg)
}

// newAnthropicChatModel 创建 Anthropic (Claude) ChatModel
func newAnthropicChatModel(ctx context.Context, cfg *entity.UserLLMConfig) (model.ChatModel, error) {
	baseURL := "https://api.anthropic.com"
	if cfg.APIURL != "" {
		baseURL = cfg.APIURL
	}
	return NewAnthropicChatModel(ctx, cfg.APIKey, cfg.Model, baseURL)
}
