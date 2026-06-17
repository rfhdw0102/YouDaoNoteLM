package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/schema"

	"YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/pkg/cache"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

// buildAgentMessages 构建 Agent 消息
func (s *chatAgentService) buildAgentMessages(ctx context.Context, userID uint, conversationID uint, content string) ([]*schema.Message, error) {
	var messages []*schema.Message

	// 加载历史消息（如果对话已存在）
	if conversationID > 0 {
		// 1. 获取摘要（如果有）
		summary, hasSummary, err := s.cache.GetSummary(ctx, conversationID)
		if err == nil && hasSummary && summary != "" {
			// 将摘要作为系统消息放在最前面
			messages = append(messages, &schema.Message{
				Role:    schema.System,
				Content: fmt.Sprintf("以下是之前对话的摘要：\n%s", summary),
			})
			logger.Info("[Agent] 已加载对话摘要", zap.Uint("conversationID", conversationID), zap.Int("summaryLen", len(summary)))
		}

		// 2. 获取最近 10 轮消息
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

// saveAgentMessages 保存 Agent 消息
// 返回被移出缓存的最老一轮消息（如果有的话），用于更新摘要
func (s *chatAgentService) saveAgentMessages(ctx context.Context, conversationID uint, userContent, assistantContent string, references []response.Reference) (*cache.MessagePair, error) {
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
		return nil, fmt.Errorf("批量保存消息失败: %w", err)
	}

	// 在添加新消息前，获取缓存中当前的消息
	// 如果已有 10 轮，最老的那轮会被移出
	var evictedPair *cache.MessagePair
	recentMessages, err := s.cache.GetRecentMessages(ctx, conversationID)
	if err == nil && len(recentMessages) >= 10 {
		// 取出最老的那轮
		evictedPair = &recentMessages[0]
	}

	// 更新缓存
	if err := s.cache.AddMessage(ctx, conversationID, userContent, assistantContent); err != nil {
		logger.Warn("[Agent] 更新消息缓存失败", zap.Error(err))
	}

	return evictedPair, nil
}

// updateSummary 更新对话摘要
// 增量更新：使用被移出缓存的那轮消息，与现有摘要合并
func (s *chatAgentService) updateSummary(ctx context.Context, conversationID uint, userID uint, evictedPair *cache.MessagePair) error {
	// 如果没有被移出的消息，不需要更新摘要
	if evictedPair == nil {
		return nil
	}

	// 1. 获取现有摘要
	existingSummary, _, _ := s.cache.GetSummary(ctx, conversationID)

	// 2. 构建增量更新 prompt
	newMessagesText := fmt.Sprintf("用户: %s\n助手: %s", evictedPair.User, evictedPair.Assistant)
	summaryPrompt := buildIncrementalSummaryPrompt(existingSummary, newMessagesText)

	// 3. 调用 LLM 生成摘要
	llmModel, err := s.getChatModel(ctx, userID)
	if err != nil {
		return fmt.Errorf("获取 LLM 失败: %w", err)
	}

	messages := []*schema.Message{
		{Role: schema.User, Content: summaryPrompt},
	}

	stream, err := (*llmModel).Stream(ctx, messages)
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

	// 4. 保存摘要到 Redis
	if err := s.cache.SetSummary(ctx, conversationID, summary); err != nil {
		return fmt.Errorf("保存摘要失败: %w", err)
	}

	logger.Info("[Agent] 对话摘要更新成功",
		zap.Uint("conversationID", conversationID),
		zap.Int("summaryLen", len(summary)),
	)
	return nil
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
