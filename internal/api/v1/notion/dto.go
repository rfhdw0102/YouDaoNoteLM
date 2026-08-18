package notion

type BatchImportRequest struct {
	NotebookID uint     `json:"notebook_id" binding:"required"`
	PageIDs    []string `json:"page_ids" binding:"required,min=1"`
}
