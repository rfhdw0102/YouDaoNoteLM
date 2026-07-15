package quiz

import "strings"

// 压缩资料片段，避免题目解释过长。
func summarizeLine(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}

// 按位置循环选择资料要点。
func pickPoint(points []string, index int) string {
	if len(points) == 0 {
		return ""
	}
	if index < len(points) {
		return points[index]
	}
	return points[index%len(points)]
}
