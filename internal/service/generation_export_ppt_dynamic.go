package service

import (
	stdhtml "html"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/duynguyendang/docxgo/v3/pptx"
	"golang.org/x/net/html"

	bizerrors "YoudaoNoteLm/pkg/errors"
)

const (
	dynamicPPTDefaultFontFamily = "Aptos"
	dynamicPPTTitleFontFamily   = "Aptos"
	dynamicPPTSlideWidth        = 13.333
	dynamicPPTSlideHeight       = 7.5
	dynamicPPTOuterMarginX      = 0.32
	dynamicPPTOuterMarginY      = 0.26
	dynamicPPTDefaultGap        = 0.14
	dynamicPPTDefaultBodyFont   = 17
	dynamicPPTMinBodyFontSize   = 14
)

var (
	dynamicPPTDefaultSlideBackground = pptx.Color{R: 246, G: 244, B: 239}
	dynamicPPTDefaultSectionFill     = pptx.Color{R: 252, G: 251, B: 248}
	dynamicPPTDefaultSectionBorder   = pptx.Color{R: 231, G: 224, B: 214}
	dynamicPPTDefaultText            = pptx.Color{R: 47, G: 42, B: 36}
	dynamicPPTDefaultMuted           = pptx.Color{R: 111, G: 104, B: 95}
	dynamicPPTDefaultAccent          = pptx.Color{R: 183, G: 170, B: 150}
)

type pptHTMLDocument struct {
	BodyStyle pptStyle
	Rules     []pptCSSRule
	Vars      map[string]string
	Slides    []pptHTMLSlide
}

type pptHTMLSlide struct {
	SectionStyle pptStyle
	Blocks       []pptHTMLBlock
}

type pptHTMLBlock struct {
	Kind     string
	Layout   string
	Text     string
	Runs     []pptHTMLTextRun
	Style    pptStyle
	Classes  map[string]bool
	Children []pptHTMLBlock
}

type pptHTMLTextRun struct {
	Text  string
	Style pptStyle
}

type pptCSSRule struct {
	Selector    string
	Parts       []pptCSSSelectorPart
	Style       pptStyle
	Specificity int
	Order       int
}

type pptCSSSelectorPart struct {
	Tag        string
	Classes    []string
	FirstChild bool
}

type pptStyleDeclaration struct {
	Key   string
	Value string
}

type pptStyle struct {
	TextColor           *pptx.Color
	BackgroundColor     *pptx.Color
	BorderColor         *pptx.Color
	BorderLeftColor     *pptx.Color
	BorderBottomColor   *pptx.Color
	FontSize            *int
	FontWeight          *int
	LineHeight          *float64
	FontFamily          string
	TextAlign           string
	Display             string
	GridTemplateColumns string
	FlexWrap            string
	Gap                 *float64
	Padding             pptEdges
	Margin              pptEdges
	BorderWidth         *int
	BorderLeftWidth     *int
	BorderBottomWidth   *int
	BorderRadius        *int
	ClearBackground     bool
	ClearBorder         bool
	ClearBorderLeft     bool
	ClearBorderBottom   bool
}

type pptEdges struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
	Set    bool
}

type pptLayoutCursor struct {
	x     float64
	y     float64
	width float64
}

type pptSectionFrame struct {
	x             float64
	y             float64
	width         float64
	height        float64
	paddingTop    float64
	paddingRight  float64
	paddingBottom float64
	paddingLeft   float64
}

type dynamicLayoutConfig struct {
	SlideWidth          float64
	SlideHeight         float64
	OuterMarginX        float64
	OuterMarginY        float64
	DefaultGap          float64
	ConservativeColumns bool
}

type measuredDynamicHTMLSlide struct {
	SectionStyle  pptStyle
	Frame         pptSectionFrame
	Blocks        []measuredDynamicBlock
	ContentBottom float64
}

type measuredDynamicBlock struct {
	Block    pptHTMLBlock
	X        float64
	Y        float64
	Width    float64
	Height   float64
	Children []measuredDynamicBlock
}

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

func parseHTMLSection(section *html.Node, doc *pptHTMLDocument) pptHTMLSlide {
	sectionStyle := computeNodeStyle(section, inheritTextStyle(doc.BodyStyle), doc)
	blocks := parseHTMLChildren(section, doc, inheritTextStyle(sectionStyle))
	return pptHTMLSlide{
		SectionStyle: sectionStyle,
		Blocks:       blocks,
	}
}

func parseHTMLChildren(node *html.Node, doc *pptHTMLDocument, inheritedText pptStyle) []pptHTMLBlock {
	var blocks []pptHTMLBlock
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		blocks = append(blocks, parseHTMLBlocks(child, doc, inheritedText)...)
	}
	return blocks
}

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
		// Code blocks: extract text preserving line breaks and indentation
		text := extractRawNodeText(node)
		if strings.TrimSpace(text) == "" {
			return nil
		}
		// If the first child is <code>, get its text instead
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

func shouldIgnoreHTMLElement(tag string) bool {
	switch tag {
	case "html", "head", "body", "style", "script", "meta", "title", "link":
		return true
	default:
		return false
	}
}

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

func isCardBlock(tag string, classes map[string]bool, style pptStyle, node *html.Node) bool {
	if tag != "div" && tag != "aside" {
		return false
	}
	if classes["highlight"] || classes["highlight-box"] || classes["card"] || classes["dir-item"] || classes["footnote"] || classes["evidence"] || classes["callout"] {
		return true
	}
	return hasCardVisualStyle(style)
}

func hasCardVisualStyle(style pptStyle) bool {
	return style.BackgroundColor != nil || style.BorderColor != nil || style.BorderLeftColor != nil || style.BorderRadius != nil
}

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

func isInlineWhitespace(r rune) bool {
	return r == ' ' || r == '\n' || r == '\r' || r == '\t'
}

func textFromRuns(runs []pptHTMLTextRun) string {
	var b strings.Builder
	for _, run := range runs {
		b.WriteString(run.Text)
	}
	return strings.TrimSpace(b.String())
}

func prependInlineRunPrefix(runs []pptHTMLTextRun, prefix string, style pptStyle) []pptHTMLTextRun {
	if strings.TrimSpace(prefix) == "" {
		return runs
	}
	prefixed := make([]pptHTMLTextRun, 0, len(runs)+1)
	prefixed = append(prefixed, pptHTMLTextRun{Text: prefix, Style: inheritInlineTextStyle(style)})
	prefixed = append(prefixed, runs...)
	return mergeAdjacentInlineRuns(prefixed)
}

func inheritInlineTextStyle(style pptStyle) pptStyle {
	return inheritTextStyle(style)
}

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

func inlineStylesEqual(a, b pptStyle) bool {
	return colorsEqual(a.TextColor, b.TextColor) &&
		intPointersEqual(a.FontSize, b.FontSize) &&
		intPointersEqual(a.FontWeight, b.FontWeight) &&
		a.FontFamily == b.FontFamily
}

func colorsEqual(a, b *pptx.Color) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func intPointersEqual(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

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

func matchingCSSRules(node *html.Node, rules []pptCSSRule) []pptCSSRule {
	matches := make([]pptCSSRule, 0, len(rules))
	for _, rule := range rules {
		if matchesCSSSelector(node, rule.Parts) {
			matches = append(matches, rule)
		}
	}
	return matches
}

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

func isCSSWhitespace(value byte) bool {
	return value == ' ' || value == '\n' || value == '\r' || value == '\t'
}

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

func applyStyleDeclarations(style pptStyle, declarations map[string]string) pptStyle {
	for rawKey, rawValue := range declarations {
		key := strings.ToLower(strings.TrimSpace(rawKey))
		value := strings.TrimSpace(rawValue)
		if key == "" || value == "" {
			continue
		}
		switch key {
		case "color":
			if color, ok := parsePPTColor(value); ok {
				style.TextColor = &color
			}
		case "background", "background-color":
			if isCSSNoneValue(value) {
				style.ClearBackground = true
				style.BackgroundColor = nil
				continue
			}
			if color, ok := parsePPTColor(value); ok {
				style.BackgroundColor = &color
			}
		case "font-size":
			if size := parseCSSFontSize(value); size > 0 {
				style.FontSize = intPtr(size)
			}
		case "font-weight":
			if weight := parseCSSFontWeight(value); weight > 0 {
				style.FontWeight = intPtr(weight)
			}
		case "line-height":
			if lineHeight := parseCSSLineHeight(value, style.FontSize); lineHeight > 0 {
				style.LineHeight = float64Ptr(lineHeight)
			}
		case "font-family":
			if family := normalizeFontFamily(value); family != "" {
				style.FontFamily = family
			}
		case "text-align":
			style.TextAlign = strings.ToLower(value)
		case "display":
			style.Display = strings.ToLower(value)
		case "grid-template-columns":
			style.GridTemplateColumns = strings.ToLower(value)
		case "flex-wrap":
			style.FlexWrap = strings.ToLower(value)
		case "gap":
			if gap := parseCSSSpacingInches(value); gap > 0 {
				style.Gap = float64Ptr(gap)
			}
		case "padding":
			if edges, ok := parseCSSBoxEdges(value); ok {
				style.Padding = edges
			}
		case "padding-top":
			style.Padding = updateEdge(style.Padding, "top", parseCSSSpacingInches(value))
		case "padding-right":
			style.Padding = updateEdge(style.Padding, "right", parseCSSSpacingInches(value))
		case "padding-bottom":
			style.Padding = updateEdge(style.Padding, "bottom", parseCSSSpacingInches(value))
		case "padding-left":
			style.Padding = updateEdge(style.Padding, "left", parseCSSSpacingInches(value))
		case "margin":
			if edges, ok := parseCSSBoxEdges(value); ok {
				style.Margin = edges
			}
		case "margin-top":
			style.Margin = updateEdge(style.Margin, "top", parseCSSSpacingInches(value))
		case "margin-bottom":
			style.Margin = updateEdge(style.Margin, "bottom", parseCSSSpacingInches(value))
		case "border":
			if isCSSNoneValue(value) {
				style.ClearBorder = true
				style.BorderColor = nil
				style.BorderWidth = nil
				continue
			}
			if width, color, ok := parseCSSBorder(value); ok {
				style.BorderWidth = intPtr(width)
				style.BorderColor = &color
			}
		case "border-width":
			if width := parseCSSBorderWidth(value); width > 0 {
				style.BorderWidth = intPtr(width)
			}
		case "border-color":
			if color, ok := parsePPTColor(value); ok {
				style.BorderColor = &color
			}
		case "border-radius":
			if radius := parseCSSRadius(value); radius > 0 {
				style.BorderRadius = intPtr(radius)
			}
		case "border-left":
			if isCSSNoneValue(value) {
				style.ClearBorderLeft = true
				style.BorderLeftColor = nil
				style.BorderLeftWidth = nil
				continue
			}
			if width, color, ok := parseCSSBorder(value); ok {
				style.BorderLeftWidth = intPtr(width)
				style.BorderLeftColor = &color
			}
		case "border-left-width":
			if width := parseCSSBorderWidth(value); width > 0 {
				style.BorderLeftWidth = intPtr(width)
			}
		case "border-left-color":
			if color, ok := parsePPTColor(value); ok {
				style.BorderLeftColor = &color
			}
		case "border-bottom":
			if isCSSNoneValue(value) {
				style.ClearBorderBottom = true
				style.BorderBottomColor = nil
				style.BorderBottomWidth = nil
				continue
			}
			if width, color, ok := parseCSSBorder(value); ok {
				style.BorderBottomWidth = intPtr(width)
				style.BorderBottomColor = &color
			}
		case "border-bottom-width":
			if width := parseCSSBorderWidth(value); width > 0 {
				style.BorderBottomWidth = intPtr(width)
			}
		case "border-bottom-color":
			if color, ok := parsePPTColor(value); ok {
				style.BorderBottomColor = &color
			}
		}
	}
	return style
}

func applyOrderedStyleDeclarations(style pptStyle, declarations []pptStyleDeclaration) pptStyle {
	for _, declaration := range declarations {
		style = applyStyleDeclarations(style, map[string]string{
			declaration.Key: declaration.Value,
		})
	}
	return style
}

func newDynamicLayoutConfig() dynamicLayoutConfig {
	return dynamicLayoutConfig{
		SlideWidth:          dynamicPPTSlideWidth,
		SlideHeight:         dynamicPPTSlideHeight,
		OuterMarginX:        dynamicPPTOuterMarginX,
		OuterMarginY:        dynamicPPTOuterMarginY,
		DefaultGap:          dynamicPPTDefaultGap,
		ConservativeColumns: true,
	}
}

func renderDynamicHTMLSlide(slide *pptx.SlideBuilder, doc *pptHTMLDocument, slideData pptHTMLSlide) {
	measured := measureDynamicHTMLSlide(doc, slideData, newDynamicLayoutConfig())
	renderMeasuredDynamicHTMLSlide(slide, doc, measured)
}

func renderMeasuredDynamicHTMLSlide(slide *pptx.SlideBuilder, doc *pptHTMLDocument, measured measuredDynamicHTMLSlide) {
	slide.SetBackgroundColor(resolveSlideBackground(doc.BodyStyle))

	renderSectionFrameAt(slide, measured.Frame, measured.SectionStyle)
	for _, block := range measured.Blocks {
		renderMeasuredDynamicBlock(slide, block)
	}
}

func renderSectionFrame(slide *pptx.SlideBuilder, style pptStyle) pptSectionFrame {
	frame := pptSectionFrame{
		x:             dynamicPPTOuterMarginX,
		y:             dynamicPPTOuterMarginY,
		width:         dynamicPPTSlideWidth - dynamicPPTOuterMarginX*2,
		height:        dynamicPPTSlideHeight - dynamicPPTOuterMarginY*2,
		paddingTop:    edgeOr(style.Padding, "top", 0.36),
		paddingRight:  edgeOr(style.Padding, "right", 0.42),
		paddingBottom: edgeOr(style.Padding, "bottom", 0.36),
		paddingLeft:   edgeOr(style.Padding, "left", 0.42),
	}

	fill := resolveSectionFill(style)
	borderColor := resolveBorderColor(style, dynamicPPTDefaultSectionBorder)
	borderWidth := dynamicPPTValueOrInt(style.BorderWidth, 1)
	shapeType := pptx.ShapeRoundedRectangle
	if dynamicPPTValueOrInt(style.BorderRadius, 28) <= 0 {
		shapeType = pptx.ShapeRectangle
	}
	slide.AddShape(shapeType).
		SetPosition(pptx.Inches(frame.x), pptx.Inches(frame.y)).
		SetSize(pptx.Inches(frame.width), pptx.Inches(frame.height)).
		SetFillColor(fill).
		SetLine(borderColor, borderWidth).
		End()

	if style.BorderBottomColor != nil {
		height := 0.05
		width := frame.width - 0.4
		if dynamicPPTValueOrInt(style.BorderBottomWidth, 0) >= 3 {
			height = 0.06
		}
		slide.AddShape(pptx.ShapeRectangle).
			SetPosition(pptx.Inches(frame.x+0.2), pptx.Inches(frame.y+frame.height-height-0.02)).
			SetSize(pptx.Inches(width), pptx.Inches(height)).
			SetFillColor(*style.BorderBottomColor).
			SetNoLine().
			End()
	}

	return frame
}

func renderSectionFrameAt(slide *pptx.SlideBuilder, frame pptSectionFrame, style pptStyle) {
	shapeType := pptx.ShapeRoundedRectangle
	if dynamicPPTValueOrInt(style.BorderRadius, 28) <= 0 {
		shapeType = pptx.ShapeRectangle
	}
	slide.AddShape(shapeType).
		SetPosition(pptx.Inches(frame.x), pptx.Inches(frame.y)).
		SetSize(pptx.Inches(frame.width), pptx.Inches(frame.height)).
		SetFillColor(resolveSectionFill(style)).
		SetLine(resolveBorderColor(style, dynamicPPTDefaultSectionBorder), dynamicPPTValueOrInt(style.BorderWidth, 1)).
		End()
	if style.BorderBottomColor != nil {
		height := 0.05
		width := frame.width - 0.4
		if dynamicPPTValueOrInt(style.BorderBottomWidth, 0) >= 3 {
			height = 0.06
		}
		slide.AddShape(pptx.ShapeRectangle).
			SetPosition(pptx.Inches(frame.x+0.2), pptx.Inches(frame.y+frame.height-height-0.02)).
			SetSize(pptx.Inches(width), pptx.Inches(height)).
			SetFillColor(*style.BorderBottomColor).
			SetNoLine().
			End()
	}
}

func measureDynamicHTMLSlide(doc *pptHTMLDocument, slideData pptHTMLSlide, config dynamicLayoutConfig) measuredDynamicHTMLSlide {
	frame := pptSectionFrame{
		x:             config.OuterMarginX,
		y:             config.OuterMarginY,
		width:         config.SlideWidth - config.OuterMarginX*2,
		height:        config.SlideHeight - config.OuterMarginY*2,
		paddingTop:    edgeOr(slideData.SectionStyle.Padding, "top", 0.36),
		paddingRight:  edgeOr(slideData.SectionStyle.Padding, "right", 0.42),
		paddingBottom: edgeOr(slideData.SectionStyle.Padding, "bottom", 0.36),
		paddingLeft:   edgeOr(slideData.SectionStyle.Padding, "left", 0.42),
	}
	cursor := &pptLayoutCursor{
		x:     frame.x + frame.paddingLeft,
		y:     frame.y + frame.paddingTop,
		width: frame.width - frame.paddingLeft - frame.paddingRight,
	}
	measuredBlocks := measureDynamicBlocks(slideData.Blocks, cursor, config)
	contentBottom := cursor.y
	_ = doc
	return measuredDynamicHTMLSlide{
		SectionStyle:  slideData.SectionStyle,
		Frame:         frame,
		Blocks:        measuredBlocks,
		ContentBottom: contentBottom,
	}
}

func measureDynamicBlocks(blocks []pptHTMLBlock, cursor *pptLayoutCursor, config dynamicLayoutConfig) []measuredDynamicBlock {
	measured := make([]measuredDynamicBlock, 0, len(blocks))
	for _, block := range blocks {
		entry := measureDynamicBlock(block, cursor, config)
		if entry.Height <= 0 && len(entry.Children) == 0 && block.Kind != "section-number" {
			continue
		}
		measured = append(measured, entry)
	}
	return measured
}

func measureDynamicBlock(block pptHTMLBlock, cursor *pptLayoutCursor, config dynamicLayoutConfig) measuredDynamicBlock {
	switch block.Kind {
	case "section-number":
		return measuredDynamicBlock{
			Block:  block,
			X:      dynamicPPTSlideWidth - 1.35,
			Y:      0.38,
			Width:  0.88,
			Height: 0.24,
		}
	case "container":
		return measureContainerBlock(block, cursor, config)
	case "card":
		return measureCardBlock(block, cursor, config)
	default:
		return measureTextBlock(block, cursor)
	}
}

func measureContainerBlock(block pptHTMLBlock, cursor *pptLayoutCursor, config dynamicLayoutConfig) measuredDynamicBlock {
	startY := cursor.y + edgeOr(block.Style.Margin, "top", 0)
	cursor.y = startY
	measured := measuredDynamicBlock{
		Block: block,
		X:     cursor.x,
		Y:     startY,
		Width: cursor.width,
	}
	switch block.Layout {
	case "row", "grid":
		measured.Children = measureGridChildren(block, cursor, config)
	default:
		childCursor := &pptLayoutCursor{x: cursor.x, y: cursor.y, width: cursor.width}
		measured.Children = measureDynamicBlocks(block.Children, childCursor, config)
		cursor.y = childCursor.y
	}
	measured.Height = cursor.y - startY + edgeOr(block.Style.Margin, "bottom", 0)
	cursor.y += edgeOr(block.Style.Margin, "bottom", 0)
	return measured
}

func measureGridChildren(block pptHTMLBlock, cursor *pptLayoutCursor, config dynamicLayoutConfig) []measuredDynamicBlock {
	if len(block.Children) == 0 {
		return nil
	}
	cols := resolveContainerColumns(block, cursor.width, config.ConservativeColumns)
	if cols <= 1 {
		return measureDynamicBlocks(block.Children, cursor, config)
	}
	gap := resolveGap(block.Style, 0.18)
	cellWidth := (cursor.width - gap*float64(cols-1)) / float64(cols)
	currentY := cursor.y
	measured := make([]measuredDynamicBlock, 0, len(block.Children))
	for start := 0; start < len(block.Children); start += cols {
		end := start + cols
		if end > len(block.Children) {
			end = len(block.Children)
		}
		row := make([]measuredDynamicBlock, 0, end-start)
		rowHeight := 0.0
		for i, child := range block.Children[start:end] {
			cellCursor := &pptLayoutCursor{
				x:     cursor.x + float64(i)*(cellWidth+gap),
				y:     currentY,
				width: cellWidth,
			}
			childMeasured := measureDynamicBlockInRect(child, cellCursor, config)
			if childMeasured.Height > rowHeight {
				rowHeight = childMeasured.Height
			}
			row = append(row, childMeasured)
		}
		for i := range row {
			row[i].Height = rowHeight
		}
		measured = append(measured, row...)
		currentY += rowHeight + gap
	}
	cursor.y = currentY + config.DefaultGap
	return measured
}

func measureDynamicBlockInRect(block pptHTMLBlock, cursor *pptLayoutCursor, config dynamicLayoutConfig) measuredDynamicBlock {
	switch block.Kind {
	case "card":
		return measureCardAt(block, cursor.x, cursor.y, cursor.width, config)
	default:
		textBlock := block
		textBlock.Style.Margin = pptEdges{}
		textCursor := &pptLayoutCursor{x: cursor.x, y: cursor.y, width: cursor.width}
		return measureTextBlock(textBlock, textCursor)
	}
}

func measureCardBlock(block pptHTMLBlock, cursor *pptLayoutCursor, config dynamicLayoutConfig) measuredDynamicBlock {
	cursor.y += edgeOr(block.Style.Margin, "top", 0)
	measured := measureCardAt(block, cursor.x, cursor.y, cursor.width, config)
	cursor.y += measured.Height + edgeOr(block.Style.Margin, "bottom", dynamicPPTDefaultGap)
	return measured
}

func measureCardAt(block pptHTMLBlock, x, y, width float64, config dynamicLayoutConfig) measuredDynamicBlock {
	contentWidth := width - edgeOr(block.Style.Padding, "left", 0.22) - edgeOr(block.Style.Padding, "right", 0.22)
	if contentWidth < 0.5 {
		contentWidth = width - 0.18
	}
	innerCursor := &pptLayoutCursor{
		x:     x + edgeOr(block.Style.Padding, "left", 0.22),
		y:     y + edgeOr(block.Style.Padding, "top", 0.18),
		width: contentWidth,
	}
	children := measureDynamicBlocks(block.Children, innerCursor, config)
	height := innerCursor.y - y + edgeOr(block.Style.Padding, "bottom", 0.18)
	if height < 0.56 {
		height = 0.56
	}
	return measuredDynamicBlock{
		Block:    block,
		X:        x,
		Y:        y,
		Width:    width,
		Height:   height,
		Children: children,
	}
}

func measureTextBlock(block pptHTMLBlock, cursor *pptLayoutCursor) measuredDynamicBlock {
	if strings.TrimSpace(block.Text) == "" {
		return measuredDynamicBlock{Block: block}
	}
	cursor.y += edgeOr(block.Style.Margin, "top", 0)
	fontSize := resolveBlockFontSize(block, defaultFontSizeForBlock(block.Kind))
	x := cursor.x + edgeOr(block.Style.Padding, "left", 0)
	width := cursor.width - edgeOr(block.Style.Padding, "left", 0) - edgeOr(block.Style.Padding, "right", 0)
	if width <= 0.2 {
		width = cursor.width
	}
	height := estimateTextHeightWithStyle(block.Text, block.Style, fontSize, width)
	measured := measuredDynamicBlock{
		Block:  block,
		X:      x,
		Y:      cursor.y,
		Width:  width,
		Height: height,
	}
	cursor.y += height + edgeOr(block.Style.Margin, "bottom", dynamicPPTDefaultGap)
	return measured
}

func renderMeasuredDynamicBlock(slide *pptx.SlideBuilder, measured measuredDynamicBlock) {
	switch measured.Block.Kind {
	case "section-number":
		renderSectionNumber(slide, measured.Block)
	case "container":
		for _, child := range measured.Children {
			renderMeasuredDynamicBlock(slide, child)
		}
	case "card":
		renderCardChrome(slide, measured.Block, measured.X, measured.Y, measured.Width, measured.Height)
		for _, child := range measured.Children {
			renderMeasuredDynamicBlock(slide, child)
		}
	default:
		renderTextAt(slide, measured)
	}
}

func renderTextAt(slide *pptx.SlideBuilder, measured measuredDynamicBlock) {
	block := measured.Block
	if strings.TrimSpace(block.Text) == "" {
		return
	}
	if len(block.Runs) > 1 {
		renderInlineRunsAt(slide, measured)
		return
	}
	fontSize := resolveBlockFontSize(block, defaultFontSizeForBlock(block.Kind))
	fontFamily := resolveFontFamily(block.Style, defaultFontFamilyForBlock(block.Kind))
	color := resolveTextColor(block.Style, dynamicPPTDefaultText)
	text := slide.AddText(block.Text).
		SetFontSize(fontSize).
		SetFontFamily(fontFamily).
		SetColor(color).
		SetAlignment(resolveAlignment(block.Style.TextAlign)).
		SetPosition(pptx.Inches(measured.X), pptx.Inches(measured.Y)).
		SetSize(pptx.Inches(measured.Width), pptx.Inches(measured.Height))
	if dynamicPPTValueOrInt(block.Style.FontWeight, 0) >= 600 || block.Kind == "h1" || block.Kind == "h2" || block.Kind == "h3" {
		text.SetBold(true)
	}
	text.End()

	if block.Style.BorderLeftColor != nil {
		leftWidth := 0.04
		if dynamicPPTValueOrInt(block.Style.BorderLeftWidth, 0) >= 4 {
			leftWidth = 0.05
		}
		slide.AddShape(pptx.ShapeRectangle).
			SetPosition(pptx.Inches(measured.X-edgeOr(block.Style.Padding, "left", 0)), pptx.Inches(measured.Y+0.02)).
			SetSize(pptx.Inches(leftWidth), pptx.Inches(measured.Height-0.02)).
			SetFillColor(*block.Style.BorderLeftColor).
			SetNoLine().
			End()
	}
}

func renderInlineRunsAt(slide *pptx.SlideBuilder, measured measuredDynamicBlock) {
	block := measured.Block
	x := measured.X
	y := measured.Y
	remainingWidth := measured.Width
	totalWeight := inlineRunsWidthWeight(block.Runs, block)
	if totalWeight <= 0 {
		renderPlainTextAt(slide, measured, block)
		return
	}
	for i, run := range block.Runs {
		runText := run.Text
		if runText == "" {
			continue
		}
		runBlock := block
		runBlock.Text = runText
		runBlock.Runs = nil
		runBlock.Style = mergePPTStyle(block.Style, run.Style)
		runWeight := inlineRunWidthWeight(run, runBlock)
		width := measured.Width * runWeight / totalWeight
		if i == len(block.Runs)-1 || width > remainingWidth {
			width = remainingWidth
		}
		if width <= 0 {
			continue
		}
		renderPlainTextAt(slide, measuredDynamicBlock{
			Block:  runBlock,
			X:      x,
			Y:      y,
			Width:  width,
			Height: measured.Height,
		}, runBlock)
		x += width
		remainingWidth -= width
		if remainingWidth <= 0 {
			break
		}
	}
}

func renderPlainTextAt(slide *pptx.SlideBuilder, measured measuredDynamicBlock, block pptHTMLBlock) {
	fontSize := resolveBlockFontSize(block, defaultFontSizeForBlock(block.Kind))
	fontFamily := resolveFontFamily(block.Style, defaultFontFamilyForBlock(block.Kind))
	color := resolveTextColor(block.Style, dynamicPPTDefaultText)
	text := slide.AddText(block.Text).
		SetFontSize(fontSize).
		SetFontFamily(fontFamily).
		SetColor(color).
		SetAlignment(resolveAlignment(block.Style.TextAlign)).
		SetPosition(pptx.Inches(measured.X), pptx.Inches(measured.Y)).
		SetSize(pptx.Inches(measured.Width), pptx.Inches(measured.Height))
	if dynamicPPTValueOrInt(block.Style.FontWeight, 0) >= 600 || block.Kind == "h1" || block.Kind == "h2" || block.Kind == "h3" {
		text.SetBold(true)
	}
	text.End()
}

func inlineRunsWidthWeight(runs []pptHTMLTextRun, block pptHTMLBlock) float64 {
	total := 0.0
	for _, run := range runs {
		runBlock := block
		runBlock.Text = run.Text
		runBlock.Style = mergePPTStyle(block.Style, run.Style)
		total += inlineRunWidthWeight(run, runBlock)
	}
	return total
}

func inlineRunWidthWeight(run pptHTMLTextRun, block pptHTMLBlock) float64 {
	size := resolveBlockFontSize(block, defaultFontSizeForBlock(block.Kind))
	weight := float64(len([]rune(run.Text))) * float64(size)
	if dynamicPPTValueOrInt(block.Style.FontWeight, 0) >= 600 {
		weight *= 1.05
	}
	if weight <= 0 {
		return 1
	}
	return weight
}

func renderDynamicBlock(slide *pptx.SlideBuilder, cursor *pptLayoutCursor, block pptHTMLBlock) {
	switch block.Kind {
	case "section-number":
		renderSectionNumber(slide, block)
	case "container":
		renderContainerBlock(slide, cursor, block)
	case "card":
		renderCardBlock(slide, cursor, block)
	default:
		renderTextBlock(slide, cursor, block)
	}
}

func renderSectionNumber(slide *pptx.SlideBuilder, block pptHTMLBlock) {
	fontSize := resolveBlockFontSize(block, 13)
	text := slide.AddText(block.Text).
		SetFontSize(fontSize).
		SetFontFamily(resolveFontFamily(block.Style, dynamicPPTDefaultFontFamily)).
		SetColor(resolveTextColor(block.Style, dynamicPPTDefaultMuted)).
		SetAlignment(resolveAlignment(block.Style.TextAlign)).
		SetPosition(pptx.Inches(dynamicPPTSlideWidth-1.35), pptx.Inches(0.38)).
		SetSize(pptx.Inches(0.88), pptx.Inches(0.24))
	if dynamicPPTValueOrInt(block.Style.FontWeight, 0) >= 600 {
		text.SetBold(true)
	}
	text.End()
}

func renderContainerBlock(slide *pptx.SlideBuilder, cursor *pptLayoutCursor, block pptHTMLBlock) {
	cursor.y += edgeOr(block.Style.Margin, "top", 0)
	switch block.Layout {
	case "row", "grid":
		renderGridContainer(slide, cursor, block)
	default:
		for _, child := range block.Children {
			renderDynamicBlock(slide, cursor, child)
		}
	}
	cursor.y += edgeOr(block.Style.Margin, "bottom", 0)
}

func renderGridContainer(slide *pptx.SlideBuilder, cursor *pptLayoutCursor, block pptHTMLBlock) {
	if len(block.Children) == 0 {
		return
	}
	cols := resolveContainerColumns(block, cursor.width, false)
	if cols <= 1 {
		for _, child := range block.Children {
			renderDynamicBlock(slide, cursor, child)
		}
		return
	}

	gap := resolveGap(block.Style, 0.18)
	cellWidth := (cursor.width - gap*float64(cols-1)) / float64(cols)
	currentY := cursor.y

	for start := 0; start < len(block.Children); start += cols {
		end := start + cols
		if end > len(block.Children) {
			end = len(block.Children)
		}
		row := block.Children[start:end]
		maxHeight := 0.0
		heights := make([]float64, len(row))
		for i, child := range row {
			heights[i] = estimateRenderedHeight(child, cellWidth)
			if heights[i] > maxHeight {
				maxHeight = heights[i]
			}
		}
		for i, child := range row {
			x := cursor.x + float64(i)*(cellWidth+gap)
			renderBlockInRect(slide, child, x, currentY, cellWidth, maxHeight)
		}
		currentY += maxHeight + gap
	}

	cursor.y = currentY + dynamicPPTDefaultGap
}

func renderBlockInRect(slide *pptx.SlideBuilder, block pptHTMLBlock, x, y, width, height float64) {
	switch block.Kind {
	case "card":
		renderCardInRect(slide, block, x, y, width, height)
	default:
		textBlock := block
		textBlock.Style.Margin = pptEdges{}
		textCursor := &pptLayoutCursor{x: x, y: y, width: width}
		renderTextBlock(slide, textCursor, textBlock)
	}
}

func renderCardBlock(slide *pptx.SlideBuilder, cursor *pptLayoutCursor, block pptHTMLBlock) {
	cursor.y += edgeOr(block.Style.Margin, "top", 0)
	height := estimateRenderedHeight(block, cursor.width)
	renderCardInRect(slide, block, cursor.x, cursor.y, cursor.width, height)
	cursor.y += height + edgeOr(block.Style.Margin, "bottom", dynamicPPTDefaultGap)
}

func renderCardInRect(slide *pptx.SlideBuilder, block pptHTMLBlock, x, y, width, height float64) {
	renderCardChrome(slide, block, x, y, width, height)
	innerCursor := &pptLayoutCursor{
		x:     x + edgeOr(block.Style.Padding, "left", 0.22),
		y:     y + edgeOr(block.Style.Padding, "top", 0.18),
		width: width - edgeOr(block.Style.Padding, "left", 0.22) - edgeOr(block.Style.Padding, "right", 0.22),
	}
	if innerCursor.width < 0.5 {
		innerCursor.width = width - 0.18
	}
	for _, child := range block.Children {
		renderDynamicBlock(slide, innerCursor, child)
	}
}

func renderCardChrome(slide *pptx.SlideBuilder, block pptHTMLBlock, x, y, width, height float64) {
	fill := resolveCardFill(block.Style)
	borderColor := resolveBorderColor(block.Style, dynamicPPTDefaultSectionBorder)
	borderWidth := dynamicPPTValueOrInt(block.Style.BorderWidth, 1)
	radius := dynamicPPTValueOrInt(block.Style.BorderRadius, 18)
	shapeType := pptx.ShapeRoundedRectangle
	if radius <= 0 {
		shapeType = pptx.ShapeRectangle
	}
	slide.AddShape(shapeType).
		SetPosition(pptx.Inches(x), pptx.Inches(y)).
		SetSize(pptx.Inches(width), pptx.Inches(height)).
		SetFillColor(fill).
		SetLine(borderColor, borderWidth).
		End()

	if block.Style.BorderLeftColor != nil {
		leftWidth := 0.05
		if dynamicPPTValueOrInt(block.Style.BorderLeftWidth, 0) >= 3 {
			leftWidth = 0.06
		}
		slide.AddShape(pptx.ShapeRectangle).
			SetPosition(pptx.Inches(x), pptx.Inches(y+0.03)).
			SetSize(pptx.Inches(leftWidth), pptx.Inches(height-0.06)).
			SetFillColor(*block.Style.BorderLeftColor).
			SetNoLine().
			End()
	}
}

func renderTextBlock(slide *pptx.SlideBuilder, cursor *pptLayoutCursor, block pptHTMLBlock) {
	if strings.TrimSpace(block.Text) == "" {
		return
	}
	cursor.y += edgeOr(block.Style.Margin, "top", 0)

	fontSize := resolveBlockFontSize(block, defaultFontSizeForBlock(block.Kind))
	fontFamily := resolveFontFamily(block.Style, defaultFontFamilyForBlock(block.Kind))
	color := resolveTextColor(block.Style, dynamicPPTDefaultText)
	height := estimateTextHeight(block.Text, fontSize, cursor.width)
	x := cursor.x + edgeOr(block.Style.Padding, "left", 0)
	width := cursor.width - edgeOr(block.Style.Padding, "left", 0) - edgeOr(block.Style.Padding, "right", 0)
	if width <= 0.2 {
		width = cursor.width
	}
	text := slide.AddText(block.Text).
		SetFontSize(fontSize).
		SetFontFamily(fontFamily).
		SetColor(color).
		SetAlignment(resolveAlignment(block.Style.TextAlign)).
		SetPosition(pptx.Inches(x), pptx.Inches(cursor.y)).
		SetSize(pptx.Inches(width), pptx.Inches(height))
	if dynamicPPTValueOrInt(block.Style.FontWeight, 0) >= 600 || block.Kind == "h1" || block.Kind == "h2" || block.Kind == "h3" {
		text.SetBold(true)
	}
	text.End()

	if block.Style.BorderLeftColor != nil {
		leftWidth := 0.04
		if dynamicPPTValueOrInt(block.Style.BorderLeftWidth, 0) >= 4 {
			leftWidth = 0.05
		}
		slide.AddShape(pptx.ShapeRectangle).
			SetPosition(pptx.Inches(cursor.x), pptx.Inches(cursor.y+0.02)).
			SetSize(pptx.Inches(leftWidth), pptx.Inches(height-0.02)).
			SetFillColor(*block.Style.BorderLeftColor).
			SetNoLine().
			End()
	}

	cursor.y += height + edgeOr(block.Style.Margin, "bottom", dynamicPPTDefaultGap)
}

func estimateRenderedHeight(block pptHTMLBlock, width float64) float64 {
	switch block.Kind {
	case "container":
		if block.Layout == "row" || block.Layout == "grid" {
			cols := resolveContainerColumns(block, width, false)
			if cols <= 0 {
				cols = 1
			}
			gap := resolveGap(block.Style, 0.18)
			cellWidth := width
			if cols > 1 {
				cellWidth = (width - gap*float64(cols-1)) / float64(cols)
			}
			total := 0.0
			for start := 0; start < len(block.Children); start += cols {
				end := start + cols
				if end > len(block.Children) {
					end = len(block.Children)
				}
				rowHeight := 0.0
				for _, child := range block.Children[start:end] {
					h := estimateRenderedHeight(child, cellWidth)
					if h > rowHeight {
						rowHeight = h
					}
				}
				total += rowHeight + gap
			}
			if total == 0 {
				return 0
			}
			return total + edgeOr(block.Style.Margin, "top", 0) + edgeOr(block.Style.Margin, "bottom", 0)
		}
		total := 0.0
		for _, child := range block.Children {
			total += estimateRenderedHeight(child, width)
		}
		return total + edgeOr(block.Style.Margin, "top", 0) + edgeOr(block.Style.Margin, "bottom", 0)
	case "card":
		contentWidth := width - edgeOr(block.Style.Padding, "left", 0.22) - edgeOr(block.Style.Padding, "right", 0.22)
		if contentWidth < 0.5 {
			contentWidth = width - 0.18
		}
		total := edgeOr(block.Style.Padding, "top", 0.18) + edgeOr(block.Style.Padding, "bottom", 0.18)
		for _, child := range block.Children {
			total += estimateRenderedHeight(child, contentWidth)
		}
		if total < 0.56 {
			total = 0.56
		}
		return total + edgeOr(block.Style.Margin, "top", 0) + edgeOr(block.Style.Margin, "bottom", 0)
	case "section-number":
		return 0
	default:
		fontSize := resolveBlockFontSize(block, defaultFontSizeForBlock(block.Kind))
		height := estimateTextHeightWithStyle(block.Text, block.Style, fontSize, width)
		return height + edgeOr(block.Style.Margin, "top", 0) + edgeOr(block.Style.Margin, "bottom", dynamicPPTDefaultGap)
	}
}

func resolveContainerColumns(block pptHTMLBlock, width float64, conservative bool) int {
	template := strings.ToLower(strings.TrimSpace(block.Style.GridTemplateColumns))
	if conservative && strings.Contains(template, "repeat(") && strings.Contains(template, "auto-fit") && strings.Contains(template, "minmax(") {
		return dynamicPPTMaxInt(1, dynamicPPTMinInt(2, len(block.Children)))
	}
	if strings.Contains(template, "repeat(") && strings.Contains(template, "auto-fit") && strings.Contains(template, "minmax(") {
		if width >= 6.0 && len(block.Children) >= 3 {
			return 3
		}
		if len(block.Children) >= 2 {
			return 2
		}
	}
	switch {
	case block.Classes["dir-list"]:
		if len(block.Children) >= 6 {
			return 3
		}
		if len(block.Children) >= 4 {
			return 2
		}
		return dynamicPPTMaxInt(1, len(block.Children))
	case block.Classes["row"]:
		if len(block.Children) >= 3 {
			return 3
		}
		return dynamicPPTMaxInt(1, len(block.Children))
	default:
		if len(block.Children) >= 3 {
			return 3
		}
		if len(block.Children) == 2 {
			return 2
		}
		return 1
	}
}

func resolveSlideBackground(style pptStyle) pptx.Color {
	if style.BackgroundColor != nil {
		return *style.BackgroundColor
	}
	return dynamicPPTDefaultSlideBackground
}

func resolveSectionFill(style pptStyle) pptx.Color {
	if style.BackgroundColor != nil {
		return *style.BackgroundColor
	}
	return dynamicPPTDefaultSectionFill
}

func resolveCardFill(style pptStyle) pptx.Color {
	if style.BackgroundColor != nil {
		return *style.BackgroundColor
	}
	return pptx.Color{R: 252, G: 251, B: 248}
}

func resolveTextColor(style pptStyle, fallback pptx.Color) pptx.Color {
	if style.TextColor != nil {
		return *style.TextColor
	}
	return fallback
}

func resolveBorderColor(style pptStyle, fallback pptx.Color) pptx.Color {
	if style.BorderColor != nil {
		return *style.BorderColor
	}
	return fallback
}

func resolveGap(style pptStyle, fallback float64) float64 {
	if style.Gap != nil && *style.Gap > 0 {
		return *style.Gap
	}
	return fallback
}

func resolveBlockFontSize(block pptHTMLBlock, fallback int) int {
	size := fallback
	if block.Style.FontSize != nil && *block.Style.FontSize > 0 {
		size = *block.Style.FontSize
	}
	if block.Kind != "h1" && block.Kind != "h2" && block.Kind != "h3" && size < dynamicPPTMinBodyFontSize {
		size = dynamicPPTMinBodyFontSize
	}
	return size
}

func defaultFontSizeForBlock(kind string) int {
	switch kind {
	case "h1":
		return 34
	case "h2":
		return 24
	case "h3":
		return 19
	case "section-number":
		return 13
	case "pre":
		return 14
	case "list-item":
		return dynamicPPTDefaultBodyFont
	default:
		return dynamicPPTDefaultBodyFont
	}
}

func defaultFontFamilyForBlock(kind string) string {
	switch kind {
	case "h1", "h2", "h3":
		return dynamicPPTTitleFontFamily
	case "pre":
		return "Consolas"
	default:
		return dynamicPPTDefaultFontFamily
	}
}

func resolveFontFamily(style pptStyle, fallback string) string {
	if strings.TrimSpace(style.FontFamily) == "" {
		return fallback
	}
	return style.FontFamily
}

func resolveAlignment(value string) pptx.Alignment {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "center":
		return pptx.AlignmentCenter
	case "right":
		return pptx.AlignmentRight
	case "justify":
		return pptx.AlignmentJustify
	default:
		return pptx.AlignmentLeft
	}
}

func containsCJK(text string) bool {
	for _, r := range text {
		if (r >= 0x4E00 && r <= 0x9FFF) ||
			(r >= 0x3400 && r <= 0x4DBF) ||
			(r >= 0xF900 && r <= 0xFAFF) ||
			(r >= 0x3000 && r <= 0x303F) ||
			(r >= 0xFF00 && r <= 0xFFEF) {
			return true
		}
	}
	return false
}

func charsPerLineForText(text string, width float64) int {
	density := 6.5
	if containsCJK(text) {
		density = 4.6
	}
	n := int(width * density)
	if n < 8 {
		n = 8
	}
	return n
}

func estimateTextHeight(text string, fontSize int, width float64) float64 {
	if fontSize <= 0 {
		fontSize = dynamicPPTDefaultBodyFont
	}
	if width <= 0 {
		width = 4
	}
	runes := len([]rune(strings.TrimSpace(text)))
	if runes == 0 {
		return 0.22
	}
	charsPerLine := charsPerLineForText(text, width)
	lines := (runes + charsPerLine - 1) / charsPerLine
	lineHeight := float64(fontSize) * 1.28 / 72.0
	return dynamicPPTMaxFloat(lineHeight*float64(lines), 0.26)
}

func estimateTextHeightWithStyle(text string, style pptStyle, fontSize int, width float64) float64 {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0.22
	}

	// For code blocks (text with explicit newlines), count actual lines
	// rather than estimating from character density, to preserve formatting.
	newlineCount := strings.Count(trimmed, "\n")
	if newlineCount > 0 && fontSize <= 18 {
		lines := newlineCount + 1
		lineHeight := float64(fontSize) * 1.4 / 72.0
		if style.LineHeight != nil && *style.LineHeight > 0 {
			lineHeight = *style.LineHeight
		}
		return dynamicPPTMaxFloat(lineHeight*float64(lines)+0.1, 0.26)
	}

	runes := len([]rune(trimmed))
	if runes == 0 {
		return 0.22
	}
	charsPerLine := charsPerLineForText(trimmed, width)
	lines := (runes + charsPerLine - 1) / charsPerLine

	if style.LineHeight != nil && *style.LineHeight > 0 {
		return dynamicPPTMaxFloat(*style.LineHeight*float64(lines), 0.26)
	}

	if fontSize <= 0 {
		fontSize = dynamicPPTDefaultBodyFont
	}
	lineHeight := float64(fontSize) * 1.28 / 72.0
	return dynamicPPTMaxFloat(lineHeight*float64(lines), 0.26)
}

func parseInlineStyleMap(value string) map[string]string {
	styles := map[string]string{}
	for _, declaration := range parseInlineStyleDeclarations(value) {
		styles[declaration.Key] = declaration.Value
	}
	return styles
}

func parseInlineStyleDeclarations(value string) []pptStyleDeclaration {
	declarations := make([]pptStyleDeclaration, 0)
	for _, part := range strings.Split(value, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		val := strings.TrimSpace(kv[1])
		if key == "" || val == "" {
			continue
		}
		declarations = append(declarations, pptStyleDeclaration{
			Key:   key,
			Value: val,
		})
	}
	return declarations
}

func parsePPTColor(value string) (pptx.Color, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return pptx.Color{}, false
	}
	if named, ok := namedPPTColors()[value]; ok {
		return named, true
	}
	if strings.Contains(value, "gradient") {
		return extractFirstColorToken(value)
	}
	if strings.HasPrefix(value, "#") {
		return parseHexColor(value)
	}
	if strings.HasPrefix(value, "rgb(") || strings.HasPrefix(value, "rgba(") {
		return parseRGBColor(value)
	}
	if strings.Contains(value, "#") {
		return extractFirstColorToken(value)
	}
	return pptx.Color{}, false
}

func extractFirstColorToken(value string) (pptx.Color, bool) {
	tokens := strings.FieldsFunc(value, func(r rune) bool {
		return r == ' ' || r == ',' || r == '(' || r == ')' || r == ';'
	})
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if strings.HasPrefix(token, "#") {
			if color, ok := parseHexColor(token); ok {
				return color, true
			}
		}
		if strings.HasPrefix(token, "rgb") {
			if color, ok := parseRGBColor(token); ok {
				return color, true
			}
		}
	}
	return pptx.Color{}, false
}

func parseHexColor(value string) (pptx.Color, bool) {
	hex := strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(hex) == 3 {
		hex = strings.Repeat(string(hex[0]), 2) + strings.Repeat(string(hex[1]), 2) + strings.Repeat(string(hex[2]), 2)
	}
	if len(hex) != 6 {
		return pptx.Color{}, false
	}
	r, err1 := strconv.ParseUint(hex[0:2], 16, 8)
	g, err2 := strconv.ParseUint(hex[2:4], 16, 8)
	b, err3 := strconv.ParseUint(hex[4:6], 16, 8)
	if err1 != nil || err2 != nil || err3 != nil {
		return pptx.Color{}, false
	}
	return pptx.Color{R: uint8(r), G: uint8(g), B: uint8(b)}, true
}

func parseRGBColor(value string) (pptx.Color, bool) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "rgba(")
	value = strings.TrimPrefix(value, "rgb(")
	value = strings.TrimSuffix(value, ")")
	parts := strings.Split(value, ",")
	if len(parts) < 3 {
		return pptx.Color{}, false
	}
	r, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	g, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	b, err3 := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err1 != nil || err2 != nil || err3 != nil || r < 0 || g < 0 || b < 0 || r > 255 || g > 255 || b > 255 {
		return pptx.Color{}, false
	}
	return pptx.Color{R: uint8(r), G: uint8(g), B: uint8(b)}, true
}

func namedPPTColors() map[string]pptx.Color {
	return map[string]pptx.Color{
		"white":       pptx.White,
		"black":       {R: 0, G: 0, B: 0},
		"gray":        {R: 107, G: 114, B: 128},
		"grey":        {R: 107, G: 114, B: 128},
		"red":         {R: 220, G: 38, B: 38},
		"green":       {R: 22, G: 163, B: 74},
		"blue":        {R: 37, G: 99, B: 235},
		"yellow":      {R: 234, G: 179, B: 8},
		"orange":      {R: 249, G: 115, B: 22},
		"transparent": pptx.White,
	}
}

func parseCSSFontSize(value string) int {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return 0
	}
	switch {
	case strings.HasSuffix(value, "rem"):
		f, err := strconv.ParseFloat(strings.TrimSuffix(value, "rem"), 64)
		if err != nil {
			return 0
		}
		return int(f*16 + 0.5)
	case strings.HasSuffix(value, "px"):
		f, err := strconv.ParseFloat(strings.TrimSuffix(value, "px"), 64)
		if err != nil {
			return 0
		}
		return int(f + 0.5)
	case strings.HasSuffix(value, "pt"):
		f, err := strconv.ParseFloat(strings.TrimSuffix(value, "pt"), 64)
		if err != nil {
			return 0
		}
		return int(f + 0.5)
	default:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0
		}
		return int(f + 0.5)
	}
}

func parseCSSFontWeight(value string) int {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "", "normal":
		return 0
	case "bold":
		return 700
	case "medium":
		return 500
	case "semibold":
		return 600
	default:
		weight, err := strconv.Atoi(value)
		if err != nil {
			return 0
		}
		return weight
	}
}

func parseCSSLineHeight(value string, inheritedFontSize *int) float64 {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || value == "normal" {
		if inheritedFontSize != nil && *inheritedFontSize > 0 {
			return float64(*inheritedFontSize) * 1.22 / 72.0
		}
		return float64(dynamicPPTDefaultBodyFont) * 1.22 / 72.0
	}
	if strings.HasSuffix(value, "rem") || strings.HasSuffix(value, "px") || strings.HasSuffix(value, "pt") {
		return parseCSSSpacingInches(value)
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil && f > 0 {
		fontSize := dynamicPPTDefaultBodyFont
		if inheritedFontSize != nil && *inheritedFontSize > 0 {
			fontSize = *inheritedFontSize
		}
		return float64(fontSize) * f / 72.0
	}
	return 0
}

func parseCSSSpacingInches(value string) float64 {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return 0
	}
	switch {
	case strings.HasSuffix(value, "rem"):
		f, err := strconv.ParseFloat(strings.TrimSuffix(value, "rem"), 64)
		if err != nil {
			return 0
		}
		return (f * 16.0) / 96.0
	case strings.HasSuffix(value, "px"):
		f, err := strconv.ParseFloat(strings.TrimSuffix(value, "px"), 64)
		if err != nil {
			return 0
		}
		return f / 96.0
	case strings.HasSuffix(value, "pt"):
		f, err := strconv.ParseFloat(strings.TrimSuffix(value, "pt"), 64)
		if err != nil {
			return 0
		}
		return f / 72.0
	default:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0
		}
		return f / 96.0
	}
}

func parseCSSBorder(value string) (int, pptx.Color, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, pptx.Color{}, false
	}
	parts := strings.Fields(value)
	width := 1
	var color pptx.Color
	var hasColor bool
	for _, part := range parts {
		if borderWidth := parseCSSBorderWidth(part); borderWidth > 0 {
			width = borderWidth
			continue
		}
		if borderColor, ok := parsePPTColor(part); ok {
			color = borderColor
			hasColor = true
		}
	}
	return width, color, hasColor
}

func parseCSSBorderWidth(value string) int {
	value = strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.HasSuffix(value, "px"):
		f, err := strconv.ParseFloat(strings.TrimSuffix(value, "px"), 64)
		if err != nil || f <= 0 {
			return 0
		}
		return dynamicPPTMaxInt(1, int(f+0.5))
	case strings.HasSuffix(value, "pt"):
		f, err := strconv.ParseFloat(strings.TrimSuffix(value, "pt"), 64)
		if err != nil || f <= 0 {
			return 0
		}
		return dynamicPPTMaxInt(1, int(f+0.5))
	default:
		return 0
	}
}

func parseCSSRadius(value string) int {
	value = strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.HasSuffix(value, "px"):
		f, err := strconv.ParseFloat(strings.TrimSuffix(value, "px"), 64)
		if err != nil {
			return 0
		}
		return int(f + 0.5)
	case strings.HasSuffix(value, "pt"):
		f, err := strconv.ParseFloat(strings.TrimSuffix(value, "pt"), 64)
		if err != nil {
			return 0
		}
		return int(f + 0.5)
	default:
		return 0
	}
}

func parseCSSBoxEdges(value string) (pptEdges, bool) {
	parts := strings.Fields(strings.TrimSpace(value))
	if len(parts) == 0 {
		return pptEdges{}, false
	}
	values := make([]float64, 0, len(parts))
	for _, part := range parts {
		values = append(values, parseCSSSpacingInches(part))
	}
	edges := pptEdges{Set: true}
	switch len(values) {
	case 1:
		edges.Top, edges.Right, edges.Bottom, edges.Left = values[0], values[0], values[0], values[0]
	case 2:
		edges.Top, edges.Bottom = values[0], values[0]
		edges.Right, edges.Left = values[1], values[1]
	case 3:
		edges.Top = values[0]
		edges.Right, edges.Left = values[1], values[1]
		edges.Bottom = values[2]
	default:
		edges.Top = values[0]
		edges.Right = values[1]
		edges.Bottom = values[2]
		edges.Left = values[3]
	}
	return edges, true
}

func updateEdge(edges pptEdges, side string, value float64) pptEdges {
	edges.Set = true
	switch side {
	case "top":
		edges.Top = value
	case "right":
		edges.Right = value
	case "bottom":
		edges.Bottom = value
	case "left":
		edges.Left = value
	}
	return edges
}

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
