package service

import (
	"YoudaoNoteLm/internal/agent/chat"
	agentPrompts "YoudaoNoteLm/internal/agent/chat/prompts"
	"YoudaoNoteLm/internal/agent/chat/tools"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

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
	llmConfig, err := s.llmConfigRepo.FindDefaultByUserID(req.UserID)
	if err != nil || llmConfig == nil {
		logger.Error("[Agent] 获取 LLM 配置失败", zap.Error(err))
		s.sendAgentError(eventCh, "获取 AI 配置失败")
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
	tools := s.buildTools(req.UserID, req.SourceIDs)

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
	fullContent, references := s.forwardAgentEvents(ctx, eventCh, iter)

	// 9. 保存消息
	logger.Info("[Agent] 步骤7: 保存消息", zap.Int("contentLen", len(fullContent)))
	if len(fullContent) > 0 {
		if err := s.saveAgentMessages(ctx, conversationID, req.Content, fullContent, references); err != nil {
			logger.Error("[Agent] 保存消息失败", zap.Error(err))
		}

		// 10. 检查是否需要生成标题
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

	// 11. 发送完成事件
	eventCh <- AgentStreamEvent{Type: AgentEventDone}
	logger.Info("[Agent] 处理完成")
}

// buildTools 构建工具集
func (s *chatAgentService) buildTools(userID uint, sourceIDs []uint) []tool.BaseTool {
	ragTool := tools.NewRAGRetrieverTool(s.retriever, userID, sourceIDs)
	historyTool := tools.NewChatHistoryTool(s.messageRepo, s.cache)

	return []tool.BaseTool{ragTool, historyTool}
}

// buildAgentMessages 构建 Agent 消息
func (s *chatAgentService) buildAgentMessages(ctx context.Context, userID uint, conversationID uint, content string) ([]*schema.Message, error) {
	var messages []*schema.Message

	// 加载历史消息（如果对话已存在）
	if conversationID > 0 {
		// 优先从缓存获取
		history, err := s.cache.GetRecentMessages(ctx, conversationID)
		if err != nil || len(history) == 0 {
			// 降级到数据库
			recentMsgs, dbErr := s.messageRepo.FindRecentByConversationID(conversationID, 20)
			if dbErr == nil {
				history = convertToMessagePairs(recentMsgs, 10)
			}
		}

		// 转换为 schema.Message
		for _, pair := range history {
			messages = append(messages, schema.UserMessage(pair.User))
			messages = append(messages, schema.AssistantMessage(pair.Assistant, nil))
		}
	}

	// 添加当前用户消息
	messages = append(messages, schema.UserMessage(content))

	return messages, nil
}

// convertToMessagePairs 将数据库消息转为 MessagePair
func convertToMessagePairs(msgs []*entity.Message, limit int) []cache.MessagePair {
	var pairs []cache.MessagePair
	var pendingUserMsg string
	for _, msg := range msgs {
		switch msg.Role {
		case "user":
			pendingUserMsg = msg.Content
		case "assistant":
			pairs = append(pairs, cache.MessagePair{
				User:      pendingUserMsg,
				Assistant: msg.Content,
			})
			pendingUserMsg = ""
		}
	}
	if len(pairs) > limit {
		pairs = pairs[len(pairs)-limit:]
	}
	return pairs
}

// forwardAgentEvents 转发 Agent 事件，返回最终内容和引用
func (s *chatAgentService) forwardAgentEvents(ctx context.Context, eventCh chan<- AgentStreamEvent, iter *adk.AsyncIterator[*adk.AgentEvent]) (string, []response.Reference) {
	var fullContent string
	var references []response.Reference

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		// 处理错误事件
		if event.Err != nil {
			logger.Error("[Agent] Agent 错误", zap.Error(event.Err))
			s.sendAgentError(eventCh, "Agent 执行失败: "+event.Err.Error())
			return fullContent, references
		}

		// 处理输出事件
		if event.Output != nil && event.Output.MessageOutput != nil {
			output := event.Output.MessageOutput

			// 获取消息内容
			msg, msgErr := output.GetMessage()
			if msgErr != nil {
				logger.Error("[Agent] 获取消息失败", zap.Error(msgErr))
				continue
			}

			// 处理 assistant 消息（token 或 tool call）
			if output.Role == schema.Assistant {
				hasToolCalls := len(msg.ToolCalls) > 0

				// 只有最终回答（没有 tool calls）才累加到 fullContent
				if msg.Content != "" {
					if !hasToolCalls {
						fullContent += msg.Content
					}
					eventCh <- AgentStreamEvent{
						Type:    AgentEventToken,
						Content: msg.Content,
					}
				}

				// 处理工具调用
				if hasToolCalls {
					for _, tc := range msg.ToolCalls {
						eventCh <- AgentStreamEvent{
							Type:    AgentEventToolCall,
							Content: tc.Function.Name,
							Data:    tc.Function.Arguments,
						}
					}
				}
			}

			// 处理 tool 结果消息
			if output.Role == schema.Tool {
				eventCh <- AgentStreamEvent{
					Type:    AgentEventToolResult,
					Content: msg.Content,
					Data:    output.ToolName,
				}
			}
		}
	}

	return fullContent, references
}

// sendAgentError 发送错误事件
func (s *chatAgentService) sendAgentError(eventCh chan<- AgentStreamEvent, msg string) {
	eventCh <- AgentStreamEvent{
		Type:    AgentEventError,
		Content: msg,
	}
}

// saveAgentMessages 保存 Agent 消息
func (s *chatAgentService) saveAgentMessages(ctx context.Context, conversationID uint, userContent, assistantContent string, references []response.Reference) error {
	metadataJSON := "{}"
	if len(references) > 0 {
		metadata := response.MessageMetadata{References: references}
		data, err := json.Marshal(metadata)
		if err == nil {
			metadataJSON = string(data)
		}
	}

	msgs := []*entity.Message{
		{
			ConversationID: conversationID,
			Role:           "user",
			Content:        userContent,
			Metadata:       "{}",
		},
		{
			ConversationID: conversationID,
			Role:           "assistant",
			Content:        assistantContent,
			Metadata:       metadataJSON,
		},
	}

	if err := s.messageRepo.CreateBatch(msgs); err != nil {
		return fmt.Errorf("批量保存消息失败: %w", err)
	}

	// 更新缓存
	if err := s.cache.AddMessage(ctx, conversationID, userContent, assistantContent); err != nil {
		logger.Warn("[Agent] 更新消息缓存失败", zap.Error(err))
	}

	return nil
}

// generateAndUpdateTitle 根据用户问题生成会话标题，返回生成的标题
func (s *chatAgentService) generateAndUpdateTitle(ctx context.Context, conversationID uint, userID uint, userQuestion string) string {
	// 构建生成标题的 prompt
	titlePrompt := fmt.Sprintf(`请根据以下用户问题，生成一个简短的会话标题（不超过20个字符）。

要求：
1. 标题要简洁明了，概括问题主题
2. 不要使用引号或特殊符号
3. 只输出标题，不要其他内容

用户问题：%s

标题：`, userQuestion)

	// 获取用户的 LLM
	llmModel, err := s.getChatModel(ctx, userID)
	if err != nil {
		logger.Warn("[Agent] 获取 LLM 失败，跳过标题生成", zap.Error(err))
		return ""
	}

	// 调用 LLM 生成标题
	messages := []*schema.Message{
		{Role: schema.User, Content: titlePrompt},
	}

	stream, err := (*llmModel).Stream(ctx, messages)
	if err != nil {
		logger.Warn("[Agent] 调用 LLM 生成标题失败", zap.Error(err))
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
			logger.Warn("[Agent] 读取标题生成结果失败", zap.Error(err))
			return ""
		}
		title += chunk.Content
	}

	// 清理标题
	title = cleanTitle(title)
	if title == "" {
		return ""
	}

	// 更新数据库中的标题
	conv, err := s.conversationRepo.FindByID(conversationID)
	if err != nil || conv == nil {
		logger.Warn("[Agent] 查询对话失败", zap.Error(err))
		return ""
	}

	conv.Title = title
	if err := s.conversationRepo.Update(conv); err != nil {
		logger.Warn("[Agent] 更新对话标题失败", zap.Error(err))
		return ""
	}

	logger.Info("[Agent] 会话标题生成成功", zap.String("title", title))
	return title
}

// getChatModel 获取用户的 ChatModel
func (s *chatAgentService) getChatModel(ctx context.Context, userID uint) (*model.ChatModel, error) {
	cfg, err := s.llmConfigRepo.FindDefaultByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("获取用户 LLM 配置失败: %w", err)
	}
	if cfg == nil {
		return nil, fmt.Errorf("用户 %d 未配置 LLM", userID)
	}

	chatModel, err := llm.NewChatModel(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &chatModel, nil
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

// cleanTitle 清理标题
func cleanTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.Trim(title, "\"'")

	// 限制标题长度
	runes := []rune(title)
	if len(runes) > 20 {
		title = string(runes[:20])
	}

	return title
}

// CreateConversation 创建对话
func (s *chatAgentService) CreateConversation(ctx context.Context, userID, notebookID uint, title string) (uint, error) {
	conv := &entity.Conversation{
		NotebookID: notebookID,
		UserID:     userID,
		Title:      title,
	}
	if conv.Title == "" {
		conv.Title = "新对话"
	}

	if err := s.conversationRepo.Create(conv); err != nil {
		return 0, bizerrors.NewWithErr(bizerrors.CodeInternalError, "创建对话失败", err)
	}
	return conv.ID, nil
}

// GetConversation 获取对话详情
func (s *chatAgentService) GetConversation(ctx context.Context, conversationID uint) (*response.ConversationResponse, error) {
	conv, err := s.conversationRepo.FindByID(conversationID)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询对话失败", err)
	}
	if conv == nil {
		return nil, bizerrors.ErrNotFound
	}

	return &response.ConversationResponse{
		ID:         conv.ID,
		Title:      conv.Title,
		NotebookID: conv.NotebookID,
		CreatedAt:  conv.CreatedAt,
		UpdatedAt:  conv.UpdatedAt,
	}, nil
}

// ListConversations 获取对话列表
func (s *chatAgentService) ListConversations(ctx context.Context, notebookID uint) ([]*response.ConversationResponse, error) {
	convs, err := s.conversationRepo.FindByNotebookID(notebookID)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询对话列表失败", err)
	}

	result := make([]*response.ConversationResponse, 0, len(convs))
	for _, conv := range convs {
		result = append(result, &response.ConversationResponse{
			ID:         conv.ID,
			Title:      conv.Title,
			NotebookID: conv.NotebookID,
			CreatedAt:  conv.CreatedAt,
			UpdatedAt:  conv.UpdatedAt,
		})
	}
	return result, nil
}

// UpdateConversation 更新对话标题
func (s *chatAgentService) UpdateConversation(ctx context.Context, conversationID uint, title string) error {
	conv, err := s.conversationRepo.FindByID(conversationID)
	if err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询对话失败", err)
	}
	if conv == nil {
		return bizerrors.ErrNotFound
	}

	conv.Title = title
	if err := s.conversationRepo.Update(conv); err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "更新对话失败", err)
	}
	return nil
}

// DeleteConversation 删除对话
func (s *chatAgentService) DeleteConversation(ctx context.Context, conversationID uint) error {
	conv, err := s.conversationRepo.FindByID(conversationID)
	if err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询对话失败", err)
	}
	if conv == nil {
		return bizerrors.ErrNotFound
	}

	// 先删除关联的消息
	if err := s.messageRepo.DeleteByConversationID(conversationID); err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "删除对话消息失败", err)
	}

	// 再删除对话
	if err := s.conversationRepo.Delete(conversationID); err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "删除对话失败", err)
	}

	// 清除 Redis 缓存
	if err := s.cache.DeleteConversationCache(ctx, conversationID); err != nil {
		logger.Warn("[Agent] 清除对话缓存失败", zap.Error(err))
	}

	return nil
}

// GetMessages 获取消息历史
func (s *chatAgentService) GetMessages(ctx context.Context, conversationID uint) ([]*response.MessageResponse, error) {
	msgs, err := s.messageRepo.FindByConversationID(conversationID)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询消息失败", err)
	}

	result := make([]*response.MessageResponse, 0, len(msgs))
	for _, msg := range msgs {
		resp := &response.MessageResponse{
			ID:        msg.ID,
			Role:      msg.Role,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt,
		}

		if msg.Metadata != "" {
			var metadata response.MessageMetadata
			if err := json.Unmarshal([]byte(msg.Metadata), &metadata); err == nil {
				resp.Metadata = &metadata
			}
		}

		result = append(result, resp)
	}
	return result, nil
}
