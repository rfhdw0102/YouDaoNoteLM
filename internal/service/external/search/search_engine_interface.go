// internal/service/external/search_engine_interface.go
package search

// SearchResultItem 搜索结果项
type SearchResultItem struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// SearchEngine 搜索引擎接口
type SearchEngine interface {
	Search(query string, limit int) ([]SearchResultItem, error)
	Name() string
}
