// internal/service/external/llm_interface.go
package llm

// Message 对话消息
type Message struct {
	Role       string     `json:"role"` // system/user/assistant/tool
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // assistant 角色时使用
	ToolCallID string     `json:"tool_call_id,omitempty"` // tool 角色时使用
}

// ToolDef 工具定义
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolCall 工具调用
type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// ToolCallResponse 工具调用响应
type ToolCallResponse struct {
	Content   string     `json:"content"`    // Agent 的文本回复
	ToolCalls []ToolCall `json:"tool_calls"` // 要调用的工具列表（为空表示 Agent 结束）
}

// LLMClient LLM 客户端接口
type LLMClient interface {
	// Chat 普通对话（无工具）
	Chat(messages []Message) (string, error)
	// ChatWithTools 带工具调用的对话
	ChatWithTools(messages []Message, tools []ToolDef) (*ToolCallResponse, error)
}
