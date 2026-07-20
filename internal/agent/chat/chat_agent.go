package chat

import (
	"context"
	"sync"

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
	EventReference  = "reference"   // 检索引用（兼容旧版，新流程用 EventDone.Data）
	EventTitle      = "title"       // 对话标题更新
	EventDone       = "done"        // 生成完成（Data=references）
	EventError      = "error"       // 错误
	// 主从协同：子 agent 事件（EmitInternalEvents 转发）
	EventSubAgentStart      = "sub_agent_start"       // 子 agent 开始（Data=agentName）
	EventSubAgentEnd        = "sub_agent_end"         // 子 agent 结束（Data=agentName）
	EventSubAgentToken      = "sub_agent_token"       // 子 agent 的 token
	EventSubAgentToolCall   = "sub_agent_tool_call"   // 子 agent 的工具调用
	EventSubAgentToolResult = "sub_agent_tool_result" // 子 agent 的工具结果
	// 主从协同：搜索子 agent 专用事件
	EventSearchStarted = "search_started" // 搜索开始
	EventSearchResults = "search_results" // 搜索结果（Data={results, summary}）
	// 主从协同：生成触发事件
	EventGenerationStarted = "generation_started" // 生成开始（Data=type）
	EventGenerationResult  = "generation_result"  // 生成结果（Data={type, content}）
)

// SearchAgentExecutor 搜索 Agent 流式执行接口。
type SearchAgentExecutor interface {
	ExecuteStream(ctx context.Context, userID, notebookID uint, task string) <-chan *SearchEvent
}

// SearchEvent 搜索 Agent 流式事件
type SearchEvent struct {
	Type         string `json:"type"`
	Content      string `json:"content,omitempty"`
	SearchRounds int    `json:"search_rounds,omitempty"`
	Error        string `json:"error,omitempty"`
	ErrorCode    int    `json:"error_code,omitempty"`
}

// SearchAgentResultItem 搜索子 agent 返回的搜索结果项
type SearchAgentResultItem struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
	Reason  string  `json:"reason,omitempty"`
}

// SearchRefCallback 搜索结果转引用的回调函数类型。
type SearchRefCallback func(results []SearchAgentResultItem)

type ctxKeySearchRefCallback struct{}

func withSearchRefCallback(ctx context.Context, cb SearchRefCallback) context.Context {
	return context.WithValue(ctx, ctxKeySearchRefCallback{}, cb)
}

func getSearchRefCallback(ctx context.Context) SearchRefCallback {
	cb, _ := ctx.Value(ctxKeySearchRefCallback{}).(SearchRefCallback)
	return cb
}

// ChatAgent 自包含的对话 Agent
type ChatAgent struct {
	agent           *adk.ChatModelAgent
	refCollector    *chatTools.ReferenceCollector
	contextBuilder  *ContextBuilder
	streamProcessor *StreamProcessor

	fullContent string
	references  []response.Reference
}

// Process 处理消息，返回事件通道
func (a *ChatAgent) Process(ctx context.Context, conversationID uint, content string) <-chan StreamEvent {
	eventCh := make(chan StreamEvent, 64)

	// 搜索子 agent 结果 → 引用的回调（sync.Once 防止重复收集）
	var searchOnce sync.Once
	searchRefCallback := func(results []SearchAgentResultItem) {
		searchOnce.Do(func() {
			refs := make([]response.Reference, 0, len(results))
			for _, r := range results {
				refs = append(refs, response.Reference{
					SourceName:    r.URL,
					ChunkContent: r.Snippet,
					Score:        float32(r.Score),
				})
			}
			a.refCollector.Add(refs)
		})
	}
	ctx = withSearchRefCallback(ctx, searchRefCallback)

	go func() {
		defer close(eventCh)

		messages, err := a.contextBuilder.BuildMessages(ctx, conversationID, content)
		if err != nil {
			logger.Error("[ChatAgent] 构建消息失败", zap.Error(err))
			eventCh <- StreamEvent{Type: EventError, Content: "加载对话历史失败"}
			return
		}

		iter := a.agent.Run(ctx, &adk.AgentInput{
			Messages:        messages,
			EnableStreaming: true,
		})

		a.fullContent = a.streamProcessor.ProcessEvents(ctx, eventCh, iter)

		// 引用附加到 EventDone，原子发送，消除竞态
		a.references = a.refCollector.All()
		eventCh <- StreamEvent{Type: EventDone, Data: a.references}
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
