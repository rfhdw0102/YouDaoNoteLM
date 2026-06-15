package service

import "context"

// GenerationType identifies a supported generation sub-agent.
type GenerationType string

const (
	GenerationTypeMindmap GenerationType = "mindmap"
	GenerationTypePPT     GenerationType = "ppt"
	GenerationTypeQuiz    GenerationType = "quiz"
	GenerationTypeNote    GenerationType = "note"
)

// GenerationRequest is the internal request for the supervisor generation agent.
type GenerationRequest struct {
	UserID       uint           `json:"user_id,omitempty"`
	NotebookID   uint           `json:"notebook_id,omitempty"`
	Markdown     string         `json:"markdown"`
	Type         GenerationType `json:"type"`
	Prompt       string         `json:"prompt,omitempty"`
	Options      map[string]any `json:"options,omitempty"`
	SourceIDs    []uint         `json:"source_ids,omitempty"`
	UseWeb       bool           `json:"use_web,omitempty"`
	AllowDegrade bool           `json:"allow_degrade,omitempty"`
}

// GenerationReference records a local RAG reference used for generation.
type GenerationReference struct {
	SourceID    uint    `json:"source_id"`
	SourceName  string  `json:"source_name,omitempty"`
	Content     string  `json:"content"`
	Score       float32 `json:"score,omitempty"`
	Heading     string  `json:"heading,omitempty"`
	ChapterPath string  `json:"chapter_path,omitempty"`
}

// GenerationResponse is the unified output returned by all generation agents.
type GenerationResponse struct {
	Type          GenerationType        `json:"type"`
	Content       string                `json:"content"`
	References    []GenerationReference `json:"references,omitempty"`
	SearchResults []SearchResult        `json:"search_results,omitempty"`
	Meta          map[string]any        `json:"meta,omitempty"`
}

// GenerationPrompt is the prompt payload passed to the model-backed sub-agent.
type GenerationPrompt struct {
	AgentName    string
	System       string
	User         string
	Context      string
	OutputFormat string
}

// GenerationModel abstracts Eino-backed model generation for testable agents.
type GenerationModel interface {
	Generate(ctx context.Context, prompt GenerationPrompt) (string, error)
}

// GenerationService is the supervisor entry point for generation.
type GenerationService interface {
	Generate(ctx context.Context, req *GenerationRequest) (*GenerationResponse, error)
}
