package rag

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

// SemanticTransformer 语义增强器
// 在 Embedding 前为 ChildChunk 注入结构化上下文
// 实现 eino document.Transformer 接口
type SemanticTransformer struct{}

// NewSemanticTransformer 创建语义增强器
func NewSemanticTransformer() *SemanticTransformer {
	return &SemanticTransformer{}
}

// Transform 实现 eino document.Transformer 接口
// 为每个 ChildChunk 文档的内容注入章节路径和结构信息
func (t *SemanticTransformer) Transform(ctx context.Context, src []*schema.Document, opts ...document.TransformerOption) ([]*schema.Document, error) {
	result := make([]*schema.Document, len(src))
	for i, doc := range src {
		chapterPath, _ := doc.MetaData["chapter_path"].(string)
		chunkType, _ := doc.MetaData["chunk_type"].(string)

		enhanced := t.enhance(doc.Content, chapterPath, chunkType)
		newDoc := &schema.Document{
			ID:       doc.ID,
			Content:  enhanced,
			MetaData: doc.MetaData,
		}
		result[i] = newDoc
	}
	return result, nil
}

// enhance 注入结构化上下文
func (t *SemanticTransformer) enhance(content, chapterPath, chunkType string) string {
	if chapterPath == "" {
		return content
	}
	switch chunkType {
	case "code":
		return fmt.Sprintf("标题路径：%s\n代码内容：\n%s", chapterPath, content)
	case "table":
		return fmt.Sprintf("标题路径：%s\n表格数据：\n%s", chapterPath, content)
	case "mermaid":
		return fmt.Sprintf("标题路径：%s\n流程图：\n%s", chapterPath, content)
	default:
		return fmt.Sprintf("标题路径：%s\n正文：%s", chapterPath, content)
	}
}
