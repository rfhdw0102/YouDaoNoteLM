package service

import (
	"context"
	"time"
)

// SearchScene 搜索使用场景。
type SearchScene string

const (
	SearchSceneGeneration    SearchScene = "generation"
	SearchSceneChat          SearchScene = "chat"
	SearchSceneImport        SearchScene = "import"
	SearchSceneSourcePreview SearchScene = "source_preview"
)

// SearchRequest 统一搜索请求。
type SearchRequest struct {
	UserID         uint        `json:"user_id,omitempty"`
	Scene          SearchScene `json:"scene"`
	Query          string      `json:"query"`
	Freshness      string      `json:"freshness,omitempty"`
	Count          int         `json:"count,omitempty"`
	NeedSummary    bool        `json:"need_summary,omitempty"`
	NeedContent    bool        `json:"need_content,omitempty"`
	Language       string      `json:"language,omitempty"`
	AllowedDomains []string    `json:"allowed_domains,omitempty"`
	BlockedDomains []string    `json:"blocked_domains,omitempty"`
	NotebookID     uint        `json:"notebook_id,omitempty"`
	SourceID       uint        `json:"source_id,omitempty"`
	TraceID        string      `json:"trace_id,omitempty"`
	AllowDegrade   bool        `json:"allow_degrade,omitempty"`
}

// SearchQuota 统一搜索额度信息。
type SearchQuota struct {
	DailyQuota *int       `json:"daily_quota,omitempty"`
	Used       int        `json:"used"`
	Remaining  *int       `json:"remaining,omitempty"`
	ResetAt    *time.Time `json:"reset_at,omitempty"`
}

// SearchResult 统一搜索结果。
type SearchResult struct {
	Title         string         `json:"title"`
	Snippet       string         `json:"snippet,omitempty"`
	URL           string         `json:"url"`
	DisplayURL    string         `json:"display_url,omitempty"`
	PublishedAt   string         `json:"published_at,omitempty"`
	SiteName      string         `json:"site_name,omitempty"`
	Score         float64        `json:"score,omitempty"`
	Content       string         `json:"content,omitempty"`
	ProviderRawID string         `json:"provider_raw_id,omitempty"`
	Meta          map[string]any `json:"meta,omitempty"`
}

// SearchResponse 统一搜索响应。
type SearchResponse struct {
	Query    string         `json:"query"`
	Provider string         `json:"provider"`
	Results  []SearchResult `json:"results"`
	Summary  string         `json:"summary,omitempty"`
	Total    int            `json:"total"`
	Cached   bool           `json:"cached"`
	Quota    *SearchQuota   `json:"quota,omitempty"`
	Meta     map[string]any `json:"meta,omitempty"`
}

// SearchImportRequest 搜索导入请求。
type SearchImportRequest struct {
	SearchRequest
}

// SearchImportResponse 搜索导入预览结果。
type SearchImportResponse struct {
	Query    string         `json:"query"`
	Provider string         `json:"provider"`
	Results  []SearchResult `json:"results"`
	URLs     []string       `json:"urls"`
	Total    int            `json:"total"`
	Cached   bool           `json:"cached"`
	Quota    *SearchQuota   `json:"quota,omitempty"`
	Meta     map[string]any `json:"meta,omitempty"`
}

// SearchService 统一搜索服务接口。
type SearchService interface {
	Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error)
	SearchAndSummarize(ctx context.Context, req *SearchRequest) (*SearchResponse, error)
	SearchForImport(ctx context.Context, req *SearchImportRequest) (*SearchImportResponse, error)
}
