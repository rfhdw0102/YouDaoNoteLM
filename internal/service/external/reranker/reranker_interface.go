package reranker

// RerankerService Reranker 精排服务接口
// 用于对检索结果进行精排，提高相关性
type RerankerService interface {
	// Rerank 对文档进行精排
	// query: 查询文本
	// documents: 待排序的文档列表
	// topN: 返回的结果数量，0 表示返回全部
	Rerank(query string, documents []string, topN int) ([]RerankResult, error)
	// Name 服务名称
	Name() string
}

// RerankResult 精排结果
type RerankResult struct {
	Index int     // 原始文档索引
	Score float64 // 相关度分数 (0-1)
	Text  string  // 文档内容
}
