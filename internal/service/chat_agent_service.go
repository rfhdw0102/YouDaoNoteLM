package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"YoudaoNoteLm/internal/agent/chat"
	"YoudaoNoteLm/internal/llm"
	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/model/dto/response"
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
	encryptionKey    []byte
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
func (s *chatAgentService) ProcessMessageWithAgent(ctx context.Context, req *request.ProcessMessageRequest) (<-chan chat.StreamEvent, error) {
	// 1. 准备/校验对话
	conversationID, err := s.prepareConversation(ctx, req)
	if err != nil {
		return nil, err
	}

	// 2. 获取并发锁
	lockValue, err := s.acquireLock(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	// 3. 创建可取消的 context
	processCtx, cancel := context.WithCancel(ctx)
	s.cancelFuncs.Store(conversationID, cancel)

	// 4. 启动 goroutine 处理
	eventCh := make(chan chat.StreamEvent, 64)

	go func() {
		defer func() {
			s.cancelFuncs.Delete(conversationID)
			s.cache.ReleaseLock(context.Background(), conversationID, lockValue)
			close(eventCh)
		}()

		s.processWithAgentAsync(processCtx, conversationID, req, eventCh)
	}()

	return eventCh, nil
}

// prepareConversation 准备对话（创建或校验）
func (s *chatAgentService) prepareConversation(ctx context.Context, req *request.ProcessMessageRequest) (uint, error) {
	if req.ConversationID == 0 {
		if req.NotebookID == 0 {
			return 0, bizerrors.New(bizerrors.CodeBadRequest, "新建对话需要传入 notebook_id")
		}
		conv := &entity.Conversation{
			NotebookID: req.NotebookID,
			UserID:     req.UserID,
			Title:      "新对话",
		}
		if err := s.conversationRepo.Create(conv); err != nil {
			return 0, bizerrors.NewWithErr(bizerrors.CodeInternalError, "创建对话失败", err)
		}
		return conv.ID, nil
	}

	conv, err := s.conversationRepo.FindByIDAndUserID(req.ConversationID, req.UserID)
	if err != nil {
		return 0, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询对话失败", err)
	}
	if conv == nil {
		return 0, bizerrors.ErrNotFound
	}

	return req.ConversationID, nil
}

// acquireLock 获取并发锁
func (s *chatAgentService) acquireLock(ctx context.Context, conversationID uint) (string, error) {
	lockValue, acquired, err := s.cache.AcquireLock(ctx, conversationID)
	if err != nil {
		return "", bizerrors.NewWithErr(bizerrors.CodeInternalError, "获取并发锁失败", err)
	}
	if !acquired {
		return "", bizerrors.New(bizerrors.CodeConflict, "该对话正在处理中，请稍后再试")
	}
	return lockValue, nil
}

// processWithAgentAsync 异步处理 Agent 消息
func (s *chatAgentService) processWithAgentAsync(ctx context.Context, conversationID uint, req *request.ProcessMessageRequest, eventCh chan<- chat.StreamEvent) {
	logger.Info("[Agent] ====== 开始处理消息 ======",
		zap.Uint("conversationID", conversationID),
		zap.Uint("userID", req.UserID),
		zap.String("content", req.Content),
	)

	// 1. 校验资料来源
	if len(req.SourceIDs) == 0 {
		eventCh <- chat.StreamEvent{Type: chat.EventToken, Content: "请先选中资料再进行提问"}
		eventCh <- chat.StreamEvent{Type: chat.EventDone}
		return
	}

	// 2. 获取 LLM 配置
	llmConfig, err := s.getLLMConfig(req.UserID, req.LLMConfigID)
	if err != nil {
		logger.Error("[Agent] 获取 LLM 配置失败", zap.Error(err))
		s.sendAgentError(eventCh, "获取 AI 配置失败，请先在设置中配置 LLM 服务")
		return
	}

	// 3. 创建 ChatAgent
	chatAgent, err := s.createChatAgent(ctx, llmConfig, req.UserID, req.SourceIDs)
	if err != nil {
		logger.Error("[Agent] 创建 ChatAgent 失败", zap.Error(err))
		s.sendAgentError(eventCh, err.Error())
		return
	}

	// 4. 调用 Process，直接转发事件
	fullContent := s.processAndForward(ctx, chatAgent, conversationID, req.Content, eventCh)

	// 5. 保存结果
	s.saveResults(ctx, conversationID, req.UserID, req.Content, fullContent, chatAgent.GetReferences())

	// 6. 生成标题并发送给前端
	if title := s.maybeGenerateTitle(ctx, conversationID, req.UserID, req.Content, fullContent); title != "" {
		eventCh <- chat.StreamEvent{
			Type:    chat.EventTitle,
			Content: title,
			Data:    conversationID,
		}
	}
}

// getLLMConfig 获取用户的 LLM 配置
func (s *chatAgentService) getLLMConfig(userID, llmConfigID uint) (*entity.UserLLMConfig, error) {
	var llmConfig *entity.UserLLMConfig
	var err error

	if llmConfigID > 0 {
		llmConfig, err = s.llmConfigRepo.FindByIDAndUserID(llmConfigID, userID)
	} else {
		llmConfig, err = s.llmConfigRepo.FindDefaultByUserID(userID)
	}
	if err != nil || llmConfig == nil {
		return nil, bizerrors.New(bizerrors.CodeBadRequest, "未找到 LLM 配置")
	}

	if !llmConfig.Enabled {
		return nil, bizerrors.New(bizerrors.CodeBadRequest, "该 LLM 配置已被禁用，请在设置中启用或选择其他配置")
	}

	llmConfig.APIKey = utils.DecryptAPIKey(llmConfig.APIKey, s.encryptionKey)
	return llmConfig, nil
}

// createChatAgent 创建 ChatAgent
func (s *chatAgentService) createChatAgent(ctx context.Context, llmConfig *entity.UserLLMConfig, userID uint, sourceIDs []uint) (*chat.ChatAgent, error) {
	logger.Info("[Agent] 创建 ChatAgent",
		zap.Uint("userID", userID),
		zap.Uints("sourceIDs", sourceIDs),
		zap.String("llmProvider", llmConfig.Provider),
		zap.String("llmModel", llmConfig.Model),
	)

	chatModel, err := llm.NewToolCallingChatModel(ctx, llmConfig)
	if err != nil {
		logger.Error("[Agent] 创建 AI 模型失败",
			zap.String("provider", llmConfig.Provider),
			zap.String("model", llmConfig.Model),
			zap.Error(err),
		)
		return nil, fmt.Errorf("创建 AI 模型失败: %w", err)
	}

	logger.Debug("[Agent] AI 模型创建成功，开始创建 ChatAgent")
	agent, err := chat.NewChatAgent(
		ctx,
		chatModel,
		s.conversationRepo,
		s.messageRepo,
		s.cache,
		s.retriever,
		userID,
		sourceIDs,
	)
	if err != nil {
		logger.Error("[Agent] 创建 ChatAgent 失败", zap.Error(err))
		return nil, err
	}

	logger.Info("[Agent] ChatAgent 创建成功")
	return agent, nil
}

// processAndForward 调用 Process 并转发事件，返回完整内容
func (s *chatAgentService) processAndForward(ctx context.Context, chatAgent *chat.ChatAgent, conversationID uint, content string, eventCh chan<- chat.StreamEvent) string {
	agentEventCh := chatAgent.Process(ctx, conversationID, content)

	var fullContent string
	for event := range agentEventCh {
		eventCh <- event // 直接转发，不需要转换
		if event.Type == chat.EventToken {
			fullContent += event.Content
		}
	}

	return fullContent
}

// saveResults 保存结果
func (s *chatAgentService) saveResults(ctx context.Context, conversationID, userID uint, userContent, fullContent string, references []response.Reference) {
	saveCtx := context.Background()

	// 保存消息
	evictedPair, err := s.saveMessages(saveCtx, conversationID, userContent, fullContent, references)
	if err != nil {
		logger.Error("[Agent] 保存消息失败", zap.Error(err))
		return
	}

	// 异步更新摘要
	if ctx.Err() == nil && len(fullContent) > 0 && evictedPair != nil {
		go func() {
			if err := s.updateSummary(context.Background(), conversationID, userID, evictedPair); err != nil {
				logger.Warn("[Agent] 更新摘要失败", zap.Error(err))
			}
		}()
	}
}

// saveMessages 保存消息
func (s *chatAgentService) saveMessages(ctx context.Context, conversationID uint, userContent, assistantContent string, references []response.Reference) (*cache.MessagePair, error) {
	msgs := []*entity.Message{
		{ConversationID: conversationID, Role: "user", Content: userContent, Metadata: "{}"},
	}

	if len(assistantContent) > 0 {
		assistantMetadata := "{}"
		if len(references) > 0 {
			meta := response.MessageMetadata{References: references}
			if data, err := json.Marshal(meta); err == nil {
				assistantMetadata = string(data)
			}
		}
		msgs = append(msgs, &entity.Message{
			ConversationID: conversationID,
			Role:           "assistant",
			Content:        assistantContent,
			Metadata:       assistantMetadata,
		})
	}

	if err := s.messageRepo.CreateBatch(msgs); err != nil {
		return nil, fmt.Errorf("批量保存消息失败: %w", err)
	}

	if len(assistantContent) == 0 {
		return nil, nil
	}

	var evictedPair *cache.MessagePair
	recentMessages, err := s.cache.GetRecentMessages(ctx, conversationID)
	if err == nil && len(recentMessages) >= chat.RecentRoundsLimit {
		evicted := recentMessages[0]
		evictedPair = &evicted
	}

	if err := s.cache.AddMessage(ctx, conversationID, userContent, assistantContent); err != nil {
		logger.Warn("[Agent] 更新消息缓存失败", zap.Error(err))
	}

	return evictedPair, nil
}

// updateSummary 更新对话摘要
func (s *chatAgentService) updateSummary(ctx context.Context, conversationID, userID uint, evictedPair *cache.MessagePair) error {
	existingSummary := s.getSummaryFromDB(ctx, conversationID)
	newMessagesText := fmt.Sprintf("用户: %s\n助手: %s", evictedPair.User, evictedPair.Assistant)
	summaryPrompt := buildIncrementalSummaryPrompt(existingSummary, newMessagesText)

	llmModel, err := s.getChatModel(ctx, userID)
	if err != nil {
		return fmt.Errorf("获取 LLM 失败: %w", err)
	}

	stream, err := llmModel.Stream(ctx, []*schema.Message{{Role: schema.User, Content: summaryPrompt}})
	if err != nil {
		return fmt.Errorf("调用 LLM 生成摘要失败: %w", err)
	}
	defer stream.Close()

	var summary string
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取摘要结果失败: %w", err)
		}
		summary += chunk.Content
	}

	summary = strings.TrimSpace(summary)
	if summary == "" {
		return nil
	}

	if err := s.cache.SetSummary(ctx, conversationID, summary); err != nil {
		logger.Warn("[Agent] 保存摘要到 Redis 失败", zap.Error(err))
	}
	if err := s.conversationRepo.UpdateSummary(conversationID, summary); err != nil {
		logger.Warn("[Agent] 保存摘要到数据库失败", zap.Error(err))
	}

	return nil
}

// getSummaryFromDB 从数据库获取摘要
func (s *chatAgentService) getSummaryFromDB(ctx context.Context, conversationID uint) string {
	conv, err := s.conversationRepo.FindByID(conversationID)
	if err != nil || conv == nil {
		return ""
	}
	return conv.Summary
}

// getChatModel 获取用户的 ChatModel（用于标题/摘要生成）
func (s *chatAgentService) getChatModel(ctx context.Context, userID uint) (model.ToolCallingChatModel, error) {
	cfg, err := s.llmConfigRepo.FindDefaultByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("获取用户 LLM 配置失败: %w", err)
	}
	if cfg == nil {
		return nil, fmt.Errorf("用户 %d 未配置 LLM", userID)
	}

	cfg.APIKey = utils.DecryptAPIKey(cfg.APIKey, s.encryptionKey)
	return llm.NewChatModel(ctx, cfg)
}

// maybeGenerateTitle 生成标题（仅在新对话时），返回生成的标题
func (s *chatAgentService) maybeGenerateTitle(ctx context.Context, conversationID, userID uint, userContent, fullContent string) string {
	if len(fullContent) == 0 {
		return ""
	}

	conv, err := s.conversationRepo.FindByID(conversationID)
	if err != nil || conv == nil || conv.Title != "新对话" {
		return ""
	}

	title := s.generateTitle(ctx, userID, userContent)
	if title == "" {
		return ""
	}

	if err := s.conversationRepo.UpdateTitle(conversationID, title); err != nil {
		logger.Warn("[Agent] 更新对话标题失败", zap.Error(err))
		return ""
	}

	logger.Info("[Agent] 会话标题生成成功", zap.String("title", title))
	return title
}

// generateTitle 生成标题
func (s *chatAgentService) generateTitle(ctx context.Context, userID uint, userQuestion string) string {
	titlePrompt := fmt.Sprintf(`请根据以下用户问题，生成一个简短的会话标题（不超过20个字符）。

要求：
1. 标题要简洁明了，概括问题主题
2. 不要使用引号或特殊符号
3. 只输出标题，不要其他内容

用户问题：%s

标题：`, userQuestion)

	llmModel, err := s.getChatModel(ctx, userID)
	if err != nil {
		return ""
	}

	stream, err := llmModel.Stream(ctx, []*schema.Message{{Role: schema.User, Content: titlePrompt}})
	if err != nil {
		return ""
	}
	defer stream.Close()

	var title string
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ""
		}
		title += chunk.Content
	}

	return cleanTitle(title)
}

// buildIncrementalSummaryPrompt 构建增量摘要更新的 prompt
func buildIncrementalSummaryPrompt(existingSummary, newMessagesText string) string {
	if existingSummary != "" {
		return fmt.Sprintf(`请将以下新对话内容合并到现有摘要中，保持简洁。

要求：
1. 摘要不超过 500 字
2. 保留重要的问题、结论和决策
3. 使用中文
4. 只输出更新后的摘要内容

现有摘要：
%s

新对话内容：
%s

更新后的摘要：`, existingSummary, newMessagesText)
	}

	return fmt.Sprintf(`请将以下对话内容压缩为简洁的摘要，保留关键信息。

要求：
1. 摘要不超过 500 字
2. 保留重要的问题、结论和决策
3. 使用中文
4. 只输出摘要内容

对话内容：
%s

摘要：`, newMessagesText)
}

// cleanTitle 清理标题
func cleanTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.Trim(title, "\"'")
	runes := []rune(title)
	if len(runes) > 20 {
		title = string(runes[:20])
	}
	return title
}

// StopGeneration 终止 Agent 生成
func (s *chatAgentService) StopGeneration(ctx context.Context, userID, conversationID uint) error {
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
	s.cancelFuncs.Delete(conversationID)
	return nil
}

// sendAgentError 发送错误事件
func (s *chatAgentService) sendAgentError(eventCh chan<- chat.StreamEvent, msg string) {
	eventCh <- chat.StreamEvent{
		Type:    chat.EventError,
		Content: msg,
	}
}
