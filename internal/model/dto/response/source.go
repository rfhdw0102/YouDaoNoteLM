package response

import "time"

// SourceResponse 资料来源响应
type SourceResponse struct {
	ID           uint      `json:"id"`
	NotebookID   uint      `json:"notebook_id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	OriginalURL  string    `json:"original_url"`
	FilePath     string    `json:"file_path"`
	FileSize     int64     `json:"file_size"`
	MimeType     string    `json:"mime_type"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message"`
	Vectorized   bool      `json:"vectorized"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SourceContentResponse 来源内容响应
type SourceContentResponse struct {
	Content string `json:"content"`
	Type    string `json:"type,omitempty"`
}
