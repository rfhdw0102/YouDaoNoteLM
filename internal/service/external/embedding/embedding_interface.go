package embedding

// EmbeddingService 向量化服务接口
// TODO: 后续模块接入时实现具体 provider
type EmbeddingService interface {
	Embed(text string) ([]float64, error)
	EmbedBatch(texts []string) ([][]float64, error)
	Name() string
}
