package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"

	"YoudaoNoteLm/internal/agent/chat/prompts"
	chatTools "YoudaoNoteLm/internal/agent/chat/tools"
	"YoudaoNoteLm/internal/rag"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/pkg/cache"
)

// ChatAgentBuilder ChatAgent 构建器
// 采用 Builder 模式分离工具构建和 Agent 组装，便于扩展和测试
type ChatAgentBuilder struct {
	// 必需参数
	llmModel model.ToolCallingChatModel
	ctx      context.Context

	// 资料相关
	sourceIDs   []uint
	sourceNames map[uint]string

	// 工具相关（可选，支持自定义扩展）
	tools []tool.BaseTool

	// Agent 配置（可选，有默认值）
	maxIterations int

	// 依赖组件
	retriever        rag.RAGRetriever
	refCollector     *chatTools.ReferenceCollector
	sourceRepo       repository.SourceRepository
	summaryCache     *cache.SourceSummaryCache
	conversationRepo repository.ConversationRepository
	messageRepo      repository.MessageRepository
	chatCache        *cache.ChatCache
	userID           uint
}

// NewChatAgentBuilder 创建构建器
func NewChatAgentBuilder(ctx context.Context) *ChatAgentBuilder {
	return &ChatAgentBuilder{
		ctx:           ctx,
		sourceNames:   make(map[uint]string),
		tools:         make([]tool.BaseTool, 0),
		maxIterations: 10,
		refCollector:  chatTools.NewReferenceCollector(),
	}
}

// WithLLM 设置 LLM 模型
func (b *ChatAgentBuilder) WithLLM(m model.ToolCallingChatModel) *ChatAgentBuilder {
	b.llmModel = m
	return b
}

// WithSources 设置资料列表
func (b *ChatAgentBuilder) WithSources(ids []uint, names map[uint]string) *ChatAgentBuilder {
	b.sourceIDs = ids
	if names != nil {
		b.sourceNames = names
	}
	return b
}

// WithUserID 设置用户 ID
func (b *ChatAgentBuilder) WithUserID(userID uint) *ChatAgentBuilder {
	b.userID = userID
	return b
}

// WithRetriever 设置 RAG 检索器（用于默认 RAG 工具）
func (b *ChatAgentBuilder) WithRetriever(r rag.RAGRetriever) *ChatAgentBuilder {
	b.retriever = r
	return b
}

// WithSourceRepo 设置资料仓库（用于默认摘要工具）
func (b *ChatAgentBuilder) WithSourceRepo(repo repository.SourceRepository) *ChatAgentBuilder {
	b.sourceRepo = repo
	return b
}

// WithSummaryCache 设置摘要缓存（用于默认摘要工具）
func (b *ChatAgentBuilder) WithSummaryCache(c *cache.SourceSummaryCache) *ChatAgentBuilder {
	b.summaryCache = c
	return b
}

// WithContextRepos 设置上下文构建所需仓库
func (b *ChatAgentBuilder) WithContextRepos(
	convRepo repository.ConversationRepository,
	msgRepo repository.MessageRepository,
	chatCache *cache.ChatCache,
) *ChatAgentBuilder {
	b.conversationRepo = convRepo
	b.messageRepo = msgRepo
	b.chatCache = chatCache
	return b
}

// WithMaxIterations 设置最大迭代次数
func (b *ChatAgentBuilder) WithMaxIterations(n int) *ChatAgentBuilder {
	if n > 0 {
		b.maxIterations = n
	}
	return b
}

// WithTool 添加自定义工具（可多次调用）
func (b *ChatAgentBuilder) WithTool(t tool.BaseTool) *ChatAgentBuilder {
	b.tools = append(b.tools, t)
	return b
}

// WithTools 批量添加工具
func (b *ChatAgentBuilder) WithTools(tools ...tool.BaseTool) *ChatAgentBuilder {
	b.tools = append(b.tools, tools...)
	return b
}

// GetReferenceCollector 获取引用收集器
func (b *ChatAgentBuilder) GetReferenceCollector() *chatTools.ReferenceCollector {
	return b.refCollector
}

// Build 构建 ChatAgent
func (b *ChatAgentBuilder) Build() (*ChatAgent, error) {
	if b.llmModel == nil {
		return nil, fmt.Errorf("ChatModel 不能为空")
	}

	// 1. 构建默认工具
	b.buildDefaultTools()

	if len(b.tools) == 0 {
		return nil, fmt.Errorf("至少需要一个工具")
	}

	// 2. 构建系统提示词
	systemPrompt := b.buildSystemPrompt()

	// 3. 创建 ChatModelAgent
	agent, err := adk.NewChatModelAgent(b.ctx, &adk.ChatModelAgentConfig{
		Model:       b.llmModel,
		Instruction: systemPrompt,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: b.tools,
			},
		},
		MaxIterations:    b.maxIterations,
		ModelRetryConfig: buildRetryConfig(),
	})
	if err != nil {
		return nil, fmt.Errorf("创建 ChatModelAgent 失败: %w", err)
	}

	// 4. 构建上下文构建器
	contextBuilder := NewContextBuilder(b.conversationRepo, b.messageRepo, b.chatCache)

	return &ChatAgent{
		agent:           agent,
		refCollector:    b.refCollector,
		contextBuilder:  contextBuilder,
		streamProcessor: NewStreamProcessor(),
	}, nil
}

// buildDefaultTools 构建默认工具
func (b *ChatAgentBuilder) buildDefaultTools() {
	// 检查是否已有同名工具
	hasRAG := false
	hasSummary := false
	for _, t := range b.tools {
		if info, err := t.Info(b.ctx); err == nil {
			switch info.Name {
			case "search_knowledge":
				hasRAG = true
			case "get_sources_summary":
				hasSummary = true
			}
		}
	}

	// 自动添加 RAG 检索工具
	if !hasRAG && b.retriever != nil {
		ragTool := chatTools.NewRAGRetrieverTool(b.retriever, b.userID, b.sourceIDs, b.refCollector)
		b.tools = append([]tool.BaseTool{ragTool}, b.tools...) // RAG 工具优先
	}

	// 自动添加摘要工具
	if !hasSummary && b.sourceRepo != nil {
		summaryTool := chatTools.NewGetSourcesSummaryTool(b.sourceRepo, b.summaryCache, b.sourceIDs, b.sourceNames)
		b.tools = append(b.tools, summaryTool)
	}
}

// buildSystemPrompt 构建系统提示词，注入资料列表
func (b *ChatAgentBuilder) buildSystemPrompt() string {
	if len(b.sourceIDs) == 0 {
		return strings.Replace(prompts.ChatAgentSystemPrompt, "{{.SourceList}}", "（用户未选定特定资料）", 1)
	}

	var sb strings.Builder
	for i, id := range b.sourceIDs {
		name := b.sourceNames[id]
		if name == "" {
			name = fmt.Sprintf("资料#%d", id)
		}
		sb.WriteString(fmt.Sprintf("%d. %s (ID: %d)\n", i+1, name, id))
	}

	return strings.Replace(prompts.ChatAgentSystemPrompt, "{{.SourceList}}", sb.String(), 1)
}
