package request

// CreateConversationRequest 创建对话请求
type CreateConversationRequest struct {
	NotebookID uint   `json:"notebook_id" binding:"required"`
	Title      string `json:"title"`
}

// UpdateConversationRequest 更新对话请求
type UpdateConversationRequest struct {
	Title string `json:"title" binding:"required"`
}

// SendMessageRequest 发送消息请求
type SendMessageRequest struct {
	Content   string `json:"content" binding:"required"`
	SourceIDs []uint `json:"source_ids"`
}

// ProcessMessageRequest 处理消息请求（内部使用）
type ProcessMessageRequest struct {
	ConversationID uint   `json:"conversation_id"` // 对话 ID，0 表示新建
	Content        string `json:"content"`         // 用户消息
	SourceIDs      []uint `json:"source_ids"`      // 资料来源 ID
	UserID         uint   `json:"user_id"`         // 用户 ID
	NotebookID     uint   `json:"notebook_id"`     // 笔记本 ID（新建对话时需要）
}
