// internal/model/dto/request/search.go
package request

// SearchRequest 智能搜索请求
type SearchRequest struct {
	Query string `json:"query" binding:"required,max=500"`
}

// URLImportRequest URL 直接导入请求
type URLImportRequest struct {
	URL string `json:"url" binding:"required,url"`
}

// SearchImportItem 搜索结果导入项
type SearchImportItem struct {
	Title string `json:"title"`                      // 网页标题
	URL   string `json:"url" binding:"required,url"` // 网页URL
}

// SearchImportRequest 搜索结果批量导入请求
type SearchImportRequest struct {
	URLs  []string           `json:"urls,omitempty"`  // 兼容旧接口：纯URL列表
	Items []SearchImportItem `json:"items,omitempty"` // 新接口：带标题的导入项
}

// Validate 验证请求（URLs 和 Items 至少有一个）
func (r *SearchImportRequest) Validate() bool {
	return len(r.URLs) > 0 || len(r.Items) > 0
}
