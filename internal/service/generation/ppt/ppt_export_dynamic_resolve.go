package ppt

import (
	"github.com/duynguyendang/docxgo/v3/pptx"
	"strings"
)

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

// resolveContainerColumns 解析容器的列数。
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

// resolveSlideBackground 解析幻灯片背景色。
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

// resolveCardFill 解析卡片填充色。
func resolveCardFill(style pptStyle) pptx.Color {
	if style.BackgroundColor != nil {
		return *style.BackgroundColor
	}
	return pptx.Color{R: 252, G: 251, B: 248}
}

// resolveTextColor 解析文本颜色，缺失则返回兜底色。
func resolveTextColor(style pptStyle, fallback pptx.Color) pptx.Color {
	if style.TextColor != nil {
		return *style.TextColor
	}
	return fallback
}

// resolveBorderColor 解析边框颜色，缺失则返回兜底色。
func resolveBorderColor(style pptStyle, fallback pptx.Color) pptx.Color {
	if style.BorderColor != nil {
		return *style.BorderColor
	}
	return fallback
}

// resolveGap 解析容器间距，缺失则返回兜底值。
func resolveGap(style pptStyle, fallback float64) float64 {
	if style.Gap != nil && *style.Gap > 0 {
		return *style.Gap
	}
	return fallback
}

// resolveBlockFontSize 解析块字体大小，缺失则返回兜底值。
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

// defaultFontSizeForBlock 返回块类型的默认字体大小。
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

// defaultFontFamilyForBlock 返回块类型的默认字体族。
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

// resolveFontFamily 解析字体族，缺失则返回兜底值。
func resolveFontFamily(style pptStyle, fallback string) string {
	if strings.TrimSpace(style.FontFamily) == "" {
		return fallback
	}
	return style.FontFamily
}

// resolveAlignment 将 CSS 对齐值转换为 pptx.Alignment。
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

// containsCJK 判断文本是否包含中日韩字符。
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

// charsPerLineForText 估算指定宽度下每行可容纳的字符数。
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

// estimateTextHeight 估算文本在指定宽度和字号下的高度。
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

// estimateTextHeightWithStyle 基于样式估算文本渲染高度。
func estimateTextHeightWithStyle(text string, style pptStyle, fontSize int, width float64) float64 {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0.22
	}

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

// parseInlineStyleMap 将内联样式字符串解析为键值映射。
func parseInlineStyleMap(value string) map[string]string {
	styles := map[string]string{}
	for _, declaration := range parseInlineStyleDeclarations(value) {
		styles[declaration.Key] = declaration.Value
	}
	return styles
}
