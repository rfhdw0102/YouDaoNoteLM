package request

// BatchDeleteRequest 批量删除请求
type BatchDeleteRequest struct {
	IDs []uint `json:"ids" binding:"required,min=1"`
}

// AudioConfirmRequest 确认音频导入请求
type AudioConfirmRequest struct {
	PreviewID  string  `json:"preview_id" binding:"required"`
	Content    *string `json:"content"`
	NotebookID uint    `json:"notebook_id" binding:"required"`
}
