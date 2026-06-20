package chat

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"

	"YoudaoNoteLm/internal/agent/chat/prompts"
	chatTools "YoudaoNoteLm/internal/agent/chat/tools"
	"YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/rag"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/pkg/cache"
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
type ChatAgent struct {
	agent           *adk.ChatModelAgent
	refCollector    *chatTools.ReferenceCollector
	contextBuilder  *ContextBuilder
	streamProcessor *StreamProcessor

	// 运行时状态
	fullContent string
	references  []response.Reference
}

// NewChatAgent 创建 Agent 实例
// 内部自动构建工具集，外部只需传入 retriever 和 sourceIDs
func NewChatAgent(
	ctx context.Context,
	llmModel model.ToolCallingChatModel,
	conversationRepo repository.ConversationRepository,
	messageRepo repository.MessageRepository,
	chatCache *cache.ChatCache,
	retriever rag.RAGRetriever,
	userID uint,
	sourceIDs []uint,
) (*ChatAgent, error) {
	if llmModel == nil {
		return nil, fmt.Errorf("ChatModel 不能为空")
	}

	// 创建引用收集器和工具
	collector := chatTools.NewReferenceCollector()
	ragTool := chatTools.NewRAGRetrieverTool(retriever, userID, sourceIDs, collector)

	// 创建 ChatModelAgent（ReAct 循环）
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Model:       llmModel,
		Instruction: prompts.ChatAgentSystemPrompt,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{ragTool},
			},
		},
		MaxIterations: 10,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 ChatModelAgent 失败: %w", err)
	}

	return &ChatAgent{
		agent:           agent,
		refCollector:    collector,
		contextBuilder:  NewContextBuilder(conversationRepo, messageRepo, chatCache),
		streamProcessor: NewStreamProcessor(),
	}, nil
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
