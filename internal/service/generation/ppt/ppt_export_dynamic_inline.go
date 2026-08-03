package ppt

import (
	"github.com/duynguyendang/docxgo/v3/pptx"
	"golang.org/x/net/html"
	stdhtml "html"
	"strings"
)

// extractInlineTextRuns 提取节点内的内联文本运行序列。
func extractInlineTextRuns(node *html.Node, doc *pptHTMLDocument, inheritedText pptStyle) []pptHTMLTextRun {
	var runs []pptHTMLTextRun
	var walk func(*html.Node, pptStyle)
	walk = func(current *html.Node, currentStyle pptStyle) {
		if current == nil {
			return
		}
		switch current.Type {
		case html.TextNode:
			text := normalizeInlineText(current.Data)
			if text != "" {
				runs = append(runs, pptHTMLTextRun{Text: text, Style: currentStyle})
			}
			return
		case html.ElementNode:
			tag := strings.ToLower(current.Data)
			if shouldIgnoreHTMLElement(tag) {
				return
			}
			nextStyle := computeNodeStyle(current, currentStyle, doc)
			switch tag {
			case "strong", "b":
				nextStyle.FontWeight = intPtr(700)
			case "em", "i":
				if nextStyle.FontWeight == nil {
					nextStyle.FontWeight = currentStyle.FontWeight
				}
			}
			for child := current.FirstChild; child != nil; child = child.NextSibling {
				walk(child, inheritInlineTextStyle(nextStyle))
			}
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walk(child, inheritInlineTextStyle(inheritedText))
	}
	return mergeAdjacentInlineRuns(runs)
}

// normalizeInlineText 规范化内联文本空白字符。
func normalizeInlineText(value string) string {
	value = strings.ReplaceAll(value, "&nbsp;", " ")
	value = stdhtml.UnescapeString(value)
	if strings.TrimSpace(value) == "" {
		if strings.ContainsAny(value, " \n\r\t") {
			return " "
		}
		return ""
	}
	leading := len(value) > 0 && isInlineWhitespace(rune(value[0]))
	trailingRunes := []rune(value)
	trailing := len(trailingRunes) > 0 && isInlineWhitespace(trailingRunes[len(trailingRunes)-1])
	collapsed := strings.Join(strings.Fields(value), " ")
	if leading {
		collapsed = " " + collapsed
	}
	if trailing {
		collapsed += " "
	}
	if !leading && startsWithPPTMarkdownSyntaxMarker(collapsed) {
		collapsed = cleanPPTVisibleText(collapsed)
	}
	return collapsed
}

// startsWithPPTMarkdownSyntaxMarker 判断文本是否以 Markdown 标记开头。
func startsWithPPTMarkdownSyntaxMarker(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "• ") {
		return true
	}
	return false
}

// isInlineWhitespace 判断字符是否为内联空白字符。
func isInlineWhitespace(r rune) bool {
	return r == ' ' || r == '\n' || r == '\r' || r == '\t'
}

// textFromRuns 拼接文本运行序列为完整字符串。
func textFromRuns(runs []pptHTMLTextRun) string {
	var b strings.Builder
	for _, run := range runs {
		b.WriteString(run.Text)
	}
	return strings.TrimSpace(b.String())
}

// prependInlineRunPrefix 在文本运行序列前插入前缀。
func prependInlineRunPrefix(runs []pptHTMLTextRun, prefix string, style pptStyle) []pptHTMLTextRun {
	if strings.TrimSpace(prefix) == "" {
		return runs
	}
	prefixed := make([]pptHTMLTextRun, 0, len(runs)+1)
	prefixed = append(prefixed, pptHTMLTextRun{Text: prefix, Style: inheritInlineTextStyle(style)})
	prefixed = append(prefixed, runs...)
	return mergeAdjacentInlineRuns(prefixed)
}

// inheritInlineTextStyle 继承内联文本样式。
func inheritInlineTextStyle(style pptStyle) pptStyle {
	return inheritTextStyle(style)
}

// mergeAdjacentInlineRuns 合并相邻同样式文本运行。
func mergeAdjacentInlineRuns(runs []pptHTMLTextRun) []pptHTMLTextRun {
	merged := make([]pptHTMLTextRun, 0, len(runs))
	for _, run := range runs {
		if run.Text == "" {
			continue
		}
		if len(merged) > 0 && inlineStylesEqual(merged[len(merged)-1].Style, run.Style) {
			merged[len(merged)-1].Text += run.Text
			continue
		}
		merged = append(merged, run)
	}
	return merged
}

// inlineStylesEqual 判断两个内联样式是否相等。
func inlineStylesEqual(a, b pptStyle) bool {
	return colorsEqual(a.TextColor, b.TextColor) &&
		intPointersEqual(a.FontSize, b.FontSize) &&
		intPointersEqual(a.FontWeight, b.FontWeight) &&
		a.FontFamily == b.FontFamily
}

// colorsEqual 判断两个颜色指针是否相等。
func colorsEqual(a, b *pptx.Color) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// intPointersEqual 判断两个 int 指针是否相等。
func intPointersEqual(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
