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
	maxTokens int // 每个父块的最大 token 数，默认 1000
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
					"parent_index": blockIndex, // 记录在原章节内的顺序
					"block_type":   "parent",
				},
			}
			result = append(result, newDoc)
			blockIndex++
		}
	}
	return result, nil
}

// splitByTokens 按 token 上限切分文本，尽量保持段落完整
// 代码块（```...```）作为不可分割的原子单元，不会被切断
func (t *ParentTransformer) splitByTokens(content string, maxTokens int) []string {
	paragraphs := strings.Split(content, "\n\n") // 按空行分成段落
	// 合并被空行分割的代码块
	// 代码块如 ```go\n...\n``` 内部可能包含 \n\n，导致 Split 将其拆分
	// 这里将它们重新组装
	paragraphs = reassembleCodeBlockParagraphs(paragraphs)

	var chunks []string
	var current []string // 当前块包含的段落
	currentTokens := 0

	for _, p := range paragraphs {
		tokens := estimateTokens(p) // 估算当前段落 token 数

		// 代码块（以 ``` 开头）作为原子单元，不会被跨块切断
		isCodeBlock := strings.HasPrefix(strings.TrimSpace(p), "```")

		if isCodeBlock {
			// 如果当前块非空且加上代码块会超限，先结束当前块
			if currentTokens+tokens > maxTokens && len(current) > 0 {
				chunks = append(chunks, strings.Join(current, "\n\n"))
				current = nil
				currentTokens = 0
			}
			// 即使代码块本身超过 maxTokens，也作为独立块，避免拆分
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

// reassembleCodeBlockParagraphs 将属于同一个代码块的段落重新合并
// 当内容包含 ```...``` 且内部有空行时，strings.Split("\n\n") 会将代码块拆分成多个段落
// 此函数将它们重新组装为一个完整的段落
func reassembleCodeBlockParagraphs(paragraphs []string) []string {
	var result []string
	var codeBuf strings.Builder
	inCode := false

	for _, p := range paragraphs {
		trimmed := strings.TrimSpace(p)
		if !inCode {
			if strings.HasPrefix(trimmed, "```") && !strings.HasSuffix(trimmed, "```") {
				// 开始一个代码块（未在同一行闭合）
				inCode = true
				codeBuf.Reset()
				codeBuf.WriteString(p)
			} else {
				result = append(result, p)
			}
		} else {
			codeBuf.WriteString("\n\n")
			codeBuf.WriteString(p)
			// 检查当前段落是否闭合了代码块
			// 闭合的 ``` 会单独出现在行末
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
	// 处理未闭合的代码块
	if inCode {
		result = append(result, codeBuf.String())
	}
	return result
}

// estimateTokens 估算 token 数：中文每字 1 token，英文每单词 1 token
func estimateTokens(text string) int {
	chars := 0      // 非 ASCII 字符数（中文等）
	words := 0      // 英文单词数
	inWord := false // 是否处于单词中

	for _, r := range text {
		if r > 127 { // 非 ASCII（中文）
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
