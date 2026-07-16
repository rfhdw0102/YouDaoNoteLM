package rag

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/document/transformer/reranker/score"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

// einoRerankerConfig Score Reranker 配置
type einoRerankerConfig struct {
	// ScoreFieldKey 指定从 metadata 中获取 score 的 key，为空则使用 schema.Document.Score()
	ScoreFieldKey string
}

// einoReranker 基于 eino-ext score 包的 Reranker
type einoReranker struct {
	transformer document.Transformer
	config      *einoRerankerConfig
}

// newEinoReranker 创建 Score Reranker
func newEinoReranker(ctx context.Context, config *einoRerankerConfig) (*einoReranker, error) {
	if config == nil {
		config = &einoRerankerConfig{}
	}

	rerankerConfig := &score.Config{}
	if config.ScoreFieldKey != "" {
		rerankerConfig.ScoreFieldKey = &config.ScoreFieldKey
	}

	transformer, err := score.NewReranker(ctx, rerankerConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 score reranker 失败: %w", err)
	}

	return &einoReranker{
		transformer: transformer,
		config:      config,
	}, nil
}

// rerank 对文档进行重排序，将高分文档放在数组的开头和结尾
func (r *einoReranker) rerank(ctx context.Context, docs []*schema.Document) ([]*schema.Document, error) {
	if len(docs) == 0 {
		return docs, nil
	}
	return r.transformer.Transform(ctx, docs)
}

// rerankWithScore 对 RetrieveResult 进行重排序
// 先转换为 schema.Document，执行 rerank，再转换回 RetrieveResult
func (r *einoReranker) rerankWithScore(ctx context.Context, results []*RetrieveResult) ([]*RetrieveResult, error) {
	if len(results) == 0 {
		return results, nil
	}

	// 转换为 schema.Document
	docs := make([]*schema.Document, len(results))
	for i, result := range results {
		doc := &schema.Document{
			Content: result.Content,
			MetaData: map[string]any{
				"source_id":       result.SourceID,
				"source_name":     result.SourceName,
				"parent_block_id": result.ParentBlockID,
				"parent_content":  result.ParentContent,
				"heading":         result.Heading,
				"chapter_path":    result.ChapterPath,
				"chunk_type":      result.ChunkType,
			},
		}
		doc.WithScore(float64(result.Score))
		docs[i] = doc
	}

	// 执行 rerank
	rerankedDocs, err := r.rerank(ctx, docs)
	if err != nil {
		return nil, err
	}

	// 转换回 RetrieveResult
	rerankedResults := make([]*RetrieveResult, len(rerankedDocs))
	for i, doc := range rerankedDocs {
		result := &RetrieveResult{
			Content: doc.Content,
			Score:   float32(doc.Score()),
		}
		if sourceID, ok := doc.MetaData["source_id"].(uint); ok {
			result.SourceID = sourceID
		}
		if sourceName, ok := doc.MetaData["source_name"].(string); ok {
			result.SourceName = sourceName
		}
		if parentBlockID, ok := doc.MetaData["parent_block_id"].(int64); ok {
			result.ParentBlockID = parentBlockID
		}
		if parentContent, ok := doc.MetaData["parent_content"].(string); ok {
			result.ParentContent = parentContent
		}
		if heading, ok := doc.MetaData["heading"].(string); ok {
			result.Heading = heading
		}
		if chapterPath, ok := doc.MetaData["chapter_path"].(string); ok {
			result.ChapterPath = chapterPath
		}
		if chunkType, ok := doc.MetaData["chunk_type"].(string); ok {
			result.ChunkType = chunkType
		}
		rerankedResults[i] = result
	}

	return rerankedResults, nil
}
