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

var pptExportTheme = struct {
	Background pptx.Color
	Accent     pptx.Color
	AccentSoft pptx.Color
	Border     pptx.Color
	Title      pptx.Color
	Text       pptx.Color
	Muted      pptx.Color
	White      pptx.Color
}{
	Background: pptx.Color{R: 246, G: 241, B: 232},
	Accent:     pptx.Color{R: 15, G: 118, B: 110},
	AccentSoft: pptx.Color{R: 233, G: 247, B: 243},
	Border:     pptx.Color{R: 183, G: 228, B: 214},
	Title:      pptx.Color{R: 18, G: 60, B: 58},
	Text:       pptx.Color{R: 31, G: 41, B: 55},
	Muted:      pptx.Color{R: 91, G: 112, B: 111},
	White:      pptx.White,
}

var (
	pptSectionPattern = regexp.MustCompile(`(?is)<section\b[^>]*>(.*?)</section>`)
	pptH1Pattern      = regexp.MustCompile(`(?is)<h1\b[^>]*>(.*?)</h1>`)
	pptH2Pattern      = regexp.MustCompile(`(?is)<h2\b[^>]*>(.*?)</h2>`)
	pptBulletPattern  = regexp.MustCompile(`(?is)<li\b[^>]*>(.*?)</li>`)
)

func exportPPT(content, title string) (*GenerationExportResult, error) {
	slides, err := parsePPTExportSlides(content)
	if err != nil {
		return nil, err
	}

	filename := resolveExportFilename(title, content, "ppt-export", ".pptx")
	data, err := buildPPTXBytes(slides, strings.TrimSuffix(filename, ".pptx"))
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "build ppt export failed", err)
	}

	return &GenerationExportResult{
		Filename:    filename,
		ContentType: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		Data:        data,
	}, nil
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

func buildPPTXBytes(slides []pptExportSlide, deckTitle string) ([]byte, error) {
	builder := pptx.NewPresentationBuilder(
		pptx.WithTitle(firstNonEmpty(deckTitle, "ppt-export")),
		pptx.WithLayout(pptx.Layout16x9),
	)

	for i, slideData := range slides {
		slide := builder.AddSlide().SetBackgroundColor(pptExportTheme.Background)
		addPPTThemeFrame(slide, i+1)

		if slideData.Title != "" {
			slide.AddText(slideData.Title).
				SetBold(true).
				SetFontSize(30).
				SetFontFamily("Aptos Display").
				SetColor(pptExportTheme.Title).
				SetPosition(pptx.Inches(1.15), pptx.Inches(0.72)).
				SetSize(pptx.Inches(10.2), pptx.Inches(0.78)).
				End()
		}

		startY := 1.75
		if slideData.Title == "" {
			startY = 0.95
		}
		for i, bullet := range slideData.Bullets {
			addPPTBulletCard(slide, i, bullet, startY)
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

func addPPTThemeFrame(slide *pptx.SlideBuilder, slideNumber int) {
	slide.AddShape(pptx.ShapeRectangle).
		SetPosition(pptx.Inches(0), pptx.Inches(0)).
		SetSize(pptx.Inches(13.333), pptx.Inches(0.16)).
		SetFillColor(pptExportTheme.Accent).
		SetNoLine().
		End()

	slide.AddShape(pptx.ShapeRectangle).
		SetPosition(pptx.Inches(0.62), pptx.Inches(0.72)).
		SetSize(pptx.Inches(0.12), pptx.Inches(5.85)).
		SetFillColor(pptExportTheme.Accent).
		SetNoLine().
		End()

	slide.AddShape(pptx.ShapeRoundedRectangle).
		SetPosition(pptx.Inches(11.65), pptx.Inches(0.54)).
		SetSize(pptx.Inches(0.75), pptx.Inches(0.42)).
		SetFillColor(pptExportTheme.Accent).
		SetNoLine().
		End()

	slide.AddText(fmt.Sprintf("%02d", slideNumber)).
		SetBold(true).
		SetFontSize(13).
		SetFontFamily("Aptos").
		SetColor(pptExportTheme.White).
		SetPosition(pptx.Inches(11.73), pptx.Inches(0.61)).
		SetSize(pptx.Inches(0.58), pptx.Inches(0.22)).
		End()

	slide.AddText("YoudaoNoteLM").
		SetFontSize(10).
		SetFontFamily("Aptos").
		SetColor(pptExportTheme.Muted).
		SetPosition(pptx.Inches(1.15), pptx.Inches(6.92)).
		SetSize(pptx.Inches(2.2), pptx.Inches(0.22)).
		End()
}

func addPPTBulletCard(slide *pptx.SlideBuilder, index int, bullet string, startY float64) {
	y := startY + float64(index)*0.72
	slide.AddShape(pptx.ShapeRoundedRectangle).
		SetPosition(pptx.Inches(1.12), pptx.Inches(y)).
		SetSize(pptx.Inches(10.8), pptx.Inches(0.54)).
		SetFillColor(pptExportTheme.AccentSoft).
		SetLine(pptExportTheme.Border, 1).
		End()

	slide.AddShape(pptx.ShapeRoundedRectangle).
		SetPosition(pptx.Inches(1.34), pptx.Inches(y+0.18)).
		SetSize(pptx.Inches(0.18), pptx.Inches(0.18)).
		SetFillColor(pptExportTheme.Accent).
		SetNoLine().
		End()

	slide.AddText(bullet).
		SetFontSize(17).
		SetFontFamily("Aptos").
		SetColor(pptExportTheme.Text).
		SetPosition(pptx.Inches(1.7), pptx.Inches(y+0.13)).
		SetSize(pptx.Inches(9.8), pptx.Inches(0.3)).
		End()
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
