package ppt

import (
	"github.com/duynguyendang/docxgo/v3/pptx"
	"strings"
)

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

// renderTextAt 在测量位置渲染文本块。
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

// renderInlineRunsAt 按权重分配宽度渲染内联文本运行。
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

// renderPlainTextAt 在指定位置渲染纯文本块。
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

// inlineRunsWidthWeight 计算内联运行序列的总宽度权重。
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

// inlineRunWidthWeight 计算单个内联运行的宽度权重。
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

// renderDynamicBlock 渲染单个动态块。
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

// renderSectionNumber 渲染幻灯片序号。
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

// renderContainerBlock 渲染容器块。
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

// renderGridContainer 渲染网格布局容器。
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

// renderCardBlock 渲染卡片块。
func renderCardBlock(slide *pptx.SlideBuilder, cursor *pptLayoutCursor, block pptHTMLBlock) {
	cursor.y += edgeOr(block.Style.Margin, "top", 0)
	height := estimateRenderedHeight(block, cursor.width)
	renderCardInRect(slide, block, cursor.x, cursor.y, cursor.width, height)
	cursor.y += height + edgeOr(block.Style.Margin, "bottom", dynamicPPTDefaultGap)
}

// renderCardInRect 在矩形区域内渲染卡片块。
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

// renderCardChrome 渲染卡片外观（填充与边框）。
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

// renderTextBlock 渲染文本块。
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
