package rag

import (
	"context"
	"encoding/json"
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
	blockIndex := 0 // 父块全局索引

	for _, doc := range src {
		// 从元数据提取字段，类型断言 + 默认值
		heading, _ := doc.MetaData["heading"].(string)
		level, _ := doc.MetaData["level"].(int)
		chapterPath, _ := doc.MetaData["chapter_path"].(string)

		// 按 maxTokens 切分正文
		chunks := t.splitByTokens(doc.Content, t.maxTokens)

		for _, chunk := range chunks {
			newDoc := &schema.Document{
				Content: chunk,
				MetaData: map[string]any{
					"heading":      heading,
					"level":        level,
					"chapter_path": chapterPath,
					"parent_index": blockIndex, // 记录原章节内顺序
					"block_type":   "parent",
				},
			}
			result = append(result, newDoc)
			blockIndex++
		}
	}
	return result, nil
}

// 按 token 上限切分文本，尽量保持段落完整
// 代码块（```...```）作为不可分割的原子单元，不会被切断
func (t *ParentTransformer) splitByTokens(content string, maxTokens int) []string {
	paragraphs := strings.Split(content, "\n\n") // 按空行分成段落
	// Merge code blocks that were split across paragraphs.
	// A code block like ```go\n...\n``` may contain \n\n inside it,
	// causing Split to break it. We reassemble them here.
	paragraphs = reassembleCodeBlockParagraphs(paragraphs)

	var chunks []string
	var current []string // 当前块包含的段落
	currentTokens := 0

	for _, p := range paragraphs {
		tokens := estimateTokens(p) // 估算当前段落 token 数

		// If the paragraph is a code block (starts with ```), always keep
		// it as an atomic unit — never split a code block across chunks.
		isCodeBlock := strings.HasPrefix(strings.TrimSpace(p), "```")

		if isCodeBlock {
			// If the current chunk is non-empty and adding the code block
			// would exceed the limit, flush the current chunk first.
			if currentTokens+tokens > maxTokens && len(current) > 0 {
				chunks = append(chunks, strings.Join(current, "\n\n"))
				current = nil
				currentTokens = 0
			}
			// If the code block alone exceeds maxTokens, we still add it
			// as its own chunk to avoid splitting it.
			current = append(current, p)
			currentTokens += tokens
		} else {
			// 如果加上当前段落会超限，且当前块非空 → 结束当前块
			if currentTokens+tokens > maxTokens && len(current) > 0 {
				chunks = append(chunks, strings.Join(current, "\n\n"))
				current = []string{p} // 新块从当前段落开始
				currentTokens = tokens
			} else {
				current = append(current, p)
				currentTokens += tokens
			}
		}
	}
	// 最后一块
	if len(current) > 0 {
		chunks = append(chunks, strings.Join(current, "\n\n"))
	}
	return chunks
}

// reassembleCodeBlockParagraphs merges paragraphs that belong to the same
// fenced code block. When content contains ```...``` with blank lines inside,
// strings.Split("\n\n") will break the code block into separate paragraphs.
// This function reassembles them back into a single paragraph.
func reassembleCodeBlockParagraphs(paragraphs []string) []string {
	var result []string
	var codeBuf strings.Builder
	inCode := false

	for _, p := range paragraphs {
		trimmed := strings.TrimSpace(p)
		if !inCode {
			if strings.HasPrefix(trimmed, "```") && !strings.HasSuffix(trimmed, "```") {
				// Opening a code block that doesn't close on the same line
				inCode = true
				codeBuf.Reset()
				codeBuf.WriteString(p)
			} else {
				result = append(result, p)
			}
		} else {
			codeBuf.WriteString("\n\n")
			codeBuf.WriteString(p)
			// Check if this paragraph closes the code block
			// A closing ``` appears on its own line at the end
			lines := strings.Split(trimmed, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "```" {
					inCode = false
					result = append(result, codeBuf.String())
					codeBuf.Reset()
					break
				}
			}
		}
	}
	// Handle unclosed code block
	if inCode {
		result = append(result, codeBuf.String())
	}
	return result
}

// 估算 token 数：中文每字1 token，英文每单词1 token
func estimateTokens(text string) int {
	chars := 0      // 非ASCII字符数（中文等）
	words := 0      // 英文单词数
	inWord := false // 是否处于单词中

	for _, r := range text {
		if r > 127 { // 非ASCII（中文）
			chars++
			inWord = false
		} else if r == ' ' || r == '\n' || r == '\t' { // 分隔符
			if inWord {
				words++
			}
			inWord = false
		} else { // 英文/数字/符号
			inWord = true
		}
	}
	if inWord { // 末尾单词
		words++
	}
	return chars + words
}

// toParentBlocks 将 eino 文档列表转换为 ParentBlock 实体列表
func toParentBlocks(docs []*schema.Document, sourceID uint) []entity.ParentBlock {
	var blocks []entity.ParentBlock
	for _, doc := range docs {
		level, _ := doc.MetaData["level"].(int)
		chapterPath, _ := doc.MetaData["chapter_path"].(string)
		parentIndex, _ := doc.MetaData["parent_index"].(int)
		heading, _ := doc.MetaData["heading"].(string)

		// 使用 json.Marshal 确保特殊字符正确转义
		metadataMap := map[string]any{
			"chapter_path": chapterPath,
			"level":        level,
		}
		metadataBytes, err := json.Marshal(metadataMap)
		if err != nil {
			// 如果序列化失败，使用空 JSON 对象
			metadataBytes = []byte("{}")
		}

		blocks = append(blocks, entity.ParentBlock{
			SourceID:    sourceID,
			Heading:     heading,
			Level:       level,
			ChapterPath: chapterPath,
			Content:     doc.Content,
			ChunkIndex:  parentIndex,
			Metadata:    string(metadataBytes),
		})
	}
	return blocks
}
