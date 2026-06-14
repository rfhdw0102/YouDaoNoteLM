package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/pkg/cache"
)

// ChatHistoryTool 对话历史工具
type ChatHistoryTool struct {
	messageRepo repository.MessageRepository
	cache       *cache.ChatCache
}

// NewChatHistoryTool 创建对话历史工具
func NewChatHistoryTool(messageRepo repository.MessageRepository, cache *cache.ChatCache) tool.InvokableTool {
	return &ChatHistoryTool{
		messageRepo: messageRepo,
		cache:       cache,
	}
}

// Info 返回工具元信息
func (t *ChatHistoryTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "get_chat_history",
		Desc: "获取当前对话的历史消息，用于理解上下文、指代消解（如\"它\"、\"这个\"指代什么）。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"conversation_id": {
				Type:     schema.Integer,
				Desc:     "对话 ID",
				Required: true,
			},
			"limit": {
				Type: schema.Integer,
				Desc: "获取最近 N 轮对话，默认 10",
			},
		}),
	}, nil
}

// InvokableRun 执行获取对话历史
func (t *ChatHistoryTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params struct {
		ConversationID uint `json:"conversation_id"`
		Limit          int  `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}
	if params.ConversationID == 0 {
		return "错误：conversation_id 不能为空", nil
	}
	if params.Limit <= 0 {
		params.Limit = 10
	}

	// 优先从缓存获取
	history, err := t.cache.GetRecentMessages(ctx, params.ConversationID)
	if err != nil || len(history) == 0 {
		// 降级到数据库
		msgs, dbErr := t.messageRepo.FindRecentByConversationID(params.ConversationID, params.Limit*2)
		if dbErr != nil {
			return "获取对话历史失败: " + dbErr.Error(), nil
		}
		history = convertToCachePairs(msgs, params.Limit)
	}

	if len(history) == 0 {
		return "暂无对话历史", nil
	}

	return FormatChatHistory(history), nil
}

// convertToCachePairs 将数据库消息转为缓存格式的 MessagePair
func convertToCachePairs(msgs []*entity.Message, limit int) []cache.MessagePair {
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
	// 只返回最近的 limit 轮
	if len(pairs) > limit {
		pairs = pairs[len(pairs)-limit:]
	}
	return pairs
}
