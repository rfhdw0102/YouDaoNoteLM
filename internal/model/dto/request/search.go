package request

// SearchPreviewRequest 联网搜索预览请求。
type SearchPreviewRequest struct {
	Query          string   `json:"query" binding:"required"`
	Freshness      string   `json:"freshness"`
	Count          int      `json:"count"`
	NeedSummary    bool     `json:"need_summary"`
	NeedContent    bool     `json:"need_content"`
	Language       string   `json:"language"`
	AllowedDomains []string `json:"allowed_domains"`
	BlockedDomains []string `json:"blocked_domains"`
	TraceID        string   `json:"trace_id"`
	AllowDegrade   bool     `json:"allow_degrade"`
}

// SearchImportRequest 导入选中的搜索结果请求。
type SearchImportRequest struct {
	URLs []string `json:"urls" binding:"required"`
}
