package rag

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/indexer/milvus2"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// UserCollectionName 返回用户专属的 Milvus Collection 名称
func UserCollectionName(userID uint) string {
	return fmt.Sprintf("user_%d_chunks", userID)
}

// EinoIndexerConfig eino Milvus2 Indexer 配置
type EinoIndexerConfig struct {
	Address   string // Milvus 地址
	VectorDim int64  // 向量维度
}

// EinoIndexerFactory Indexer 工厂，缓存用户专属的 Indexer 实例
type EinoIndexerFactory struct {
	clientConfig *milvusclient.ClientConfig
	client       *milvusclient.Client // 共享的 Milvus 客户端，用于删除等操作
	indexers     map[uint]*milvus2.Indexer
}

// NewEinoIndexerFactory 创建 Indexer 工厂
func NewEinoIndexerFactory(ctx context.Context, address string) (*EinoIndexerFactory, error) {
	clientConfig := &milvusclient.ClientConfig{
		Address: address,
	}

	// 创建共享的 Milvus 客户端
	client, err := milvusclient.New(ctx, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 Milvus 客户端失败: %w", err)
	}

	return &EinoIndexerFactory{
		clientConfig: clientConfig,
		client:       client,
		indexers:     make(map[uint]*milvus2.Indexer),
	}, nil
}

// GetIndexer 获取用户的 Indexer 实例（懒加载 + 缓存）
func (f *EinoIndexerFactory) GetIndexer(ctx context.Context, userID uint, vectorDim int64) (*milvus2.Indexer, error) {
	if idx, ok := f.indexers[userID]; ok {
		return idx, nil
	}

	collectionName := UserCollectionName(userID)
	idx, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
		ClientConfig: f.clientConfig,
		Collection:   collectionName,
		Description:  fmt.Sprintf("User %d knowledge base", userID),
		Vector: &milvus2.VectorConfig{
			Dimension:    vectorDim,
			MetricType:   milvus2.COSINE,
			IndexBuilder: milvus2.NewHNSWIndexBuilder().WithM(16).WithEfConstruction(200),
			VectorField:  "vector",
		},
		Sparse: &milvus2.SparseVectorConfig{
			IndexBuilder: milvus2.NewSparseInvertedIndexBuilder(),
			VectorField:  "sparse_vector",
			MetricType:   milvus2.BM25,
			Method:       milvus2.SparseMethodAuto, // 由 Milvus 服务端 BM25 Function 自动生成 sparse vector
		},
		FieldParams: map[string]map[string]string{
			"content": {
				"enable_analyzer": "true",
				"analyzer_params": `{"type": "chinese"}`, // 中文分词器，BM25 Function 要求
			},
		},
		EnableDynamicSchema: true, // 允许动态字段
	})
	if err != nil {
		return nil, fmt.Errorf("创建 Indexer 失败: %w", err)
	}

	f.indexers[userID] = idx
	return idx, nil
}

// EinoIndexerWrapper 封装 eino Indexer，适配现有的 IngestionService
type EinoIndexerWrapper struct {
	factory *EinoIndexerFactory
}

// NewEinoIndexerWrapper 创建 EinoIndexerWrapper
func NewEinoIndexerWrapper(ctx context.Context, address string) (*EinoIndexerWrapper, error) {
	factory, err := NewEinoIndexerFactory(ctx, address)
	if err != nil {
		return nil, err
	}
	return &EinoIndexerWrapper{
		factory: factory,
	}, nil
}

// StoreWithSparse 将文档和 dense vector 写入用户专属 Collection
// sparse vector 由 Milvus 服务端 BM25 Function 自动生成，客户端无需提供
func (w *EinoIndexerWrapper) StoreWithSparse(ctx context.Context, userID uint, docs []*schema.Document, denseVectors [][]float32, vectorDim int) error {
	if len(docs) == 0 {
		return fmt.Errorf("没有可写入的文档")
	}
	if len(docs) != len(denseVectors) {
		return fmt.Errorf("文档和dense向量数量不匹配")
	}

	// 获取用户的 Indexer
	idx, err := w.factory.GetIndexer(ctx, userID, int64(vectorDim))
	if err != nil {
		return err
	}

	// 为文档设置 dense vector（sparse vector 由 Milvus BM25 Function 自动生成）
	for i, doc := range docs {
		vec := make([]float64, len(denseVectors[i]))
		for j, v := range denseVectors[i] {
			vec[j] = float64(v)
		}
		doc.WithDenseVector(vec)

		// 生成文档 ID（如果未设置）
		if doc.ID == "" {
			doc.ID = fmt.Sprintf("%d_%d", userID, i)
		}
	}

	// 使用 Indexer 存储
	_, err = idx.Store(ctx, docs)
	if err != nil {
		return fmt.Errorf("写入 Milvus 失败: %w", err)
	}

	return nil
}

// EnsureCollection 确保用户专属 Collection 存在
// 注意：eino-ext Indexer 会自动创建 Collection，此方法主要为了兼容现有接口
func (w *EinoIndexerWrapper) EnsureCollection(ctx context.Context, userID uint, vectorDim int) error {
	_, err := w.factory.GetIndexer(ctx, userID, int64(vectorDim))
	return err
}

// DeleteBySourceID 删除指定 source 在用户专属 Collection 中的所有文档
func (w *EinoIndexerWrapper) DeleteBySourceID(ctx context.Context, userID uint, sourceID uint) error {
	collName := UserCollectionName(userID)
	expr := fmt.Sprintf(`metadata["source_id"] == %d`, sourceID)

	// 使用 Milvus 客户端直接删除
	_, err := w.factory.client.Delete(ctx, milvusclient.NewDeleteOption(collName).WithExpr(expr))
	if err != nil {
		return fmt.Errorf("删除文档失败: %w", err)
	}
	return nil
}

// DropUserCollection 删除用户的整个 Milvus Collection
func (w *EinoIndexerWrapper) DropUserCollection(ctx context.Context, userID uint) error {
	collName := UserCollectionName(userID)

	// 检查集合是否存在
	has, err := w.factory.client.HasCollection(ctx, milvusclient.NewHasCollectionOption(collName))
	if err != nil {
		return fmt.Errorf("检查 Collection 失败: %w", err)
	}
	if !has {
		return nil // 集合不存在，无需删除
	}

	// 删除集合
	err = w.factory.client.DropCollection(ctx, milvusclient.NewDropCollectionOption(collName))
	if err != nil {
		return fmt.Errorf("删除 Collection 失败: %w", err)
	}

	// 从缓存中移除
	delete(w.factory.indexers, userID)
	return nil
}
