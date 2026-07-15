package rag

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/indexer/milvus2"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// userCollectionName 返回用户专属的 Milvus Collection 名称
func userCollectionName(userID uint) string {
	return fmt.Sprintf("user_%d_chunks", userID)
}

// einoIndexerFactory Indexer 工厂，缓存用户专属的 Indexer 实例
type einoIndexerFactory struct {
	clientConfig *milvusclient.ClientConfig
	client       *milvusclient.Client // 共享的 Milvus 客户端，用于删除等操作
	indexers     map[uint]*milvus2.Indexer
}

// newEinoIndexerFactory 创建 Indexer 工厂
func newEinoIndexerFactory(ctx context.Context, address string) (*einoIndexerFactory, error) {
	clientConfig := &milvusclient.ClientConfig{
		Address: address,
	}
	client, err := milvusclient.New(ctx, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 Milvus 客户端失败: %w", err)
	}
	return &einoIndexerFactory{
		clientConfig: clientConfig,
		client:       client,
		indexers:     make(map[uint]*milvus2.Indexer),
	}, nil
}

// getIndexer 获取用户的 Indexer 实例（懒加载 + 缓存）
// embedder 配置到 Indexer 中，Store 时自动完成向量化
func (f *einoIndexerFactory) getIndexer(ctx context.Context, userID uint, embedder embedding.Embedder, vectorDim int64) (*milvus2.Indexer, error) {
	if idx, ok := f.indexers[userID]; ok {
		return idx, nil
	}

	collectionName := userCollectionName(userID)
	idx, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
		ClientConfig: f.clientConfig,
		Collection:   collectionName,
		Description:  fmt.Sprintf("User %d knowledge base", userID),
		Embedding:    embedder, // Indexer 内置 Embedding，Store 时自动向量化
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
		EnableDynamicSchema: true,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 Indexer 失败: %w", err)
	}

	f.indexers[userID] = idx
	return idx, nil
}

// EinoIndexerWrapper 封装 eino Indexer，适配现有的 IngestionService
type EinoIndexerWrapper struct {
	factory *einoIndexerFactory
}

// NewEinoIndexerWrapper 创建 EinoIndexerWrapper
func NewEinoIndexerWrapper(ctx context.Context, address string) (*EinoIndexerWrapper, error) {
	factory, err := newEinoIndexerFactory(ctx, address)
	if err != nil {
		return nil, err
	}
	return &EinoIndexerWrapper{factory: factory}, nil
}

// Store 将文档写入用户专属 Collection
// Indexer 内置 Embedding，自动完成向量化；sparse vector 由 Milvus BM25 Function 自动生成
// 分批处理
func (w *EinoIndexerWrapper) Store(ctx context.Context, userID uint, docs []*schema.Document, embedder embedding.Embedder, vectorDim int) error {
	if len(docs) == 0 {
		return fmt.Errorf("没有可写入的文档")
	}

	idx, err := w.factory.getIndexer(ctx, userID, embedder, int64(vectorDim))
	if err != nil {
		return err
	}

	// 生成文档 ID、使用 sourceID + parentBlockID + childIndex 确保全局唯一
	for _, doc := range docs {
		if doc.ID == "" {
			sourceID, _ := doc.MetaData["source_id"].(uint)
			parentBlockID, _ := doc.MetaData["parent_block_id"].(uint)
			childIndex, _ := doc.MetaData["child_index"].(int)
			doc.ID = fmt.Sprintf("%d_%d_%d_%d", userID, sourceID, parentBlockID, childIndex)
		}
	}

	// 分批写入，每批最多 20 个文档
	batchSize := 20
	for i := 0; i < len(docs); i += batchSize {
		end := i + batchSize
		if end > len(docs) {
			end = len(docs)
		}
		batch := docs[i:end]

		if _, err := idx.Store(ctx, batch); err != nil {
			return fmt.Errorf("写入 Milvus 失败 (batch %d-%d): %w", i, end, err)
		}
	}
	return nil
}

// EnsureCollection 确保用户专属 Collection 存在
func (w *EinoIndexerWrapper) EnsureCollection(ctx context.Context, userID uint, embedder embedding.Embedder, vectorDim int) error {
	_, err := w.factory.getIndexer(ctx, userID, embedder, int64(vectorDim))
	return err
}

// DeleteBySourceID 删除指定 source 在用户专属 Collection 中的所有文档
func (w *EinoIndexerWrapper) DeleteBySourceID(ctx context.Context, userID uint, sourceID uint) error {
	collName := userCollectionName(userID)
	expr := fmt.Sprintf(`metadata["source_id"] == %d`, sourceID)
	_, err := w.factory.client.Delete(ctx, milvusclient.NewDeleteOption(collName).WithExpr(expr))
	if err != nil {
		return fmt.Errorf("删除文档失败: %w", err)
	}
	return nil
}

// DropUserCollection 删除用户的整个 Milvus Collection
func (w *EinoIndexerWrapper) DropUserCollection(ctx context.Context, userID uint) error {
	collName := userCollectionName(userID)
	has, err := w.factory.client.HasCollection(ctx, milvusclient.NewHasCollectionOption(collName))
	if err != nil {
		return fmt.Errorf("检查 Collection 失败: %w", err)
	}
	if !has {
		return nil
	}
	err = w.factory.client.DropCollection(ctx, milvusclient.NewDropCollectionOption(collName))
	if err != nil {
		return fmt.Errorf("删除 Collection 失败: %w", err)
	}
	delete(w.factory.indexers, userID)
	return nil
}
