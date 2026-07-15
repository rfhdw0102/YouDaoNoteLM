package note

import "strings"

// 校验笔记输出结构是否可用。
func ValidateContent(content string) bool {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "#") {
		return false
	}
	lines := strings.Split(content, "\n")
	bodyRunes := 0
	hasSection := false
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") {
			hasSection = true
		}
		if line != "" && !strings.HasPrefix(line, "#") {
			bodyRunes += len([]rune(line))
		}
	}
	return hasSection || bodyRunes >= 8
}
