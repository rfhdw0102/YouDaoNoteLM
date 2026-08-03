package response

import "time"

// ConversationResponse 对话响应
type ConversationResponse struct {
	ID         uint      `json:"id"`
	Title      string    `json:"title"`
	NotebookID uint      `json:"notebook_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// MessageResponse 消息响应
type MessageResponse struct {
	ID        uint             `json:"id"`
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Metadata  *MessageMetadata `json:"metadata,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
}

// MessageMetadata 消息元数据
type MessageMetadata struct {
	References []Reference `json:"references,omitempty"`
}

// Reference 引用来源
type Reference struct {
	SourceID      uint    `json:"source_id"`
	SourceName    string  `json:"source_name"`
	ParentBlockID int64   `json:"parent_block_id"`
	ChunkContent  string  `json:"chunk_content"`
	Score         float32 `json:"score"`
}
