package rag

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

// ChildChunk 子块分割（内部使用）
type ChildChunk struct {
	ParentBlockIndex int
	Content          string
	ChunkType        string // paragraph/code/table/image/mermaid/quote
	ChapterPath      string
}

// ChildTransformer 将 ParentBlock 文档分割为 ChildChunk 文档
// 实现 eino document.Transformer 接口
type ChildTransformer struct {
	maxTokens int // 默认 400
}

// NewChildTransformer 创建 ChildChunk 分割器
func NewChildTransformer(maxTokens int) *ChildTransformer {
	if maxTokens <= 0 {
		maxTokens = 400
	}
	return &ChildTransformer{maxTokens: maxTokens}
}

// Transform 实现 eino document.Transformer 接口
func (t *ChildTransformer) Transform(ctx context.Context, src []*schema.Document, opts ...document.TransformerOption) ([]*schema.Document, error) {
	var result []*schema.Document

	for _, parentDoc := range src {
		parentIndex, _ := parentDoc.MetaData["parent_index"].(int)
		chapterPath, _ := parentDoc.MetaData["chapter_path"].(string)
		heading, _ := parentDoc.MetaData["heading"].(string)

		chunks := t.splitContent(parentDoc.Content)
		for i, chunk := range chunks {
			childDoc := &schema.Document{
				Content: chunk.Content,
				MetaData: map[string]any{
					"parent_index": parentIndex,
					"chunk_type":   chunk.ChunkType,
					"chapter_path": chapterPath,
					"heading":      heading,
					"child_index":  i,
					"block_type":   "child",
				},
			}
			result = append(result, childDoc)
		}
	}
	return result, nil
}

// splitContent 分割内容，保持代码块等特殊块的完整性
func (t *ChildTransformer) splitContent(content string) []ChildChunk {
	// 先识别并提取特殊块（代码块、表格、mermaid）
	specialBlocks, remainingContent := extractSpecialBlocks(content)

	var chunks []ChildChunk

	// 特殊块作为独立 chunk
	for _, block := range specialBlocks {
		chunks = append(chunks, ChildChunk{
			Content:   block.Content,
			ChunkType: block.BlockType,
		})
	}

	// 剩余内容按段落分割
	if strings.TrimSpace(remainingContent) != "" {
		paragraphs := splitIntoParagraphs(remainingContent)
		textChunks := t.mergeParagraphs(paragraphs, t.maxTokens)
		for _, tc := range textChunks {
			chunks = append(chunks, ChildChunk{
				Content:   tc,
				ChunkType: "paragraph",
			})
		}
	}

	return chunks
}

// specialBlock 特殊块
type specialBlock struct {
	Content   string
	BlockType string
}

// extractSpecialBlocks 从内容中提取特殊块（代码块、表格、mermaid）
func extractSpecialBlocks(content string) ([]specialBlock, string) {
	var blocks []specialBlock
	var remaining strings.Builder
	lines := strings.Split(content, "\n")

	inCodeBlock := false
	codeBlockContent := strings.Builder{}
	codeBlockLang := ""

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// 检测代码块开始/结束
		if strings.HasPrefix(trimmed, "```") {
			if !inCodeBlock {
				// 代码块开始
				inCodeBlock = true
				codeBlockContent.Reset()
				codeBlockLang = strings.TrimPrefix(trimmed, "```")
				codeBlockLang = strings.TrimSpace(codeBlockLang)
				continue
			} else {
				// 代码块结束
				inCodeBlock = false
				blockType := "code"
				if strings.EqualFold(codeBlockLang, "mermaid") {
					blockType = "mermaid"
				}
				blocks = append(blocks, specialBlock{
					Content:   "```" + codeBlockLang + "\n" + codeBlockContent.String() + "```",
					BlockType: blockType,
				})
				continue
			}
		}

		if inCodeBlock {
			codeBlockContent.WriteString(line)
			codeBlockContent.WriteString("\n")
			continue
		}

		// 检测表格（以 | 开头，包含 ---）
		if strings.HasPrefix(trimmed, "|") && strings.Contains(trimmed, "---") {
			// 收集整个表格
			tableContent := strings.Builder{}
			tableContent.WriteString(line)
			tableContent.WriteString("\n")
			for j := i + 1; j < len(lines); j++ {
				nextLine := strings.TrimSpace(lines[j])
				if !strings.HasPrefix(nextLine, "|") {
					break
				}
				tableContent.WriteString(lines[j])
				tableContent.WriteString("\n")
				i = j
			}
			blocks = append(blocks, specialBlock{
				Content:   tableContent.String(),
				BlockType: "table",
			})
			continue
		}

		// 普通行
		remaining.WriteString(line)
		remaining.WriteString("\n")
	}

	// 如果代码块没有闭合，作为代码块处理
	if inCodeBlock {
		blocks = append(blocks, specialBlock{
			Content:   "```" + codeBlockLang + "\n" + codeBlockContent.String(),
			BlockType: "code",
		})
	}

	return blocks, remaining.String()
}

// splitIntoParagraphs 按空行分割为段落
func splitIntoParagraphs(content string) []string {
	// 按两个换行符分割（即空行）
	paragraphs := strings.Split(content, "\n\n")
	var result []string
	for _, p := range paragraphs {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// mergeParagraphs 将段落合并为不超过 maxTokens 的 chunk
func (t *ChildTransformer) mergeParagraphs(paragraphs []string, maxTokens int) []string {
	if len(paragraphs) == 0 {
		return nil
	}

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
