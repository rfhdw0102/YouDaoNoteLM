package chat

import (
	"context"
	"time"

	"github.com/cloudwego/eino/adk"

	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

// metricsHandler 主 Agent 可观测性处理器（基于 eino Handlers 钩子）。
// 记录每次 LLM 调用的耗时与 token 用量，嵌入 *adk.BaseChatModelAgentMiddleware 获得 no-op 默认实现。
type metricsHandler struct {
	*adk.BaseChatModelAgentMiddleware
}

type metricsStartKey struct{}

func newMetricsHandler() *metricsHandler {
	return &metricsHandler{BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{}}
}

// BeforeModelRewriteState 每次模型调用前注入开始时间
func (h *metricsHandler) BeforeModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, mc *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	return context.WithValue(ctx, metricsStartKey{}, time.Now()), state, nil
}

// AfterModelRewriteState 每次模型调用后记录耗时与 token 用量
func (h *metricsHandler) AfterModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, mc *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	start, ok := ctx.Value(metricsStartKey{}).(time.Time)
	if !ok {
		return ctx, state, nil
	}
	logger.Info("主 Agent LLM 调用",
		zap.Duration("duration", time.Since(start)),
	)
	if len(state.Messages) > 0 {
		last := state.Messages[len(state.Messages)-1]
		if last.ResponseMeta != nil && last.ResponseMeta.Usage != nil {
			logger.Info("主 Agent token 用量",
				zap.Int("prompt_tokens", last.ResponseMeta.Usage.PromptTokens),
				zap.Int("completion_tokens", last.ResponseMeta.Usage.CompletionTokens),
				zap.Int("total_tokens", last.ResponseMeta.Usage.TotalTokens),
			)
		}
	}
	return ctx, state, nil
}

// buildRetryConfig 构造 LLM 重试配置（基于 eino ModelRetryConfig）。
// 仅对返回 error 的 LLM 调用重试（如限流、网络抖动），业务错误不在此路径。
func buildRetryConfig() *adk.ModelRetryConfig {
	return &adk.ModelRetryConfig{
		MaxRetries: 2,
		ShouldRetry: func(ctx context.Context, retryCtx *adk.RetryContext) *adk.RetryDecision {
			if retryCtx.Err != nil {
				return &adk.RetryDecision{Retry: true}
			}
			return &adk.RetryDecision{Retry: false}
		},
	}
}
