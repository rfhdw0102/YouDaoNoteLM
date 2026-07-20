package ppt

import (
	"github.com/duynguyendang/docxgo/v3/pptx"
	"golang.org/x/net/html"
	"strings"
)

// edgeOr 读取 pptEdges 的指定边，未设置时返回兜底值。
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

// resolveCSSVars 解析字符串中的 CSS var() 引用。
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

// normalizeFontFamily 规范化字体族名称。
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

// isCSSNoneValue 判断 CSS 值是否表示无（none/0/transparent）。
func isCSSNoneValue(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "none" || value == "0" || value == "0px" || value == "transparent"
}

// parseClassSet 解析节点的 class 属性为集合。
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

// getHTMLAttribute 获取节点指定属性值。
func getHTMLAttribute(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, key) {
			return attr.Val
		}
	}
	return ""
}

// extractNodeText 提取节点及其后代的可见文本（带空格分隔）。
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

// extractRawNodeText 提取节点及其后代的原始文本。
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

// cloneColor 克隆颜色指针。
func cloneColor(color *pptx.Color) *pptx.Color {
	if color == nil {
		return nil
	}
	value := *color
	return &value
}

// cloneInt 克隆 int 指针。
func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

// cloneFloat64 克隆 float64 指针。
func cloneFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

// intPtr 返回 int 值的指针。
func intPtr(value int) *int {
	return &value
}

// float64Ptr 返回 float64 值的指针。
func float64Ptr(value float64) *float64 {
	return &value
}

// dynamicPPTValueOrInt 返回指针值，为空或非正时返回兜底值。
func dynamicPPTValueOrInt(value *int, fallback int) int {
	if value == nil || *value <= 0 {
		return fallback
	}
	return *value
}

// dynamicPPTMaxInt 返回两个 int 中的较大值。
func dynamicPPTMaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// dynamicPPTMinInt 返回两个 int 中的较小值。
func dynamicPPTMinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// dynamicPPTMaxFloat 返回两个 float64 中的较大值。
func dynamicPPTMaxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
