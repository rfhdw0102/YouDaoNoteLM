package ppt

import (
	"regexp"
	"strings"
)

var invalidFilenameChars = regexp.MustCompile(`[\\/:*?"<>|]+`)
var invalidFilenameWhitespace = regexp.MustCompile(`[\r\n\t]+`)
var invalidFilenameHyphenSpacing = regexp.MustCompile(`\s*-\s*`)

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

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

func resolveExportFilename(title, content, fallback, ext string) string {
	for _, candidate := range []string{title, extractExportHeading(content), fallback} {
		base := sanitizeExportFilenameBase(candidate)
		if base != "" {
			return base + ext
		}
	}
	return fallback + ext
}

func sanitizeExportFilenameBase(value string) string {
	base := strings.TrimSpace(value)
	base = invalidFilenameWhitespace.ReplaceAllString(base, " ")
	base = invalidFilenameChars.ReplaceAllString(base, "-")
	base = strings.Join(strings.Fields(base), " ")
	base = invalidFilenameHyphenSpacing.ReplaceAllString(base, "-")
	return strings.Trim(base, ". -")
}

func extractExportHeading(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if line != "" {
			return line
		}
	}
	return ""
}
