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
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/rag"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/pkg/cache"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"
	"YoudaoNoteLm/pkg/utils"

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
	encryptionKey    []byte // API Key 加密密钥
}

// NewChatAgentService 创建 Agent 对话服务
func NewChatAgentService(
	llmConfigRepo repository.UserLLMConfigRepository,
	retriever rag.RAGRetriever,
	conversationRepo repository.ConversationRepository,
	messageRepo repository.MessageRepository,
	chatCache *cache.ChatCache,
	encryptionKey string,
) ChatAgentService {
	return &chatAgentService{
		llmConfigRepo:    llmConfigRepo,
		retriever:        retriever,
		conversationRepo: conversationRepo,
		messageRepo:      messageRepo,
		cache:            chatCache,
		encryptionKey:    []byte(encryptionKey),
	}
}

// ProcessMessageWithAgent 使用 Agent 处理消息
func (s *chatAgentService) ProcessMessageWithAgent(ctx context.Context, req *request.ProcessMessageRequest) (<-chan AgentStreamEvent, error) {
	// 1. 准备/校验对话：必须先得到一个属于该用户的真实 conversationID，
	//    再用它去加并发锁，避免多个用户的"新建对话首条消息"全部抢同一把 chat:0:lock 的全局锁。
	conversationID := req.ConversationID
	isNewConversation := false

	if conversationID == 0 {
		// 新建对话场景
		if req.NotebookID == 0 {
			return nil, bizerrors.New(bizerrors.CodeBadRequest, "新建对话需要传入 notebook_id")
		}
		conv := &entity.Conversation{
			NotebookID: req.NotebookID,
			UserID:     req.UserID,
			Title:      "新对话",
		}
		if err := s.conversationRepo.Create(conv); err != nil {
			return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "创建对话失败", err)
		}
		conversationID = conv.ID
		isNewConversation = true
	} else {
		// 已有对话场景：校验归属
		conv, err := s.conversationRepo.FindByIDAndUserID(conversationID, req.UserID)
		if err != nil {
			return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询对话失败", err)
		}
		if conv == nil {
			return nil, bizerrors.ErrNotFound
		}
	}

	// 2. 获取该对话的并发锁
	lockValue, acquired, err := s.cache.AcquireLock(ctx, conversationID)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "获取并发锁失败", err)
	}
	if !acquired {
		return nil, bizerrors.New(bizerrors.CodeConflict, "该对话正在处理中，请稍后再试")
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

	// 解密 API Key
	if llmConfig.APIKey != "" && len(s.encryptionKey) > 0 {
		decrypted, decErr := utils.Decrypt(llmConfig.APIKey, s.encryptionKey)
		if decErr != nil {
			logger.Debug("[Agent] 解密 API Key 失败（可能未加密）", zap.Error(decErr))
		} else {
			llmConfig.APIKey = decrypted
		}
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
	tools, refCollector := s.buildTools(req.UserID, req.SourceIDs)

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

	// 9. 发送引用事件，跨多次 search_knowledge 调用累积，编号与 LLM 看到的一致
	references := refCollector.All()
	if len(references) > 0 {
		eventCh <- AgentStreamEvent{
			Type:    AgentEventReference,
			Content: "",
			Data:    references,
		}
	}

	// 10. 保存消息、即使取消也保存用户消息，保留对话完整性
	//     这里用 background ctx，避免请求 ctx 被取消导致 DB/Redis 写入失败
	logger.Info("[Agent] 步骤7: 保存消息", zap.Int("contentLen", len(fullContent)))
	saveCtx := context.Background()
	evictedPair, err := s.saveAgentMessages(saveCtx, conversationID, req.Content, fullContent, references)
	if err != nil {
		logger.Error("[Agent] 保存消息失败", zap.Error(err))
	}

	// 11. 异步更新摘要：仅在生成正常完成且确实有消息被淘汰时才做
	if ctx.Err() == nil && len(fullContent) > 0 && evictedPair != nil {
		go func(ep *cache.MessagePair) {
			if err := s.updateSummary(context.Background(), conversationID, req.UserID, ep); err != nil {
				logger.Warn("[Agent] 更新摘要失败", zap.Error(err))
			}
		}(evictedPair)
	}

	// 12. 检查是否需要生成标题（仅在有回答内容、且当前为初始 "新对话" 标题时）
	if len(fullContent) > 0 {
		conv, findErr := s.conversationRepo.FindByID(conversationID)
		if findErr != nil {
			logger.Warn("[Agent] 查询对话失败，跳过标题生成", zap.Error(findErr))
		} else if conv != nil && conv.Title == "新对话" {
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
func (s *chatAgentService) buildTools(userID uint, sourceIDs []uint) ([]tool.BaseTool, *chatTools.ReferenceCollector) {
	collector := chatTools.NewReferenceCollector()
	ragTool := chatTools.NewRAGRetrieverTool(s.retriever, userID, sourceIDs, collector)
	historyTool := chatTools.NewChatHistoryTool(s.messageRepo, s.conversationRepo, s.cache)

	return []tool.BaseTool{ragTool, historyTool}, collector
}

// StopGeneration 终止 Agent 生成（校验对话归属后再取消）
func (s *chatAgentService) StopGeneration(ctx context.Context, userID, conversationID uint) error {
	// 校验对话归属：避免任意登录用户随意中断别人的会话
	conv, err := s.conversationRepo.FindByIDAndUserID(conversationID, userID)
	if err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询对话失败", err)
	}
	if conv == nil {
		return bizerrors.ErrNotFound
	}

	cancelFunc, ok := s.cancelFuncs.Load(conversationID)
	if !ok {
		return bizerrors.New(bizerrors.CodeNotFound, "未找到正在进行的生成任务")
	}

	cancel, ok := cancelFunc.(context.CancelFunc)
	if !ok {
		return bizerrors.New(bizerrors.CodeInternalError, "取消函数类型断言失败")
	}

	cancel()
	// 主动从 map 中删除，避免下一次请求被复用到已取消的 cancel
	s.cancelFuncs.Delete(conversationID)
	return nil
}
