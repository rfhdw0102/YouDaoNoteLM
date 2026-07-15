package youdao

// BindRequest 绑定 API Key 请求
type BindRequest struct {
	APIKey string `json:"api_key" binding:"required"`
}

// ImportNoteRequest 单篇导入请求
type ImportNoteRequest struct {
	FileID     string `json:"file_id" binding:"required"`
	NotebookID uint   `json:"notebook_id" binding:"required"`
}

// ImportBatchRequest 批量导入请求
type ImportBatchRequest struct {
	FileIDs    []string          `json:"file_ids" binding:"required,min=1"`
	NotebookID uint              `json:"notebook_id" binding:"required"`
	FileNames  map[string]string `json:"file_names,omitempty"` // fileID -> 笔记标题（可选）
}
