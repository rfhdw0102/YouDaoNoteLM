package ppt

import (
	"github.com/duynguyendang/docxgo/v3/pptx"
)

const (
	dynamicPPTDefaultFontFamily = "Aptos"
	dynamicPPTTitleFontFamily   = "Aptos"
	dynamicPPTSlideWidth        = 13.333
	dynamicPPTSlideHeight       = 7.5
	dynamicPPTOuterMarginX      = 0.32
	dynamicPPTOuterMarginY      = 0.26
	dynamicPPTDefaultGap        = 0.14
	dynamicPPTDefaultBodyFont   = 17
	dynamicPPTMinBodyFontSize   = 14
)

var (
	dynamicPPTDefaultSlideBackground = pptx.Color{R: 246, G: 244, B: 239}
	dynamicPPTDefaultSectionFill     = pptx.Color{R: 252, G: 251, B: 248}
	dynamicPPTDefaultSectionBorder   = pptx.Color{R: 231, G: 224, B: 214}
	dynamicPPTDefaultText            = pptx.Color{R: 47, G: 42, B: 36}
	dynamicPPTDefaultMuted           = pptx.Color{R: 111, G: 104, B: 95}
	dynamicPPTDefaultAccent          = pptx.Color{R: 183, G: 170, B: 150}
)

type pptHTMLDocument struct {
	BodyStyle pptStyle
	Rules     []pptCSSRule
	Vars      map[string]string
	Slides    []pptHTMLSlide
}

type pptHTMLSlide struct {
	SectionStyle pptStyle
	Blocks       []pptHTMLBlock
}

type pptHTMLBlock struct {
	Kind     string
	Layout   string
	Text     string
	Runs     []pptHTMLTextRun
	Style    pptStyle
	Classes  map[string]bool
	Children []pptHTMLBlock
}

type pptHTMLTextRun struct {
	Text  string
	Style pptStyle
}

type pptCSSRule struct {
	Selector    string
	Parts       []pptCSSSelectorPart
	Style       pptStyle
	Specificity int
	Order       int
}

type pptCSSSelectorPart struct {
	Tag        string
	Classes    []string
	FirstChild bool
}

type pptStyleDeclaration struct {
	Key   string
	Value string
}

type pptStyle struct {
	TextColor           *pptx.Color
	BackgroundColor     *pptx.Color
	BorderColor         *pptx.Color
	BorderLeftColor     *pptx.Color
	BorderBottomColor   *pptx.Color
	FontSize            *int
	FontWeight          *int
	LineHeight          *float64
	FontFamily          string
	TextAlign           string
	Display             string
	GridTemplateColumns string
	FlexWrap            string
	Gap                 *float64
	Padding             pptEdges
	Margin              pptEdges
	BorderWidth         *int
	BorderLeftWidth     *int
	BorderBottomWidth   *int
	BorderRadius        *int
	ClearBackground     bool
	ClearBorder         bool
	ClearBorderLeft     bool
	ClearBorderBottom   bool
}

type pptEdges struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
	Set    bool
}

type pptLayoutCursor struct {
	x     float64
	y     float64
	width float64
}

type pptSectionFrame struct {
	x             float64
	y             float64
	width         float64
	height        float64
	paddingTop    float64
	paddingRight  float64
	paddingBottom float64
	paddingLeft   float64
}

type dynamicLayoutConfig struct {
	SlideWidth          float64
	SlideHeight         float64
	OuterMarginX        float64
	OuterMarginY        float64
	DefaultGap          float64
	ConservativeColumns bool
}

type measuredDynamicHTMLSlide struct {
	SectionStyle  pptStyle
	Frame         pptSectionFrame
	Blocks        []measuredDynamicBlock
	ContentBottom float64
}

type measuredDynamicBlock struct {
	Block    pptHTMLBlock
	X        float64
	Y        float64
	Width    float64
	Height   float64
	Children []measuredDynamicBlock
}
