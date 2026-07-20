package ppt

import (
	bizerrors "YoudaoNoteLm/pkg/errors"
	"github.com/duynguyendang/docxgo/v3/pptx"
	"golang.org/x/net/html"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// buildDynamicHTMLPPTX 根据动态 HTML 内容构建 PPTX 字节流。
func buildDynamicHTMLPPTX(content, deckTitle string) ([]byte, error) {
	doc, err := parseDynamicHTMLDocument(content)
	if err != nil {
		return nil, err
	}
	if len(doc.Slides) == 0 {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "ppt export content does not contain any valid slides")
	}

	builder := pptx.NewPresentationBuilder(
		pptx.WithTitle(firstNonEmpty(deckTitle, "ppt-export")),
		pptx.WithLayout(pptx.Layout16x9),
	)

	layoutConfig := newDynamicLayoutConfig()
	for _, slideData := range doc.Slides {
		measured := measureDynamicHTMLSlide(doc, slideData, layoutConfig)
		renderMeasuredDynamicHTMLSlide(builder.AddSlide(), doc, measured)
	}

	presentation, err := builder.Build()
	if err != nil {
		return nil, err
	}

	tempDir, err := os.MkdirTemp("", "youdaonotelm-ppt-export-dynamic-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	path := filepath.Join(tempDir, "export.pptx")
	if err := presentation.SaveAs(path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return fixPPTXPackage(data, len(doc.Slides))
}

// parseDynamicHTMLDocument 解析动态 HTML 内容为文档对象。
func parseDynamicHTMLDocument(content string) (*pptHTMLDocument, error) {
	root, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInvalidParam, "invalid ppt export html", err)
	}

	doc := &pptHTMLDocument{
		Vars: make(map[string]string),
	}
	for _, cssText := range collectStyleTagContents(root) {
		vars, rules := parseCSSRules(cssText, doc.Vars, len(doc.Rules))
		for key, value := range vars {
			doc.Vars[key] = value
		}
		doc.Rules = append(doc.Rules, rules...)
	}

	bodyNode := findFirstHTMLElement(root, "body")
	if bodyNode != nil {
		doc.BodyStyle = computeNodeStyle(bodyNode, pptStyle{}, doc)
	}

	sections := findHTMLSections(root)
	if len(sections) == 0 {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "ppt export content must contain at least one <section> slide")
	}

	doc.Slides = make([]pptHTMLSlide, 0, len(sections))
	for _, section := range sections {
		slide := parseHTMLSection(section, doc)
		if len(slide.Blocks) == 0 {
			continue
		}
		doc.Slides = append(doc.Slides, slide)
	}
	return doc, nil
}

// collectStyleTagContents 收集所有 style 标签的内容。
func collectStyleTagContents(root *html.Node) []string {
	var values []string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil {
			return
		}
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, "style") {
			if css := strings.TrimSpace(extractRawNodeText(node)); css != "" {
				values = append(values, css)
			}
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return values
}

// findFirstHTMLElement 查找首个指定名称的 HTML 元素节点。
func findFirstHTMLElement(root *html.Node, name string) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil || found != nil {
			return
		}
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, name) {
			found = node
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return found
}

// findHTMLSections 查找所有 section 元素节点。
func findHTMLSections(root *html.Node) []*html.Node {
	var sections []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil {
			return
		}
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, "section") {
			sections = append(sections, node)
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return sections
}

// parseHTMLSection 解析 section 节点为幻灯片数据。
func parseHTMLSection(section *html.Node, doc *pptHTMLDocument) pptHTMLSlide {
	sectionStyle := computeNodeStyle(section, inheritTextStyle(doc.BodyStyle), doc)
	blocks := parseHTMLChildren(section, doc, inheritTextStyle(sectionStyle))
	return pptHTMLSlide{
		SectionStyle: sectionStyle,
		Blocks:       blocks,
	}
}

// parseHTMLChildren 解析节点的子节点为块列表。
func parseHTMLChildren(node *html.Node, doc *pptHTMLDocument, inheritedText pptStyle) []pptHTMLBlock {
	var blocks []pptHTMLBlock
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		blocks = append(blocks, parseHTMLBlocks(child, doc, inheritedText)...)
	}
	return blocks
}

// parseHTMLBlocks 解析 HTML 节点为块列表。
func parseHTMLBlocks(node *html.Node, doc *pptHTMLDocument, inheritedText pptStyle) []pptHTMLBlock {
	if node == nil {
		return nil
	}
	if node.Type == html.TextNode {
		text := normalizePPTExportText(node.Data)
		if text == "" {
			return nil
		}
		return []pptHTMLBlock{{
			Kind:  "text",
			Text:  text,
			Style: inheritedText,
		}}
	}
	if node.Type != html.ElementNode {
		return parseHTMLChildren(node, doc, inheritedText)
	}

	tag := strings.ToLower(node.Data)
	if shouldIgnoreHTMLElement(tag) {
		return nil
	}

	style := computeNodeStyle(node, inheritedText, doc)
	classes := parseClassSet(node)

	if tag == "span" && classes["section-number"] {
		text := normalizePPTExportText(extractNodeText(node))
		if text == "" {
			return nil
		}
		return []pptHTMLBlock{{
			Kind:    "section-number",
			Text:    text,
			Style:   style,
			Classes: classes,
		}}
	}

	if isLayoutContainer(tag, classes, style, node) {
		children := parseHTMLChildren(node, doc, inheritTextStyle(style))
		if len(children) == 0 {
			return nil
		}
		return []pptHTMLBlock{{
			Kind:     "container",
			Layout:   resolveContainerLayout(classes, style),
			Style:    style,
			Classes:  classes,
			Children: children,
		}}
	}

	if isCardBlock(tag, classes, style, node) {
		children := parseHTMLChildren(node, doc, inheritTextStyle(style))
		if len(children) == 0 {
			text := normalizePPTExportText(extractNodeText(node))
			if text != "" {
				children = append(children, pptHTMLBlock{
					Kind:  "text",
					Text:  text,
					Style: inheritTextStyle(style),
				})
			}
		}
		if len(children) == 0 {
			return nil
		}
		return []pptHTMLBlock{{
			Kind:     "card",
			Style:    style,
			Classes:  classes,
			Children: children,
		}}
	}

	switch tag {
	case "h1", "h2", "h3", "p":
		runs := extractInlineTextRuns(node, doc, style)
		text := textFromRuns(runs)
		if text == "" {
			return nil
		}
		return []pptHTMLBlock{{
			Kind:    tag,
			Text:    text,
			Runs:    runs,
			Style:   style,
			Classes: classes,
		}}
	case "pre":
		text := extractRawNodeText(node)
		if strings.TrimSpace(text) == "" {
			return nil
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode && strings.EqualFold(child.Data, "code") {
				text = extractRawNodeText(child)
				break
			}
		}
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return []pptHTMLBlock{{
			Kind:    "pre",
			Text:    text,
			Style:   style,
			Classes: classes,
		}}
	case "ul", "ol":
		return parseHTMLList(node, doc, inheritTextStyle(style), tag == "ol")
	case "li":
		runs := extractInlineTextRuns(node, doc, style)
		text := textFromRuns(runs)
		if text == "" {
			return nil
		}
		prefix := "- "
		runs = prependInlineRunPrefix(runs, prefix, style)
		return []pptHTMLBlock{{
			Kind:    "list-item",
			Text:    prefix + text,
			Runs:    runs,
			Style:   style,
			Classes: classes,
		}}
	case "span":
		text := normalizePPTExportText(extractNodeText(node))
		if text == "" {
			return nil
		}
		return []pptHTMLBlock{{
			Kind:    "text",
			Text:    text,
			Style:   style,
			Classes: classes,
		}}
	default:
		children := parseHTMLChildren(node, doc, inheritTextStyle(style))
		if len(children) > 0 {
			return children
		}
		text := normalizePPTExportText(extractNodeText(node))
		if text == "" {
			return nil
		}
		return []pptHTMLBlock{{
			Kind:    "p",
			Text:    text,
			Style:   style,
			Classes: classes,
		}}
	}
}

// shouldIgnoreHTMLElement 判断 HTML 标签是否应被忽略。
func shouldIgnoreHTMLElement(tag string) bool {
	switch tag {
	case "html", "head", "body", "style", "script", "meta", "title", "link":
		return true
	default:
		return false
	}
}

// isLayoutContainer 判断节点是否为布局容器。
func isLayoutContainer(tag string, classes map[string]bool, style pptStyle, node *html.Node) bool {
	if tag == "section" {
		return true
	}
	if classes["slider"] || classes["row"] || classes["dir-list"] {
		return true
	}
	if style.Display == "flex" || style.Display == "grid" {
		return true
	}
	if tag == "div" {
		return countMeaningfulElementChildren(node) > 1 && !hasCardVisualStyle(style)
	}
	return false
}

// resolveContainerLayout 解析容器的布局类型。
func resolveContainerLayout(classes map[string]bool, style pptStyle) string {
	switch {
	case classes["row"]:
		return "row"
	case classes["dir-list"]:
		return "grid"
	case style.Display == "grid":
		return "grid"
	case style.Display == "flex":
		return "row"
	default:
		return "stack"
	}
}

// isCardBlock 判断节点是否为卡片块。
func isCardBlock(tag string, classes map[string]bool, style pptStyle, node *html.Node) bool {
	if tag != "div" && tag != "aside" {
		return false
	}
	if classes["highlight"] || classes["highlight-box"] || classes["card"] || classes["dir-item"] || classes["footnote"] || classes["evidence"] || classes["callout"] {
		return true
	}
	return hasCardVisualStyle(style)
}

// hasCardVisualStyle 判断样式是否具有卡片视觉特征。
func hasCardVisualStyle(style pptStyle) bool {
	return style.BackgroundColor != nil || style.BorderColor != nil || style.BorderLeftColor != nil || style.BorderRadius != nil
}

// countMeaningfulElementChildren 统计有效子元素数量。
func countMeaningfulElementChildren(node *html.Node) int {
	count := 0
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		if shouldIgnoreHTMLElement(strings.ToLower(child.Data)) {
			continue
		}
		count++
	}
	return count
}

// parseHTMLList 解析 ul/ol 列表为块列表。
func parseHTMLList(node *html.Node, doc *pptHTMLDocument, inheritedText pptStyle, ordered bool) []pptHTMLBlock {
	var blocks []pptHTMLBlock
	index := 1
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode || !strings.EqualFold(child.Data, "li") {
			continue
		}
		style := computeNodeStyle(child, inheritedText, doc)
		runs := extractInlineTextRuns(child, doc, style)
		text := textFromRuns(runs)
		if text == "" {
			continue
		}
		prefix := "- "
		if ordered {
			prefix = strconv.Itoa(index) + ". "
		}
		runs = prependInlineRunPrefix(runs, prefix, style)
		blocks = append(blocks, pptHTMLBlock{
			Kind:    "list-item",
			Text:    prefix + text,
			Runs:    runs,
			Style:   style,
			Classes: parseClassSet(child),
		})
		index++
	}
	return blocks
}
