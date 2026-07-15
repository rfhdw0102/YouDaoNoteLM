package chat

import (
	"context"
	"time"

	"github.com/cloudwego/eino/adk"

	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

// metricsHandler 对话 Agent 可观测性处理器（基于 eino Handlers 钩子）
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

	duration := time.Since(start)

	// 记录 LLM 调用耗时
	logger.Info("[ChatAgent] LLM 调用完成",
		zap.Duration("duration", duration),
	)

	// 记录 token 用量（如果有的话）
	if len(state.Messages) > 0 {
		last := state.Messages[len(state.Messages)-1]
		if last.ResponseMeta != nil && last.ResponseMeta.Usage != nil {
			usage := last.ResponseMeta.Usage
			logger.Info("[ChatAgent] token 用量",
				zap.Int("prompt_tokens", usage.PromptTokens),
				zap.Int("completion_tokens", usage.CompletionTokens),
				zap.Int("total_tokens", usage.TotalTokens),
			)
		}
	}

	return ctx, state, nil
}
