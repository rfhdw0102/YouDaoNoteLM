package ppt

import "strings"

// 校验演示文稿输出是否具备可导出的页面结构。
func ValidateContent(content string) bool {
	lower := strings.ToLower(strings.TrimSpace(content))
	if strings.Contains(lower, "<ppt_file") || strings.Contains(lower, "<preview_link") {
		return false
	}
	if !strings.Contains(lower, "<section") || !strings.Contains(lower, "</section>") {
		return false
	}
	if strings.Count(lower, "<section") < 4 {
		return false
	}
	if !strings.Contains(lower, "<style") {
		return false
	}
	for _, required := range []string{"width: 1920px", "height: 1080px", "overflow: hidden"} {
		if !strings.Contains(lower, required) {
			return false
		}
	}
	if strings.Contains(lower, "max-width: 1100px") || strings.Contains(lower, "font-size: 1rem") {
		return false
	}
	text := strings.TrimSpace(stripSimpleHTML(content))
	return len([]rune(text)) >= 4
}
