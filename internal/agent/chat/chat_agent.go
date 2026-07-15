package chat

import (
	"context"
	"fmt"
	"strings"
	"sync"

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

// SearchAgentResultItem 搜索结果项（与前端 SearchResultItem 对齐）
type SearchAgentResultItem struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

// SearchEvent 搜索 Agent 流式事件（与 service.SearchAgentEvent 对齐，避免 chat→service 循环依赖）
type SearchEvent struct {
	Type         string `json:"type"` // content, tool_call, search_round, error, done
	Content      string `json:"content,omitempty"`
	SearchRounds int    `json:"search_rounds,omitempty"`
	Error        string `json:"error,omitempty"`
	ErrorCode    int    `json:"error_code,omitempty"`
}

// SearchAgentExecutor 搜索 Agent 流式执行接口。
// search.SearchAgent 天然满足此接口（ExecuteStream 返回相同结构的 channel）。
type SearchAgentExecutor interface {
	ExecuteStream(ctx context.Context, userID, notebookID uint, task string) <-chan *SearchEvent
}

// SearchRefCallback 搜索结果转引用的回调函数类型。
// 搜索子 agent 的结果格式与 RAG 不同，通过此回调统一收集到 ReferenceCollector。
type SearchRefCallback func(results []SearchAgentResultItem)

// context key for search ref callback
type ctxKeySearchRefCallback struct{}

// withSearchRefCallback 将搜索引用回调注入 context
func withSearchRefCallback(ctx context.Context, cb SearchRefCallback) context.Context {
	return context.WithValue(ctx, ctxKeySearchRefCallback{}, cb)
}

// getSearchRefCallback 从 context 取出搜索引用回调（未注入时返回 nil）
func getSearchRefCallback(ctx context.Context) SearchRefCallback {
	cb, _ := ctx.Value(ctxKeySearchRefCallback{}).(SearchRefCallback)
	return cb
}

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
	// 主从协同：子 agent 事件（EmitInternalEvents 转发）
	// 前端据 sub_agent_start（含 agentName）打开对应面板（如 SearchAgent→搜索面板），
	// sub_agent_* 内容事件路由到该面板展示，sub_agent_end 关闭面板，对话框继续显示主 agent token。
	EventSubAgentStart      = "sub_agent_start"       // 子 agent 开始（Data=agentName）
	EventSubAgentEnd        = "sub_agent_end"         // 子 agent 结束（Data=agentName）
	EventSubAgentToken      = "sub_agent_token"       // 子 agent 的 token
	EventSubAgentToolCall   = "sub_agent_tool_call"   // 子 agent 的工具调用
	EventSubAgentToolResult = "sub_agent_tool_result" // 子 agent 的工具结果
	// 主从协同：搜索子 agent 专用事件（搜索结果展示在 SourcesPanel 搜索面板，不进对话 token 流）
	EventSearchStarted = "search_started" // 搜索开始（通知前端打开搜索面板）
	EventSearchResults = "search_results" // 搜索结果（Data={results, summary}）
	EventSearchBusy    = "search_busy"    // 搜索忙碌（已有搜索任务在执行，提示等待，不清空现有结果）
	// 主从协同：生成触发事件（生成结果展示在 NotesPanel 笔记面板，不进对话 token 流）
	EventGenerationStarted = "generation_started" // 生成开始（Data=type，如 mindmap/ppt/quiz/note）
	EventGenerationResult  = "generation_result"  // 生成结果（Data={type, content}）
)

// SubAgentBuilder 子 Agent 构建器接口。
// *youdao.YoudaoAgent 和 *search.SearchAgent 天然满足此接口（都有 BuildAgent 方法）。
// 用接口解耦，避免 chat 包直接依赖 youdao/search 包（后者依赖 service 包，会循环依赖）。
type SubAgentBuilder interface {
	BuildAgent(ctx context.Context, userID uint) (*adk.ChatModelAgent, error)
	// InjectContext 把子 agent 工具执行所需的 userID 等 context 注入。
	// 各子 agent 用自己的 context key（search/youdao/agent-tools 包各自定义），
	// 由子 agent 自己负责注入，避免 chat 包直接依赖 search/youdao 包。
	// 主 agent 通过 NewAgentTool 调用子 agent 时，context 是主 agent 的 tool 执行上下文，
	// 没有 search/youdao 包的 userID context，会导致子 agent 的工具（如 web_search）读不到 userID。
	InjectContext(ctx context.Context, userID uint) context.Context
}

// subAgentToolWrapper 包裹 NewAgentTool 返回的 tool，在执行前注入子 agent 工具所需的 context。
// 解决主 agent 调用子 agent 时 context 缺 userID 的问题（如 web_search 报"请配置搜索引擎"）。
type subAgentToolWrapper struct {
	tool.InvokableTool
	builder SubAgentBuilder
	userID  uint
}

func (w *subAgentToolWrapper) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	ctx = w.builder.InjectContext(ctx, w.userID)
	return w.InvokableTool.InvokableRun(ctx, argumentsInJSON, opts...)
}

// wrapWithCtxInjector 把 NewAgentTool 返回的 tool 用 subAgentToolWrapper 包裹，
// 在执行前注入子 agent 工具所需的 userID context。
func wrapWithCtxInjector(baseTool tool.BaseTool, builder SubAgentBuilder, userID uint) tool.BaseTool {
	if invokable, ok := baseTool.(tool.InvokableTool); ok {
		return &subAgentToolWrapper{InvokableTool: invokable, builder: builder, userID: userID}
	}
	return baseTool
}

// ChatAgent 自包含的对话 Agent
type ChatAgent struct {
	agent           *adk.ChatModelAgent
	refCollector    *chatTools.ReferenceCollector
	contextBuilder  *ContextBuilder
	streamProcessor *StreamProcessor
	searchTrigger   *triggerSearchTool

	// 运行时状态
	fullContent string
	references  []response.Reference
}

// NewChatAgent 创建 Agent 实例
// 内部自动构建工具集，外部只需传入 retriever、sourceIDs 和 sourceNames
// mainAgentEnabled 为 true 时，挂载 youdao(同步)/search(异步触发)/generation(异步触发) 子 agent tool（主从协同模式）
func NewChatAgent(
	ctx context.Context,
	llmModel model.ToolCallingChatModel,
	conversationRepo repository.ConversationRepository,
	messageRepo repository.MessageRepository,
	chatCache *cache.ChatCache,
	retriever rag.RAGRetriever,
	sourceRepo repository.SourceRepository,
	summaryCache *cache.SourceSummaryCache,
	userID uint,
	notebookID uint,
	sourceIDs []uint,
	sourceNames map[uint]string,
	youdaoAgent SubAgentBuilder,
	searchAgent SearchAgentExecutor,
	generateFn GenerateFunc,
	mainAgentEnabled bool,
) (*ChatAgent, error) {
	if llmModel == nil {
		return nil, fmt.Errorf("ChatModel 不能为空")
	}

	// 创建引用收集器（即使无资料也创建，保持引用收集逻辑统一）
	collector := chatTools.NewReferenceCollector()

	// 构建工具集：有资料时才注册 RAG/摘要工具
	tools := make([]tool.BaseTool, 0, 4)
	var searchTrigger *triggerSearchTool
	if len(sourceIDs) > 0 {
		ragTool := chatTools.NewRAGRetrieverTool(retriever, userID, sourceIDs, collector)
		summaryTool := chatTools.NewGetSourcesSummaryTool(sourceRepo, summaryCache, sourceIDs, sourceNames)
		tools = append(tools, ragTool, summaryTool)
	}

	// 主从协同模式：搜索用异步触发型 tool（不阻塞），生成用异步触发型 tool，有道用同步 NewAgentTool
	emitInternalEvents := false
	maxIter := 10
	if mainAgentEnabled {
		// 有道子 agent：同步 NewAgentTool（操作通常快速，需要结果才能回答）
		if youdaoAgent != nil {
			if yma, err := youdaoAgent.BuildAgent(ctx, userID); err == nil {
				agentTool := adk.NewAgentTool(ctx, yma, adk.WithFullChatHistoryAsInput())
				tools = append(tools, wrapWithCtxInjector(agentTool, youdaoAgent, userID))
			} else {
				logger.Warn("[ChatAgent] 构建 YoudaoAgent 失败，跳过挂载", zap.Error(err))
			}
		}
		// 搜索子 agent：异步触发型 tool（不阻塞主 agent，结果进搜索面板）
		if searchAgent != nil {
			searchTrigger = newTriggerSearchTool(searchAgent, userID, notebookID)
			tools = append(tools, searchTrigger)
		}
		// 生成服务：异步触发型 tool（不阻塞主 agent，结果进笔记面板）
		if generateFn != nil {
			tools = append(tools, newTriggerGenerationTool(generateFn, sourceRepo, userID, notebookID, sourceIDs))
		}
		emitInternalEvents = true // 有道子 agent 的事件转发需要
		maxIter = 15              // 主 agent 调用子 agent 会占用迭代轮数，适当放大
	}

	// 构建系统提示词（注入资料列表）
	systemPrompt := buildSystemPrompt(sourceIDs, sourceNames, mainAgentEnabled)

	// 创建 ChatModelAgent（ReAct 循环）
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Model:       llmModel,
		Instruction: systemPrompt,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: tools,
			},
			EmitInternalEvents: emitInternalEvents,
		},
		MaxIterations:    maxIter,
		ModelRetryConfig: buildRetryConfig(),                                  // LLM 调用失败自动重试（限流/网络抖动）
		Handlers:         []adk.ChatModelAgentMiddleware{newMetricsHandler()}, // 监控 LLM 调用耗时与 token 用量
	})
	if err != nil {
		return nil, fmt.Errorf("创建 ChatModelAgent 失败: %w", err)
	}

	return &ChatAgent{
		agent:           agent,
		refCollector:    collector,
		contextBuilder:  NewContextBuilder(conversationRepo, messageRepo, chatCache),
		streamProcessor: NewStreamProcessor(),
		searchTrigger:   searchTrigger,
	}, nil
}

// buildSystemPrompt 构建系统提示词，注入资料列表
// mainAgentEnabled 为 true 时使用总控 prompt（主从协同模式）
func buildSystemPrompt(sourceIDs []uint, sourceNames map[uint]string, mainAgentEnabled bool) string {
	template := prompts.ChatAgentSystemPrompt
	if mainAgentEnabled {
		template = prompts.MainAgentSystemPrompt
	}

	if len(sourceIDs) == 0 {
		return strings.Replace(template, "{{.SourceList}}", "（用户未选定特定资料，RAG 检索/摘要工具不可用）", 1)
	}

	var sb strings.Builder
	for i, id := range sourceIDs {
		name := sourceNames[id]
		if name == "" {
			name = fmt.Sprintf("资料#%d", id)
		}
		sb.WriteString(fmt.Sprintf("%d. %s (ID: %d)\n", i+1, name, id))
	}

	return strings.Replace(template, "{{.SourceList}}", sb.String(), 1)
}

// Process 处理消息，返回事件通道。
// eventCh 在主 Agent 完成后立即 close，使 processAndForward 尽快返回、释放锁。
// bgEventCh 用于后台任务（搜索/生成）发送事件，由调用方管理生命周期（不在此处 close）。
// 服务层通过 WithBgWaitGroup 注入 WaitGroup，触发工具注册后服务 goroutine 等待完成再 close bgEventCh。
func (a *ChatAgent) Process(ctx context.Context, conversationID uint, content string, bgEventCh chan<- StreamEvent) <-chan StreamEvent {
	eventCh := make(chan StreamEvent, 64)

	// 主 agent 事件发到 eventCh（主 Agent 完成后 close）
	ctx = withEventCh(ctx, eventCh)
	// 后台任务事件发到 bgEventCh（由调用方 close）
	ctx = withBgEventCh(ctx, bgEventCh)

	// 搜索子 agent 结果 → 引用的回调（sync.Once 防止重复收集：forwardSearchResults 可能被调用两次）
	var searchOnce sync.Once
	searchRefCallback := func(results []SearchAgentResultItem) {
		searchOnce.Do(func() {
			refs := make([]response.Reference, 0, len(results))
			for _, r := range results {
				refs = append(refs, response.Reference{
					SourceName:   r.URL,
					ChunkContent: r.Snippet,
					Score:        float32(r.Score),
				})
			}
			a.refCollector.Add(refs)
		})
	}
	ctx = withSearchRefCallback(ctx, searchRefCallback)

	// Explicit search commands are routed deterministically. Some compatible
	// models may describe a search without emitting a tool call.
	if query, ok := explicitSearchQuery(content); ok && a.searchTrigger != nil {
		go func() {
			defer close(eventCh)
			reply, err := a.searchTrigger.RunQuery(ctx, query)
			if err != nil {
				logger.Error("[ChatAgent] 启动搜索失败", zap.Error(err))
				reply = "搜索未能启动，请稍后重试。"
			}
			a.fullContent = reply
			a.references = a.refCollector.All()
			eventCh <- StreamEvent{Type: EventToken, Content: reply}
			eventCh <- StreamEvent{Type: EventDone, Data: a.references}
			logger.Info("[ChatAgent] 明确搜索指令已路由到 SearchAgent", zap.String("query", query))
		}()
		return eventCh
	}

	go func() {
		defer close(eventCh) // 主 Agent 完成即 close，processAndForward 可立即返回

		messages, err := a.contextBuilder.BuildMessages(ctx, conversationID, content)
		if err != nil {
			logger.Error("[ChatAgent] 构建消息失败", zap.Error(err))
			eventCh <- StreamEvent{Type: EventError, Content: "加载对话历史失败"}
			return
		}
		iter := a.agent.Run(ctx, &adk.AgentInput{Messages: messages, EnableStreaming: true})
		a.fullContent = a.streamProcessor.ProcessEvents(ctx, eventCh, iter)
		a.references = a.refCollector.All()
		// 引用附加到 EventDone 的 Data 字段，原子发送，消除 SSE 断连导致引用丢失的竞态
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
