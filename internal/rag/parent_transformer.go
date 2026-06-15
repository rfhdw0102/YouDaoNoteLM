package rag

import (
	"context"
	"fmt"
	"strings"

	"YoudaoNoteLm/internal/model/entity"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

// ParentTransformer 将 eino 文档转换为 ParentBlock
// 实现 eino document.Transformer 接口
type ParentTransformer struct {
	maxTokens int // 默认 1000
}

// NewParentTransformer 创建 ParentBlock 构建器
func NewParentTransformer(maxTokens int) *ParentTransformer {
	if maxTokens <= 0 {
		maxTokens = 1000
	}
	return &ParentTransformer{maxTokens: maxTokens}
}

// Transform 实现 eino document.Transformer 接口
// 输入: eino 文档列表（每个文档代表一个章节）
// 输出: 转换后的 eino 文档列表（每个文档代表一个 ParentBlock）
func (t *ParentTransformer) Transform(ctx context.Context, src []*schema.Document, opts ...document.TransformerOption) ([]*schema.Document, error) {
	var result []*schema.Document
	blockIndex := 0

	for _, doc := range src {
		heading, _ := doc.MetaData["heading"].(string)
		level, _ := doc.MetaData["level"].(int)
		chapterPath, _ := doc.MetaData["chapter_path"].(string)

		chunks := t.splitByTokens(doc.Content, t.maxTokens)
		for _, chunk := range chunks {
			newDoc := &schema.Document{
				Content: chunk,
				MetaData: map[string]any{
					"heading":      heading,
					"level":        level,
					"chapter_path": chapterPath,
					"parent_index": blockIndex,
					"block_type":   "parent",
				},
			}
			result = append(result, newDoc)
			blockIndex++
		}
	}
	return result, nil
}

// splitByTokens 按段落边界分割内容，每个块不超过 maxTokens
func (t *ParentTransformer) splitByTokens(content string, maxTokens int) []string {
	paragraphs := strings.Split(content, "\n\n")
	var chunks []string
	var current []string
	currentTokens := 0

	for _, p := range paragraphs {
		tokens := estimateTokens(p)
		if currentTokens+tokens > maxTokens && len(current) > 0 {
			chunks = append(chunks, strings.Join(current, "\n\n"))
			current = []string{p}
			currentTokens = tokens
		} else {
			current = append(current, p)
			currentTokens += tokens
		}
	}
	if len(current) > 0 {
		chunks = append(chunks, strings.Join(current, "\n\n"))
	}
	return chunks
}

// estimateTokens 粗略估算 token 数（中文字符 + 英文单词）
func estimateTokens(text string) int {
	chars := 0
	words := 0
	inWord := false
	for _, r := range text {
		if r > 127 {
			chars++
			inWord = false
		} else if r == ' ' || r == '\n' || r == '\t' {
			if inWord {
				words++
			}
			inWord = false
		} else {
			inWord = true
		}
	}
	if inWord {
		words++
	}
	return chars + words
}

// ToParentBlocks 将 eino 文档列表转换为 ParentBlock 实体列表
// 供 IngestionService 写入 MySQL
func ToParentBlocks(docs []*schema.Document, sourceID uint) []entity.ParentBlock {
	var blocks []entity.ParentBlock
	for _, doc := range docs {
		level, _ := doc.MetaData["level"].(int)
		chapterPath, _ := doc.MetaData["chapter_path"].(string)
		parentIndex, _ := doc.MetaData["parent_index"].(int)
		heading, _ := doc.MetaData["heading"].(string)

		blocks = append(blocks, entity.ParentBlock{
			SourceID:    sourceID,
			Heading:     heading,
			Level:       level,
			ChapterPath: chapterPath,
			Content:     doc.Content,
			ChunkIndex:  parentIndex,
			Metadata:    fmt.Sprintf(`{"chapter_path":"%s","level":%d}`, chapterPath, level),
		})
	}
	return blocks
}
