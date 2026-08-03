// search_types.go 定义搜索服务的类型和接口。
//
// SearchService 接口抽象了 RAG 检索能力，供生成模块在 factEnhance 步骤
// 检索补充背景知识。SearchRequest/SearchResponse/SearchResult 是请求/响应类型。
package generation

import "context"

type SearchScene string

const (
	SearchSceneGeneration SearchScene = "generation"
)

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
	SkipUserConfig bool        `json:"-"`
}

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

type SearchResponse struct {
	Query    string         `json:"query"`
	Provider string         `json:"provider"`
	Results  []SearchResult `json:"results"`
	Summary  string         `json:"summary,omitempty"`
	Total    int            `json:"total"`
	Cached   bool           `json:"cached"`
	Meta     map[string]any `json:"meta,omitempty"`
}

type SearchService interface {
	SearchAndSummarize(ctx context.Context, req *SearchRequest) (*SearchResponse, error)
}

// firstNonEmpty 返回第一个非空字符串，全空时返回空串。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// truncate 将字符串截断到 maxLen 长度并追加省略号。
func truncate(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
