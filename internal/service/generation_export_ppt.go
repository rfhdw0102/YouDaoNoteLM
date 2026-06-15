package service

import (
	"archive/zip"
	"bytes"
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
			Background: pptx.Color{R: 255, G: 247, B: 237},
			Surface:    pptx.Color{R: 255, G: 251, B: 245},
			SurfaceAlt: pptx.Color{R: 255, G: 237, B: 213},
			Accent:     pptx.Color{R: 234, G: 88, B: 12},
			AccentDark: pptx.Color{R: 194, G: 65, B: 12},
			AccentSoft: pptx.Color{R: 255, G: 237, B: 213},
			Border:     pptx.Color{R: 253, G: 186, B: 116},
			Title:      pptx.Color{R: 67, G: 20, B: 7},
			Text:       pptx.Color{R: 41, G: 37, B: 36},
			Muted:      pptx.Color{R: 120, G: 113, B: 108},
			White:      pptx.White,
		},
		Kicker:      "LEARNING DECK",
		TitleFont:   "Microsoft YaHei UI",
		BodyFont:    "Microsoft YaHei",
		FooterLabel: "YoudaoNoteLM / Classic",
		TitleSize:   36,
		BodySize:    18,
	},
	"clean": {
		ID:   "clean",
		Name: "Clean",
		Theme: pptExportTheme{
			Background: pptx.Color{R: 248, G: 250, B: 252},
			Surface:    pptx.Color{R: 255, G: 255, B: 255},
			SurfaceAlt: pptx.Color{R: 248, G: 250, B: 252},
			Accent:     pptx.Color{R: 156, G: 163, B: 175},
			AccentDark: pptx.Color{R: 107, G: 114, B: 128},
			AccentSoft: pptx.Color{R: 229, G: 231, B: 235},
			Border:     pptx.Color{R: 229, G: 231, B: 235},
			Title:      pptx.Color{R: 31, G: 41, B: 55},
			Text:       pptx.Color{R: 30, G: 41, B: 59},
			Muted:      pptx.Color{R: 100, G: 116, B: 139},
			White:      pptx.White,
		},
		Kicker:      "FOCUS NOTES",
		TitleFont:   "Microsoft YaHei UI",
		BodyFont:    "Microsoft YaHei",
		FooterLabel: "YoudaoNoteLM / Clean",
		TitleSize:   34,
		BodySize:    18,
	},
	"business": {
		ID:   "business",
		Name: "Business",
		Theme: pptExportTheme{
			Background: pptx.Color{R: 247, G: 248, B: 245},
			Surface:    pptx.Color{R: 255, G: 255, B: 255},
			SurfaceAlt: pptx.Color{R: 229, G: 241, B: 239},
			Accent:     pptx.Color{R: 63, G: 125, B: 122},
			AccentDark: pptx.Color{R: 61, G: 102, B: 99},
			AccentSoft: pptx.Color{R: 229, G: 241, B: 239},
			Border:     pptx.Color{R: 184, G: 216, B: 213},
			Title:      pptx.Color{R: 31, G: 41, B: 55},
			Text:       pptx.Color{R: 55, G: 65, B: 81},
			Muted:      pptx.Color{R: 91, G: 101, B: 112},
			White:      pptx.White,
		},
		Kicker:      "EXECUTIVE BRIEF",
		TitleFont:   "Microsoft YaHei UI",
		BodyFont:    "Microsoft YaHei",
		FooterLabel: "YoudaoNoteLM / Business",
		TitleSize:   34,
		BodySize:    18,
	},
}

var (
	pptSectionPattern = regexp.MustCompile(`(?is)<section\b[^>]*>(.*?)</section>`)
	pptH1Pattern      = regexp.MustCompile(`(?is)<h1\b[^>]*>(.*?)</h1>`)
	pptH2Pattern      = regexp.MustCompile(`(?is)<h2\b[^>]*>(.*?)</h2>`)
	pptBulletPattern  = regexp.MustCompile(`(?is)<li\b[^>]*>(.*?)</li>`)
)

func exportPPT(content, title, templateID string) (*GenerationExportResult, error) {
	slides, err := parsePPTExportSlides(content)
	if err != nil {
		return nil, err
	}
	template, err := resolvePPTExportTemplate(templateID)
	if err != nil {
		return nil, err
	}

	filename := resolveExportFilename(title, content, "ppt-export", ".pptx")
	data, err := buildPPTXBytes(slides, strings.TrimSuffix(filename, ".pptx"), template)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "build ppt export failed", err)
	}

	return &GenerationExportResult{
		Filename:    filename,
		ContentType: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		Data:        data,
	}, nil
}

func resolvePPTExportTemplate(templateID string) (pptExportTemplate, error) {
	id := strings.ToLower(strings.TrimSpace(templateID))
	if id == "" {
		id = pptDefaultTemplateID
	}
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
	return value
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
			startY = 1.24
		}
		for i, bullet := range slideData.Bullets {
			addPPTBulletCard(slide, i, bullet, startY, template)
		}
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
	slide.AddShape(pptx.ShapeRoundedRectangle).
		SetPosition(pptx.Inches(0.74), pptx.Inches(0.5)).
		SetSize(pptx.Inches(11.88), pptx.Inches(6.22)).
		SetFillColor(template.Theme.Surface).
		SetLine(template.Theme.Border, 1).
		End()

	slide.AddShape(pptx.ShapeRectangle).
		SetPosition(pptx.Inches(0), pptx.Inches(0)).
		SetSize(pptx.Inches(13.333), pptx.Inches(0.22)).
		SetFillColor(template.Theme.Accent).
		SetNoLine().
		End()

	slide.AddShape(pptx.ShapeRectangle).
		SetPosition(pptx.Inches(0.74), pptx.Inches(0.5)).
		SetSize(pptx.Inches(0.08), pptx.Inches(6.22)).
		SetFillColor(template.Theme.AccentDark).
		SetNoLine().
		End()

	slide.AddShape(pptx.ShapeRoundedRectangle).
		SetPosition(pptx.Inches(0.98), pptx.Inches(0.78)).
		SetSize(pptx.Inches(1.54), pptx.Inches(0.34)).
		SetFillColor(template.Theme.Accent).
		SetNoLine().
		End()

	slide.AddText(template.Kicker).
		SetBold(true).
		SetFontSize(9).
		SetFontFamily(template.BodyFont).
		SetAlignment(pptx.AlignmentCenter).
		SetColor(template.Theme.White).
		SetPosition(pptx.Inches(1.03), pptx.Inches(0.85)).
		SetSize(pptx.Inches(1.44), pptx.Inches(0.14)).
		End()

	slide.AddShape(pptx.ShapeRoundedRectangle).
		SetPosition(pptx.Inches(11.54), pptx.Inches(0.73)).
		SetSize(pptx.Inches(0.82), pptx.Inches(0.46)).
		SetFillColor(template.Theme.AccentDark).
		SetNoLine().
		End()

	slide.AddText(fmt.Sprintf("%02d", slideNumber)).
		SetBold(true).
		SetFontSize(13).
		SetFontFamily(template.BodyFont).
		SetAlignment(pptx.AlignmentCenter).
		SetColor(template.Theme.White).
		SetPosition(pptx.Inches(11.65), pptx.Inches(0.84)).
		SetSize(pptx.Inches(0.6), pptx.Inches(0.16)).
		End()

	slide.AddShape(pptx.ShapeRectangle).
		SetPosition(pptx.Inches(0.98), pptx.Inches(6.55)).
		SetSize(pptx.Inches(11.18), pptx.Inches(0.02)).
		SetFillColor(template.Theme.AccentSoft).
		SetNoLine().
		End()

	slide.AddText(template.FooterLabel).
		SetFontSize(10).
		SetFontFamily(template.BodyFont).
		SetColor(template.Theme.Muted).
		SetPosition(pptx.Inches(0.98), pptx.Inches(6.68)).
		SetSize(pptx.Inches(3.2), pptx.Inches(0.18)).
		End()
}

func addPPTSlideTitle(slide *pptx.SlideBuilder, title string, template pptExportTemplate) {
	slide.AddText(title).
		SetBold(true).
		SetFontSize(template.TitleSize).
		SetFontFamily(template.TitleFont).
		SetColor(template.Theme.Title).
		SetPosition(pptx.Inches(0.98), pptx.Inches(1.18)).
		SetSize(pptx.Inches(10.65), pptx.Inches(0.72)).
		End()

	slide.AddShape(pptx.ShapeRoundedRectangle).
		SetPosition(pptx.Inches(0.98), pptx.Inches(1.9)).
		SetSize(pptx.Inches(1.08), pptx.Inches(0.07)).
		SetFillColor(template.Theme.Accent).
		SetNoLine().
		End()
}

func addPPTBulletCard(slide *pptx.SlideBuilder, index int, bullet string, startY float64, template pptExportTemplate) {
	y := startY + float64(index)*0.96
	slide.AddShape(pptx.ShapeRoundedRectangle).
		SetPosition(pptx.Inches(1.04), pptx.Inches(y)).
		SetSize(pptx.Inches(11.08), pptx.Inches(0.78)).
		SetFillColor(template.Theme.SurfaceAlt).
		SetLine(template.Theme.Border, 1).
		End()

	slide.AddShape(pptx.ShapeEllipse).
		SetPosition(pptx.Inches(1.28), pptx.Inches(y+0.21)).
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
		SetPosition(pptx.Inches(1.31), pptx.Inches(y+0.29)).
		SetSize(pptx.Inches(0.3), pptx.Inches(0.13)).
		End()

	slide.AddText(bullet).
		SetFontSize(pptBulletFontSize(bullet, template.BodySize)).
		SetFontFamily(template.BodyFont).
		SetColor(template.Theme.Text).
		SetPosition(pptx.Inches(1.86), pptx.Inches(y+0.22)).
		SetSize(pptx.Inches(9.7), pptx.Inches(0.36)).
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
