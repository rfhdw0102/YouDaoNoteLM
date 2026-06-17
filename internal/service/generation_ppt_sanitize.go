package service

import "strings"

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
