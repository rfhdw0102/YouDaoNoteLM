package rag

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/cloudwego/eino/components/embedding"
	"go.uber.org/zap"

	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/pkg/logger"
)

// RAGRetriever RAG 检索接口
type RAGRetriever interface {
	Retrieve(ctx context.Context, req *RetrieveRequest) ([]*RetrieveResult, error)
}

// RetrieveRequest 检索请求
type RetrieveRequest struct {
	Query       string    // 改写后的查询文本
	UserID      uint      // 用户 ID（定位 Milvus collection）
	SourceIDs   []uint    // 限定的资料来源范围
	TopK        int       // 最终返回数量，默认 5
	QueryVector []float32 // 预计算的查询向量（可选）
}

// RetrieveResult 检索结果
type RetrieveResult struct {
	Content       string  // chunk 内容
	SourceID      uint    // 资料来源 ID
	SourceName    string  // 资料来源名称
	ParentBlockID int64   // 父块 ID
	ParentContent string  // 父块完整内容
	Heading       string  // 父块标题
	ChapterPath   string  // 章节路径
	Score         float32 // 最终相关度分数
	ChunkType     string  // chunk 类型
	Metadata      string  // 元数据 JSON
}

const (
	defaultTopK        = 5
	semanticCandidateK = 20
	keywordCandidateK  = 20
	rrfK               = 60
)

// RetrieverEmbedderProvider 根据 userID 获取 Embedder
type RetrieverEmbedderProvider func(ctx context.Context, userID uint) (embedding.Embedder, error)

type ragRetriever struct {
	milvusSearcher   MilvusSearcher
	parentBlockRepo  repository.ParentBlockRepository
	sourceRepo       repository.SourceRepository
	embedderProvider RetrieverEmbedderProvider
	topK             int
}

// NewRAGRetriever 创建 RAGRetriever
func NewRAGRetriever(
	milvusSearcher MilvusSearcher,
	parentBlockRepo repository.ParentBlockRepository,
	sourceRepo repository.SourceRepository,
	embedderProvider RetrieverEmbedderProvider,
	topK int,
) RAGRetriever {
	if topK <= 0 {
		topK = defaultTopK
	}
	return &ragRetriever{
		milvusSearcher:   milvusSearcher,
		parentBlockRepo:  parentBlockRepo,
		sourceRepo:       sourceRepo,
		embedderProvider: embedderProvider,
		topK:             topK,
	}
}

// Retrieve 执行 RAG 检索：语义 + 关键词双路召回 -> RRF 融合 -> Rerank -> Parent Recovery
func (r *ragRetriever) Retrieve(ctx context.Context, req *RetrieveRequest) ([]*RetrieveResult, error) {
	topK := r.topK
	if req.TopK > 0 {
		topK = req.TopK
	}

	// 1. 获取查询向量
	queryVector := req.QueryVector
	if len(queryVector) == 0 {
		embedder, err := r.embedderProvider(ctx, req.UserID)
		if err != nil {
			return nil, fmt.Errorf("获取 embedder 失败: %w", err)
		}
		vectors, err := embedder.EmbedStrings(ctx, []string{req.Query})
		if err != nil {
			return nil, fmt.Errorf("查询向量化失败: %w", err)
		}
		if len(vectors) > 0 {
			queryVector = make([]float32, len(vectors[0]))
			for i, v := range vectors[0] {
				queryVector[i] = float32(v)
			}
		}
	}

	// 2. 并行语义检索 + 关键词检索
	var (
		semanticResults []MilvusSearchResult
		keywordResults  []MilvusSearchResult
		semanticErr     error
		keywordErr      error
		wg              sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		semanticResults, semanticErr = r.milvusSearcher.SemanticSearch(ctx, req.UserID, queryVector, req.SourceIDs, semanticCandidateK)
	}()
	go func() {
		defer wg.Done()
		keywordResults, keywordErr = r.milvusSearcher.KeywordSearch(ctx, req.UserID, req.Query, req.SourceIDs, keywordCandidateK)
	}()
	wg.Wait()

	if semanticErr != nil {
		logger.Warn("语义检索失败，降级为仅关键词检索", zap.Error(semanticErr))
	}
	if keywordErr != nil {
		logger.Warn("关键词检索失败，降级为仅语义检索", zap.Error(keywordErr))
	}

	// 3. RRF 融合
	fused := r.fuse(semanticResults, keywordResults)
	if len(fused) == 0 {
		return nil, nil
	}

	// 4. 候选
	candidateK := topK * 4
	if candidateK > len(fused) {
		candidateK = len(fused)
	}
	candidates := fused[:candidateK]

	// 5. TopK
	if len(candidates) > topK {
		candidates = candidates[:topK]
	}

	// 6. Parent Recovery
	results, err := r.parentRecovery(ctx, candidates)
	if err != nil {
		logger.Warn("Parent Recovery 失败，返回原始结果", zap.Error(err))
		return candidates, nil
	}

	return results, nil
}

// fuse 使用 RRF (Reciprocal Rank Fusion) 融合语义检索和关键词检索结果
func (r *ragRetriever) fuse(semanticResults, keywordResults []MilvusSearchResult) []*RetrieveResult {
	type resultKey struct {
		sourceID      int64
		parentBlockID int64
	}

	scoreMap := make(map[resultKey]*RetrieveResult)
	rankMap := make(map[resultKey][]float64)

	for rank, item := range semanticResults {
		key := resultKey{sourceID: item.SourceID, parentBlockID: item.ParentBlockID}
		if _, exists := scoreMap[key]; !exists {
			scoreMap[key] = &RetrieveResult{
				Content:       item.Content,
				SourceID:      uint(item.SourceID),
				ParentBlockID: item.ParentBlockID,
				Score:         item.Score,
				ChunkType:     item.ChunkType,
				Metadata:      item.Metadata,
			}
		}
		rankMap[key] = append(rankMap[key], 1.0/float64(rrfK+rank+1))
	}

	for rank, item := range keywordResults {
		key := resultKey{sourceID: item.SourceID, parentBlockID: item.ParentBlockID}
		if _, exists := scoreMap[key]; !exists {
			scoreMap[key] = &RetrieveResult{
				Content:       item.Content,
				SourceID:      uint(item.SourceID),
				ParentBlockID: item.ParentBlockID,
				Score:         item.Score,
				ChunkType:     item.ChunkType,
				Metadata:      item.Metadata,
			}
		}
		rankMap[key] = append(rankMap[key], 1.0/float64(rrfK+rank+1))
	}

	for key, scores := range rankMap {
		var total float64
		for _, s := range scores {
			total += s
		}
		scoreMap[key].Score = float32(total)
	}

	results := make([]*RetrieveResult, 0, len(scoreMap))
	for _, res := range scoreMap {
		results = append(results, res)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// parentRecovery 为候选结果填充 ParentBlock 的完整内容、标题、章节路径以及资料来源名称
func (r *ragRetriever) parentRecovery(ctx context.Context, candidates []*RetrieveResult) ([]*RetrieveResult, error) {
	seen := make(map[uint]bool)
	var parentIDs []uint
	for _, c := range candidates {
		pid := uint(c.ParentBlockID)
		if !seen[pid] {
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
		if !sourceSeen[c.SourceID] {
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
