package rag

import (
	"context"
	"fmt"
	"strings"

	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/pkg/logger"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// embedderProvider 根据 userID 获取对应的 Embedder
// 返回: embedder, vectorDim, error
type embedderProvider func(ctx context.Context, userID uint) (embedding.Embedder, int, error)

// IngestionService 入库服务接口
type IngestionService interface {
	// Ingest 批量入库源内容
	Ingest(ctx context.Context, sourceIDs []uint) error
	// IngestSingle 单个源入库
	IngestSingle(ctx context.Context, sourceID uint) error
	// DeleteSource 删除源的向量数据
	DeleteSource(ctx context.Context, userID uint, sourceID uint) error
	// DropUserCollection 删除用户的整个 Milvus Collection（不可逆操作）
	DropUserCollection(ctx context.Context, userID uint) error
}

type ingestionService struct {
	sourceRepo       repository.SourceRepository
	parentRepo       repository.ParentBlockRepository
	embedderProvider embedderProvider
	einoIndexer      *EinoIndexerWrapper
	maxRetries       int // 默认 3
}

// NewIngestionService 创建入库服务
func NewIngestionService(
	sourceRepo repository.SourceRepository,
	parentRepo repository.ParentBlockRepository,
	provider embedderProvider,
	einoIndexer *EinoIndexerWrapper,
) IngestionService {
	return &ingestionService{
		sourceRepo:       sourceRepo,
		parentRepo:       parentRepo,
		embedderProvider: provider,
		einoIndexer:      einoIndexer,
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
	childTransformer := newChildTransformer(400)
	childDocs, err := childTransformer.Transform(ctx, parentDocs)
	if err != nil {
		s.updateFailedStatus(sourceID, err.Error())
		return err
	}

	// 6. 语义增强
	enhancer := newSemanticTransformer()
	enhancedDocs, err := enhancer.Transform(ctx, childDocs)
	if err != nil {
		s.updateFailedStatus(sourceID, err.Error())
		return err
	}

	// 7. 检查是否有可入库的文档
	if len(enhancedDocs) == 0 {
		s.updateFailedStatus(sourceID, "解析后无有效内容，跳过入库")
		return fmt.Errorf("源 %d 解析后无有效内容", sourceID)
	}

	enhancedDocs = wrapDocuments(enhancedDocs, sourceID)

	// 8. 先写入 MySQL ParentBlock，拿到真实的自增 ID
	blocks := toParentBlocks(parentDocs, sourceID)
	if err := s.retry(func() error {
		return s.parentRepo.BatchCreate(blocks)
	}); err != nil {
		s.updateFailedStatus(sourceID, "写入 MySQL 失败: "+err.Error())
		return err
	}
	logger.Info("MySQL ParentBlock 写入成功",
		zap.Uint("source_id", sourceID),
		zap.Int("block_count", len(blocks)),
	)

	// 8.5 构建 parent_index → MySQL ID 的映射，更新子块 metadata
	parentIndexToID := make(map[int]uint, len(blocks))
	for _, b := range blocks {
		parentIndexToID[b.ChunkIndex] = b.ID
	}
	for _, doc := range enhancedDocs {
		if pidx, ok := doc.MetaData["parent_index"].(int); ok {
			if realID, exists := parentIndexToID[pidx]; exists {
				doc.MetaData["parent_block_id"] = realID
			}
		}
	}

	// 9. 获取 Embedder
	logger.Info("准备向量化入库",
		zap.Uint("source_id", sourceID),
		zap.Int("chunk_count", len(enhancedDocs)),
	)
	embedder, vectorDim, err := s.embedderProvider(ctx, source.UserID)
	if err != nil {
		errMsg := "获取 Embedder 失败: " + err.Error()
		logger.Error(errMsg, zap.Uint("source_id", sourceID), zap.Uint("user_id", source.UserID))
		s.updateFailedStatus(sourceID, errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	// 10. 确保用户的 Milvus Collection 存在
	if err := s.einoIndexer.EnsureCollection(ctx, source.UserID, embedder, vectorDim); err != nil {
		errMsg := "确保 Milvus Collection 失败: " + err.Error()
		logger.Error(errMsg, zap.Uint("source_id", sourceID), zap.Uint("user_id", source.UserID))
		s.updateFailedStatus(sourceID, errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	// 11. 写入 Milvus（Indexer 内置 Embedding，自动完成向量化）
	logger.Info("写入 Milvus", zap.Uint("source_id", sourceID), zap.Uint("user_id", source.UserID), zap.Int("doc_count", len(enhancedDocs)))
	if err := s.retry(func() error {
		return s.einoIndexer.Store(ctx, source.UserID, enhancedDocs, embedder, vectorDim)
	}); err != nil {
		errMsg := "写入 Milvus 失败: " + err.Error()
		logger.Error(errMsg, zap.Uint("source_id", sourceID))
		s.updateFailedStatus(sourceID, errMsg)
		return fmt.Errorf("%s", errMsg)
	}
	logger.Info("Milvus 写入成功", zap.Uint("source_id", sourceID))

	// 12. 更新状态为就绪
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

// DeleteSource 删除源的向量数据和父块数据
func (s *ingestionService) DeleteSource(ctx context.Context, userID uint, sourceID uint) error {
	logger.Info("删除源数据",
		zap.Uint("user_id", userID),
		zap.Uint("source_id", sourceID),
	)
	if err := s.einoIndexer.DeleteBySourceID(ctx, userID, sourceID); err != nil {
		logger.Error("删除 Milvus 数据失败",
			zap.Uint("user_id", userID),
			zap.Uint("source_id", sourceID),
			zap.Error(err),
		)
		return fmt.Errorf("删除向量数据失败: %w", err)
	}
	// 删除 MySQL 中的 parent_blocks（source 软删除不会触发 CASCADE）
	if err := s.parentRepo.DeleteBySourceID(sourceID); err != nil {
		logger.Error("删除 parent_blocks 失败",
			zap.Uint("source_id", sourceID),
			zap.Error(err),
		)
		return fmt.Errorf("删除 parent_blocks 失败: %w", err)
	}
	logger.Info("删除源数据成功",
		zap.Uint("user_id", userID),
		zap.Uint("source_id", sourceID),
	)
	return nil
}

// DropUserCollection 删除用户的整个 Milvus Collection
// 注意：此操作不可逆，会永久删除该用户的所有向量数据
func (s *ingestionService) DropUserCollection(ctx context.Context, userID uint) error {
	logger.Info("删除用户 Milvus Collection",
		zap.Uint("user_id", userID),
	)
	if err := s.einoIndexer.DropUserCollection(ctx, userID); err != nil {
		logger.Error("删除用户 Milvus Collection 失败",
			zap.Uint("user_id", userID),
			zap.Error(err),
		)
		return fmt.Errorf("删除用户 Collection 失败: %w", err)
	}

	// 重置用户所有资料的向量化状态，使其可以重新导入
	if err := s.sourceRepo.ResetVectorizedByUserID(userID); err != nil {
		logger.Error("重置用户资料向量化状态失败",
			zap.Uint("user_id", userID),
			zap.Error(err),
		)
		// 不返回错误，因为 collection 已经删除成功
	}

	logger.Info("删除用户 Milvus Collection 成功",
		zap.Uint("user_id", userID),
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

// wrapDocuments 为 ChildChunk Document 添加 source_id 元数据
func wrapDocuments(docs []*schema.Document, sourceID uint) []*schema.Document {
	for _, doc := range docs {
		doc.MetaData["source_id"] = sourceID
	}
	return docs
}
