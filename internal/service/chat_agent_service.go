package service

import (
	"context"
	"sync"

	"github.com/cloudwego/eino/components/tool"

	"YoudaoNoteLm/internal/agent/chat"
	agentPrompts "YoudaoNoteLm/internal/agent/chat/prompts"
	chatTools "YoudaoNoteLm/internal/agent/chat/tools"
	"YoudaoNoteLm/internal/llm"
	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/rag"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/pkg/cache"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

// chatAgentService Agent 对话服务实现
type chatAgentService struct {
	llmConfigRepo    repository.UserLLMConfigRepository
	retriever        rag.RAGRetriever
	conversationRepo repository.ConversationRepository
	messageRepo      repository.MessageRepository
	cache            *cache.ChatCache
	cancelFuncs      sync.Map
}

// NewChatAgentService 创建 Agent 对话服务
func NewChatAgentService(
	llmConfigRepo repository.UserLLMConfigRepository,
	retriever rag.RAGRetriever,
	conversationRepo repository.ConversationRepository,
	messageRepo repository.MessageRepository,
	chatCache *cache.ChatCache,
) ChatAgentService {
	return &chatAgentService{
		llmConfigRepo:    llmConfigRepo,
		retriever:        retriever,
		conversationRepo: conversationRepo,
		messageRepo:      messageRepo,
		cache:            chatCache,
	}
}

// ProcessMessageWithAgent 使用 Agent 处理消息
func (s *chatAgentService) ProcessMessageWithAgent(ctx context.Context, req *request.ProcessMessageRequest) (<-chan AgentStreamEvent, error) {
	// 1. 获取并发锁
	lockValue, acquired, err := s.cache.AcquireLock(ctx, req.ConversationID)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "获取并发锁失败", err)
	}
	if !acquired {
		return nil, bizerrors.New(bizerrors.CodeConflict, "该对话正在处理中，请稍后再试")
	}

	// 2. 创建对话（如果是新对话）
	conversationID := req.ConversationID
	isNewConversation := false
	if conversationID == 0 {
		isNewConversation = true
		conv := &entity.Conversation{
			NotebookID: req.NotebookID,
			UserID:     req.UserID,
			Title:      "新对话",
		}
		if err := s.conversationRepo.Create(conv); err != nil {
			s.cache.ReleaseLock(ctx, conversationID, lockValue)
			return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "创建对话失败", err)
		}
		conversationID = conv.ID
	}

	// 3. 创建可取消的 context
	processCtx, cancel := context.WithCancel(ctx)
	s.cancelFuncs.Store(conversationID, cancel)

	// 4. 启动 goroutine 处理
	eventCh := make(chan AgentStreamEvent, 64)

	go func() {
		defer func() {
			s.cancelFuncs.Delete(conversationID)
			s.cache.ReleaseLock(context.Background(), conversationID, lockValue)
			close(eventCh)
		}()

		s.processWithAgentAsync(processCtx, conversationID, isNewConversation, req, eventCh)
	}()

	return eventCh, nil
}

// processWithAgentAsync 异步处理 Agent 消息
func (s *chatAgentService) processWithAgentAsync(ctx context.Context, conversationID uint, isNewConversation bool, req *request.ProcessMessageRequest, eventCh chan<- AgentStreamEvent) {
	logger.Info("[Agent] ====== 开始处理消息 ======",
		zap.Uint("conversationID", conversationID),
		zap.Uint("userID", req.UserID),
		zap.Bool("isNewConversation", isNewConversation),
		zap.String("content", req.Content),
	)

	// 1. 检查是否选中资料来源
	if len(req.SourceIDs) == 0 {
		logger.Info("[Agent] 未选中资料，提示用户")
		eventCh <- AgentStreamEvent{Type: AgentEventToken, Content: "请先选中资料再进行提问"}
		eventCh <- AgentStreamEvent{Type: AgentEventDone}
		return
	}

	// 2. 获取用户的 LLM 配置
	logger.Info("[Agent] 步骤1: 获取用户 LLM 配置")
	var llmConfig *entity.UserLLMConfig
	var err error
	if req.LLMConfigID > 0 {
		llmConfig, err = s.llmConfigRepo.FindByIDAndUserID(req.LLMConfigID, req.UserID)
	} else {
		llmConfig, err = s.llmConfigRepo.FindDefaultByUserID(req.UserID)
	}
	if err != nil || llmConfig == nil {
		logger.Error("[Agent] 获取 LLM 配置失败", zap.Error(err))
		s.sendAgentError(eventCh, "获取 AI 配置失败，请先在设置中配置 LLM 服务")
		return
	}

	// 3. 创建 ToolCallingChatModel
	logger.Info("[Agent] 步骤2: 创建 ToolCallingChatModel")
	chatModel, err := llm.NewToolCallingChatModel(ctx, llmConfig)
	if err != nil {
		logger.Error("[Agent] 创建模型失败", zap.Error(err))
		s.sendAgentError(eventCh, "创建 AI 模型失败")
		return
	}

	// 4. 准备工具集
	logger.Info("[Agent] 步骤3: 准备工具集")
	tools, references := s.buildTools(req.UserID, req.SourceIDs)

	// 5. 创建 Agent
	logger.Info("[Agent] 步骤4: 创建 ChatAgent")
	chatAgent, err := chat.NewChatAgent(ctx, &chat.ChatAgentConfig{
		Model:        chatModel,
		Tools:        tools,
		MaxSteps:     10,
		SystemPrompt: agentPrompts.ChatAgentSystemPrompt,
	})
	if err != nil {
		logger.Error("[Agent] 创建 Agent 失败", zap.Error(err))
		s.sendAgentError(eventCh, "创建 Agent 失败")
		return
	}

	// 6. 构建消息（包含历史）
	logger.Info("[Agent] 步骤5: 构建消息")
	messages, err := s.buildAgentMessages(ctx, req.UserID, conversationID, req.Content)
	if err != nil {
		logger.Error("[Agent] 构建消息失败", zap.Error(err))
		s.sendAgentError(eventCh, "加载对话历史失败")
		return
	}

	// 7. 运行 Agent
	logger.Info("[Agent] 步骤6: 运行 Agent")
	iter := chatAgent.Run(ctx, messages)

	// 8. 转发流式事件
	fullContent := s.forwardAgentEvents(ctx, eventCh, iter)

	// 9. 发送引用事件,不去重，保持与 LLM 看到的编号一致
	if len(*references) > 0 {
		eventCh <- AgentStreamEvent{
			Type:    AgentEventReference,
			Content: "",
			Data:    *references,
		}
	}

	// 10. 保存消息（即使取消也保存用户消息，保留对话完整性）
	logger.Info("[Agent] 步骤7: 保存消息", zap.Int("contentLen", len(fullContent)))
	evictedPair, err := s.saveAgentMessages(ctx, conversationID, req.Content, fullContent, *references)
	if err != nil {
		logger.Error("[Agent] 保存消息失败", zap.Error(err))
	}

	// 11. 异步更新摘要（不阻塞主流程）
	go func() {
		if err := s.updateSummary(context.Background(), conversationID, req.UserID, evictedPair); err != nil {
			logger.Warn("[Agent] 更新摘要失败", zap.Error(err))
		}
	}()

	// 12. 检查是否需要生成标题（仅在有回答内容时）
	if len(fullContent) > 0 {
		conv, findErr := s.conversationRepo.FindByID(conversationID)
		if findErr != nil {
			logger.Warn("[Agent] 查询对话失败，跳过标题生成", zap.Error(findErr))
		}
		if conv != nil && conv.Title == "新对话" {
			title := s.generateAndUpdateTitle(ctx, conversationID, req.UserID, req.Content)
			if title != "" {
				eventCh <- AgentStreamEvent{
					Type:    AgentEventTitle,
					Content: title,
					Data:    conversationID,
				}
			}
		}
	}

	// 13. 发送完成事件
	eventCh <- AgentStreamEvent{Type: AgentEventDone}
	logger.Info("[Agent] 处理完成")
}

// buildTools 构建工具集，同时返回引用收集器
func (s *chatAgentService) buildTools(userID uint, sourceIDs []uint) ([]tool.BaseTool, *[]response.Reference) {
	var refs []response.Reference
	ragTool := chatTools.NewRAGRetrieverTool(s.retriever, userID, sourceIDs, &refs)
	historyTool := chatTools.NewChatHistoryTool(s.messageRepo, s.conversationRepo, s.cache)

	return []tool.BaseTool{ragTool, historyTool}, &refs
}

// StopGeneration 终止 Agent 生成
func (s *chatAgentService) StopGeneration(ctx context.Context, conversationID uint) error {
	cancelFunc, ok := s.cancelFuncs.Load(conversationID)
	if !ok {
		return bizerrors.New(bizerrors.CodeNotFound, "未找到正在进行的生成任务")
	}

	cancel, ok := cancelFunc.(context.CancelFunc)
	if !ok {
		return bizerrors.New(bizerrors.CodeInternalError, "取消函数类型断言失败")
	}

	cancel()
	return nil
}
