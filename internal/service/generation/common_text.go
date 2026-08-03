// common_text.go 提供通用文本处理工具函数。
//
// 包括标题提取、要点提取、行摘要、HTML 转义、引用片段拼接等，
// 供各类型生成器共用。
package generation

import (
	"fmt"
	"strings"
)

// extractTitle 从 Markdown 中提取首个标题文本，无则返回 fallback。
func extractTitle(markdown, fallback string) string {
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			title := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if title != "" {
				return title
			}
		}
	}
	return fallback
}

// extractKeyPoints 从 Markdown 中提取最多 limit 条要点。
func extractKeyPoints(markdown string, limit int) []string {
	var points []string
	lines := strings.Split(markdown, "\n")
	mergedLines := mergeCodeBlockLines(lines)

	for _, line := range mergedLines {
		if isPPTCodeBlockBullet(line) {
			points = append(points, line)
			if len(points) >= limit {
				return points
			}
			continue
		}
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "-*#0123456789. "))
		if len([]rune(line)) < 4 {
			continue
		}
		points = append(points, line)
		if len(points) >= limit {
			return points
		}
	}
	return points
}

// appendReferenceSection 向生成结果追加参考资料章节。
func appendReferenceSection(b *strings.Builder, refs []GenerationReference) {
	if len(refs) == 0 {
		return
	}
	b.WriteString("\n\n## 参考资料\n")
	for i, ref := range refs {
		label := generationReferenceLabel(ref)
		b.WriteString(fmt.Sprintf("- [%d] %s: %s\n", i+1, label, summarizeLine(ref.Content, 120)))
	}
}

// summarizeLine 压缩空白并按字符数截断单行文本。
func summarizeLine(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}

// htmlEscape 转义 HTML 特殊字符。
func htmlEscape(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return replacer.Replace(value)
}
