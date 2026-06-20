package chat

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/schema"

	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/pkg/cache"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

// RecentRoundsLimit 缓存中保留的最近对话轮数
const RecentRoundsLimit = 10

// ContextBuilder 上下文构建器
type ContextBuilder struct {
	conversationRepo repository.ConversationRepository
	messageRepo      repository.MessageRepository
	cache            *cache.ChatCache
}

// NewContextBuilder 创建上下文构建器
func NewContextBuilder(
	conversationRepo repository.ConversationRepository,
	messageRepo repository.MessageRepository,
	cache *cache.ChatCache,
) *ContextBuilder {
	return &ContextBuilder{
		conversationRepo: conversationRepo,
		messageRepo:      messageRepo,
		cache:            cache,
	}
}

// BuildMessages 构建 Agent 消息（包含摘要和历史）
func (b *ContextBuilder) BuildMessages(ctx context.Context, conversationID uint, content string) ([]*schema.Message, error) {
	var messages []*schema.Message

	// 加载历史消息（如果对话已存在）
	if conversationID > 0 {
		// 1. 获取摘要（Redis 优先，失败则从数据库降级）
		summary := b.getSummaryWithFallback(ctx, conversationID)
		if summary != "" {
			// 将摘要作为系统消息放在最前面
			messages = append(messages, &schema.Message{
				Role:    schema.System,
				Content: fmt.Sprintf("以下是之前对话的摘要：\n%s", summary),
			})
			logger.Info("[ContextBuilder] 已加载对话摘要",
				zap.Uint("conversationID", conversationID),
				zap.Int("summaryLen", len(summary)),
			)
		}

		// 2. 获取最近 N 轮消息
		history, err := b.cache.GetRecentMessages(ctx, conversationID)
		if err != nil || len(history) == 0 {
			// 降级到数据库
			recentMsgs, dbErr := b.messageRepo.FindRecentByConversationID(conversationID, RecentRoundsLimit*2)
			if dbErr == nil {
				history = convertToMessagePairs(recentMsgs, RecentRoundsLimit)
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

// getSummaryWithFallback 获取摘要
func (b *ContextBuilder) getSummaryWithFallback(ctx context.Context, conversationID uint) string {
	// 1. 尝试从 Redis 获取
	summary, found, err := b.cache.GetSummary(ctx, conversationID)
	if err == nil && found && summary != "" {
		return summary
	}

	if err != nil {
		logger.Warn("[ContextBuilder] 从 Redis 获取摘要失败，尝试从数据库降级",
			zap.Uint("conversationID", conversationID),
			zap.Error(err),
		)
	}

	// 2. 降级：从数据库获取
	conv, dbErr := b.conversationRepo.FindByID(conversationID)
	if dbErr != nil || conv == nil {
		return ""
	}

	if conv.Summary != "" {
		// 回写到 Redis，下次直接从缓存读
		if err := b.cache.SetSummary(ctx, conversationID, conv.Summary); err != nil {
			logger.Warn("[ContextBuilder] 写入redis摘要失败", zap.Error(err))
		}
	}

	return conv.Summary
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
