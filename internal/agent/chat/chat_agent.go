package chat

import (
	"context"

	"github.com/cloudwego/eino/adk"

	chatTools "YoudaoNoteLm/internal/agent/chat/tools"
	"YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

// StreamEvent Agent 流式事件
type StreamEvent struct {
	Type    string      `json:"type"`           // 事件类型
	Content string      `json:"content"`        // 事件内容
	Data    interface{} `json:"data,omitempty"` // 附加数据
}

// 事件类型常量
const (
	EventToken      = "token"       // LLM 生成的 token
	EventToolCall   = "tool_call"   // 工具调用开始
	EventToolResult = "tool_result" // 工具调用结果
	EventReference  = "reference"   // 检索引用
	EventTitle      = "title"       // 对话标题更新
	EventDone       = "done"        // 生成完成
	EventError      = "error"       // 错误
)

// ChatAgent 自包含的对话 Agent
// 使用 NewChatAgentBuilder 创建实例
type ChatAgent struct {
	agent           *adk.ChatModelAgent
	refCollector    *chatTools.ReferenceCollector
	contextBuilder  *ContextBuilder
	streamProcessor *StreamProcessor

	// 运行时状态
	fullContent string
	references  []response.Reference
}

// Process 处理消息，返回事件通道
// 内部自动完成：构建消息、执行 Agent、处理流式事件、发送引用和完成事件
func (a *ChatAgent) Process(ctx context.Context, conversationID uint, content string) <-chan StreamEvent {
	eventCh := make(chan StreamEvent, 64)

	go func() {
		defer close(eventCh)

		// 1. 构建消息（包含历史和摘要）
		messages, err := a.contextBuilder.BuildMessages(ctx, conversationID, content)
		if err != nil {
			logger.Error("[ChatAgent] 构建消息失败", zap.Error(err))
			eventCh <- StreamEvent{Type: EventError, Content: "加载对话历史失败"}
			return
		}

		// 2. 执行 Agent
		iter := a.agent.Run(ctx, &adk.AgentInput{
			Messages:        messages,
			EnableStreaming: true,
		})

		// 3. 处理流式事件
		a.fullContent = a.streamProcessor.ProcessEvents(ctx, eventCh, iter)

		// 4. 收集引用
		a.references = a.refCollector.All()
		if len(a.references) > 0 {
			eventCh <- StreamEvent{
				Type:    EventReference,
				Content: "",
				Data:    a.references,
			}
		}

		// 5. 发送完成事件
		eventCh <- StreamEvent{Type: EventDone}
		logger.Info("[ChatAgent] 处理完成", zap.Int("contentLen", len(a.fullContent)))
	}()

	return eventCh
}

// GetFullContent 获取完整内容
func (a *ChatAgent) GetFullContent() string {
	return a.fullContent
}

// GetReferences 获取引用
func (a *ChatAgent) GetReferences() []response.Reference {
	return a.references
}

// buildRetryConfig 构造 LLM 重试配置，仅对返回 error 的 LLM 调用重试，业务错误不在此路径。
func buildRetryConfig() *adk.ModelRetryConfig {
	return &adk.ModelRetryConfig{
		MaxRetries: 1,
		ShouldRetry: func(ctx context.Context, retryCtx *adk.RetryContext) *adk.RetryDecision {
			if retryCtx.Err != nil {
				return &adk.RetryDecision{Retry: true}
			}
			return &adk.RetryDecision{Retry: false}
		},
	}
}
