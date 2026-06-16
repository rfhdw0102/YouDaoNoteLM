package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"YoudaoNoteLm/pkg/logger"

	"github.com/cloudwego/eino/schema"
	milvusclient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"go.uber.org/zap"
)

const (
	VectorDim = 2048 // 默认维度，实际由 embedder 决定
)

// UserCollectionName 返回用户专属的 Milvus Collection 名称
func UserCollectionName(userID uint) string {
	return fmt.Sprintf("user_%d_chunks", userID)
}

// MilvusIndexerConfig Milvus 连接配置
type MilvusIndexerConfig struct {
	Address string
}

// MilvusWriter 封装 Milvus 写入操作
type MilvusWriter struct {
	client milvusclient.Client
}

var newMilvusClient = milvusclient.NewClient

// NewMilvusWriter 创建 Milvus 写入器
func NewMilvusWriter(ctx context.Context, cfg MilvusIndexerConfig) (*MilvusWriter, error) {
	start := time.Now()
	logger.Info("Milvus connection started", zap.String("address", cfg.Address))

	cli, err := newMilvusClient(ctx, milvusclient.Config{
		Address: cfg.Address,
	})
	if err != nil {
		logger.Error("Milvus connection failed",
			zap.String("address", cfg.Address),
			zap.Duration("elapsed", time.Since(start)),
			zap.Error(err),
		)
		return nil, fmt.Errorf("创建 Milvus 客户端失败: %w", err)
	}
	logger.Info("Milvus connection succeeded",
		zap.String("address", cfg.Address),
		zap.Duration("elapsed", time.Since(start)),
	)
	return &MilvusWriter{client: cli}, nil
}

// EnsureCollection 确保用户专属 Collection 存在，不存在则创建
func (w *MilvusWriter) EnsureCollection(ctx context.Context, userID uint) error {
	collName := UserCollectionName(userID)

	has, err := w.client.HasCollection(ctx, collName)
	if err != nil {
		return fmt.Errorf("检查 Collection 失败: %w", err)
	}
	if has {
		return nil
	}

	schema := &entity.Schema{
		CollectionName: collName,
		AutoID:         true,
		Fields: []*entity.Field{
			{
				Name:       "id",
				DataType:   entity.FieldTypeInt64,
				AutoID:     true,
				PrimaryKey: true,
			},
			{
				Name:     "content",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					"max_length": "32768",
				},
			},
			{
				Name:     "vector",
				DataType: entity.FieldTypeFloatVector,
				TypeParams: map[string]string{
					"dim": fmt.Sprintf("%d", VectorDim),
				},
			},
			{
				Name:     "parent_block_id",
				DataType: entity.FieldTypeInt64,
			},
			{
				Name:     "source_id",
				DataType: entity.FieldTypeInt64,
			},
			{
				Name:     "chunk_type",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					"max_length": "32",
				},
			},
			{
				Name:     "metadata",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					"max_length": "2048",
				},
			},
			{
				Name:     "sparse_vector",
				DataType: entity.FieldTypeSparseVector,
			},
		},
	}

	if err := w.client.CreateCollection(ctx, schema, 2); err != nil {
		return fmt.Errorf("创建 Collection 失败: %w", err)
	}

	// 创建 HNSW 索引
	idxParam, err := entity.NewIndexHNSW(entity.COSINE, 16, 200)
	if err != nil {
		return fmt.Errorf("创建 HNSW 索引参数失败: %w", err)
	}
	if err := w.client.CreateIndex(ctx, collName, "vector", idxParam, false); err != nil {
		return fmt.Errorf("创建索引失败: %w", err)
	}

	// 创建 BM25 索引（全文检索）
	// 注意: SDK v2.4.2 没有 NewIndexBM25Sparse，使用 NewIndexSparseInverted + IP 代替
	bm25IdxParam, err := entity.NewIndexSparseInverted(entity.IP, 0.0)
	if err != nil {
		return fmt.Errorf("创建 BM25 索引参数失败: %w", err)
	}
	if err := w.client.CreateIndex(ctx, collName, "sparse_vector", bm25IdxParam, false); err != nil {
		return fmt.Errorf("创建 BM25 索引失败: %w", err)
	}

	// 加载 Collection
	if err := w.client.LoadCollection(ctx, collName, false); err != nil {
		return fmt.Errorf("加载 Collection 失败: %w", err)
	}

	return nil
}

// Store 将文档和对应的向量写入用户专属 Collection
// Deprecated: 使用 StoreWithSparse 代替，新 collection 包含 sparse_vector 字段
func (w *MilvusWriter) Store(ctx context.Context, userID uint, docs []*schema.Document, vectors [][]float32) error {
	if len(docs) == 0 || len(docs) != len(vectors) {
		return fmt.Errorf("文档和向量数量不匹配: docs=%d, vectors=%d", len(docs), len(vectors))
	}

	n := len(docs)
	contents := make([]string, n)
	vectorsData := make([][]float32, n)
	parentBlockIDs := make([]int64, n)
	sourceIDs := make([]int64, n)
	chunkTypes := make([]string, n)
	metadatas := make([]string, n)

	const maxContentLen = 32768
	for i, doc := range docs {
		c := doc.Content
		if len(c) > maxContentLen {
			c = c[:maxContentLen]
		}
		contents[i] = c
		vectorsData[i] = vectors[i]

		if pid, ok := doc.MetaData["parent_index"].(int); ok {
			parentBlockIDs[i] = int64(pid)
		}
		if sid, ok := doc.MetaData["source_id"].(uint); ok {
			sourceIDs[i] = int64(sid)
		}
		if ct, ok := doc.MetaData["chunk_type"].(string); ok {
			chunkTypes[i] = ct
		}
		metaJSON, err := json.Marshal(doc.MetaData)
		if err != nil {
			return fmt.Errorf("序列化文档元数据失败: %w", err)
		}
		metadatas[i] = string(metaJSON)
	}

	// 按 batch 写入
	batchSize := 100
	for start := 0; start < n; start += batchSize {
		end := start + batchSize
		if end > n {
			end = n
		}

		columns := []entity.Column{
			entity.NewColumnVarChar("content", contents[start:end]),
			entity.NewColumnFloatVector("vector", VectorDim, vectorsData[start:end]),
			entity.NewColumnInt64("parent_block_id", parentBlockIDs[start:end]),
			entity.NewColumnInt64("source_id", sourceIDs[start:end]),
			entity.NewColumnVarChar("chunk_type", chunkTypes[start:end]),
			entity.NewColumnVarChar("metadata", metadatas[start:end]),
		}

		if _, err := w.client.Insert(ctx, UserCollectionName(userID), "", columns...); err != nil {
			return fmt.Errorf("写入 Milvus 失败 (batch %d-%d): %w", start, end, err)
		}
	}

	return nil
}

// mapToSparseEmbedding 将 map[int32]float32 转换为 Milvus SDK 的 SparseEmbedding
func mapToSparseEmbedding(m map[int32]float32) (entity.SparseEmbedding, error) {
	positions := make([]uint32, 0, len(m))
	values := make([]float32, 0, len(m))
	for k, v := range m {
		positions = append(positions, uint32(k))
		values = append(values, v)
	}
	return entity.NewSliceSparseEmbedding(positions, values)
}

// mapsToSparseEmbeddings 批量转换
func mapsToSparseEmbeddings(maps []map[int32]float32) ([]entity.SparseEmbedding, error) {
	result := make([]entity.SparseEmbedding, len(maps))
	for i, m := range maps {
		se, err := mapToSparseEmbedding(m)
		if err != nil {
			return nil, fmt.Errorf("转换 sparse embedding #%d 失败: %w", i, err)
		}
		result[i] = se
	}
	return result, nil
}

// StoreWithSparse 将文档、dense vector 和 sparse vector 写入用户专属 Collection
func (w *MilvusWriter) StoreWithSparse(ctx context.Context, userID uint, docs []*schema.Document, denseVectors [][]float32, sparseVectors []map[int32]float32) error {
	if len(docs) == 0 {
		return fmt.Errorf("没有可写入的文档（内容可能为空或解析结果为空）")
	}
	if len(docs) != len(denseVectors) || len(docs) != len(sparseVectors) {
		return fmt.Errorf("文档、dense向量和sparse向量数量不匹配: docs=%d, dense=%d, sparse=%d", len(docs), len(denseVectors), len(sparseVectors))
	}

	n := len(docs)
	contents := make([]string, n)
	denseData := make([][]float32, n)
	parentBlockIDs := make([]int64, n)
	sourceIDs := make([]int64, n)
	chunkTypes := make([]string, n)
	metadatas := make([]string, n)

	const maxContentLen = 32768
	for i, doc := range docs {
		c := doc.Content
		if len(c) > maxContentLen {
			c = c[:maxContentLen]
		}
		contents[i] = c
		denseData[i] = denseVectors[i]

		if pid, ok := doc.MetaData["parent_index"].(int); ok {
			parentBlockIDs[i] = int64(pid)
		}
		if sid, ok := doc.MetaData["source_id"].(uint); ok {
			sourceIDs[i] = int64(sid)
		}
		if ct, ok := doc.MetaData["chunk_type"].(string); ok {
			chunkTypes[i] = ct
		}
		metaJSON, err := json.Marshal(doc.MetaData)
		if err != nil {
			return fmt.Errorf("序列化文档元数据失败: %w", err)
		}
		metadatas[i] = string(metaJSON)
	}

	// 转换 sparse vectors 为 Milvus SDK 格式
	allSparseEmbeddings, err := mapsToSparseEmbeddings(sparseVectors)
	if err != nil {
		return fmt.Errorf("转换 sparse vectors 失败: %w", err)
	}

	// 按 batch 写入
	batchSize := 100
	for start := 0; start < n; start += batchSize {
		end := start + batchSize
		if end > n {
			end = n
		}

		sparseColumn := entity.NewColumnSparseVectors("sparse_vector", allSparseEmbeddings[start:end])

		columns := []entity.Column{
			entity.NewColumnVarChar("content", contents[start:end]),
			entity.NewColumnFloatVector("vector", VectorDim, denseData[start:end]),
			entity.NewColumnInt64("parent_block_id", parentBlockIDs[start:end]),
			entity.NewColumnInt64("source_id", sourceIDs[start:end]),
			entity.NewColumnVarChar("chunk_type", chunkTypes[start:end]),
			entity.NewColumnVarChar("metadata", metadatas[start:end]),
			sparseColumn,
		}

		if _, err := w.client.Insert(ctx, UserCollectionName(userID), "", columns...); err != nil {
			return fmt.Errorf("写入 Milvus 失败 (batch %d-%d): %w", start, end, err)
		}
	}

	return nil
}

// GenerateSparseVector 将文本转换为 BM25 sparse vector
func GenerateSparseVector(text string) map[int32]float32 {
	words := segmentText(text)
	freq := make(map[string]int)
	for _, w := range words {
		freq[w]++
	}

	sv := make(map[int32]float32)
	for word, count := range freq {
		idx := hashToIndex(word)
		sv[idx] = float32(count) / float32(count+1)
	}
	return sv
}

// segmentText 简单分词：中文按单字+bigram，英文按空格分词
func segmentText(text string) []string {
	var words []string
	runes := []rune(text)
	i := 0
	for i < len(runes) {
		r := runes[i]
		if r >= 0x4e00 && r <= 0x9fff {
			words = append(words, string(r))
			if i+1 < len(runes) && runes[i+1] >= 0x4e00 && runes[i+1] <= 0x9fff {
				words = append(words, string(runes[i:i+2]))
			}
			i++
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			j := i
			for j < len(runes) && ((runes[j] >= 'a' && runes[j] <= 'z') || (runes[j] >= 'A' && runes[j] <= 'Z') || (runes[j] >= '0' && runes[j] <= '9')) {
				j++
			}
			words = append(words, strings.ToLower(string(runes[i:j])))
			i = j
		} else {
			i++
		}
	}
	return words
}

// hashToIndex 将词 hash 为 int32 索引 (FNV-1a 变体)
func hashToIndex(word string) int32 {
	var h uint32 = 2166136261
	for _, b := range []byte(word) {
		h ^= uint32(b)
		h *= 16777619
	}
	return int32(h % 100000)
}

// DeleteBySourceID 删除指定 source 在用户专属 Collection 中的所有 ChildChunk
func (w *MilvusWriter) DeleteBySourceID(ctx context.Context, userID uint, sourceID uint) error {
	expr := fmt.Sprintf(`source_id == %d`, sourceID)
	return w.client.Delete(ctx, UserCollectionName(userID), "", expr)
}

// MilvusSearchResult Milvus 检索结果
type MilvusSearchResult struct {
	ID            int64
	Content       string
	ParentBlockID int64
	SourceID      int64
	ChunkType     string
	Metadata      string
	Score         float32
}

// MilvusSearcher Milvus 检索接口
type MilvusSearcher interface {
	SemanticSearch(ctx context.Context, userID uint, queryVector []float32, sourceIDs []uint, topK int) ([]MilvusSearchResult, error)
	KeywordSearch(ctx context.Context, userID uint, queryText string, sourceIDs []uint, topK int) ([]MilvusSearchResult, error)
}

// SemanticSearch 基于 dense vector 的语义检索
func (w *MilvusWriter) SemanticSearch(ctx context.Context, userID uint, queryVector []float32, sourceIDs []uint, topK int) ([]MilvusSearchResult, error) {
	collName := UserCollectionName(userID)

	filter := ""
	if len(sourceIDs) > 0 {
		ids := make([]string, len(sourceIDs))
		for i, id := range sourceIDs {
			ids[i] = fmt.Sprintf("%d", id)
		}
		filter = fmt.Sprintf("source_id in [%s]", strings.Join(ids, ","))
	}

	searchParams, err := entity.NewIndexHNSWSearchParam(200)
	if err != nil {
		return nil, fmt.Errorf("创建 HNSW 搜索参数失败: %w", err)
	}

	outputFields := []string{"content", "parent_block_id", "source_id", "chunk_type", "metadata"}

	results, err := w.client.Search(ctx, collName, []string{}, filter, outputFields,
		[]entity.Vector{entity.FloatVector(queryVector)},
		"vector", entity.COSINE, topK, searchParams,
	)
	if err != nil {
		return nil, fmt.Errorf("语义检索失败: %w", err)
	}

	return parseSearchResults(results), nil
}

// KeywordSearch 基于 sparse vector 的关键词检索
func (w *MilvusWriter) KeywordSearch(ctx context.Context, userID uint, queryText string, sourceIDs []uint, topK int) ([]MilvusSearchResult, error) {
	collName := UserCollectionName(userID)

	filter := ""
	if len(sourceIDs) > 0 {
		ids := make([]string, len(sourceIDs))
		for i, id := range sourceIDs {
			ids[i] = fmt.Sprintf("%d", id)
		}
		filter = fmt.Sprintf("source_id in [%s]", strings.Join(ids, ","))
	}

	searchParams, err := entity.NewIndexSparseInvertedSearchParam(0.0)
	if err != nil {
		return nil, fmt.Errorf("创建 sparse 搜索参数失败: %w", err)
	}

	outputFields := []string{"content", "parent_block_id", "source_id", "chunk_type", "metadata"}

	// 将 queryText 转换为 sparse vector
	querySparseMap := GenerateSparseVector(queryText)
	querySparseVec, err := mapToSparseEmbedding(querySparseMap)
	if err != nil {
		return nil, fmt.Errorf("转换查询 sparse vector 失败: %w", err)
	}

	results, err := w.client.Search(ctx, collName, []string{}, filter, outputFields,
		[]entity.Vector{querySparseVec},
		"sparse_vector", entity.IP, topK, searchParams,
	)
	if err != nil {
		return nil, fmt.Errorf("关键词检索失败: %w", err)
	}

	return parseSearchResults(results), nil
}

// parseSearchResults 将 Milvus SearchResult 转换为 MilvusSearchResult 切片
func parseSearchResults(results []milvusclient.SearchResult) []MilvusSearchResult {
	if len(results) == 0 {
		return nil
	}

	var parsed []MilvusSearchResult
	for _, result := range results {
		if result.Err != nil {
			continue
		}
		for i := 0; i < result.ResultCount; i++ {
			item := MilvusSearchResult{
				Score: result.Scores[i],
			}

			// 从 ID 列获取主键
			if result.IDs != nil {
				if val, err := result.IDs.Get(i); err == nil {
					if id, ok := val.(int64); ok {
						item.ID = id
					}
				}
			}

			// 从 Fields 获取各字段
			if col := result.Fields.GetColumn("content"); col != nil {
				if val, err := col.Get(i); err == nil {
					if s, ok := val.(string); ok {
						item.Content = s
					}
				}
			}
			if col := result.Fields.GetColumn("parent_block_id"); col != nil {
				if val, err := col.Get(i); err == nil {
					if id, ok := val.(int64); ok {
						item.ParentBlockID = id
					}
				}
			}
			if col := result.Fields.GetColumn("source_id"); col != nil {
				if val, err := col.Get(i); err == nil {
					if id, ok := val.(int64); ok {
						item.SourceID = id
					}
				}
			}
			if col := result.Fields.GetColumn("chunk_type"); col != nil {
				if val, err := col.Get(i); err == nil {
					if s, ok := val.(string); ok {
						item.ChunkType = s
					}
				}
			}
			if col := result.Fields.GetColumn("metadata"); col != nil {
				if val, err := col.Get(i); err == nil {
					if s, ok := val.(string); ok {
						item.Metadata = s
					}
				}
			}

			parsed = append(parsed, item)
		}
	}
	return parsed
}

// Close 关闭 Milvus 客户端
func (w *MilvusWriter) Close() {
	w.client.Close()
}

// WrapDocuments 为 ChildChunk Document 添加 source_id 元数据
func WrapDocuments(docs []*schema.Document, sourceID uint) []*schema.Document {
	for _, doc := range docs {
		doc.MetaData["source_id"] = sourceID
	}
	return docs
}
