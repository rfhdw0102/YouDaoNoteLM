package ppt

import (
	"strings"
)

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
