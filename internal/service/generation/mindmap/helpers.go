package mindmap

import "strings"

// 压缩资料片段，避免节点说明过长。
func summarizeLine(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}

// 保留非空且不重复的要点。
func uniqueNonEmpty(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// 判断文本是否包含任一关键词。
func containsAnyFold(value string, terms ...string) bool {
	value = strings.ToLower(value)
	for _, term := range terms {
		if strings.Contains(value, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

// 提取用于证据匹配的关键词。
func extractSignificantWords(text string) []string {
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\t' || r == ',' || r == '，' ||
			r == '.' || r == '。' || r == ';' || r == '；' || r == ':' || r == '：' ||
			r == '(' || r == ')' || r == '（' || r == '）' || r == '"' || r == '\''
	})
	var words []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len([]rune(part)) >= 2 {
			words = append(words, part)
		}
	}
	return words
}
