package response

// AudioPreviewResponse 音频预览响应
type AudioPreviewResponse struct {
	PreviewID string `json:"preview_id"`
	Content   string `json:"content"`
	FileName  string `json:"file_name"`
}

// ImportTaskResponse 导入任务响应
type ImportTaskResponse struct {
	TaskID       string `json:"task_id"`
	TaskType     string `json:"task_type"`
	TotalCount   int    `json:"total_count"`
	SuccessCount int    `json:"success_count"`
	FailCount    int    `json:"fail_count"`
	Status       string `json:"status"`
	ErrorDetail  string `json:"error_detail,omitempty"`
}
