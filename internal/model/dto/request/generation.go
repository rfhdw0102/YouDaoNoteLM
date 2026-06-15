package request

// GenerationRequest is the HTTP request body for content generation.
type GenerationRequest struct {
	NotebookID   uint           `json:"notebook_id"`
	Markdown     string         `json:"markdown" binding:"required"`
	Type         string         `json:"type" binding:"required"`
	Prompt       string         `json:"prompt"`
	Options      map[string]any `json:"options"`
	SourceIDs    []uint         `json:"source_ids"`
	UseWeb       bool           `json:"use_web"`
	AllowDegrade bool           `json:"allow_degrade"`
}
