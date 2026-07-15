package external

import (
	"context"
	"time"
)

// SearchProviderConfig 统一搜索 provider 配置。
type SearchProviderConfig struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

// SearchProviderRequest provider 层请求模型。
type SearchProviderRequest struct {
	Query          string
	Freshness      string
	Count          int
	NeedSummary    bool
	NeedContent    bool
	Language       string
	AllowedDomains []string
	BlockedDomains []string
	TraceID        string
}

// SearchProviderResult provider 层中间结果。
type SearchProviderResult struct {
	ID          string
	Title       string
	URL         string
	DisplayURL  string
	Snippet     string
	Summary     string
	SiteName    string
	SiteIcon    string
	PublishedAt string
	Score       float64
	Language    string
	Meta        map[string]any
}

// SearchProviderResponse provider 层标准化响应。
type SearchProviderResponse struct {
	Query    string
	Provider string
	Summary  string
	Total    int
	Results  []SearchProviderResult
	Meta     map[string]any
}

// WebSearchClient 搜索 provider 客户端。
type WebSearchClient interface {
	Search(ctx context.Context, cfg SearchProviderConfig, req *SearchProviderRequest) (*SearchProviderResponse, error)
}
