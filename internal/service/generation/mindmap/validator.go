package mindmap

import "strings"

// 校验思维导图输出结构是否可用。
func ValidateContent(content string) bool {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "#") {
		return false
	}
	return strings.Contains(content, "\n## ") || strings.Contains(content, "\n- ")
}
