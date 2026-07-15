package ppt

import (
	"github.com/duynguyendang/docxgo/v3/pptx"
	"regexp"
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
