package ppt

import (
	"golang.org/x/net/html"
	"sort"
	"strings"
)

// computeNodeStyle 计算节点最终样式，合并继承样式与匹配的 CSS 规则。
func computeNodeStyle(node *html.Node, inheritedText pptStyle, doc *pptHTMLDocument) pptStyle {
	style := inheritTextStyle(inheritedText)

	matches := matchingCSSRules(node, doc.Rules)
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Specificity == matches[j].Specificity {
			return matches[i].Order < matches[j].Order
		}
		return matches[i].Specificity < matches[j].Specificity
	})
	for _, rule := range matches {
		style = mergePPTStyle(style, rule.Style)
	}

	inlineStyle := parseInlineStyleDeclarations(resolveCSSVars(getHTMLAttribute(node, "style"), doc.Vars))
	style = applyOrderedStyleDeclarations(style, inlineStyle)
	return style
}

// matchingCSSRules 返回与节点选择器匹配的 CSS 规则列表。
func matchingCSSRules(node *html.Node, rules []pptCSSRule) []pptCSSRule {
	matches := make([]pptCSSRule, 0, len(rules))
	for _, rule := range rules {
		if matchesCSSSelector(node, rule.Parts) {
			matches = append(matches, rule)
		}
	}
	return matches
}

// matchesCSSSelector 判断节点是否匹配由多段组成的选择器（含祖先关系）。
func matchesCSSSelector(node *html.Node, parts []pptCSSSelectorPart) bool {
	if len(parts) == 0 || node == nil {
		return false
	}
	partIndex := len(parts) - 1
	if !matchesSelectorPart(node, parts[partIndex]) {
		return false
	}
	partIndex--
	current := node.Parent
	for current != nil && partIndex >= 0 {
		if matchesSelectorPart(current, parts[partIndex]) {
			partIndex--
		}
		current = current.Parent
	}
	return partIndex < 0
}

// matchesSelectorPart 判断节点是否匹配单段选择器（标签与类名）。
func matchesSelectorPart(node *html.Node, part pptCSSSelectorPart) bool {
	if node == nil || node.Type != html.ElementNode {
		return false
	}
	if part.Tag != "" && part.Tag != "*" && !strings.EqualFold(node.Data, part.Tag) {
		return false
	}
	classSet := parseClassSet(node)
	for _, className := range part.Classes {
		if !classSet[className] {
			return false
		}
	}
	if part.FirstChild && !isFirstElementChild(node) {
		return false
	}
	return true
}

// isFirstElementChild 判断节点是否为其父节点的首个元素子节点。
func isFirstElementChild(node *html.Node) bool {
	if node == nil || node.Parent == nil {
		return false
	}
	for sibling := node.Parent.FirstChild; sibling != nil; sibling = sibling.NextSibling {
		if sibling.Type != html.ElementNode {
			continue
		}
		return sibling == node
	}
	return false
}

// parseCSSRules 解析 CSS 文本，提取变量与规则列表。
func parseCSSRules(css string, existingVars map[string]string, startOrder int) (map[string]string, []pptCSSRule) {
	vars := make(map[string]string)
	for key, value := range existingVars {
		vars[key] = value
	}

	cleaned := stripCSSComments(css)
	var rules []pptCSSRule
	order := startOrder
	for _, block := range splitTopLevelCSSBlocks(cleaned) {
		selector := strings.TrimSpace(block.selector)
		if selector == "" || strings.Contains(selector, "::") || strings.Contains(selector, ":last-child") {
			continue
		}
		declarations := parseInlineStyleDeclarations(resolveCSSVars(block.body, vars))
		if selector == ":root" {
			for _, value := range declarations {
				if strings.HasPrefix(value.Key, "--") {
					vars[value.Key] = value.Value
				}
			}
			continue
		}

		style := applyOrderedStyleDeclarations(pptStyle{}, declarations)
		for _, item := range strings.Split(selector, ",") {
			item = strings.TrimSpace(item)
			if item == "" || strings.Contains(item, "::") || strings.Contains(item, ":last-child") {
				continue
			}
			parts, specificity, ok := parseCSSSelector(item)
			if !ok {
				continue
			}
			rules = append(rules, pptCSSRule{
				Selector:    item,
				Parts:       parts,
				Style:       style,
				Specificity: specificity,
				Order:       order,
			})
			order++
		}
	}
	return vars, rules
}

type cssBlock struct {
	selector string
	body     string
}

// splitTopLevelCSSBlocks 将 CSS 文本拆分为顶层选择器与对应块体。
func splitTopLevelCSSBlocks(css string) []cssBlock {
	var blocks []cssBlock
	for i := 0; i < len(css); {
		for i < len(css) && isCSSWhitespace(css[i]) {
			i++
		}
		if i >= len(css) {
			break
		}
		if css[i] == '@' {
			i = skipAtRule(css, i)
			continue
		}
		start := i
		for i < len(css) && css[i] != '{' {
			i++
		}
		if i >= len(css) {
			break
		}
		selector := strings.TrimSpace(css[start:i])
		i++
		bodyStart := i
		depth := 1
		for i < len(css) && depth > 0 {
			switch css[i] {
			case '{':
				depth++
			case '}':
				depth--
			}
			i++
		}
		if depth != 0 {
			break
		}
		body := strings.TrimSpace(css[bodyStart : i-1])
		blocks = append(blocks, cssBlock{selector: selector, body: body})
	}
	return blocks
}

// skipAtRule 跳过 CSS 中的 @ 规则，返回新的索引位置。
func skipAtRule(css string, index int) int {
	for index < len(css) && css[index] != '{' && css[index] != ';' {
		index++
	}
	if index >= len(css) {
		return index
	}
	if css[index] == ';' {
		return index + 1
	}
	depth := 1
	index++
	for index < len(css) && depth > 0 {
		switch css[index] {
		case '{':
			depth++
		case '}':
			depth--
		}
		index++
	}
	return index
}

// stripCSSComments 移除 CSS 中的注释内容。
func stripCSSComments(css string) string {
	var b strings.Builder
	for i := 0; i < len(css); i++ {
		if i+1 < len(css) && css[i] == '/' && css[i+1] == '*' {
			i += 2
			for i+1 < len(css) && !(css[i] == '*' && css[i+1] == '/') {
				i++
			}
			i++
			continue
		}
		b.WriteByte(css[i])
	}
	return b.String()
}

// parseCSSSelector 解析选择器字符串，返回分段、特异性与是否解析成功。
func parseCSSSelector(selector string) ([]pptCSSSelectorPart, int, bool) {
	tokens := strings.Fields(selector)
	if len(tokens) == 0 {
		return nil, 0, false
	}
	parts := make([]pptCSSSelectorPart, 0, len(tokens))
	specificity := 0
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		part := pptCSSSelectorPart{}
		if strings.Contains(token, ":first-child") {
			part.FirstChild = true
			token = strings.ReplaceAll(token, ":first-child", "")
			specificity += 100
		}
		segments := strings.Split(token, ".")
		if len(segments) > 0 {
			head := strings.TrimSpace(segments[0])
			if head != "" {
				part.Tag = strings.ToLower(head)
				if part.Tag != "*" {
					specificity += 10
				}
			}
			for _, className := range segments[1:] {
				className = strings.TrimSpace(className)
				if className == "" {
					continue
				}
				part.Classes = append(part.Classes, className)
				specificity += 100
			}
		}
		if strings.HasPrefix(token, ".") && part.Tag == "" {
			part.Tag = "*"
		}
		if part.Tag == "" && len(part.Classes) == 0 && !part.FirstChild {
			return nil, 0, false
		}
		parts = append(parts, part)
	}
	return parts, specificity, len(parts) > 0
}

// isCSSWhitespace 判断字节是否为 CSS 中的空白字符。
func isCSSWhitespace(value byte) bool {
	return value == ' ' || value == '\n' || value == '\r' || value == '\t'
}

// inheritTextStyle 返回可被子节点继承的文本相关样式。
func inheritTextStyle(style pptStyle) pptStyle {
	return pptStyle{
		TextColor:           cloneColor(style.TextColor),
		FontSize:            cloneInt(style.FontSize),
		FontWeight:          cloneInt(style.FontWeight),
		LineHeight:          cloneFloat64(style.LineHeight),
		FontFamily:          style.FontFamily,
		TextAlign:           style.TextAlign,
		Display:             style.Display,
		GridTemplateColumns: style.GridTemplateColumns,
		FlexWrap:            style.FlexWrap,
	}
}

// mergePPTStyle 将 patch 中的非空样式字段合并到 base 上。
func mergePPTStyle(base, patch pptStyle) pptStyle {
	if patch.ClearBackground {
		base.BackgroundColor = nil
	}
	if patch.ClearBorder {
		base.BorderColor = nil
		base.BorderWidth = nil
	}
	if patch.ClearBorderLeft {
		base.BorderLeftColor = nil
		base.BorderLeftWidth = nil
	}
	if patch.ClearBorderBottom {
		base.BorderBottomColor = nil
		base.BorderBottomWidth = nil
	}
	if patch.TextColor != nil {
		base.TextColor = cloneColor(patch.TextColor)
	}
	if patch.BackgroundColor != nil {
		base.BackgroundColor = cloneColor(patch.BackgroundColor)
	}
	if patch.BorderColor != nil {
		base.BorderColor = cloneColor(patch.BorderColor)
	}
	if patch.BorderLeftColor != nil {
		base.BorderLeftColor = cloneColor(patch.BorderLeftColor)
	}
	if patch.BorderBottomColor != nil {
		base.BorderBottomColor = cloneColor(patch.BorderBottomColor)
	}
	if patch.FontSize != nil {
		base.FontSize = cloneInt(patch.FontSize)
	}
	if patch.FontWeight != nil {
		base.FontWeight = cloneInt(patch.FontWeight)
	}
	if patch.LineHeight != nil {
		base.LineHeight = cloneFloat64(patch.LineHeight)
	}
	if patch.FontFamily != "" {
		base.FontFamily = patch.FontFamily
	}
	if patch.TextAlign != "" {
		base.TextAlign = patch.TextAlign
	}
	if patch.Display != "" {
		base.Display = patch.Display
	}
	if patch.GridTemplateColumns != "" {
		base.GridTemplateColumns = patch.GridTemplateColumns
	}
	if patch.FlexWrap != "" {
		base.FlexWrap = patch.FlexWrap
	}
	if patch.Gap != nil {
		base.Gap = cloneFloat64(patch.Gap)
	}
	if patch.Padding.Set {
		base.Padding = patch.Padding
	}
	if patch.Margin.Set {
		base.Margin = patch.Margin
	}
	if patch.BorderWidth != nil {
		base.BorderWidth = cloneInt(patch.BorderWidth)
	}
	if patch.BorderLeftWidth != nil {
		base.BorderLeftWidth = cloneInt(patch.BorderLeftWidth)
	}
	if patch.BorderBottomWidth != nil {
		base.BorderBottomWidth = cloneInt(patch.BorderBottomWidth)
	}
	if patch.BorderRadius != nil {
		base.BorderRadius = cloneInt(patch.BorderRadius)
	}
	return base
}
