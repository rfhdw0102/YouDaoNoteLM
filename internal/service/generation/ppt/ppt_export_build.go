package ppt

import (
	"fmt"
	"github.com/duynguyendang/docxgo/v3/pptx"
	"os"
	"path/filepath"
	"strings"
)

// buildPPTXBytes 根据幻灯片数据与模板构建 PPTX 二进制内容。
func buildPPTXBytes(slides []pptExportSlide, deckTitle string, template pptExportTemplate) ([]byte, error) {
	builder := pptx.NewPresentationBuilder(
		pptx.WithTitle(firstNonEmpty(deckTitle, "ppt-export")),
		pptx.WithLayout(pptx.Layout16x9),
	)

	for i, slideData := range slides {
		slide := builder.AddSlide().SetBackgroundColor(template.Theme.Background)
		addPPTThemeFrame(slide, i+1, template)

		if slideData.Title != "" {
			addPPTSlideTitle(slide, slideData.Title, template)
		}

		startY := 2.14
		if slideData.Title == "" {
			startY = 1.28
		}
		addPPTBulletCards(slide, slideData.Bullets, startY, template)
	}

	presentation, err := builder.Build()
	if err != nil {
		return nil, err
	}

	tempDir, err := os.MkdirTemp("", "youdaonotelm-ppt-export-*")
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
	return fixPPTXPackage(data, len(slides))
}

// addPPTThemeFrame 为幻灯片添加主题背景、强调条、页码与页脚等装饰元素。
func addPPTThemeFrame(slide *pptx.SlideBuilder, slideNumber int, template pptExportTemplate) {
	// 主内容卡片
	slide.AddShape(pptx.ShapeRoundedRectangle).
		SetPosition(pptx.Inches(0.72), pptx.Inches(0.46)).
		SetSize(pptx.Inches(11.92), pptx.Inches(6.28)).
		SetFillColor(template.Theme.Surface).
		SetLine(template.Theme.Border, 1).
		End()

	// 顶部强调色条（稍厚一点，更有视觉分量）
	slide.AddShape(pptx.ShapeRectangle).
		SetPosition(pptx.Inches(0), pptx.Inches(0)).
		SetSize(pptx.Inches(13.333), pptx.Inches(0.28)).
		SetFillColor(template.Theme.Accent).
		SetNoLine().
		End()

	// 左侧强调竖条（稍宽，视觉锚点更清晰）
	slide.AddShape(pptx.ShapeRectangle).
		SetPosition(pptx.Inches(0.72), pptx.Inches(0.46)).
		SetSize(pptx.Inches(0.10), pptx.Inches(6.28)).
		SetFillColor(template.Theme.AccentDark).
		SetNoLine().
		End()

	slide.AddShape(pptx.ShapeRoundedRectangle).
		SetPosition(pptx.Inches(0.96), pptx.Inches(0.76)).
		SetSize(pptx.Inches(1.60), pptx.Inches(0.34)).
		SetFillColor(template.Theme.Accent).
		SetNoLine().
		End()

	slide.AddText(template.Kicker).
		SetBold(true).
		SetFontSize(9).
		SetFontFamily(template.BodyFont).
		SetAlignment(pptx.AlignmentCenter).
		SetColor(template.Theme.White).
		SetPosition(pptx.Inches(1.01), pptx.Inches(0.84)).
		SetSize(pptx.Inches(1.50), pptx.Inches(0.14)).
		End()

	// 页码徽标（圆角矩形，更宽一点显得不局促）
	slide.AddShape(pptx.ShapeRoundedRectangle).
		SetPosition(pptx.Inches(11.50), pptx.Inches(0.70)).
		SetSize(pptx.Inches(0.96), pptx.Inches(0.46)).
		SetFillColor(template.Theme.AccentDark).
		SetNoLine().
		End()

	slide.AddText(fmt.Sprintf("%02d", slideNumber)).
		SetBold(true).
		SetFontSize(14).
		SetFontFamily(template.BodyFont).
		SetAlignment(pptx.AlignmentCenter).
		SetColor(template.Theme.White).
		SetPosition(pptx.Inches(11.58), pptx.Inches(0.82)).
		SetSize(pptx.Inches(0.80), pptx.Inches(0.18)).
		End()

	slide.AddShape(pptx.ShapeRectangle).
		SetPosition(pptx.Inches(0.96), pptx.Inches(6.52)).
		SetSize(pptx.Inches(11.42), pptx.Inches(0.022)).
		SetFillColor(template.Theme.Border).
		SetNoLine().
		End()

	slide.AddText(template.FooterLabel).
		SetFontSize(10).
		SetFontFamily(template.BodyFont).
		SetColor(template.Theme.Muted).
		SetPosition(pptx.Inches(0.96), pptx.Inches(6.64)).
		SetSize(pptx.Inches(3.4), pptx.Inches(0.20)).
		End()
}

// addPPTSlideTitle 在幻灯片上添加标题文本及装饰下划线。
func addPPTSlideTitle(slide *pptx.SlideBuilder, title string, template pptExportTemplate) {
	slide.AddText(title).
		SetBold(true).
		SetFontSize(template.TitleSize).
		SetFontFamily(template.TitleFont).
		SetColor(template.Theme.Title).
		SetPosition(pptx.Inches(0.96), pptx.Inches(1.16)).
		SetSize(pptx.Inches(10.82), pptx.Inches(0.80)).
		End()

	// 标题下方装饰线（稍长，与内容宽度协调）
	slide.AddShape(pptx.ShapeRoundedRectangle).
		SetPosition(pptx.Inches(0.96), pptx.Inches(1.96)).
		SetSize(pptx.Inches(1.40), pptx.Inches(0.06)).
		SetFillColor(template.Theme.Accent).
		SetNoLine().
		End()
}

// calcBulletCardHeight 根据文本长度与字号估算卡片高度。
func calcBulletCardHeight(bullet string, baseSize int) float64 {
	length := len([]rune(strings.TrimSpace(bullet)))
	fs := pptBulletFontSize(bullet, baseSize)
	// 文字区宽度约 9.7 英寸，按每英寸约 6 个字符（中英混排保守估算）
	charsPerLine := 58
	if fs < baseSize-1 {
		charsPerLine = 65
	}
	if charsPerLine < 10 {
		charsPerLine = 10
	}
	lines := (length + charsPerLine - 1) / charsPerLine
	if lines < 1 {
		lines = 1
	}
	// 每行约 0.26 英寸，加上上下内边距 0.44 英寸
	h := float64(lines)*0.26 + 0.44
	if h < 0.74 {
		h = 0.74
	}
	return h
}

// addPPTBulletCards 在幻灯片上按顺序排列多个要点卡片并自适应高度。
func addPPTBulletCards(slide *pptx.SlideBuilder, bullets []string, startY float64, template pptExportTemplate) {
	if len(bullets) == 0 {
		return
	}
	const cardGap = 0.10
	const maxBottom = 6.44

	heights := make([]float64, len(bullets))
	total := 0.0
	for i, b := range bullets {
		heights[i] = calcBulletCardHeight(b, template.BodySize)
		total += heights[i]
		if i > 0 {
			total += cardGap
		}
	}

	// 超出可用高度时等比缩小
	available := maxBottom - startY
	if total > available {
		scale := available / total
		for i := range heights {
			heights[i] *= scale
		}
	}

	y := startY
	for i, bullet := range bullets {
		addPPTBulletCard(slide, i, bullet, y, heights[i], template)
		y += heights[i] + cardGap
	}
}

// addPPTBulletCard 在幻灯片指定位置绘制单个要点卡片及其编号指示器。
func addPPTBulletCard(slide *pptx.SlideBuilder, index int, bullet string, y float64, height float64, template pptExportTemplate) {
	slide.AddShape(pptx.ShapeRoundedRectangle).
		SetPosition(pptx.Inches(1.04), pptx.Inches(y)).
		SetSize(pptx.Inches(11.08), pptx.Inches(height)).
		SetFillColor(template.Theme.SurfaceAlt).
		SetLine(template.Theme.Border, 1).
		End()

	// 圆形编号指示器，垂直居中于卡片
	circleY := y + (height-0.36)/2
	slide.AddShape(pptx.ShapeEllipse).
		SetPosition(pptx.Inches(1.26), pptx.Inches(circleY)).
		SetSize(pptx.Inches(0.36), pptx.Inches(0.36)).
		SetFillColor(template.Theme.Accent).
		SetNoLine().
		End()

	slide.AddText(fmt.Sprintf("%02d", index+1)).
		SetBold(true).
		SetFontSize(10).
		SetFontFamily(template.BodyFont).
		SetAlignment(pptx.AlignmentCenter).
		SetColor(template.Theme.White).
		SetPosition(pptx.Inches(1.29), pptx.Inches(circleY+0.12)).
		SetSize(pptx.Inches(0.30), pptx.Inches(0.14)).
		End()

	textHeight := height - 0.30
	if textHeight < 0.22 {
		textHeight = 0.22
	}
	slide.AddText(bullet).
		SetFontSize(pptBulletFontSize(bullet, template.BodySize)).
		SetFontFamily(template.BodyFont).
		SetColor(template.Theme.Text).
		SetPosition(pptx.Inches(1.84), pptx.Inches(y+0.15)).
		SetSize(pptx.Inches(9.80), pptx.Inches(textHeight)).
		End()
}

// pptBulletFontSize 根据要点文本长度自适应返回合适的字号。
func pptBulletFontSize(bullet string, baseSize int) int {
	if baseSize <= 0 {
		baseSize = 15
	}
	length := len([]rune(strings.TrimSpace(bullet)))
	switch {
	case length > 110:
		return baseSize - 3
	case length > 72:
		return baseSize - 2
	case length > 44:
		return baseSize - 1
	default:
		return baseSize
	}
}
