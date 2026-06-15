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
	if err := s.saveAgentMessages(ctx, conversationID, req.Content, fullContent, *references); err != nil {
		logger.Error("[Agent] 保存消息失败", zap.Error(err))
	}

	// 11. 检查是否需要生成标题（仅在有回答内容时）
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

	// 12. 发送完成事件
	eventCh <- AgentStreamEvent{Type: AgentEventDone}
	logger.Info("[Agent] 处理完成")
}

// buildTools 构建工具集，同时返回引用收集器
func (s *chatAgentService) buildTools(userID uint, sourceIDs []uint) ([]tool.BaseTool, *[]response.Reference) {
	var refs []response.Reference
	ragTool := tools.NewRAGRetrieverTool(s.retriever, userID, sourceIDs, &refs)
	historyTool := tools.NewChatHistoryTool(s.messageRepo, s.cache)

	return []tool.BaseTool{ragTool, historyTool}, &refs
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

// forwardAgentEvents 转发 Agent 事件，返回最终内容
func (s *chatAgentService) forwardAgentEvents(ctx context.Context, eventCh chan<- AgentStreamEvent, iter *adk.AsyncIterator[*adk.AgentEvent]) string {
	var fullContent string

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		// 处理错误事件
		if event.Err != nil {
			// 检查是否是主动取消
			if ctx.Err() == context.Canceled {
				logger.Info("[Agent] 用户主动取消，保留已生成内容", zap.Int("contentLen", len(fullContent)))
				// 如果有已生成内容，发送 token 事件让前端保留
				if len(fullContent) > 0 {
					eventCh <- AgentStreamEvent{
						Type:    AgentEventToken,
						Content: "", // 空内容表示流结束
					}
				}
				return fullContent
			}
			logger.Error("[Agent] Agent 错误", zap.Error(event.Err))
			s.sendAgentError(eventCh, "Agent 执行失败: "+event.Err.Error())
			return fullContent
		}

		// 处理输出事件
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		output := event.Output.MessageOutput

		// 流式模式：逐 chunk 读取，实时推送 token
		if output.IsStreaming {
			s.handleStreamingOutput(output, eventCh, &fullContent)
			continue
		}

		// 非流式兜底：一次性读取完整消息
		msg, msgErr := output.GetMessage()
		if msgErr != nil {
			logger.Error("[Agent] 获取消息失败", zap.Error(msgErr))
			continue
		}
		s.handleCompleteMessage(msg, output, eventCh, &fullContent)
	}

	return fullContent
}

// handleStreamingOutput 处理流式输出，逐 token 推送给前端
func (s *chatAgentService) handleStreamingOutput(output *adk.MessageVariant, eventCh chan<- AgentStreamEvent, fullContent *string) {
	stream := output.MessageStream
	if stream == nil {
		return
	}
	defer stream.Close()

	if output.Role == schema.Assistant {
		var toolCalls []schema.ToolCall
		var streamedContent string // 本轮流式内容，暂存

		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				logger.Error("[Agent] 流式读取失败", zap.Error(err))
				return
			}

			// 文本内容：逐 token 推送前端（但暂不累积到 fullContent）
			if chunk.Content != "" {
				streamedContent += chunk.Content
				eventCh <- AgentStreamEvent{
					Type:    AgentEventToken,
					Content: chunk.Content,
				}
			}

			// 收集工具调用（Anthropic 流式中 tool_use 以完整块发出）
			if len(chunk.ToolCalls) > 0 {
				toolCalls = append(toolCalls, chunk.ToolCalls...)
			}
		}

		// 流结束后判断：有工具调用则为中间推理，丢弃文本；无工具调用则为最终回答，保留
		if len(toolCalls) == 0 {
			*fullContent += streamedContent
		}

		// 发送工具调用事件
		for _, tc := range toolCalls {
			eventCh <- AgentStreamEvent{
				Type:    AgentEventToolCall,
				Content: tc.Function.Name,
				Data:    tc.Function.Arguments,
			}
		}
	} else if output.Role == schema.Tool {
		// tool 结果消息：一次性读取
		msg, err := output.GetMessage()
		if err != nil {
			logger.Error("[Agent] 获取工具结果失败", zap.Error(err))
			return
		}
		eventCh <- AgentStreamEvent{
			Type:    AgentEventToolResult,
			Content: msg.Content,
			Data:    output.ToolName,
		}
	}
}

// handleCompleteMessage 处理非流式的完整消息（兜底）
func (s *chatAgentService) handleCompleteMessage(msg *schema.Message, output *adk.MessageVariant, eventCh chan<- AgentStreamEvent, fullContent *string) {
	if output.Role == schema.Assistant {
		// 文本内容和工具调用独立处理
		if msg.Content != "" {
			*fullContent += msg.Content
			eventCh <- AgentStreamEvent{
				Type:    AgentEventToken,
				Content: msg.Content,
			}
		}

		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				eventCh <- AgentStreamEvent{
					Type:    AgentEventToolCall,
					Content: tc.Function.Name,
					Data:    tc.Function.Arguments,
				}
			}
		}
	} else if output.Role == schema.Tool {
		eventCh <- AgentStreamEvent{
			Type:    AgentEventToolResult,
			Content: msg.Content,
			Data:    output.ToolName,
		}
	}
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
	// 始终保存用户消息
	msgs := []*entity.Message{
		{
			ConversationID: conversationID,
			Role:           "user",
			Content:        userContent,
			Metadata:       "{}",
		},
	}

	// 仅在有回答内容时保存 assistant 消息
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
func (s *chatAgentService) getChatModel(ctx context.Context, userID uint) (*model.ToolCallingChatModel, error) {
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
