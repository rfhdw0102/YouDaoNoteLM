package request

// CreateNotebookRequest 创建笔记本请求
type CreateNotebookRequest struct {
	Name string `json:"name" binding:"required,max=100"`
}

// RenameNotebookRequest 重命名笔记本请求
type RenameNotebookRequest struct {
	Name string `json:"name" binding:"required,max=100"`
}
