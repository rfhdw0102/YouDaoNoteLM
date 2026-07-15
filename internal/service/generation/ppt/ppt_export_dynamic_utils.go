package ppt

import (
	"github.com/duynguyendang/docxgo/v3/pptx"
	"golang.org/x/net/html"
	"strings"
)

func edgeOr(edges pptEdges, side string, fallback float64) float64 {
	if !edges.Set {
		return fallback
	}
	switch side {
	case "top":
		return edges.Top
	case "right":
		return edges.Right
	case "bottom":
		return edges.Bottom
	case "left":
		return edges.Left
	default:
		return fallback
	}
}

func resolveCSSVars(value string, vars map[string]string) string {
	resolved := value
	for range 6 {
		start := strings.Index(resolved, "var(")
		if start == -1 {
			break
		}
		end := strings.Index(resolved[start:], ")")
		if end == -1 {
			break
		}
		end += start
		token := strings.TrimSpace(resolved[start+4 : end])
		replacement := vars[token]
		resolved = resolved[:start] + replacement + resolved[end+1:]
	}
	return resolved
}

func normalizeFontFamily(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	lowerValue := strings.ToLower(value)
	switch {
	case value == "":
		return ""
	case strings.Contains(lowerValue, "system-ui"),
		strings.Contains(lowerValue, "segoe ui"),
		strings.Contains(lowerValue, "roboto"),
		strings.Contains(lowerValue, "helvetica"),
		strings.Contains(lowerValue, "arial"),
		strings.Contains(lowerValue, "sans-serif"),
		strings.Contains(lowerValue, "sans serif"):
		return dynamicPPTDefaultFontFamily
	default:
		return value
	}
}

func isCSSNoneValue(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "none" || value == "0" || value == "0px" || value == "transparent"
}

func parseClassSet(node *html.Node) map[string]bool {
	classes := map[string]bool{}
	for _, className := range strings.Fields(getHTMLAttribute(node, "class")) {
		className = strings.TrimSpace(className)
		if className != "" {
			classes[className] = true
		}
	}
	return classes
}

func getHTMLAttribute(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, key) {
			return attr.Val
		}
	}
	return ""
}

func extractNodeText(node *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current == nil {
			return
		}
		if current.Type == html.TextNode {
			b.WriteString(current.Data)
			b.WriteByte(' ')
		}
		if current.Type == html.ElementNode && shouldIgnoreHTMLElement(strings.ToLower(current.Data)) {
			return
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return b.String()
}

func extractRawNodeText(node *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current == nil {
			return
		}
		if current.Type == html.TextNode {
			b.WriteString(current.Data)
			return
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return b.String()
}

func cloneColor(color *pptx.Color) *pptx.Color {
	if color == nil {
		return nil
	}
	value := *color
	return &value
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func cloneFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func intPtr(value int) *int {
	return &value
}

func float64Ptr(value float64) *float64 {
	return &value
}

func dynamicPPTValueOrInt(value *int, fallback int) int {
	if value == nil || *value <= 0 {
		return fallback
	}
	return *value
}

func dynamicPPTMaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func dynamicPPTMinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func dynamicPPTMaxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
