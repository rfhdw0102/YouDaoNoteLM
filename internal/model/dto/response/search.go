// internal/model/dto/response/search.go
package response

// SearchResultItem 搜索结果项（带 Agent 评分）
type SearchResultItem struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`  // Agent 评分（1-10）
	Reason  string  `json:"reason"` // Agent 推荐理由
}

// SearchResponse 智能搜索响应
type SearchResponse struct {
	Results      []SearchResultItem `json:"results"`
	Summary      string             `json:"summary"`
	SearchRounds int                `json:"search_rounds"`
}
