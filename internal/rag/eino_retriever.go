package rag

import (
	"context"
	"fmt"
	"strings"

	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/pkg/logger"

	milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
	"github.com/cloudwego/eino-ext/components/retriever/milvus2/search_mode"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"go.uber.org/zap"
)

// retrieverEmbedderProvider 根据 userID 获取用于检索的 Embedder
type retrieverEmbedderProvider func(ctx context.Context, userID uint) (embedding.Embedder, error)

// einoRetrieverFactory Retriever 工厂，缓存用户专属的 Retriever 实例
type einoRetrieverFactory struct {
	clientConfig *milvusclient.ClientConfig
	retrievers   map[uint]*milvus2.Retriever
}

// newEinoRetrieverFactory 创建 Retriever 工厂
func newEinoRetrieverFactory(address string) *einoRetrieverFactory {
	return &einoRetrieverFactory{
		clientConfig: &milvusclient.ClientConfig{
			Address: address,
		},
		retrievers: make(map[uint]*milvus2.Retriever),
	}
}

// getRetriever 获取用户的 Retriever 实例（懒加载 + 缓存）
func (f *einoRetrieverFactory) getRetriever(ctx context.Context, userID uint, embedder embedding.Embedder, topK int) (*milvus2.Retriever, error) {
	if r, ok := f.retrievers[userID]; ok {
		return r, nil
	}

	collectionName := userCollectionName(userID)
	logger.Info("[EinoRetriever] 创建 Retriever",
		zap.Uint("userID", userID),
		zap.String("collection", collectionName),
	)

	// 创建 Hybrid 搜索模式：dense + sparse(BM25) 融合
	hybridMode := search_mode.NewHybrid(
		milvusclient.NewRRFReranker().WithK(60),
		&search_mode.SubRequest{
			VectorField: "vector",
			VectorType:  milvus2.DenseVector,
			TopK:        20,
			MetricType:  milvus2.COSINE,
			SearchParams: map[string]string{
				"ef": "200",
			},
		},
		&search_mode.SubRequest{
			VectorField: "sparse_vector",
			VectorType:  milvus2.SparseVector,
			TopK:        20,
			MetricType:  milvus2.BM25,
		},
	)

	r, err := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{
		ClientConfig: f.clientConfig,
		Collection:   collectionName,
		TopK:         topK,
		SearchMode:   hybridMode,
		Embedding:    embedder,
		OutputFields: []string{"content", "metadata"},
	})
	if err != nil {
		logger.Error("[EinoRetriever] 创建 Retriever 失败",
			zap.Uint("userID", userID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("创建 Retriever 失败: %w", err)
	}

	f.retrievers[userID] = r
	return r, nil
}

// EinoRetrieverWrapper 封装 eino Retriever，适配现有的 RAGRetriever 接口
type EinoRetrieverWrapper struct {
	factory          *einoRetrieverFactory
	parentBlockRepo  repository.ParentBlockRepository
	sourceRepo       repository.SourceRepository
	embedderProvider retrieverEmbedderProvider
	reranker         *einoReranker
	topK             int
}

// NewEinoRetrieverWrapper 创建 EinoRetrieverWrapper
func NewEinoRetrieverWrapper(
	ctx context.Context,
	address string,
	parentBlockRepo repository.ParentBlockRepository,
	sourceRepo repository.SourceRepository,
	embedderProvider retrieverEmbedderProvider,
	topK int,
) (*EinoRetrieverWrapper, error) {
	if topK <= 0 {
		topK = defaultTopK
	}

	reranker, err := newEinoReranker(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 reranker 失败: %w", err)
	}

	return &EinoRetrieverWrapper{
		factory:          newEinoRetrieverFactory(address),
		parentBlockRepo:  parentBlockRepo,
		sourceRepo:       sourceRepo,
		embedderProvider: embedderProvider,
		reranker:         reranker,
		topK:             topK,
	}, nil
}

// Retrieve 执行 RAG 检索：Hybrid 搜索 -> Score Rerank -> Parent Recovery
func (r *EinoRetrieverWrapper) Retrieve(ctx context.Context, req *RetrieveRequest) ([]*RetrieveResult, error) {
	topK := r.topK
	if req.TopK > 0 {
		topK = req.TopK
	}

	// 1. 获取 Embedder
	embedder, err := r.embedderProvider(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("获取 embedder 失败: %w", err)
	}

	// 2. 获取 Retriever，候选数为 topK*2 用于 rerank
	retriever, err := r.factory.getRetriever(ctx, req.UserID, embedder, topK*2)
	if err != nil {
		return nil, err
	}

	// 3. 构建 sourceIDs 过滤条件
	filter := ""
	if len(req.SourceIDs) > 0 {
		ids := make([]string, len(req.SourceIDs))
		for i, id := range req.SourceIDs {
			ids[i] = fmt.Sprintf("%d", id)
		}
		filter = fmt.Sprintf("metadata[\"source_id\"] in [%s]", strings.Join(ids, ","))
	}

	// 4. 执行 Milvus Hybrid 检索
	var docs []*schema.Document
	if filter != "" {
		docs, err = retriever.Retrieve(ctx, req.Query, milvus2.WithFilter(filter))
	} else {
		docs, err = retriever.Retrieve(ctx, req.Query)
	}
	if err != nil {
		logger.Error("[EinoRetriever] 检索失败",
			zap.String("query", req.Query),
			zap.String("filter", filter),
			zap.Error(err),
		)
		return nil, fmt.Errorf("检索失败: %w", err)
	}
	logger.Info("[EinoRetriever] 检索完成",
		zap.String("query", req.Query),
		zap.Int("docCount", len(docs)),
	)

	// 5. 转换为 RetrieveResult
	candidates := make([]*RetrieveResult, 0, len(docs))
	for _, doc := range docs {
		result := &RetrieveResult{
			Content: doc.Content,
			Score:   float32(doc.Score()),
		}
		if sourceID, ok := doc.MetaData["source_id"].(float64); ok {
			result.SourceID = uint(sourceID)
		}
		if parentBlockID, ok := doc.MetaData["parent_block_id"].(float64); ok {
			result.ParentBlockID = int64(parentBlockID)
		}
		if chunkType, ok := doc.MetaData["chunk_type"].(string); ok {
			result.ChunkType = chunkType
		}
		if chapterPath, ok := doc.MetaData["chapter_path"].(string); ok {
			result.ChapterPath = chapterPath
		}
		if heading, ok := doc.MetaData["heading"].(string); ok {
			result.Heading = heading
		}
		candidates = append(candidates, result)
	}

	// 6. Score Rerank（利用 LLM 首因效应和近因效应）
	if r.reranker != nil {
		candidates, err = r.reranker.rerankWithScore(ctx, candidates)
		if err != nil {
			logger.Warn("[EinoRetriever] Rerank 失败，降级使用原始结果", zap.Error(err))
		}
	}

	// 7. TopK 截断
	if len(candidates) > topK {
		candidates = candidates[:topK]
	}

	// 8. Parent Recovery：填充父块完整内容和来源名称
	results, err := r.parentRecovery(ctx, candidates)
	if err != nil {
		logger.Warn("[EinoRetriever] Parent Recovery 失败，降级返回原始结果", zap.Error(err))
		return candidates, nil
	}

	return results, nil
}

// parentRecovery 为候选结果填充 ParentBlock 的完整内容、标题、章节路径以及资料来源名称
func (r *EinoRetrieverWrapper) parentRecovery(ctx context.Context, candidates []*RetrieveResult) ([]*RetrieveResult, error) {
	seen := make(map[uint]bool)
	var parentIDs []uint
	for _, c := range candidates {
		pid := uint(c.ParentBlockID)
		if pid > 0 && !seen[pid] {
			seen[pid] = true
			parentIDs = append(parentIDs, pid)
		}
	}
	if len(parentIDs) == 0 {
		return candidates, nil
	}

	parentBlocks, err := r.parentBlockRepo.FindByIDs(parentIDs)
	if err != nil {
		return nil, fmt.Errorf("查询 ParentBlock 失败: %w", err)
	}
	parentMap := make(map[uint]*entity.ParentBlock)
	for _, pb := range parentBlocks {
		parentMap[pb.ID] = pb
	}

	sourceSeen := make(map[uint]bool)
	var sourceIDs []uint
	for _, c := range candidates {
		if c.SourceID > 0 && !sourceSeen[c.SourceID] {
			sourceSeen[c.SourceID] = true
			sourceIDs = append(sourceIDs, c.SourceID)
		}
	}
	sourceNames := make(map[uint]string)
	for _, sid := range sourceIDs {
		source, err := r.sourceRepo.FindByID(sid)
		if err == nil && source != nil {
			sourceNames[sid] = source.Name
		}
	}

	for _, c := range candidates {
		pid := uint(c.ParentBlockID)
		if pb, ok := parentMap[pid]; ok {
			c.ParentContent = pb.Content
			c.Heading = pb.Heading
			c.ChapterPath = pb.ChapterPath
		}
		if name, ok := sourceNames[c.SourceID]; ok {
			c.SourceName = name
		}
	}
	return candidates, nil
}
