package rag

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/document/transformer/reranker/score"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

// EinoRerankerConfig Score Reranker 配置
type EinoRerankerConfig struct {
	// ScoreFieldKey 指定从 metadata 中获取 score 的 key
	// 如果为空，则使用 schema.Document.Score() 方法
	ScoreFieldKey string
}

// EinoReranker 基于 eino-ext score 包的 Reranker
// 利用 LLM 的首因效应和近因效应，将高分文档放在开头和结尾
type EinoReranker struct {
	transformer document.Transformer
	config      *EinoRerankerConfig
}

// NewEinoReranker 创建 Score Reranker
// 基于论文 https://arxiv.org/abs/2307.03172 的发现：
// LLM 对输入上下文开头和结尾的信息处理效果更好
func NewEinoReranker(ctx context.Context, config *EinoRerankerConfig) (*EinoReranker, error) {
	if config == nil {
		config = &EinoRerankerConfig{}
	}

	rerankerConfig := &score.Config{}
	if config.ScoreFieldKey != "" {
		rerankerConfig.ScoreFieldKey = &config.ScoreFieldKey
	}

	transformer, err := score.NewReranker(ctx, rerankerConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 score reranker 失败: %w", err)
	}

	return &EinoReranker{
		transformer: transformer,
		config:      config,
	}, nil
}

// Rerank 对文档进行重排序
// 将高分文档放在数组的开头和结尾，低分文档放在中间
func (r *EinoReranker) Rerank(ctx context.Context, docs []*schema.Document) ([]*schema.Document, error) {
	if len(docs) == 0 {
		return docs, nil
	}

	result, err := r.transformer.Transform(ctx, docs)
	if err != nil {
		return nil, fmt.Errorf("rerank 失败: %w", err)
	}

	return result, nil
}

// RerankWithScore 对 RetrieveResult 进行重排序
// 先转换为 schema.Document，执行 rerank，再转换回 RetrieveResult
func (r *EinoReranker) RerankWithScore(ctx context.Context, results []*RetrieveResult) ([]*RetrieveResult, error) {
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
		// 设置 score
		doc.WithScore(float64(result.Score))
		docs[i] = doc
	}

	// 执行 rerank
	rerankedDocs, err := r.Rerank(ctx, docs)
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
