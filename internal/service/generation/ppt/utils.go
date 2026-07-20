package ppt

import (
	"regexp"
	"strings"
)

var invalidFilenameChars = regexp.MustCompile(`[\\/:*?"<>|]+`)
var invalidFilenameWhitespace = regexp.MustCompile(`[\r\n\t]+`)
var invalidFilenameHyphenSpacing = regexp.MustCompile(`\s*-\s*`)

// firstNonEmpty 返回传入字符串中第一个非空值。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// uniqueNonEmpty 去重并去除空白字符串后返回唯一列表。
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

// stripSimpleHTML 移除 HTML 标签，仅保留纯文本内容。
func stripSimpleHTML(content string) string {
	var b strings.Builder
	inTag := false
	for _, r := range content {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// cleanPPTVisibleText 清理 PPT 中可见文本的特殊字符与无意义前缀。
func cleanPPTVisibleText(value string) string {
	value = strings.ReplaceAll(value, "&nbsp;", " ")
	if cleaned, ok := stripFencedCodeBlockForPPT(value); ok {
		return cleaned
	}
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	for {
		trimmed := strings.TrimSpace(value)
		cleaned := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		if cleaned != trimmed {
			value = cleaned
			continue
		}
		cleaned = strings.TrimSpace(strings.TrimLeft(trimmed, "-*•"))
		if cleaned != trimmed {
			value = cleaned
			continue
		}
		return trimmed
	}
}

// stripFencedCodeBlockForPPT 剥离 ``` 围栏代码块标记，返回内部内容。
func stripFencedCodeBlockForPPT(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "```") {
		return "", false
	}
	firstNewline := strings.Index(trimmed, "\n")
	if firstNewline < 0 {
		inner := strings.TrimPrefix(trimmed, "```")
		inner = strings.TrimSuffix(inner, "```")
		inner = strings.TrimSpace(inner)
		if inner == "" {
			return "", false
		}
		return inner, true
	}
	inner := trimmed[firstNewline+1:]
	if strings.HasSuffix(inner, "\n```") {
		inner = inner[:len(inner)-4]
	} else if strings.HasSuffix(inner, "```") {
		inner = inner[:len(inner)-3]
	}
	inner = strings.TrimRight(inner, "\n")
	if strings.TrimSpace(inner) == "" {
		return "", false
	}
	return inner, true
}

// resolveExportFilename 依次从标题、内容首行、兜底值中解析导出文件名。
func resolveExportFilename(title, content, fallback, ext string) string {
	for _, candidate := range []string{title, extractExportHeading(content), fallback} {
		base := sanitizeExportFilenameBase(candidate)
		if base != "" {
			return base + ext
		}
	}
	return fallback + ext
}

// sanitizeExportFilenameBase 规范化文件名基础部分，去除非法字符与多余空白。
func sanitizeExportFilenameBase(value string) string {
	base := strings.TrimSpace(value)
	base = invalidFilenameWhitespace.ReplaceAllString(base, " ")
	base = invalidFilenameChars.ReplaceAllString(base, "-")
	base = strings.Join(strings.Fields(base), " ")
	base = invalidFilenameHyphenSpacing.ReplaceAllString(base, "-")
	return strings.Trim(base, ". -")
}

// extractExportHeading 从内容中提取首个非空行作为标题。
func extractExportHeading(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if line != "" {
			return line
		}
	}
	return ""
}
