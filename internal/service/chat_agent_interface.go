package service

import (
	"context"

	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/model/dto/response"
)

// AgentStreamEvent Agent 流式事件
type AgentStreamEvent struct {
	Type    string      `json:"type"`           // 事件类型
	Content string      `json:"content"`        // 事件内容
	Data    interface{} `json:"data,omitempty"` // 附加数据
}

// Agent 事件类型常量
const (
	AgentEventToken      = "token"       // LLM 生成的 token
	AgentEventToolCall   = "tool_call"   // 工具调用开始
	AgentEventToolResult = "tool_result" // 工具调用结果
	AgentEventReference  = "reference"   // 检索引用
	AgentEventTitle      = "title"       // 对话标题更新
	AgentEventDone       = "done"        // 生成完成
	AgentEventError      = "error"       // 错误
)

// ChatAgentService Agent 对话服务接口
type ChatAgentService interface {
	// ProcessMessageWithAgent 使用 Agent 处理消息
	ProcessMessageWithAgent(ctx context.Context, req *request.ProcessMessageRequest) (<-chan AgentStreamEvent, error)

	// StopGeneration 终止 Agent 生成
	StopGeneration(ctx context.Context, conversationID uint) error

	// CreateConversation 创建对话
	CreateConversation(ctx context.Context, userID, notebookID uint, title string) (uint, error)

	// GetConversation 获取对话详情
	GetConversation(ctx context.Context, conversationID uint) (*response.ConversationResponse, error)

	// ListConversations 获取对话列表
	ListConversations(ctx context.Context, notebookID uint) ([]*response.ConversationResponse, error)

	// UpdateConversation 更新对话标题
	UpdateConversation(ctx context.Context, conversationID uint, title string) error

	// DeleteConversation 删除对话
	DeleteConversation(ctx context.Context, conversationID uint) error

	// GetMessages 获取消息历史
	GetMessages(ctx context.Context, conversationID uint) ([]*response.MessageResponse, error)
}
