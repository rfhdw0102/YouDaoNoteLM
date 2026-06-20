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

// EinoRetrieverConfig eino Milvus2 Retriever 配置
type EinoRetrieverConfig struct {
	Address  string             // Milvus 地址
	TopK     int                // 返回结果数量
	Embedder embedding.Embedder // Embedder 用于查询向量化
}

// EinoRetrieverFactory Retriever 工厂，缓存用户专属的 Retriever 实例
type EinoRetrieverFactory struct {
	clientConfig *milvusclient.ClientConfig
	retrievers   map[uint]*milvus2.Retriever
}

// NewEinoRetrieverFactory 创建 Retriever 工厂
func NewEinoRetrieverFactory(address string) *EinoRetrieverFactory {
	return &EinoRetrieverFactory{
		clientConfig: &milvusclient.ClientConfig{
			Address: address,
		},
		retrievers: make(map[uint]*milvus2.Retriever),
	}
}

// GetRetriever 获取用户的 Retriever 实例（懒加载 + 缓存）
func (f *EinoRetrieverFactory) GetRetriever(ctx context.Context, userID uint, embedder embedding.Embedder, topK int) (*milvus2.Retriever, error) {
	if r, ok := f.retrievers[userID]; ok {
		logger.Debug("[EinoRetrieverFactory] 使用缓存的 Retriever", zap.Uint("userID", userID))
		return r, nil
	}

	collectionName := UserCollectionName(userID)
	logger.Info("[EinoRetrieverFactory] 创建新的 Retriever",
		zap.Uint("userID", userID),
		zap.String("collection", collectionName),
		zap.Int("topK", topK),
	)

	// 创建 Hybrid 搜索模式：dense + sparse 融合
	hybridMode := search_mode.NewHybrid(
		milvusclient.NewRRFReranker().WithK(60), // RRF reranker，K=60
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
		logger.Error("[EinoRetrieverFactory] 创建 Retriever 失败",
			zap.Uint("userID", userID),
			zap.String("collection", collectionName),
			zap.Error(err),
		)
		return nil, fmt.Errorf("创建 Retriever 失败: %w", err)
	}

	logger.Info("[EinoRetrieverFactory] Retriever 创建成功", zap.Uint("userID", userID))
	f.retrievers[userID] = r
	return r, nil
}

// EinoRetrieverWrapper 封装 eino Retriever，适配现有的 RAGRetriever 接口
type EinoRetrieverWrapper struct {
	factory          *EinoRetrieverFactory
	parentBlockRepo  repository.ParentBlockRepository
	sourceRepo       repository.SourceRepository
	embedderProvider RetrieverEmbedderProvider
	reranker         *EinoReranker
	topK             int
}

// NewEinoRetrieverWrapper 创建 EinoRetrieverWrapper
func NewEinoRetrieverWrapper(
	ctx context.Context,
	address string,
	parentBlockRepo repository.ParentBlockRepository,
	sourceRepo repository.SourceRepository,
	embedderProvider RetrieverEmbedderProvider,
	topK int,
) (*EinoRetrieverWrapper, error) {
	logger.Info("[EinoRetrieverWrapper] 初始化开始",
		zap.String("milvusAddress", address),
		zap.Int("defaultTopK", topK),
	)

	if topK <= 0 {
		topK = defaultTopK
		logger.Debug("[EinoRetrieverWrapper] 使用默认 topK", zap.Int("topK", topK))
	}

	// 创建 Score Reranker
	logger.Debug("[EinoRetrieverWrapper] 创建 Reranker")
	reranker, err := NewEinoReranker(ctx, nil)
	if err != nil {
		logger.Error("[EinoRetrieverWrapper] 创建 Reranker 失败", zap.Error(err))
		return nil, fmt.Errorf("创建 reranker 失败: %w", err)
	}
	logger.Debug("[EinoRetrieverWrapper] Reranker 创建成功")

	logger.Info("[EinoRetrieverWrapper] 初始化完成")
	return &EinoRetrieverWrapper{
		factory:          NewEinoRetrieverFactory(address),
		parentBlockRepo:  parentBlockRepo,
		sourceRepo:       sourceRepo,
		embedderProvider: embedderProvider,
		reranker:         reranker,
		topK:             topK,
	}, nil
}

// Retrieve 执行 RAG 检索：Hybrid 搜索 -> Score Rerank -> Parent Recovery
func (r *EinoRetrieverWrapper) Retrieve(ctx context.Context, req *RetrieveRequest) ([]*RetrieveResult, error) {
	logger.Info("[EinoRetriever] ====== 开始检索 ======",
		zap.String("query", req.Query),
		zap.Uint("userID", req.UserID),
		zap.Uints("sourceIDs", req.SourceIDs),
		zap.Int("topK", req.TopK),
	)

	topK := r.topK
	if req.TopK > 0 {
		topK = req.TopK
	}

	// 1. 获取 Embedder
	logger.Debug("[EinoRetriever] 步骤1: 获取 Embedder", zap.Uint("userID", req.UserID))
	embedder, err := r.embedderProvider(ctx, req.UserID)
	if err != nil {
		logger.Error("[EinoRetriever] 获取 Embedder 失败",
			zap.Uint("userID", req.UserID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取 embedder 失败: %w", err)
	}
	logger.Debug("[EinoRetriever] Embedder 获取成功")

	// 2. 获取 Retriever
	logger.Debug("[EinoRetriever] 步骤2: 获取 Retriever",
		zap.Uint("userID", req.UserID),
		zap.Int("candidateTopK", topK*2),
	)
	retriever, err := r.factory.GetRetriever(ctx, req.UserID, embedder, topK*2) // 获取更多候选用于 rerank
	if err != nil {
		logger.Error("[EinoRetriever] 获取 Retriever 失败",
			zap.Uint("userID", req.UserID),
			zap.Error(err),
		)
		return nil, err
	}
	logger.Debug("[EinoRetriever] Retriever 获取成功")

	// 3. 构建查询选项（sourceIDs 过滤）
	filter := ""
	if len(req.SourceIDs) > 0 {
		ids := make([]string, len(req.SourceIDs))
		for i, id := range req.SourceIDs {
			ids[i] = fmt.Sprintf("%d", id)
		}
		filter = fmt.Sprintf("metadata[\"source_id\"] in [%s]", strings.Join(ids, ","))
	}
	logger.Debug("[EinoRetriever] 步骤3: 构建过滤条件",
		zap.String("filter", filter),
	)

	// 4. 执行检索
	logger.Info("[EinoRetriever] 步骤4: 执行 Milvus 检索",
		zap.String("query", req.Query),
		zap.String("filter", filter),
	)
	var docs []*schema.Document
	if filter != "" {
		docs, err = retriever.Retrieve(ctx, req.Query, milvus2.WithFilter(filter))
	} else {
		docs, err = retriever.Retrieve(ctx, req.Query)
	}
	if err != nil {
		logger.Error("[EinoRetriever] Milvus 检索失败",
			zap.String("query", req.Query),
			zap.String("filter", filter),
			zap.Error(err),
		)
		return nil, fmt.Errorf("检索失败: %w", err)
	}
	logger.Info("[EinoRetriever] Milvus 检索成功，返回文档数", zap.Int("docCount", len(docs)))

	// 5. 转换为 RetrieveResult
	candidates := make([]*RetrieveResult, 0, len(docs))
	for _, doc := range docs {
		result := &RetrieveResult{
			Content: doc.Content,
			Score:   float32(doc.Score()),
		}

		// 从 metadata 提取字段
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
	logger.Debug("[EinoRetriever] 步骤5: 转换候选结果", zap.Int("candidateCount", len(candidates)))

	// 6. Score Rerank（利用 LLM 首因效应和近因效应）
	if r.reranker != nil {
		logger.Debug("[EinoRetriever] 步骤6: 执行 Rerank")
		candidates, err = r.reranker.RerankWithScore(ctx, candidates)
		if err != nil {
			// rerank 失败时降级使用原始结果
			logger.Warn("[EinoRetriever] Rerank 失败，降级使用原始结果", zap.Error(err))
		}
	}

	// 7. TopK
	if len(candidates) > topK {
		candidates = candidates[:topK]
	}
	logger.Debug("[EinoRetriever] 步骤7: TopK 截断", zap.Int("finalCount", len(candidates)))

	// 8. Parent Recovery
	logger.Debug("[EinoRetriever] 步骤8: Parent Recovery")
	results, err := r.parentRecovery(ctx, candidates)
	if err != nil {
		logger.Warn("[EinoRetriever] Parent Recovery 失败，降级返回原始结果", zap.Error(err))
		return candidates, nil // 降级返回原始结果
	}

	logger.Info("[EinoRetriever] ====== 检索完成 ======",
		zap.Int("resultCount", len(results)),
	)
	return results, nil
}

// parentRecovery 为候选结果填充 ParentBlock 的完整内容、标题、章节路径以及资料来源名称
func (r *EinoRetrieverWrapper) parentRecovery(ctx context.Context, candidates []*RetrieveResult) ([]*RetrieveResult, error) {
	logger.Debug("[EinoRetriever] parentRecovery 开始", zap.Int("candidateCount", len(candidates)))

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
		logger.Debug("[EinoRetriever] parentRecovery: 无 ParentBlock 需要恢复")
		return candidates, nil
	}

	logger.Debug("[EinoRetriever] parentRecovery: 查询 ParentBlock",
		zap.Int("parentBlockCount", len(parentIDs)),
		zap.Uints("parentBlockIDs", parentIDs),
	)
	parentBlocks, err := r.parentBlockRepo.FindByIDs(parentIDs)
	if err != nil {
		logger.Error("[EinoRetriever] parentRecovery: 查询 ParentBlock 失败",
			zap.Uints("parentBlockIDs", parentIDs),
			zap.Error(err),
		)
		return nil, fmt.Errorf("查询 ParentBlock 失败: %w", err)
	}
	parentMap := make(map[uint]*entity.ParentBlock)
	for _, pb := range parentBlocks {
		parentMap[pb.ID] = pb
	}
	logger.Debug("[EinoRetriever] parentRecovery: ParentBlock 查询成功",
		zap.Int("foundCount", len(parentBlocks)),
	)

	sourceSeen := make(map[uint]bool)
	var sourceIDs []uint
	for _, c := range candidates {
		if c.SourceID > 0 && !sourceSeen[c.SourceID] {
			sourceSeen[c.SourceID] = true
			sourceIDs = append(sourceIDs, c.SourceID)
		}
	}

	logger.Debug("[EinoRetriever] parentRecovery: 查询 Source",
		zap.Int("sourceCount", len(sourceIDs)),
		zap.Uints("sourceIDs", sourceIDs),
	)
	sourceNames := make(map[uint]string)
	for _, sid := range sourceIDs {
		source, err := r.sourceRepo.FindByID(sid)
		if err == nil && source != nil {
			sourceNames[sid] = source.Name
		} else if err != nil {
			logger.Warn("[EinoRetriever] parentRecovery: 查询 Source 失败",
				zap.Uint("sourceID", sid),
				zap.Error(err),
			)
		}
	}
	logger.Debug("[EinoRetriever] parentRecovery: Source 查询成功",
		zap.Int("foundCount", len(sourceNames)),
	)

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

	logger.Debug("[EinoRetriever] parentRecovery 完成")
	return candidates, nil
}
