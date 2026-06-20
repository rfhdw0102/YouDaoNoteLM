package service

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/duynguyendang/docxgo/v3/pptx"

	bizerrors "YoudaoNoteLm/pkg/errors"
)

type pptExportSlide struct {
	Title   string
	Bullets []string
}

type pptExportTheme struct {
	Background pptx.Color
	Surface    pptx.Color
	SurfaceAlt pptx.Color
	Accent     pptx.Color
	AccentDark pptx.Color
	AccentSoft pptx.Color
	Border     pptx.Color
	Title      pptx.Color
	Text       pptx.Color
	Muted      pptx.Color
	White      pptx.Color
}

type pptExportTemplate struct {
	ID          string
	Name        string
	Theme       pptExportTheme
	Kicker      string
	TitleFont   string
	BodyFont    string
	FooterLabel string
	TitleSize   int
	BodySize    int
}

const pptDefaultTemplateID = "classic"

var pptExportTemplates = map[string]pptExportTemplate{
	"classic": {
		ID:   "classic",
		Name: "Classic",
		Theme: pptExportTheme{
			// 暖橙色系：温暖、学术
			Background: pptx.Color{R: 255, G: 248, B: 240},
			Surface:    pptx.Color{R: 255, G: 252, B: 247},
			SurfaceAlt: pptx.Color{R: 255, G: 240, B: 220},
			Accent:     pptx.Color{R: 220, G: 78, B: 10},
			AccentDark: pptx.Color{R: 178, G: 55, B: 8},
			AccentSoft: pptx.Color{R: 255, G: 232, B: 208},
			Border:     pptx.Color{R: 248, G: 176, B: 104},
			Title:      pptx.Color{R: 60, G: 16, B: 4},
			Text:       pptx.Color{R: 38, G: 34, B: 32},
			Muted:      pptx.Color{R: 115, G: 108, B: 103},
			White:      pptx.White,
		},
		Kicker:      "LEARNING DECK",
		TitleFont:   "Microsoft YaHei UI",
		BodyFont:    "Microsoft YaHei",
		FooterLabel: "YoudaoNoteLM · Classic",
		TitleSize:   34,
		BodySize:    17,
	},
	"clean": {
		ID:   "clean",
		Name: "Clean",
		Theme: pptExportTheme{
			// 蓝灰极简：干净、现代
			Background: pptx.Color{R: 246, G: 248, B: 252},
			Surface:    pptx.Color{R: 255, G: 255, B: 255},
			SurfaceAlt: pptx.Color{R: 243, G: 246, B: 251},
			Accent:     pptx.Color{R: 79, G: 120, B: 200},
			AccentDark: pptx.Color{R: 52, G: 88, B: 168},
			AccentSoft: pptx.Color{R: 224, G: 232, B: 248},
			Border:     pptx.Color{R: 210, G: 220, B: 238},
			Title:      pptx.Color{R: 18, G: 32, B: 62},
			Text:       pptx.Color{R: 28, G: 40, B: 60},
			Muted:      pptx.Color{R: 96, G: 112, B: 140},
			White:      pptx.White,
		},
		Kicker:      "FOCUS NOTES",
		TitleFont:   "Microsoft YaHei UI",
		BodyFont:    "Microsoft YaHei",
		FooterLabel: "YoudaoNoteLM · Clean",
		TitleSize:   34,
		BodySize:    17,
	},
	"business": {
		ID:   "business",
		Name: "Business",
		Theme: pptExportTheme{
			// 深绿商务：专业、沉稳
			Background: pptx.Color{R: 245, G: 248, B: 245},
			Surface:    pptx.Color{R: 255, G: 255, B: 255},
			SurfaceAlt: pptx.Color{R: 230, G: 242, B: 238},
			Accent:     pptx.Color{R: 34, G: 110, B: 90},
			AccentDark: pptx.Color{R: 22, G: 82, B: 66},
			AccentSoft: pptx.Color{R: 216, G: 238, B: 232},
			Border:     pptx.Color{R: 170, G: 212, B: 200},
			Title:      pptx.Color{R: 16, G: 46, B: 38},
			Text:       pptx.Color{R: 42, G: 58, B: 54},
			Muted:      pptx.Color{R: 82, G: 102, B: 96},
			White:      pptx.White,
		},
		Kicker:      "EXECUTIVE BRIEF",
		TitleFont:   "Microsoft YaHei UI",
		BodyFont:    "Microsoft YaHei",
		FooterLabel: "YoudaoNoteLM · Business",
		TitleSize:   34,
		BodySize:    17,
	},
}

var (
	pptSectionPattern = regexp.MustCompile(`(?is)<section\b[^>]*>(.*?)</section>`)
	pptH1Pattern      = regexp.MustCompile(`(?is)<h1\b[^>]*>(.*?)</h1>`)
	pptH2Pattern      = regexp.MustCompile(`(?is)<h2\b[^>]*>(.*?)</h2>`)
	pptBulletPattern  = regexp.MustCompile(`(?is)<li\b[^>]*>(.*?)</li>`)
)

func exportPPT(ctx context.Context, content, title, templateID string) (*GenerationExportResult, error) {
	filename := resolveExportFilename(title, content, "ppt-export", ".pptx")

	trimmedTemplateID := strings.TrimSpace(templateID)
	var (
		data []byte
		err  error
	)

	if trimmedTemplateID == "" {
		data, err = exportPPTWithDefaultEngine(ctx, content, strings.TrimSuffix(filename, ".pptx"))
		if err != nil {
			return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "build ppt export failed", err)
		}
	} else {
		slides, parseErr := parsePPTExportSlides(content)
		if parseErr != nil {
			return nil, parseErr
		}
		template, templateErr := resolvePPTExportTemplate(trimmedTemplateID)
		if templateErr != nil {
			return nil, templateErr
		}
		data, err = buildPPTXBytes(slides, strings.TrimSuffix(filename, ".pptx"), template)
		if err != nil {
			return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "build ppt export failed", err)
		}
	}

	return &GenerationExportResult{
		Filename:    filename,
		ContentType: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		Data:        data,
	}, nil
}

func resolvePPTExportTemplate(templateID string) (pptExportTemplate, error) {
	id := strings.ToLower(strings.TrimSpace(templateID))
	template, ok := pptExportTemplates[id]
	if !ok {
		return pptExportTemplate{}, bizerrors.New(bizerrors.CodeInvalidParam, "unsupported ppt template")
	}
	return template, nil
}

func parsePPTExportSlides(content string) ([]pptExportSlide, error) {
	matches := pptSectionPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "ppt export content must contain at least one <section> slide")
	}

	slides := make([]pptExportSlide, 0, len(matches))
	for _, match := range matches {
		body := match[1]
		title := extractPPTSlideTitle(body)
		bullets := extractPPTSlideBullets(body)
		if title == "" && len(bullets) == 0 {
			continue
		}
		slides = append(slides, pptExportSlide{
			Title:   title,
			Bullets: bullets,
		})
	}

	if len(slides) == 0 {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "ppt export content does not contain any valid slides")
	}
	return slides, nil
}

func extractPPTSlideTitle(section string) string {
	for _, pattern := range []*regexp.Regexp{pptH1Pattern, pptH2Pattern} {
		match := pattern.FindStringSubmatch(section)
		if len(match) >= 2 {
			return normalizePPTExportText(match[1])
		}
	}
	return ""
}

func extractPPTSlideBullets(section string) []string {
	matches := pptBulletPattern.FindAllStringSubmatch(section, -1)
	bullets := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		bullets = append(bullets, normalizePPTExportText(match[1]))
	}
	return uniqueNonEmpty(bullets)
}

func normalizePPTExportText(value string) string {
	value = strings.ReplaceAll(value, "&nbsp;", " ")
	value = stripSimpleHTML(value)
	value = html.UnescapeString(value)
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	return cleanPPTVisibleText(value)
}

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

	// Kicker 徽标
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

	// 页脚分隔线（使用 Border 色，更精致）
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

func fixPPTXPackage(data []byte, slideCount int) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	existing := make(map[string]bool, len(reader.File))
	for _, file := range reader.File {
		existing[file.Name] = true
	}

	needsPresentationXML := true
	missing := make([]string, 0, slideCount)
	for i := 1; i <= slideCount; i++ {
		name := fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", i)
		if !existing[name] {
			missing = append(missing, name)
		}
	}

	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for _, file := range reader.File {
		if file.Name == "ppt/presentation.xml" {
			if err := writePPTXZipEntry(writer, file.Name, []byte(pptxPresentationXML(slideCount))); err != nil {
				_ = writer.Close()
				return nil, err
			}
			needsPresentationXML = false
			continue
		}
		if file.Name == "ppt/slideMasters/slideMaster1.xml" {
			if err := writePPTXZipEntry(writer, file.Name, []byte(pptxSlideMasterXML())); err != nil {
				_ = writer.Close()
				return nil, err
			}
			continue
		}
		if err := copyPPTXZipEntry(writer, file); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	if needsPresentationXML {
		if err := writePPTXZipEntry(writer, "ppt/presentation.xml", []byte(pptxPresentationXML(slideCount))); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	for _, name := range missing {
		if err := writePPTXZipEntry(writer, name, []byte(pptxSlideRelationshipXML())); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func writePPTXZipEntry(writer *zip.Writer, name string, data []byte) error {
	entry, err := writer.Create(name)
	if err != nil {
		return err
	}
	_, err = entry.Write(data)
	return err
}

func copyPPTXZipEntry(writer *zip.Writer, file *zip.File) error {
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()

	header := file.FileHeader
	entry, err := writer.CreateHeader(&header)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, reader)
	return err
}

func pptxPresentationXML(slideCount int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:sldMasterIdLst>
    <p:sldMasterId id="2147483648" r:id="rId2"/>
  </p:sldMasterIdLst>
  <p:sldIdLst>
`)
	for i := 1; i <= slideCount; i++ {
		b.WriteString(fmt.Sprintf(`    <p:sldId id="%d" r:id="rId%d"/>
`, 255+i, i+2))
	}
	b.WriteString(`  </p:sldIdLst>
  <p:sldSz cx="12192000" cy="6858000" type="screen16x9"/>
  <p:notesSz cx="6858000" cy="9144000"/>
  <p:defaultTextStyle/>
</p:presentation>`)
	return b.String()
}

func pptxSlideMasterXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr>
        <p:cNvPr id="1" name=""/>
        <p:cNvGrpSpPr/>
        <p:nvPr/>
      </p:nvGrpSpPr>
      <p:grpSpPr/>
    </p:spTree>
  </p:cSld>
  <p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>
  <p:sldLayoutIdLst>
    <p:sldLayoutId id="2147483649" r:id="rId1"/>
  </p:sldLayoutIdLst>
  <p:txStyles>
    <p:titleStyle/>
    <p:bodyStyle/>
    <p:otherStyle/>
  </p:txStyles>
</p:sldMaster>`
}

func pptxSlideRelationshipXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
</Relationships>`
}
