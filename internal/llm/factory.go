package llm

import (
	"context"
	"fmt"

	"YoudaoNoteLm/internal/model/entity"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

// NewChatModel 根据用户 LLM 配置创建 ChatModel
func NewChatModel(ctx context.Context, cfg *entity.UserLLMConfig) (model.ToolCallingChatModel, error) {
	if cfg == nil {
		return nil, fmt.Errorf("LLM 配置不能为空")
	}

	switch cfg.Provider {
	case "openai":
		return newOpenAIChatModel(ctx, cfg)
	case "anthropic", "claude":
		return newAnthropicChatModel(ctx, cfg)
	default:
		return nil, fmt.Errorf("不支持的 LLM 提供商: %s", cfg.Provider)
	}
}

// NewToolCallingChatModel 创建支持 Tool Calling 的 ChatModel
// 返回的 ToolCallingChatModel 并发安全，推荐在 Agent 场景使用
func NewToolCallingChatModel(ctx context.Context, cfg *entity.UserLLMConfig) (model.ToolCallingChatModel, error) {
	return NewChatModel(ctx, cfg)
}

// newOpenAIChatModel 创建 OpenAI ChatModel
func newOpenAIChatModel(ctx context.Context, cfg *entity.UserLLMConfig) (model.ToolCallingChatModel, error) {
	oaiCfg := &openai.ChatModelConfig{
		APIKey: cfg.APIKey,
		Model:  cfg.Model,
	}
	if cfg.APIURL != "" {
		oaiCfg.BaseURL = cfg.APIURL
	}
	return openai.NewChatModel(ctx, oaiCfg)
}

// newAnthropicChatModel 创建 Anthropic (Claude) ChatModel
func newAnthropicChatModel(ctx context.Context, cfg *entity.UserLLMConfig) (model.ToolCallingChatModel, error) {
	baseURL := "https://api.anthropic.com"
	if cfg.APIURL != "" {
		baseURL = cfg.APIURL
	}
	return NewAnthropicChatModel(ctx, cfg.APIKey, cfg.Model, baseURL)
}
