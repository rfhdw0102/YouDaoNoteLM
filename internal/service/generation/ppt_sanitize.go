// ppt_sanitize.go 提供 PPT 生成内容的清理工具。
//
// 主要职责是移除 PPT 渲染结果中不应展示给用户的"参考资料/来源"章节，
// 避免引用片段污染最终课件内容。
package generation

import "strings"

// sanitizePPTReferenceSections 移除 PPT 内容中的参考资料章节。
// 通过 </section> 分隔片段，过滤掉标题为 references/reference/参考资料/参考文献/来源 的章节。
func sanitizePPTReferenceSections(content string) string {
	if strings.TrimSpace(content) == "" {
		return content
	}
	parts := strings.Split(content, "</section>")
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		candidate := trimmed + "</section>"
		if isReferencePPTSection(candidate) {
			continue
		}
		kept = append(kept, candidate)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// isReferencePPTSection 判断给定 section 是否为参考资料章节。
func isReferencePPTSection(section string) bool {
	lower := strings.ToLower(section)
	return strings.Contains(lower, "<h1>references</h1>") ||
		strings.Contains(lower, "<h2>references</h2>") ||
		strings.Contains(lower, "<h1>reference</h1>") ||
		strings.Contains(lower, "<h2>reference</h2>") ||
		strings.Contains(section, "<h1>参考资料</h1>") ||
		strings.Contains(section, "<h2>参考资料</h2>") ||
		strings.Contains(section, "<h1>参考文献</h1>") ||
		strings.Contains(section, "<h2>参考文献</h2>") ||
		strings.Contains(section, "<h1>来源</h1>") ||
		strings.Contains(section, "<h2>来源</h2>")
}
