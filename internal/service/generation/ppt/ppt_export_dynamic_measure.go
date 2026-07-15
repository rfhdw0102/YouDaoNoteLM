package ppt

import (
	"github.com/duynguyendang/docxgo/v3/pptx"
	"strings"
)

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
