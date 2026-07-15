package ppt

import (
	"github.com/duynguyendang/docxgo/v3/pptx"
	"strconv"
	"strings"
)

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
