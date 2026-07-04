package service

import (
	"context"

	"YoudaoNoteLm/internal/agent/chat"
	"YoudaoNoteLm/internal/model/dto/request"
)

// ChatAgentService Agent 对话服务接口
type ChatAgentService interface {
	// ProcessMessageWithAgent 使用 Agent 处理消息
	ProcessMessageWithAgent(ctx context.Context, req *request.ProcessMessageRequest) (<-chan chat.StreamEvent, error)

	// StopGeneration 终止 Agent 生成
	StopGeneration(ctx context.Context, userID, conversationID uint) error
}
