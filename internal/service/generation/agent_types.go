// agent_types.go 定义 Agent 基础设施共享的类型。
//
// 主要类型：
//   - baseGenerationAgent：所有生成 Agent 的基类，持有 name/typ/model/validator/fallback
//   - generationDraft：LLM 生成的初稿结构
//   - learningContentAnalysis：学习内容分析结果
//   - pptChainState / pptOutlinePlan / pptSlidePlan / pptStyleTheme：PPT 链式状态和规划类型
//   - learningEvidence / pptSourceSection：引用证据和源材料片段
//
// 各类型 Agent（pptGenerationAgent 等）通过嵌入 baseGenerationAgent 复用基础能力。
package generation

type baseGenerationAgent struct {
	name      string
	typ       GenerationType
	model     GenerationModel
	fallback  func(generationAgentInput) string
	validator func(string) bool
}

type generationDraft struct {
	input             generationAgentInput
	content           string
	formatValid       bool
	fallbackUsed      bool
	pptRepairPlan     *pptOutlinePlan
	pptStyleTheme     pptStyleTheme
	mindmapRepairPlan *mindmapPlan
	noteRepairPlan    *noteOutlinePlan
	quizRepairPlan    *quizQuestionPlan
}

type learningContentAnalysis struct {
	Topic       string
	KeyConcepts []string
	Processes   []string
	Examples    []string
	Evidence    []learningEvidence
	Sections    []pptSourceSection
	Gaps        []string
	UserIntent  string
	Sparse      bool
}

type pptSourceSection struct {
	Title  string
	Points []string
}

type learningEvidence struct {
	Text   string
	Source string
}

type pptOutlinePlan struct {
	Title  string
	Slides []pptSlidePlan
}

type pptSlidePlan struct {
	Title   string
	Purpose string
	Bullets []string
}

type enrichedPPTSlide struct {
	Title      string   `json:"title"`
	Subtitle   string   `json:"subtitle,omitempty"`
	Paragraphs []string `json:"paragraphs"`
	Bullets    []string `json:"bullets,omitempty"`
	Insights   []string `json:"insights,omitempty"`
}

type pptRichContent struct {
	Slides []enrichedPPTSlide `json:"slides"`
}

type pptStyleTheme struct {
	Name        string // e.g. "简约商务", "学术清新", "科技深色"
	Primary     string // 主色
	Secondary   string // 辅助色
	Background  string // 页面背景色
	Surface     string // 内容面板背景色
	Text        string // 正文字色
	Heading     string // 标题字色
	Muted       string // 弱化字色
	FontHeading string // 标题字体
	FontBody    string // 正文字体
	LayoutHints string // 给模型使用的布局提示
}

const pptContentEnrichBatchSize = 4

type pptChainState struct {
	input       generationAgentInput
	analysis    learningContentAnalysis
	outlinePlan pptOutlinePlan
	expanded    pptOutlinePlan
	richContent pptRichContent
	styleTheme  pptStyleTheme
	cssBlock    string
	outline     string
}

type mindmapChainState struct {
	input    generationAgentInput
	analysis learningContentAnalysis
	plan     mindmapPlan
	expanded mindmapPlan
}

type noteChainState struct {
	input    generationAgentInput
	analysis learningContentAnalysis
	plan     noteOutlinePlan
	expanded noteOutlinePlan
}

type quizChainState struct {
	input    generationAgentInput
	analysis learningContentAnalysis
	plan     quizQuestionPlan
	expanded quizQuestionPlan
}
