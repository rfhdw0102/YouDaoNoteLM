package rag

import (
	"context"
	"fmt"
	"strings"

	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/pkg/logger"

	"github.com/cloudwego/eino/components/embedding"
	"go.uber.org/zap"
)

// EmbedderProvider 根据 userID 获取对应的 Embedder
type EmbedderProvider func(ctx context.Context, userID uint) (embedding.Embedder, error)

// IngestionService 入库服务接口
type IngestionService interface {
	// Ingest 批量入库源内容
	Ingest(ctx context.Context, sourceIDs []uint) error
	// IngestSingle 单个源入库
	IngestSingle(ctx context.Context, sourceID uint) error
	// DeleteSource 删除源的向量数据
	DeleteSource(ctx context.Context, userID uint, sourceID uint) error
}

type ingestionService struct {
	sourceRepo       repository.SourceRepository
	parentRepo       repository.ParentBlockRepository
	embedderProvider EmbedderProvider
	milvusWriter     *MilvusWriter
	maxRetries       int // 默认 3
}

// NewIngestionService 创建入库服务
func NewIngestionService(
	sourceRepo repository.SourceRepository,
	parentRepo repository.ParentBlockRepository,
	embedderProvider EmbedderProvider,
	milvusWriter *MilvusWriter,
) IngestionService {
	return &ingestionService{
		sourceRepo:       sourceRepo,
		parentRepo:       parentRepo,
		embedderProvider: embedderProvider,
		milvusWriter:     milvusWriter,
		maxRetries:       3,
	}
}

// IngestSingle 单个源入库
func (s *ingestionService) IngestSingle(ctx context.Context, sourceID uint) error {
	// 1. 查询源
	source, err := s.sourceRepo.FindByID(sourceID)
	if err != nil {
		return fmt.Errorf("查询源失败: %w", err)
	}
	if source.MarkdownContent == "" {
		logger.Info("源内容为空，跳过入库", zap.Uint("source_id", sourceID))
		return nil
	}
	logger.Info("开始入库流程",
		zap.Uint("source_id", sourceID),
		zap.Uint("user_id", source.UserID),
		zap.Int("content_len", len(source.MarkdownContent)),
	)

	// 2. 更新状态为处理中
	if err := s.sourceRepo.UpdateStatus(sourceID, "processing", ""); err != nil {
		logger.Warn("更新源状态为处理中失败", zap.Uint("source_id", sourceID), zap.Error(err))
	}

	// 3. AST 解析
	p := NewMarkdownParser()
	docs, err := p.Parse(ctx, strings.NewReader(source.MarkdownContent))
	if err != nil {
		s.updateFailedStatus(sourceID, err.Error())
		return err
	}

	// 4. 构建 ParentBlock
	parentTransformer := NewParentTransformer(1000)
	parentDocs, err := parentTransformer.Transform(ctx, docs)
	if err != nil {
		s.updateFailedStatus(sourceID, err.Error())
		return err
	}

	// 5. 分割 ChildChunk
	childTransformer := NewChildTransformer(400)
	childDocs, err := childTransformer.Transform(ctx, parentDocs)
	if err != nil {
		s.updateFailedStatus(sourceID, err.Error())
		return err
	}

	// 6. 语义增强
	enhancer := NewSemanticTransformer()
	enhancedDocs, err := enhancer.Transform(ctx, childDocs)
	if err != nil {
		s.updateFailedStatus(sourceID, err.Error())
		return err
	}

	// 7. 获取用户的 Embedder 并向量化
	enhancedDocs = WrapDocuments(enhancedDocs, sourceID)
	logger.Info("准备向量化",
		zap.Uint("source_id", sourceID),
		zap.Int("chunk_count", len(enhancedDocs)),
	)
	embedder, err := s.embedderProvider(ctx, source.UserID)
	if err != nil {
		errMsg := "获取 Embedder 失败: " + err.Error()
		logger.Error(errMsg, zap.Uint("source_id", sourceID), zap.Uint("user_id", source.UserID))
		s.updateFailedStatus(sourceID, errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	// 提取所有文本用于批量 embedding
	texts := make([]string, len(enhancedDocs))
	for i, doc := range enhancedDocs {
		texts[i] = doc.Content
	}

	// 分批调用 Embedding API（豆包限制每次最多 256 条）
	const embedBatchSize = 256
	vectors := make([][]float32, len(texts))
	logger.Info("调用 Embedding API", zap.Uint("source_id", sourceID), zap.Int("text_count", len(texts)))

	for start := 0; start < len(texts); start += embedBatchSize {
		end := start + embedBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batchTexts := texts[start:end]

		var batchVectors [][]float64
		if err := s.retry(func() error {
			var err error
			batchVectors, err = embedder.EmbedStrings(ctx, batchTexts)
			if err != nil {
				logger.Warn("Embedding 批次调用失败，重试中",
					zap.Uint("source_id", sourceID),
					zap.Int("batch_start", start),
					zap.Int("batch_size", len(batchTexts)),
					zap.Error(err),
				)
				return err
			}
			return nil
		}); err != nil {
			errMsg := "Embedding 失败: " + err.Error()
			logger.Error(errMsg, zap.Uint("source_id", sourceID))
			s.updateFailedStatus(sourceID, errMsg)
			return fmt.Errorf("%s", errMsg)
		}

		// float64 → float32
		for i, v := range batchVectors {
			vectors[start+i] = make([]float32, len(v))
			for j, f := range v {
				vectors[start+i][j] = float32(f)
			}
		}

		logger.Info("Embedding 批次完成",
			zap.Uint("source_id", sourceID),
			zap.Int("batch_start", start),
			zap.Int("batch_end", end),
		)
	}
	logger.Info("Embedding 全部完成", zap.Uint("source_id", sourceID), zap.Int("vector_count", len(vectors)))

	// 8.5 生成 sparse vector
	sparseVectors := make([]map[int32]float32, len(enhancedDocs))
	for i, doc := range enhancedDocs {
		sparseVectors[i] = GenerateSparseVector(doc.Content)
	}
	logger.Info("Sparse vector 生成完成", zap.Uint("source_id", sourceID), zap.Int("count", len(sparseVectors)))

	// 9. 确保用户的 Milvus Collection 存在
	if err := s.milvusWriter.EnsureCollection(ctx, source.UserID); err != nil {
		errMsg := "确保 Milvus Collection 失败: " + err.Error()
		logger.Error(errMsg, zap.Uint("source_id", sourceID), zap.Uint("user_id", source.UserID))
		s.updateFailedStatus(sourceID, errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	// 9. 写入 Milvus
	logger.Info("写入 Milvus", zap.Uint("source_id", sourceID), zap.Uint("user_id", source.UserID), zap.Int("doc_count", len(enhancedDocs)))
	if err := s.retry(func() error {
		return s.milvusWriter.StoreWithSparse(ctx, source.UserID, enhancedDocs, vectors, sparseVectors)
	}); err != nil {
		errMsg := "写入 Milvus 失败: " + err.Error()
		logger.Error(errMsg, zap.Uint("source_id", sourceID))
		s.updateFailedStatus(sourceID, errMsg)
		return fmt.Errorf("%s", errMsg)
	}
	logger.Info("Milvus 写入成功", zap.Uint("source_id", sourceID))

	// 10. 写入 MySQL ParentBlock
	blocks := ToParentBlocks(parentDocs, sourceID)
	if err := s.retry(func() error {
		return s.parentRepo.BatchCreate(blocks)
	}); err != nil {
		s.updateFailedStatus(sourceID, "写入 MySQL 失败: "+err.Error())
		return err
	}

	// 11. 更新状态为就绪
	if err := s.sourceRepo.UpdateStatus(sourceID, "ready", ""); err != nil {
		logger.Warn("更新源状态为就绪失败", zap.Uint("source_id", sourceID), zap.Error(err))
	}
	if err := s.sourceRepo.SetVectorized(sourceID); err != nil {
		logger.Warn("标记源已向量化失败", zap.Uint("source_id", sourceID), zap.Error(err))
	}
	return nil
}

// updateFailedStatus 更新源状态为失败
func (s *ingestionService) updateFailedStatus(sourceID uint, errMsg string) {
	if err := s.sourceRepo.UpdateStatus(sourceID, "failed", errMsg); err != nil {
		logger.Warn("更新源状态为失败失败", zap.Uint("source_id", sourceID), zap.Error(err))
	}
}

// Ingest 批量入库
func (s *ingestionService) Ingest(ctx context.Context, sourceIDs []uint) error {
	var lastErr error
	for _, sourceID := range sourceIDs {
		if err := s.IngestSingle(ctx, sourceID); err != nil {
			lastErr = err
			logger.Warn("入库失败", zap.Uint("source_id", sourceID), zap.Error(err))
			continue
		}
	}
	return lastErr
}

// DeleteSource 删除源的向量数据
func (s *ingestionService) DeleteSource(ctx context.Context, userID uint, sourceID uint) error {
	logger.Info("删除源向量数据",
		zap.Uint("user_id", userID),
		zap.Uint("source_id", sourceID),
	)
	if err := s.milvusWriter.DeleteBySourceID(ctx, userID, sourceID); err != nil {
		logger.Error("删除 Milvus 数据失败",
			zap.Uint("user_id", userID),
			zap.Uint("source_id", sourceID),
			zap.Error(err),
		)
		return fmt.Errorf("删除向量数据失败: %w", err)
	}
	logger.Info("删除源向量数据成功",
		zap.Uint("user_id", userID),
		zap.Uint("source_id", sourceID),
	)
	return nil
}

// retry 重试逻辑
func (s *ingestionService) retry(fn func() error) error {
	var err error
	for i := 0; i <= s.maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}
	}
	return err
}
