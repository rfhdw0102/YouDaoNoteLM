package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"
)

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
	mindmapRepairPlan *mindmapPlan
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

type mindmapPlan struct {
	Title    string
	Branches []mindmapBranchPlan
}

type mindmapBranchPlan struct {
	Title string
	Nodes []mindmapNodePlan
}

type mindmapNodePlan struct {
	Title   string
	Details []string
}

type pptChainState struct {
	input       generationAgentInput
	analysis    learningContentAnalysis
	outlinePlan pptOutlinePlan
	expanded    pptOutlinePlan
	outline     string
}

type mindmapChainState struct {
	input    generationAgentInput
	analysis learningContentAnalysis
	plan     mindmapPlan
	expanded mindmapPlan
}

func (a *baseGenerationAgent) Generate(ctx context.Context, input generationAgentInput) (generationAgentOutput, error) {
	chain := compose.NewChain[generationAgentInput, generationAgentOutput]().
		AppendLambda(compose.InvokableLambda(a.generateDraft)).
		AppendLambda(compose.InvokableLambda(a.structureCheck)).
		AppendLambda(compose.InvokableLambda(a.factEnhance)).
		AppendLambda(compose.InvokableLambda(a.formatValidate)).
		AppendLambda(compose.InvokableLambda(a.finalize))

	runner, err := chain.Compile(ctx)
	if err != nil {
		return generationAgentOutput{}, err
	}
	return runner.Invoke(ctx, input)
}

func (a *baseGenerationAgent) generateDraft(ctx context.Context, input generationAgentInput) (generationDraft, error) {
	content := ""
	fallbackUsed := false
	strategy := promptStrategyFor(a.typ)
	if a.model != nil {
		generated, err := a.model.Generate(ctx, GenerationPrompt{
			AgentName:    a.name,
			System:       strategy.System,
			User:         strings.TrimSpace(input.Request.Prompt),
			Context:      input.Context,
			OutputFormat: strategy.OutputFormat,
		})
		if err != nil {
			return generationDraft{}, err
		}
		content = strings.TrimSpace(generated)
	}
	if content == "" {
		content = a.fallback(input)
		fallbackUsed = true
	}
	return generationDraft{input: input, content: content, fallbackUsed: fallbackUsed}, nil
}

func (a *baseGenerationAgent) structureCheck(ctx context.Context, draft generationDraft) (generationDraft, error) {
	if strings.TrimSpace(draft.content) == "" {
		draft.content = a.fallback(draft.input)
		draft.fallbackUsed = true
	}
	return draft, nil
}

func (a *baseGenerationAgent) factEnhance(ctx context.Context, draft generationDraft) (generationDraft, error) {
	return draft, nil
}

func (a *baseGenerationAgent) formatValidate(ctx context.Context, draft generationDraft) (generationDraft, error) {
	draft.formatValid = a.validator(draft.content)
	if !draft.formatValid {
		draft.content = a.fallback(draft.input)
		draft.fallbackUsed = true
		draft.formatValid = a.validator(draft.content)
	}
	return draft, nil
}

func (a *baseGenerationAgent) finalize(ctx context.Context, draft generationDraft) (generationAgentOutput, error) {
	return generationAgentOutput{
		Content:      strings.TrimSpace(draft.content),
		FormatValid:  draft.formatValid,
		FallbackUsed: draft.fallbackUsed,
	}, nil
}

func newMindmapAgent(model GenerationModel) generationAgent {
	return &mindmapGenerationAgent{
		baseGenerationAgent: baseGenerationAgent{
			name:      "mindmap",
			typ:       GenerationTypeMindmap,
			model:     model,
			validator: validateMindmapContent,
			fallback:  fallbackMindmapContent,
		},
	}
}

func newPPTAgent(model GenerationModel) generationAgent {
	return &pptGenerationAgent{
		baseGenerationAgent: baseGenerationAgent{
			name:      "ppt",
			typ:       GenerationTypePPT,
			model:     model,
			validator: validatePPTContent,
			fallback:  fallbackPPTContent,
		},
	}
}

type pptGenerationAgent struct {
	baseGenerationAgent
}

func (a *pptGenerationAgent) Generate(ctx context.Context, input generationAgentInput) (generationAgentOutput, error) {
	chain := compose.NewChain[generationAgentInput, generationAgentOutput]().
		AppendLambda(compose.InvokableLambda(a.analyzePPTContent)).
		AppendLambda(compose.InvokableLambda(a.planPPTChainOutline)).
		AppendLambda(compose.InvokableLambda(a.expandPPTChainContent)).
		AppendLambda(compose.InvokableLambda(a.generatePPTDraft)).
		AppendLambda(compose.InvokableLambda(a.structureCheck)).
		AppendLambda(compose.InvokableLambda(a.repairPPTStructure)).
		AppendLambda(compose.InvokableLambda(a.factEnhance)).
		AppendLambda(compose.InvokableLambda(a.formatValidate)).
		AppendLambda(compose.InvokableLambda(a.finalize))

	runner, err := chain.Compile(ctx)
	if err != nil {
		return generationAgentOutput{}, err
	}
	return runner.Invoke(ctx, input)
}

type mindmapGenerationAgent struct {
	baseGenerationAgent
}

func (a *mindmapGenerationAgent) Generate(ctx context.Context, input generationAgentInput) (generationAgentOutput, error) {
	chain := compose.NewChain[generationAgentInput, generationAgentOutput]().
		AppendLambda(compose.InvokableLambda(a.analyzeMindmapContent)).
		AppendLambda(compose.InvokableLambda(a.planMindmapOutline)).
		AppendLambda(compose.InvokableLambda(a.expandMindmapChainContent)).
		AppendLambda(compose.InvokableLambda(a.generateMindmapDraft)).
		AppendLambda(compose.InvokableLambda(a.structureCheck)).
		AppendLambda(compose.InvokableLambda(a.repairMindmapStructure)).
		AppendLambda(compose.InvokableLambda(a.factEnhance)).
		AppendLambda(compose.InvokableLambda(a.formatValidate)).
		AppendLambda(compose.InvokableLambda(a.finalize))

	runner, err := chain.Compile(ctx)
	if err != nil {
		return generationAgentOutput{}, err
	}
	return runner.Invoke(ctx, input)
}

func (a *pptGenerationAgent) analyzePPTContent(ctx context.Context, input generationAgentInput) (pptChainState, error) {
	return pptChainState{
		input:    input,
		analysis: analyzeLearningContent(input),
	}, nil
}

func (a *pptGenerationAgent) planPPTChainOutline(ctx context.Context, state pptChainState) (pptChainState, error) {
	if len(state.analysis.Sections) > 0 {
		state.outline = fallbackPPTOutline(state.input)
		state.outlinePlan = planPPTOutline(state.analysis)
		return state, nil
	}
	outline, err := a.generateOutline(ctx, state.input)
	if err != nil {
		return pptChainState{}, err
	}
	state.outline = outline
	if parsed, ok := parsePPTOutlineMarkdown(outline); ok {
		state.outlinePlan = parsed
	} else {
		state.outlinePlan = planPPTOutline(state.analysis)
	}
	return state, nil
}

func (a *pptGenerationAgent) expandPPTChainContent(ctx context.Context, state pptChainState) (pptChainState, error) {
	state.expanded = expandPPTContent(state.outlinePlan, state.analysis)
	return state, nil
}

func (a *pptGenerationAgent) generatePPTDraft(ctx context.Context, state pptChainState) (generationDraft, error) {
	content := ""
	fallbackUsed := false
	strategy := promptStrategyFor(GenerationTypePPT)
	if a.model != nil {
		generated, err := a.model.Generate(ctx, GenerationPrompt{
			AgentName:    a.name,
			System:       strategy.System,
			User:         strings.TrimSpace(state.input.Request.Prompt),
			Context:      appendPPTPlansToContext(state.input.Context, state.outline, state.expanded),
			OutputFormat: strategy.OutputFormat,
		})
		if err != nil {
			return generationDraft{}, err
		}
		content = strings.TrimSpace(generated)
	}
	if content == "" {
		content = renderStyledPPTSlides(state.expanded)
		fallbackUsed = true
	}
	repairPlan := state.expanded
	return generationDraft{input: state.input, content: content, fallbackUsed: fallbackUsed, pptRepairPlan: &repairPlan}, nil
}

func (a *pptGenerationAgent) repairPPTStructure(ctx context.Context, draft generationDraft) (generationDraft, error) {
	draft.content = stripPPTExportPlaceholders(draft.content)
	draft.content = sanitizePPTReferenceSections(draft.content)
	if pptCanPatchCanvas(draft.content) {
		draft.content = patchPPTCanvasHTML(draft.content)
	}
	if pptNeedsStructureRepair(draft.content) ||
		pptContainsInternalPromptLeak(draft.content) ||
		pptContainsUnrelatedBoilerplate(draft.content, draft.input) ||
		pptNeedsPlanCoverageRepair(draft.content, draft.pptRepairPlan) {
		if draft.pptRepairPlan != nil {
			draft.content = renderStyledPPTSlides(*draft.pptRepairPlan)
		} else {
			draft.content = a.fallback(draft.input)
		}
		draft.fallbackUsed = true
	}
	return draft, nil
}

func (a *mindmapGenerationAgent) analyzeMindmapContent(ctx context.Context, input generationAgentInput) (mindmapChainState, error) {
	return mindmapChainState{
		input:    input,
		analysis: analyzeLearningContent(input),
	}, nil
}

func (a *mindmapGenerationAgent) planMindmapOutline(ctx context.Context, state mindmapChainState) (mindmapChainState, error) {
	state.plan = planMindmap(state.analysis)
	return state, nil
}

func (a *mindmapGenerationAgent) expandMindmapChainContent(ctx context.Context, state mindmapChainState) (mindmapChainState, error) {
	state.expanded = expandMindmapContent(state.plan, state.analysis)
	return state, nil
}

func (a *mindmapGenerationAgent) generateMindmapDraft(ctx context.Context, state mindmapChainState) (generationDraft, error) {
	input := state.input
	input.Context = appendMindmapPlansToContext(state.input.Context, state.plan, state.expanded)
	draft, err := a.generateDraft(ctx, input)
	if err != nil {
		return generationDraft{}, err
	}
	repairPlan := state.expanded
	if strings.TrimSpace(repairPlan.Title) == "" {
		repairPlan = state.plan
	}
	draft.mindmapRepairPlan = &repairPlan
	return draft, nil
}

func (a *mindmapGenerationAgent) repairMindmapStructure(ctx context.Context, draft generationDraft) (generationDraft, error) {
	if mindmapNeedsStructureRepair(draft.content) {
		if draft.mindmapRepairPlan != nil {
			draft.content = renderMindmap(*draft.mindmapRepairPlan)
		} else {
			draft.content = a.fallback(draft.input)
		}
		draft.fallbackUsed = true
	}
	return draft, nil
}

func (a *pptGenerationAgent) generateOutline(ctx context.Context, input generationAgentInput) (string, error) {
	if a.model == nil {
		return fallbackPPTOutline(input), nil
	}
	strategy := pptOutlinePromptStrategy()
	outline, err := a.model.Generate(ctx, GenerationPrompt{
		AgentName:    "ppt_outline",
		System:       strategy.System,
		User:         strings.TrimSpace(input.Request.Prompt),
		Context:      input.Context,
		OutputFormat: strategy.OutputFormat,
	})
	if err != nil {
		return "", err
	}
	outline = strings.TrimSpace(outline)
	if outline == "" {
		return fallbackPPTOutline(input), nil
	}
	return outline, nil
}

func fallbackPPTContent(input generationAgentInput) string {
	analysis := analyzeLearningContent(input)
	return renderStyledPPTSlides(expandPPTContent(planPPTOutline(analysis), analysis))
}
func fallbackPPTOutline(input generationAgentInput) string {
	plan := planPPTOutline(analyzeLearningContent(input))
	var outline strings.Builder
	outline.WriteString("# ")
	outline.WriteString(plan.Title)
	outline.WriteString("\n")
	for _, slide := range plan.Slides {
		outline.WriteString("- ")
		outline.WriteString(strings.TrimSpace(slide.Title))
		outline.WriteString("\n")
		for _, point := range slide.Bullets {
			point = strings.TrimSpace(point)
			if point == "" {
				continue
			}
			outline.WriteString("  - ")
			outline.WriteString(point)
			outline.WriteString("\n")
		}
	}
	return strings.TrimSpace(outline.String())
}

func parsePPTOutlineMarkdown(outline string) (pptOutlinePlan, bool) {
	lines := strings.Split(strings.TrimSpace(outline), "\n")
	plan := pptOutlinePlan{}
	var current *pptSlidePlan
	for _, line := range lines {
		raw := strings.TrimRight(line, " \t\r")
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			title := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			if title != "" && plan.Title == "" {
				plan.Title = title
			}
			continue
		}

		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))
		text := strings.TrimSpace(strings.TrimLeft(trimmed, "-*0123456789. "))
		if text == "" {
			continue
		}
		if indent == 0 {
			plan.Slides = append(plan.Slides, pptSlidePlan{Title: text})
			current = &plan.Slides[len(plan.Slides)-1]
			continue
		}
		if current == nil {
			plan.Slides = append(plan.Slides, pptSlidePlan{Title: firstNonEmpty(plan.Title, "Slide")})
			current = &plan.Slides[len(plan.Slides)-1]
		}
		current.Bullets = append(current.Bullets, text)
	}
	if plan.Title == "" && len(plan.Slides) > 0 {
		plan.Title = plan.Slides[0].Title
	}
	if strings.TrimSpace(plan.Title) == "" || len(plan.Slides) == 0 {
		return pptOutlinePlan{}, false
	}
	for i := range plan.Slides {
		normalizePPTBullets(&plan.Slides[i])
		if strings.TrimSpace(plan.Slides[i].Purpose) == "" {
			plan.Slides[i].Purpose = purposeForPPTSlide(plan.Slides[i].Title, i, len(plan.Slides))
		}
	}
	plan = ensurePPTPlanFrame(plan)
	return plan, true
}

func ensurePPTPlanFrame(plan pptOutlinePlan) pptOutlinePlan {
	if strings.TrimSpace(plan.Title) == "" {
		plan.Title = "演示文稿"
	}
	if len(plan.Slides) == 0 {
		return planPPTOutline(learningContentAnalysis{Topic: plan.Title, Sparse: true})
	}
	if !isCoverSlideTitle(plan.Slides[0].Title) {
		cover := pptSlidePlan{
			Title:   "封面页",
			Purpose: "建立演示主题",
			Bullets: []string{plan.Title},
		}
		plan.Slides = append([]pptSlidePlan{cover}, plan.Slides...)
	}
	if len(plan.Slides) < 2 || !isAgendaSlideTitle(plan.Slides[1].Title) {
		agenda := pptSlidePlan{
			Title:   "目录页",
			Purpose: "呈现演示路径",
		}
		for _, slide := range plan.Slides[1:] {
			if !isEndingSlideTitle(slide.Title) {
				agenda.Bullets = append(agenda.Bullets, slide.Title)
			}
		}
		if len(agenda.Bullets) == 0 {
			agenda.Bullets = append(agenda.Bullets, "内容页")
		}
		plan.Slides = append([]pptSlidePlan{plan.Slides[0], agenda}, plan.Slides[1:]...)
	}
	last := plan.Slides[len(plan.Slides)-1]
	if !isEndingSlideTitle(last.Title) {
		plan.Slides = append(plan.Slides, pptSlidePlan{
			Title:   "结束页",
			Purpose: "收束结论与下一步",
			Bullets: []string{"总结核心观点", "明确下一步行动"},
		})
	}
	for i := range plan.Slides {
		if strings.TrimSpace(plan.Slides[i].Purpose) == "" {
			plan.Slides[i].Purpose = purposeForPPTSlide(plan.Slides[i].Title, i, len(plan.Slides))
		}
		normalizePPTBullets(&plan.Slides[i])
	}
	return plan
}

func purposeForPPTSlide(title string, index, total int) string {
	switch {
	case index == 0 || isCoverSlideTitle(title):
		return "建立演示主题"
	case index == 1 || isAgendaSlideTitle(title):
		return "呈现演示路径"
	case index == total-1 || isEndingSlideTitle(title):
		return "收束结论与下一步"
	default:
		return "展开核心内容"
	}
}

func isCoverSlideTitle(title string) bool {
	return containsAnyFold(title, "封面", "cover", "title")
}

func isAgendaSlideTitle(title string) bool {
	return containsAnyFold(title, "目录", "agenda", "outline")
}

func isEndingSlideTitle(title string) bool {
	return containsAnyFold(title, "结束", "总结", "closing", "end", "finish")
}

func stripPPTExportPlaceholders(content string) string {
	cleaned := stripTaggedBlock(content, "PPT_FILE")
	cleaned = stripTaggedBlock(cleaned, "PREVIEW_LINK")
	return strings.TrimSpace(cleaned)
}

func stripTaggedBlock(content, tag string) string {
	lower := strings.ToLower(content)
	openTag := "<" + strings.ToLower(tag) + ">"
	closeTag := "</" + strings.ToLower(tag) + ">"
	for {
		start := strings.Index(lower, openTag)
		if start < 0 {
			return content
		}
		closeStart := strings.Index(lower[start+len(openTag):], closeTag)
		if closeStart < 0 {
			return content[:start] + content[start+len(openTag):]
		}
		end := start + len(openTag) + closeStart + len(closeTag)
		content = content[:start] + content[end:]
		lower = strings.ToLower(content)
	}
}

func appendPPTPlansToContext(contextValue, outline string, plan pptOutlinePlan) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(contextValue))
	if strings.TrimSpace(outline) != "" {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("内部演示大纲：\n")
		b.WriteString(outline)
	}
	if strings.TrimSpace(plan.Title) != "" || len(plan.Slides) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("STRUCTURED_PPT_PLAN\n")
		b.WriteString(renderPPTPlanForPrompt(plan))
		b.WriteString("\n\nPPT_HTML_GENERATION_RULES\n")
		b.WriteString("- Treat each Slide entry as the writing brief for exactly one HTML <section>; do not merge, omit, or reorder slides.\n")
		b.WriteString("- The outline bullets are source material, not the final wording. Expand each slide into polished presentation content with concrete explanations, comparisons, process details, or examples grounded in the provided Markdown.\n")
		b.WriteString("- Keep every planned slide title visible as h1/h2/h3, then add 3-5 substantial points or content blocks for that page.\n")
		b.WriteString("- The agenda slide must match the later content sections one-to-one; if the agenda lists a chapter, a later section with the same chapter title must exist.\n")
		b.WriteString("- Design every section as a real 16:9 PPT canvas: width: 1920px; height: 1080px; overflow: hidden; use px font sizes, not web-card rem layouts.\n")
		b.WriteString("- Make each HTML section production-ready as a standalone slide: include a clear visual hierarchy, title area, content grouping, and a small progress or section marker.\n")
		b.WriteString("- Page copy must be grounded in Original Markdown, Local References, Web Results, or the user's explicit prompt. Do not add generic business, market, team, investor, or motivational boilerplate unless it appears in the source.\n")
		b.WriteString("- Expand terse bullets into concise slide-ready statements only by clarifying the source meaning; do not introduce unrelated examples, claims, or slogans.\n")
		b.WriteString("- Finish all planned sections before returning. If the plan is long, make each section concise instead of truncating the deck.\n")
	}
	return strings.TrimSpace(b.String())
}

func renderPPTPlanForPrompt(plan pptOutlinePlan) string {
	var b strings.Builder
	if strings.TrimSpace(plan.Title) != "" {
		b.WriteString("Title: ")
		b.WriteString(strings.TrimSpace(plan.Title))
		b.WriteString("\n")
	}
	for i, slide := range plan.Slides {
		b.WriteString(fmt.Sprintf("Slide %02d: %s\n", i+1, strings.TrimSpace(slide.Title)))
		if strings.TrimSpace(slide.Purpose) != "" {
			b.WriteString("Purpose: ")
			b.WriteString(strings.TrimSpace(slide.Purpose))
			b.WriteString("\n")
		}
		for _, bullet := range slide.Bullets {
			bullet = strings.TrimSpace(bullet)
			if bullet == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(bullet)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func fallbackMindmapContent(input generationAgentInput) string {
	analysis := analyzeLearningContent(input)
	return renderMindmap(expandMindmapContent(planMindmap(analysis), analysis))
}

func appendMindmapPlansToContext(contextValue string, plan, expanded mindmapPlan) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(contextValue))
	if strings.TrimSpace(plan.Title) != "" {
		b.WriteString("\n\nINTERNAL_MINDMAP_PLAN\n")
		b.WriteString("内部思维导图规划：\n")
		b.WriteString(renderMindmapPlan(plan))
	}
	if strings.TrimSpace(expanded.Title) != "" {
		b.WriteString("\n\nINTERNAL_MINDMAP_EXPANDED_PLAN\n")
		b.WriteString("内部思维导图扩展：\n")
		b.WriteString(renderMindmap(expanded))
	}
	return strings.TrimSpace(b.String())
}

func renderMindmapPlan(plan mindmapPlan) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(plan.Title)
	b.WriteString("\n")
	for _, branch := range plan.Branches {
		b.WriteString("## ")
		b.WriteString(branch.Title)
		b.WriteString("\n")
		for _, node := range branch.Nodes {
			b.WriteString("- ")
			b.WriteString(node.Title)
			b.WriteString("\n")
			for _, detail := range node.Details {
				b.WriteString("  - ")
				b.WriteString(detail)
				b.WriteString("\n")
			}
		}
	}
	return strings.TrimSpace(b.String())
}
func learningDeckSections() []string {
	return []string{"背景与目标", "概念框架", "机制与流程", "案例与应用", "易错辨析", "总结复盘"}
}

func analyzeLearningContent(input generationAgentInput) learningContentAnalysis {
	markdown := ""
	prompt := ""
	if input.Request != nil {
		markdown = input.Request.Markdown
		prompt = input.Request.Prompt
	}
	sections := extractPPTSourceSections(markdown, 18)
	sections, focused := focusPPTSectionsByPrompt(sections, prompt)
	points := extractKeyPoints(markdown, 48)
	if focused && len(sections) > 0 {
		points = pointsFromPPTSections(sections, 48)
	}
	references := input.References
	if focused || len(sections) > 0 {
		references = focusPPTReferencesByPrompt(references, prompt, sections)
	}
	evidence := append(evidenceFromReferences(references), evidenceFromSearch(input.SearchResults)...)
	if len(sections) == 0 && len(references) > 0 {
		sections = pptSectionsFromReferences(references, 12)
		points = pointsFromPPTSections(sections, 48)
	}
	if focused && len(evidence) > 0 {
		for _, ev := range evidence {
			points = append(points, ev.Text)
		}
		points = uniqueNonEmpty(points)
	}
	analysis := learningContentAnalysis{
		Topic:       extractTitle(markdown, "学习资料"),
		KeyConcepts: points,
		UserIntent:  strings.TrimSpace(prompt),
		Evidence:    evidence,
		Sections:    sections,
		Sparse:      len(points) < 3,
	}
	if focused && len(sections) == 1 {
		analysis.Topic = sections[0].Title
	}
	for _, point := range points {
		switch {
		case containsAnyFold(point, "步骤", "流程", "机制", "反应", "cycle", "process"):
			analysis.Processes = append(analysis.Processes, point)
		case containsAnyFold(point, "例", "应用", "场景", "example", "case"):
			analysis.Examples = append(analysis.Examples, point)
		}
	}
	if len(analysis.KeyConcepts) == 0 {
		analysis.Gaps = append(analysis.Gaps, "核心概念")
	}
	if len(analysis.Processes) == 0 {
		analysis.Gaps = append(analysis.Gaps, "过程机制")
	}
	if len(analysis.Examples) == 0 {
		analysis.Gaps = append(analysis.Gaps, "例子应用")
	}
	return analysis
}

func focusPPTSectionsByPrompt(sections []pptSourceSection, prompt string) ([]pptSourceSection, bool) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" || len(sections) == 0 {
		return sections, false
	}
	focused := make([]pptSourceSection, 0, len(sections))
	for _, section := range sections {
		if pptPromptMatchesFocusText(prompt, section.Title) {
			focused = append(focused, section)
		}
	}
	if len(focused) == 0 {
		return sections, false
	}
	return focused, true
}

func focusPPTReferencesByPrompt(refs []GenerationReference, prompt string, sections []pptSourceSection) []GenerationReference {
	if len(refs) == 0 {
		return refs
	}
	prompt = strings.TrimSpace(prompt)
	sectionTitles := make([]string, 0, len(sections))
	for _, section := range sections {
		if strings.TrimSpace(section.Title) != "" {
			sectionTitles = append(sectionTitles, section.Title)
		}
	}
	focused := make([]GenerationReference, 0, len(refs))
	for _, ref := range refs {
		if pptReferenceMatchesFocus(ref, prompt, sectionTitles) {
			focused = append(focused, ref)
		}
	}
	if len(focused) == 0 {
		return refs
	}
	return focused
}

func pptReferenceMatchesFocus(ref GenerationReference, prompt string, sectionTitles []string) bool {
	for _, title := range sectionTitles {
		if pptPromptMatchesFocusText(ref.Heading, title) ||
			pptPromptMatchesFocusText(ref.ChapterPath, title) ||
			pptPromptMatchesFocusText(ref.Content, title) {
			return true
		}
	}
	if prompt == "" {
		return len(sectionTitles) == 0
	}
	return pptPromptMatchesFocusText(prompt, ref.Heading) ||
		pptPromptMatchesFocusText(prompt, ref.ChapterPath)
}

func pptPromptMatchesFocusText(prompt, text string) bool {
	prompt = strings.ToLower(strings.TrimSpace(prompt))
	text = strings.ToLower(strings.TrimSpace(text))
	if prompt == "" || text == "" {
		return false
	}
	if strings.Contains(prompt, text) || strings.Contains(text, prompt) {
		return true
	}
	for _, token := range splitKeywordCandidates(text) {
		token = strings.ToLower(strings.TrimSpace(token))
		if len([]rune(token)) >= 2 && strings.Contains(prompt, token) {
			return true
		}
	}
	return false
}

func pointsFromPPTSections(sections []pptSourceSection, limit int) []string {
	var points []string
	for _, section := range sections {
		points = append(points, section.Points...)
	}
	points = uniqueNonEmpty(points)
	if limit > 0 && len(points) > limit {
		return append([]string{}, points[:limit]...)
	}
	return points
}

func pptSectionsFromReferences(refs []GenerationReference, limit int) []pptSourceSection {
	sectionsByTitle := map[string]int{}
	sections := make([]pptSourceSection, 0, len(refs))
	for _, ref := range refs {
		title := firstNonEmpty(ref.Heading, ref.ChapterPath, ref.SourceName, "相关资料")
		point := strings.TrimSpace(summarizeLine(ref.Content, 120))
		if point == "" {
			continue
		}
		if idx, ok := sectionsByTitle[title]; ok {
			sections[idx].Points = append(sections[idx].Points, point)
			continue
		}
		sectionsByTitle[title] = len(sections)
		sections = append(sections, pptSourceSection{
			Title:  title,
			Points: []string{point},
		})
		if limit > 0 && len(sections) >= limit {
			break
		}
	}
	return sections
}

func evidenceFromReferences(refs []GenerationReference) []learningEvidence {
	evidence := make([]learningEvidence, 0, len(refs))
	for _, ref := range refs {
		text := strings.TrimSpace(summarizeLine(ref.Content, 120))
		if text == "" {
			continue
		}
		evidence = append(evidence, learningEvidence{Text: text, Source: generationReferenceLabel(ref)})
	}
	return evidence
}

func evidenceFromSearch(results []SearchResult) []learningEvidence {
	evidence := make([]learningEvidence, 0, len(results))
	for _, result := range results {
		text := strings.TrimSpace(summarizeLine(firstNonEmpty(result.Snippet, result.Content), 120))
		if text == "" {
			continue
		}
		evidence = append(evidence, learningEvidence{Text: text, Source: firstNonEmpty(result.Title, result.URL, "web")})
	}
	return evidence
}

func containsAnyFold(value string, terms ...string) bool {
	value = strings.ToLower(value)
	for _, term := range terms {
		if strings.Contains(value, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func extractPPTSourceSections(markdown string, maxSections int) []pptSourceSection {
	if maxSections <= 0 {
		maxSections = 18
	}
	var sections []pptSourceSection
	var overview []string
	var current *pptSourceSection
	flush := func() {
		if current == nil {
			return
		}
		current.Points = uniqueNonEmpty(current.Points)
		if strings.TrimSpace(current.Title) != "" && len(current.Points) > 0 {
			sections = append(sections, *current)
		}
		current = nil
	}

	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			level := 0
			for level < len(line) && line[level] == '#' {
				level++
			}
			title := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if title == "" {
				continue
			}
			if level == 1 && len(sections) == 0 && current == nil {
				continue
			}
			if level <= 3 {
				flush()
				if len(sections) >= maxSections {
					break
				}
				current = &pptSourceSection{Title: title}
				continue
			}
		}
		point := strings.TrimSpace(strings.TrimLeft(line, "-*0123456789. "))
		if len([]rune(point)) < 4 {
			continue
		}
		if current == nil {
			overview = append(overview, point)
			continue
		}
		current.Points = append(current.Points, point)
	}
	flush()
	if len(overview) > 0 {
		sections = append([]pptSourceSection{{
			Title:  "概述",
			Points: uniqueNonEmpty(overview),
		}}, sections...)
	}
	return sections
}

func requiredPPTSlideTitles() []string {
	return []string{"封面", "目录", "背景与目标", "概念框架", "机制与流程", "案例与应用", "易错辨析", "总结复盘"}
}

func planPPTOutline(analysis learningContentAnalysis) pptOutlinePlan {
	return planDynamicPPTOutline(analysis)
}

func planStaticPPTOutline(analysis learningContentAnalysis) pptOutlinePlan {
	plan := pptOutlinePlan{Title: analysis.Topic}
	for _, title := range requiredPPTSlideTitles() {
		slide := pptSlidePlan{Title: title}
		switch title {
		case "封面":
			slide.Purpose = "建立学习主题"
			slide.Bullets = append(slide.Bullets, analysis.Topic)
			if analysis.UserIntent != "" {
				slide.Bullets = append(slide.Bullets, analysis.UserIntent)
			}
		case "目录":
			slide.Purpose = "呈现学习路径"
			slide.Bullets = append(slide.Bullets, requiredPPTSlideTitles()[2:]...)
		case "背景与目标":
			slide.Purpose = "说明为什么学习"
			slide.Bullets = append(slide.Bullets, fmt.Sprintf("围绕“%s”建立学习背景和目标。", analysis.Topic), "先形成整体问题，再进入概念、机制和应用。")
		case "概念框架":
			slide.Purpose = "提炼关键概念并建立概念关系"
			slide.Bullets = append(slide.Bullets, analysis.KeyConcepts...)
		case "机制与流程":
			slide.Purpose = "组织过程和因果"
			slide.Bullets = append(slide.Bullets, analysis.Processes...)
		case "案例与应用":
			slide.Purpose = "连接例子和迁移"
			slide.Bullets = append(slide.Bullets, analysis.Examples...)
		case "易错辨析":
			slide.Purpose = "提示边界和误区"
			slide.Bullets = append(slide.Bullets, "区分相近概念、条件范围和结论边界。")
		case "总结复盘":
			slide.Purpose = "收束学习结论"
			slide.Bullets = append(slide.Bullets, fmt.Sprintf("用结构化方式回顾“%s”的核心内容。", analysis.Topic))
		}
		if len(slide.Bullets) == 0 {
			slide.Bullets = append(slide.Bullets, supplementBullet(title, "根据学习目标补足必要解释。"))
		}
		plan.Slides = append(plan.Slides, slide)
	}
	return plan
}

func planDynamicPPTOutline(analysis learningContentAnalysis) pptOutlinePlan {
	plan := pptOutlinePlan{Title: analysis.Topic}
	contentSlides := pptContentSlidesFromAnalysis(analysis)
	plan.Slides = append(plan.Slides, pptSlidePlan{
		Title:   "封面",
		Purpose: "建立演示主题和受众预期",
		Bullets: uniqueNonEmpty([]string{analysis.Topic, analysis.UserIntent}),
	})
	agenda := pptSlidePlan{
		Title:   "目录",
		Purpose: "呈现演示路径",
	}
	for _, slide := range contentSlides {
		agenda.Bullets = append(agenda.Bullets, slide.Title)
	}
	normalizePPTBullets(&agenda)
	plan.Slides = append(plan.Slides, agenda)
	plan.Slides = append(plan.Slides, contentSlides...)
	if len(contentSlides) == 0 {
		plan.Slides = append(plan.Slides, supplementalPPTSlides(analysis, 1)...)
	}
	plan.Slides = append(plan.Slides, pptSlidePlan{
		Title:   "总结与行动",
		Purpose: "收束核心结论并给出下一步",
		Bullets: []string{
			fmt.Sprintf("回顾“%s”的核心结构和关键判断。", analysis.Topic),
			"提炼最需要记住的结论、风险点和应用场景。",
			"明确后续复习、讲解或执行时的重点顺序。",
		},
	})
	return plan
}

func pptContentSlidesFromAnalysis(analysis learningContentAnalysis) []pptSlidePlan {
	sections := analysis.Sections
	if len(sections) == 0 {
		sections = sectionsFromFlatPoints(analysis.KeyConcepts, 12)
	}
	if len(sections) == 0 {
		sections = []pptSourceSection{{
			Title:  "核心内容",
			Points: append([]string{}, analysis.KeyConcepts...),
		}}
	}
	if len(sections) > 16 {
		sections = sections[:16]
	}
	slides := make([]pptSlidePlan, 0, len(sections))
	for i, section := range sections {
		title := strings.TrimSpace(section.Title)
		if title == "" {
			title = fmt.Sprintf("内容展开 %d", i+1)
		}
		points := uniqueNonEmpty(section.Points)
		if len(points) == 0 && i < len(analysis.Evidence) {
			points = append(points, analysis.Evidence[i].Text)
		}
		if len(points) == 0 {
			continue
		}
		for chunkIndex, chunk := range chunkPPTPoints(points, 4) {
			slideTitle := title
			if chunkIndex > 0 {
				slideTitle = fmt.Sprintf("%s (%d)", title, chunkIndex+1)
			}
			slides = append(slides, pptSlidePlan{
				Title:   slideTitle,
				Purpose: pptSectionPurpose(title, i, len(sections)),
				Bullets: chunk,
			})
		}
	}
	return slides
}

func chunkPPTPoints(points []string, size int) [][]string {
	points = uniqueNonEmpty(points)
	if len(points) == 0 {
		return nil
	}
	if size <= 0 {
		size = 4
	}
	chunks := make([][]string, 0, (len(points)+size-1)/size)
	for start := 0; start < len(points); start += size {
		end := start + size
		if end > len(points) {
			end = len(points)
		}
		chunks = append(chunks, append([]string{}, points[start:end]...))
	}
	return chunks
}

func sectionsFromFlatPoints(points []string, maxSections int) []pptSourceSection {
	points = uniqueNonEmpty(points)
	if len(points) == 0 {
		return nil
	}
	if maxSections <= 0 {
		maxSections = 12
	}
	var sections []pptSourceSection
	for i := 0; i < len(points) && len(sections) < maxSections; i += 4 {
		end := i + 4
		if end > len(points) {
			end = len(points)
		}
		sections = append(sections, pptSourceSection{
			Title:  summarizeLine(points[i], 36),
			Points: append([]string{}, points[i:end]...),
		})
	}
	return sections
}

func supplementalPPTSlides(analysis learningContentAnalysis, count int) []pptSlidePlan {
	if len(analysis.Sections) > 0 {
		return nil
	}
	candidates := []pptSlidePlan{
		{
			Title:   "背景与目标",
			Purpose: "说明材料背景和演示目标",
			Bullets: []string{fmt.Sprintf("围绕“%s”建立背景、问题和目标。", analysis.Topic), "说明为什么需要理解这些内容。", "给出本次演示的学习或行动范围。"},
		},
		{
			Title:   "关键关系",
			Purpose: "梳理概念之间的关系",
			Bullets: append([]string{}, analysis.KeyConcepts...),
		},
		{
			Title:   "应用与案例",
			Purpose: "连接材料和实际使用场景",
			Bullets: append([]string{}, analysis.Examples...),
		},
	}
	if count > len(candidates) {
		count = len(candidates)
	}
	return candidates[:count]
}

func appendRealEvidencePPTSlides(slides []pptSlidePlan, analysis learningContentAnalysis, count int) []pptSlidePlan {
	if count <= 0 || len(analysis.Evidence) == 0 {
		return slides
	}
	used := map[string]struct{}{}
	for _, slide := range slides {
		for _, bullet := range slide.Bullets {
			used[strings.TrimSpace(bullet)] = struct{}{}
		}
	}
	points := make([]string, 0, count*3)
	for _, ev := range analysis.Evidence {
		point := strings.TrimSpace(ev.Text)
		if point == "" {
			continue
		}
		if _, ok := used[point]; ok {
			continue
		}
		points = append(points, point)
		if len(points) >= count*3 {
			break
		}
	}
	for i := 0; i < len(points) && count > 0; i += 3 {
		end := i + 3
		if end > len(points) {
			end = len(points)
		}
		slides = append(slides, pptSlidePlan{
			Title:   fmt.Sprintf("补充资料 %d", len(slides)+1),
			Purpose: "呈现检索或引用资料中的真实要点",
			Bullets: append([]string{}, points[i:end]...),
		})
		count--
	}
	return slides
}

func pptSectionPurpose(title string, index, total int) string {
	switch {
	case index == 0:
		return "展开材料开头的核心背景和问题"
	case index == total-1:
		return "收束本部分材料并提炼结论"
	default:
		return fmt.Sprintf("解释“%s”的关键论点、证据和推导关系", title)
	}
}

func expandPPTContent(plan pptOutlinePlan, analysis learningContentAnalysis) pptOutlinePlan {
	evidenceIndex := 0
	for i := range plan.Slides {
		slide := &plan.Slides[i]
		if slide.Title != "封面" && slide.Title != "目录" {
			for len(slide.Bullets) < 3 && evidenceIndex < len(analysis.Evidence) {
				ev := analysis.Evidence[evidenceIndex]
				slide.Bullets = append(slide.Bullets, ev.Text)
				evidenceIndex++
			}
		}
		normalizePPTBullets(slide)
	}
	return plan
}

func normalizePPTBullets(slide *pptSlidePlan) {
	slide.Bullets = uniqueNonEmpty(slide.Bullets)
	switch slide.Title {
	case "封面", "目录":
		return
	}
	if len(slide.Bullets) > 5 {
		slide.Bullets = append([]string{}, slide.Bullets[:5]...)
	}
}

func renderPPTSlides(plan pptOutlinePlan) string {
	var b strings.Builder
	for _, slide := range plan.Slides {
		b.WriteString("<section>")
		if slide.Title == "封面" {
			b.WriteString("<h2>封面</h2>")
			b.WriteString("<h1>")
			b.WriteString(htmlEscape(plan.Title))
			b.WriteString("</h1>")
		} else {
			b.WriteString("<h2>")
			b.WriteString(htmlEscape(slide.Title))
			b.WriteString("</h2>")
		}
		if len(slide.Bullets) > 0 {
			b.WriteString("<ul>")
			for _, bullet := range slide.Bullets {
				b.WriteString("<li>")
				b.WriteString(htmlEscape(bullet))
				b.WriteString("</li>")
			}
			b.WriteString("</ul>")
		}
		b.WriteString("</section>\n")
	}
	return strings.TrimSpace(b.String())
}

func renderStyledPPTSlides(plan pptOutlinePlan) string {
	plan = sanitizePPTPlanVisibleText(plan)
	var b strings.Builder
	b.WriteString(`<style>
:root {
  --bg: #f8fafc; --surface: #ffffff; --surface-soft: #eef6f2;
  --accent: #0f766e; --accent-2: #c2410c; --accent-soft: #d9f1ec;
  --text: #111827; --muted: #4b5563; --heading: #0f172a;
  --border: #d7e3df; --panel: #f3f7f6;
}
* { margin: 0; padding: 0; box-sizing: border-box; }
.ppt-slide {
  position: relative;
  width: 1920px;
  height: 1080px;
  overflow: hidden;
  background: var(--surface);
  color: var(--text);
  font-family: system-ui, 'Segoe UI', 'Microsoft YaHei', sans-serif;
  padding: 86px 112px;
  display: flex;
  flex-direction: column;
  gap: 30px;
}
.ppt-slide::after {
  content: "";
  position: absolute;
  right: 0;
  bottom: 0;
  width: 520px;
  height: 16px;
  background: linear-gradient(90deg, var(--accent), var(--accent-2));
}
.ppt-cover {
  justify-content: center;
  background: linear-gradient(135deg, #f7fbfa 0%, #e8f2ef 58%, #fff7ed 100%);
  padding: 120px 150px;
}
.cover-meta {
  display: flex;
  align-items: center;
  gap: 18px;
  color: var(--accent);
  font-size: 28px;
  font-weight: 800;
}
.cover-subtitle {
  max-width: 1180px;
  color: var(--muted);
  font-size: 34px;
  line-height: 1.34;
  font-weight: 650;
}
.cover-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 18px;
  max-width: 1320px;
}
.cover-tag {
  background: rgba(255, 255, 255, .78);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 16px 24px;
  color: var(--heading);
  font-size: 28px;
  font-weight: 760;
}
.ppt-agenda { background: #fbfdfc; }
.section-number {
  width: fit-content;
  background: var(--accent-soft);
  color: var(--accent);
  border-radius: 999px;
  padding: 10px 24px;
  font-size: 26px;
  font-weight: 800;
}
.slide-title-wrap {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 42px;
}
.slide-title-wrap h2 { flex: 1; }
.slide-progress {
  position: absolute;
  left: 112px;
  right: 112px;
  bottom: 58px;
  height: 10px;
  border-radius: 999px;
  background: #e5e7eb;
  overflow: hidden;
}
.slide-progress span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, var(--accent), var(--accent-2));
}
h1 { max-width: 1360px; font-size: 76px; font-weight: 850; color: var(--heading); line-height: 1.12; }
h2 { max-width: 1480px; font-size: 52px; font-weight: 800; color: var(--heading); line-height: 1.18; }
.ppt-cover h2 { font-size: 34px; color: var(--accent); font-weight: 750; }
ul { list-style: none; padding-left: 0; }
li {
  display: flex;
  align-items: flex-start;
  gap: 18px;
  padding: 18px 0;
  color: var(--text);
  font-size: 32px;
  line-height: 1.36;
  border-bottom: 1px solid var(--border);
}
li::before { content: ""; width: 12px; height: 12px; margin-top: 16px; border-radius: 999px; background: var(--accent); flex-shrink: 0; }
li:last-child { border-bottom: none; }
.content-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.3fr) minmax(420px, .7fr);
  gap: 54px;
  align-items: start;
}
.main-points {
  background: var(--surface);
  border-top: 6px solid var(--accent);
  padding: 12px 0 0;
}
.insight-panel {
  min-height: 360px;
  background: linear-gradient(180deg, #ecfdf5 0%, #fff7ed 100%);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 34px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 22px;
}
.insight-token {
  color: var(--heading);
  font-size: 30px;
  line-height: 1.25;
  font-weight: 760;
}
.dir-list { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 28px; }
.dir-item {
  min-height: 118px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 28px 32px;
  color: var(--heading);
  font-weight: 750;
  font-size: 31px;
  line-height: 1.25;
}
.summary-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.05fr) minmax(0, .95fr);
  gap: 42px;
  align-items: stretch;
}
.summary-card,
.summary-actions {
  min-height: 460px;
  border-radius: 8px;
  padding: 38px 42px;
}
.summary-card {
  background: #f8fafc;
  border-left: 8px solid var(--accent);
}
.summary-actions {
  background: #fff7ed;
  border: 1px solid #fed7aa;
}
.summary-card h3,
.summary-actions h3 {
  color: var(--heading);
  font-size: 34px;
  line-height: 1.2;
  margin-bottom: 18px;
}
.summary-card p {
  color: var(--text);
  font-size: 32px;
  line-height: 1.34;
  font-weight: 700;
}
</style>`)
	for i, slide := range plan.Slides {
		className := "ppt-slide"
		if i == 0 {
			className += " ppt-cover"
		} else if i == 1 {
			className += " ppt-agenda"
		}
		b.WriteString(`<section class="`)
		b.WriteString(className)
		b.WriteString(`" data-ppt-slide="true">`)
		if i == 0 {
			writePPTCoverSlide(&b, plan, slide, i)
		} else {
			b.WriteString(`<div class="slide-title-wrap"><span class="section-number">`)
			b.WriteString(fmt.Sprintf("%02d", i+1))
			b.WriteString(`</span>`)
			b.WriteString("<h2>")
			b.WriteString(htmlEscape(slide.Title))
			b.WriteString("</h2>")
			b.WriteString(`</div>`)
		}
		if i == 1 && len(slide.Bullets) > 0 {
			b.WriteString(`<div class="dir-list">`)
			for _, bullet := range slide.Bullets {
				b.WriteString(`<div class="dir-item">`)
				b.WriteString(htmlEscape(bullet))
				b.WriteString(`</div>`)
			}
			b.WriteString(`</div>`)
		} else if i == len(plan.Slides)-1 && isEndingSlideTitle(slide.Title) {
			writePPTSummarySlideBody(&b, slide)
		} else if i > 0 && len(slide.Bullets) > 0 {
			b.WriteString(`<div class="content-grid"><ul class="main-points">`)
			for _, bullet := range slide.Bullets {
				b.WriteString("<li>")
				b.WriteString(htmlEscape(bullet))
				b.WriteString("</li>")
			}
			b.WriteString("</ul>")
			tokens := pptInsightTokens(slide)
			if len(tokens) > 0 && i > 0 {
				b.WriteString(`<div class="insight-panel">`)
				for _, token := range tokens {
					b.WriteString(`<div class="insight-token">`)
					b.WriteString(htmlEscape(token))
					b.WriteString(`</div>`)
				}
				b.WriteString(`</div>`)
			}
			b.WriteString("</div>")
		}
		b.WriteString(`<div class="slide-progress"><span style="width: `)
		b.WriteString(fmt.Sprintf("%d%%", pptSlideProgressPercent(i+1, len(plan.Slides))))
		b.WriteString(`"></span></div>`)
		b.WriteString("</section>\n")
	}
	return strings.TrimSpace(b.String())
}

func sanitizePPTPlanVisibleText(plan pptOutlinePlan) pptOutlinePlan {
	plan.Title = cleanPPTVisibleText(plan.Title)
	for i := range plan.Slides {
		plan.Slides[i].Title = cleanPPTVisibleText(plan.Slides[i].Title)
		plan.Slides[i].Purpose = cleanPPTVisibleText(plan.Slides[i].Purpose)
		for j := range plan.Slides[i].Bullets {
			plan.Slides[i].Bullets[j] = cleanPPTVisibleText(plan.Slides[i].Bullets[j])
		}
		normalizePPTBullets(&plan.Slides[i])
	}
	return plan
}

func cleanPPTVisibleText(value string) string {
	value = strings.ReplaceAll(value, "&nbsp;", " ")
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	for {
		trimmed := strings.TrimSpace(value)
		cleaned := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		if cleaned != trimmed {
			value = cleaned
			continue
		}
		cleaned = strings.TrimSpace(strings.TrimLeft(trimmed, "-*•"))
		if cleaned != trimmed {
			value = cleaned
			continue
		}
		return trimmed
	}
}

func writePPTCoverSlide(b *strings.Builder, plan pptOutlinePlan, slide pptSlidePlan, index int) {
	b.WriteString(`<div class="cover-meta"><span class="section-number">`)
	b.WriteString(fmt.Sprintf("%02d", index+1))
	b.WriteString(`</span><span>演示文稿</span></div>`)
	b.WriteString("<h1>")
	b.WriteString(htmlEscape(firstNonEmpty(plan.Title, slide.Title, "演示文稿")))
	b.WriteString("</h1>")
	b.WriteString(`<p class="cover-subtitle">基于资料自动生成的结构化演示</p>`)
	tags := uniqueNonEmpty(append([]string{}, slide.Bullets...))
	if len(tags) == 0 && strings.TrimSpace(slide.Title) != "" && !isCoverSlideTitle(slide.Title) {
		tags = append(tags, slide.Title)
	}
	if len(tags) > 4 {
		tags = tags[:4]
	}
	if len(tags) > 0 {
		b.WriteString(`<div class="cover-tags">`)
		for _, tag := range tags {
			b.WriteString(`<span class="cover-tag">`)
			b.WriteString(htmlEscape(tag))
			b.WriteString(`</span>`)
		}
		b.WriteString(`</div>`)
	}
}

func writePPTSummarySlideBody(b *strings.Builder, slide pptSlidePlan) {
	if len(slide.Bullets) == 0 {
		return
	}
	primary := slide.Bullets[0]
	actions := append([]string{}, slide.Bullets[1:]...)
	if len(actions) == 0 {
		actions = []string{primary}
	}
	b.WriteString(`<div class="summary-layout"><div class="summary-card"><h3>核心结论</h3><p>`)
	b.WriteString(htmlEscape(primary))
	b.WriteString(`</p></div><div class="summary-actions"><h3>后续行动</h3><ul>`)
	for _, action := range actions {
		b.WriteString("<li>")
		b.WriteString(htmlEscape(action))
		b.WriteString("</li>")
	}
	b.WriteString(`</ul></div></div>`)
}

func pptSlideProgressPercent(index, total int) int {
	if total <= 0 {
		return 100
	}
	if index < 1 {
		index = 1
	}
	if index > total {
		index = total
	}
	return int(float64(index)/float64(total)*100 + 0.5)
}

func pptInsightTokens(slide pptSlidePlan) []string {
	candidates := append([]string{}, slide.Bullets...)
	if len(candidates) == 0 {
		candidates = append(candidates, slide.Title)
	}
	tokens := make([]string, 0, 2)
	for _, candidate := range candidates {
		token := summarizeLine(candidate, 42)
		if token == "" {
			continue
		}
		tokens = append(tokens, token)
		if len(tokens) >= 2 {
			break
		}
	}
	return uniqueNonEmpty(tokens)
}

func pptNeedsStructureRepair(content string) bool {
	trimmed := strings.TrimSpace(content)
	lower := strings.ToLower(trimmed)
	sectionCount := strings.Count(lower, "<section")
	if sectionCount < 4 || strings.Contains(lower, "<section></section>") {
		return true
	}
	for _, required := range []string{"<style", "width: 1920px", "height: 1080px", "overflow: hidden"} {
		if !strings.Contains(lower, required) {
			return true
		}
	}
	if len([]rune(strings.TrimSpace(stripSimpleHTML(trimmed)))) < 20 {
		return true
	}
	return false
}

func pptContainsInternalPromptLeak(content string) bool {
	text := strings.ToLower(strings.Join(strings.Fields(stripSimpleHTML(content)), " "))
	if text == "" {
		return false
	}
	leakTokens := []string{
		"structured_ppt_plan",
		"ppt_html_generation_rules",
		"treat each slide entry",
		"the outline bullets are source material",
		"keep every planned slide title visible",
		"design every section as a real 16:9 ppt canvas",
		"finish all planned sections before returning",
		"internal slide outline",
		"generation constraints",
		"original markdown:",
		"local references:",
	}
	for _, token := range leakTokens {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

func pptCanPatchCanvas(content string) bool {
	lower := strings.ToLower(strings.TrimSpace(content))
	if strings.Count(lower, "<section") < 4 {
		return false
	}
	if strings.Contains(lower, "<style") && strings.Contains(lower, "width: 1920px") && strings.Contains(lower, "height: 1080px") {
		return false
	}
	if len([]rune(strings.TrimSpace(stripSimpleHTML(content)))) < 20 {
		return false
	}
	return true
}

func patchPPTCanvasHTML(content string) string {
	css := `<style>
.ppt-slide {
  position: relative;
  width: 1920px;
  height: 1080px;
  overflow: hidden;
  box-sizing: border-box;
  padding: 90px 116px;
  background: #ffffff;
  color: #111827;
  font-family: system-ui, 'Segoe UI', 'Microsoft YaHei', sans-serif;
}
.ppt-slide h1 { font-size: 78px; line-height: 1.12; font-weight: 850; margin-bottom: 36px; }
.ppt-slide h2 { font-size: 54px; line-height: 1.18; font-weight: 800; margin-bottom: 32px; }
.ppt-slide p, .ppt-slide li { font-size: 32px; line-height: 1.36; }
</style>`
	patched := strings.TrimSpace(content)
	if !strings.Contains(strings.ToLower(patched), "<style") {
		patched = css + "\n" + patched
	}
	return addPPTSlideClassToSections(patched)
}

func addPPTSlideClassToSections(content string) string {
	var b strings.Builder
	lower := strings.ToLower(content)
	pos := 0
	for {
		start := strings.Index(lower[pos:], "<section")
		if start < 0 {
			b.WriteString(content[pos:])
			break
		}
		start += pos
		end := strings.Index(content[start:], ">")
		if end < 0 {
			b.WriteString(content[pos:])
			break
		}
		end += start
		tag := content[start : end+1]
		b.WriteString(content[pos:start])
		if strings.Contains(strings.ToLower(tag), "class=") {
			b.WriteString(tag)
		} else {
			b.WriteString(strings.TrimSuffix(tag, ">"))
			b.WriteString(` class="ppt-slide" data-ppt-slide="true">`)
		}
		pos = end + 1
	}
	return b.String()
}

func pptContainsUnrelatedBoilerplate(content string, input generationAgentInput) bool {
	text := strings.ToLower(strings.Join(strings.Fields(stripSimpleHTML(content)), " "))
	if text == "" {
		return false
	}
	unrelatedTokens := []string{
		"market opportunity",
		"competitive advantage",
		"business model",
		"go-to-market",
		"revenue growth",
		"investor",
		"stakeholder alignment",
		"team introduction",
		"company overview",
	}
	source := pptAllowedSourceText(input)
	for _, token := range unrelatedTokens {
		if strings.Contains(text, token) && !strings.Contains(source, token) {
			return true
		}
	}
	return false
}

func pptAllowedSourceText(input generationAgentInput) string {
	var parts []string
	if input.Request != nil {
		parts = append(parts, input.Request.Markdown, input.Request.Prompt)
	}
	for _, ref := range input.References {
		parts = append(parts, ref.Content, ref.Heading, ref.ChapterPath, ref.SourceName)
	}
	for _, result := range input.SearchResults {
		parts = append(parts, result.Title, result.Snippet, result.Content)
	}
	return strings.ToLower(strings.Join(strings.Fields(strings.Join(parts, "\n")), " "))
}

func pptNeedsPlanCoverageRepair(content string, plan *pptOutlinePlan) bool {
	if plan == nil || len(plan.Slides) == 0 {
		return false
	}
	if strings.TrimSpace(plan.Title) != "" && !containsPlannedHeading(pptHTMLHeadings(content), strings.ToLower(strings.TrimSpace(plan.Title))) {
		return true
	}
	actual := pptSectionCount(content)
	expected := len(plan.Slides)
	if expected <= 4 {
		return actual < expected
	}
	return actual < expected || actual < 6 || pptMissingPlannedTitles(content, plan)
}

func pptSectionCount(content string) int {
	return strings.Count(strings.ToLower(content), "<section")
}

func pptMissingPlannedTitles(content string, plan *pptOutlinePlan) bool {
	headings := pptHTMLHeadings(content)
	required := 0
	missing := 0
	for _, slide := range plan.Slides {
		title := strings.ToLower(strings.TrimSpace(slide.Title))
		if title == "" || isGenericPPTPlanTitle(title) {
			continue
		}
		required++
		if !containsPlannedHeading(headings, title) {
			missing++
		}
	}
	if required == 0 {
		return false
	}
	return missing > 0
}

func containsPlannedHeading(headings []string, title string) bool {
	for _, heading := range headings {
		if strings.Contains(heading, title) || strings.Contains(title, heading) {
			return true
		}
	}
	return false
}

func pptHTMLHeadings(content string) []string {
	lower := strings.ToLower(content)
	var headings []string
	for _, tag := range []string{"h1", "h2", "h3"} {
		open := "<" + tag
		close := "</" + tag + ">"
		searchFrom := 0
		for {
			start := strings.Index(lower[searchFrom:], open)
			if start < 0 {
				break
			}
			start += searchFrom
			openEnd := strings.Index(lower[start:], ">")
			if openEnd < 0 {
				break
			}
			textStart := start + openEnd + 1
			end := strings.Index(lower[textStart:], close)
			if end < 0 {
				break
			}
			textEnd := textStart + end
			heading := strings.ToLower(strings.TrimSpace(stripSimpleHTML(content[textStart:textEnd])))
			if heading != "" {
				headings = append(headings, heading)
			}
			searchFrom = textEnd + len(close)
		}
	}
	return headings
}

func isGenericPPTPlanTitle(title string) bool {
	return containsAnyFold(title,
		"cover", "agenda", "closing", "finish", "end",
		"封面", "目录", "总结", "结束", "行动",
	)
}

func requiredMindmapBranchTitles() []string {
	return []string{"核心概念", "原理机制", "过程步骤", "应用场景", "易错点", "总结"}
}

func planMindmap(analysis learningContentAnalysis) mindmapPlan {
	plan := mindmapPlan{Title: analysis.Topic}
	for _, title := range requiredMindmapBranchTitles() {
		branch := mindmapBranchPlan{Title: title}
		switch title {
		case "核心概念":
			branch.Nodes = appendMindmapNodes(branch.Nodes, title, analysis.KeyConcepts, analysis)
		case "原理机制", "过程步骤":
			branch.Nodes = appendMindmapNodes(branch.Nodes, title, analysis.Processes, analysis)
		case "应用场景":
			branch.Nodes = appendMindmapNodes(branch.Nodes, title, analysis.Examples, analysis)
		case "易错点":
			branch.Nodes = append(branch.Nodes, newMindmapNode("注意概念边界、条件范围和常见混淆。", "用于复习时主动辨析相近概念、适用条件和结论边界。"))
		case "总结":
			branch.Nodes = append(branch.Nodes, newMindmapNode(fmt.Sprintf("围绕“%s”形成可复习的结构。", analysis.Topic), "按概念、机制、过程、应用和误区回顾学习路径。"))
		}
		if len(branch.Nodes) == 0 {
			branch.Nodes = append(branch.Nodes, newMindmapNode(supplementBullet(title, "根据学习结构补充必要节点。"), "该节点为解释补充，用于补足学习结构。"))
		}
		if analysis.Sparse && !hasSupplementMindmapNode(branch.Nodes) {
			branch.Nodes = append(branch.Nodes, newMindmapNode(supplementBullet(title, "原始材料较少，此处为解释性学习补充。"), "该节点不是来源事实，主要用于提示复习方向。"))
		}
		minNodes := 3
		if analysis.Sparse {
			minNodes = 4
		}
		for len(branch.Nodes) < minNodes {
			branch.Nodes = append(branch.Nodes, newMindmapNode(
				supplementBullet(title, fmt.Sprintf("补足层级展开-%d。", len(branch.Nodes)+1)),
				"该节点为解释补充，用于形成可展开的导图层级。",
				mindmapBranchExpansionDetail(title, analysis),
			))
		}
		branch.Nodes = uniqueMindmapNodes(branch.Nodes)
		plan.Branches = append(plan.Branches, branch)
	}
	return plan
}

func expandMindmapContent(plan mindmapPlan, analysis learningContentAnalysis) mindmapPlan {
	expanded := plan
	evidenceIndex := 0
	minNodes := 4
	if analysis.Sparse {
		minNodes = 5
	}

	for i := range expanded.Branches {
		branch := &expanded.Branches[i]
		for j := range branch.Nodes {
			branch.Nodes[j].Details = expandMindmapNodeDetails(branch.Title, branch.Nodes[j], analysis, nextMindmapEvidence(analysis.Evidence, &evidenceIndex))
		}
		for len(branch.Nodes) < minNodes {
			title := mindmapExpansionNodeTitle(branch.Title, len(branch.Nodes)+1)
			branch.Nodes = append(branch.Nodes, mindmapNodePlan{
				Title:   title,
				Details: expandMindmapNodeDetails(branch.Title, mindmapNodePlan{Title: title}, analysis, nextMindmapEvidence(analysis.Evidence, &evidenceIndex)),
			})
			branch.Nodes = uniqueMindmapNodes(branch.Nodes)
		}
		for j := range branch.Nodes {
			branch.Nodes[j].Details = expandMindmapNodeDetails(branch.Title, branch.Nodes[j], analysis, nextMindmapEvidence(analysis.Evidence, &evidenceIndex))
		}
		branch.Nodes = uniqueMindmapNodes(branch.Nodes)
	}
	return expanded
}

func appendMindmapNodes(nodes []mindmapNodePlan, branchTitle string, values []string, analysis learningContentAnalysis) []mindmapNodePlan {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		nodes = append(nodes, newMindmapNode(value, mindmapNodeDetail(branchTitle, value, analysis)))
	}
	return nodes
}

func newMindmapNode(title string, details ...string) mindmapNodePlan {
	node := mindmapNodePlan{Title: strings.TrimSpace(title)}
	for _, detail := range details {
		detail = strings.TrimSpace(detail)
		if detail != "" {
			node.Details = append(node.Details, detail)
		}
	}
	return node
}

func expandMindmapNodeDetails(branchTitle string, node mindmapNodePlan, analysis learningContentAnalysis, evidence string) []string {
	details := uniqueNonEmpty(node.Details)
	if len(details) == 0 {
		details = append(details, mindmapNodeDetail(branchTitle, node.Title, analysis))
	}
	if len(details) < 2 {
		details = append(details, mindmapNodeReviewDetail(branchTitle, node.Title))
	}
	if len(details) < 3 && evidence != "" {
		details = append(details, "资料要点："+summarizeLine(evidence, 90))
	}
	if len(details) < 3 {
		details = append(details, mindmapBranchExpansionDetail(branchTitle, analysis))
	}
	details = uniqueNonEmpty(details)
	if len(details) > 3 {
		details = append([]string{}, details[:3]...)
	}
	for len(details) < 2 {
		details = append(details, fmt.Sprintf("围绕“%s”继续补足复习说明。", strings.TrimSpace(node.Title)))
		details = uniqueNonEmpty(details)
	}
	return details
}

func mindmapNodeReviewDetail(branchTitle, nodeTitle string) string {
	switch branchTitle {
	case "核心概念":
		return "继续说明该概念的定义边界、关联概念和典型辨析。"
	case "原理机制":
		return "继续说明触发条件、关键变量和因果链条如何变化。"
	case "过程步骤":
		return "继续说明前置条件、执行顺序和每一步产出。"
	case "应用场景":
		return "继续说明适用场景、迁移方式和判断依据。"
	case "易错点":
		return "继续说明常见误解、错误推断和修正线索。"
	case "总结":
		return "继续串联概念、机制、过程和应用结论。"
	default:
		return fmt.Sprintf("继续围绕“%s”补足复习说明和展开方向。", strings.TrimSpace(nodeTitle))
	}
}

func nextMindmapEvidence(evidence []learningEvidence, index *int) string {
	if len(evidence) == 0 {
		return ""
	}
	if index == nil {
		return summarizeLine(strings.TrimSpace(evidence[0].Text), 90)
	}
	ev := evidence[*index%len(evidence)]
	*index++
	return summarizeLine(strings.TrimSpace(ev.Text), 90)
}

func mindmapNodeDetail(branchTitle, value string, analysis learningContentAnalysis) string {
	if strings.Contains(value, "解释补充") {
		return "该节点为解释补充，用于补足学习结构。"
	}
	switch branchTitle {
	case "核心概念":
		return "先明确含义，再和相关概念建立联系。"
	case "原理机制":
		return "关注该机制成立的条件、因果关系和边界。"
	case "过程步骤":
		return "按先后顺序理解输入、变化和结果。"
	case "应用场景":
		return "结合具体情境判断该知识点如何迁移使用。"
	default:
		if len(analysis.Evidence) > 0 {
			return fmt.Sprintf("可参考：%s", analysis.Evidence[0].Source)
		}
		return "用于复习时展开说明和自我检查。"
	}
}

func mindmapBranchExpansionDetail(branchTitle string, analysis learningContentAnalysis) string {
	switch branchTitle {
	case "核心概念":
		return "补充概念之间的联系、边界和典型辨析。"
	case "原理机制":
		return "补充触发条件、因果链条和关键变量。"
	case "过程步骤":
		return "补充前后顺序、输入输出和阶段性结果。"
	case "应用场景":
		if len(analysis.Examples) > 0 {
			return "结合已有例子扩展到相近场景和迁移使用。"
		}
		return "补充典型应用场景、判断方式和迁移思路。"
	case "易错点":
		return "补充常见混淆、错误推断和修正线索。"
	case "总结":
		return "补充复习路径、串联方式和回顾问题。"
	default:
		return "补充该分支下仍然缺失的学习展开。"
	}
}

func mindmapExpansionNodeTitle(branchTitle string, position int) string {
	return supplementBullet(branchTitle, fmt.Sprintf("补足层级展开-%d。", position))
}

func uniqueMindmapNodes(nodes []mindmapNodePlan) []mindmapNodePlan {
	seen := map[string]struct{}{}
	result := make([]mindmapNodePlan, 0, len(nodes))
	for _, node := range nodes {
		node.Title = strings.TrimSpace(node.Title)
		if node.Title == "" {
			continue
		}
		if _, ok := seen[node.Title]; ok {
			continue
		}
		seen[node.Title] = struct{}{}
		node.Details = uniqueNonEmpty(node.Details)
		result = append(result, node)
	}
	return result
}

func renderMindmap(plan mindmapPlan) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(plan.Title)
	b.WriteString("\n")
	for _, branch := range plan.Branches {
		b.WriteString("## ")
		b.WriteString(branch.Title)
		b.WriteString("\n")
		for _, node := range branch.Nodes {
			b.WriteString("### ")
			b.WriteString(node.Title)
			b.WriteString("\n")
			for _, detail := range node.Details {
				b.WriteString("#### ")
				b.WriteString(detail)
				b.WriteString("\n")
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func mindmapNeedsStructureRepair(content string) bool {
	trimmed := strings.TrimSpace(content)
	if len([]rune(strings.ReplaceAll(trimmed, "#", ""))) < 20 || strings.Count(trimmed, "\n## ") < len(requiredMindmapBranchTitles()) || !strings.Contains(trimmed, "\n### ") {
		return true
	}
	for _, title := range requiredMindmapBranchTitles() {
		if !strings.Contains(trimmed, "## "+title) {
			return true
		}
	}
	return false
}

func supplementBullet(title, detail string) string {
	return fmt.Sprintf("解释补充：%s - %s", title, detail)
}

func hasSupplementBullet(values []string) bool {
	for _, value := range values {
		if strings.Contains(value, "解释补充") {
			return true
		}
	}
	return false
}

func hasSupplementMindmapNode(nodes []mindmapNodePlan) bool {
	for _, node := range nodes {
		if strings.Contains(node.Title, "解释补充") {
			return true
		}
	}
	return false
}

func pickPoint(points []string, index int) string {
	if len(points) == 0 {
		return ""
	}
	if index < len(points) {
		return points[index]
	}
	return points[index%len(points)]
}

func buildPPTFallbackPoints(input generationAgentInput, limit int) []string {
	if limit <= 0 {
		limit = 9
	}
	markdown := ""
	prompt := ""
	if input.Request != nil {
		markdown = input.Request.Markdown
		prompt = input.Request.Prompt
	}
	title := extractTitle(markdown, "演示文稿")
	candidates := extractKeyPoints(markdown, limit)
	if prompt := strings.TrimSpace(prompt); prompt != "" {
		candidates = append(candidates, prompt)
	}
	for _, ref := range input.References {
		candidates = append(candidates, summarizeLine(ref.Content, 90))
	}
	for _, result := range input.SearchResults {
		candidates = append(candidates, summarizeLine(firstNonEmpty(result.Snippet, result.Content), 90))
	}
	candidates = append(candidates,
		fmt.Sprintf("围绕“%s”说明背景、问题和目标。", title),
		"提炼现有材料中的核心观点，并补充必要解释。",
		"使用来源材料、示例或数据支撑关键结论。",
		"将内容组织成适合演讲的开场、展开和收束。",
		"给出听众可以理解或执行的总结。",
	)
	points := uniqueNonEmpty(candidates)
	if len(points) > limit {
		return append([]string{}, points[:limit]...)
	}
	return points
}

func newQuizAgent(model GenerationModel) generationAgent {
	return &baseGenerationAgent{
		name:      "quiz",
		typ:       GenerationTypeQuiz,
		model:     model,
		validator: validateQuizContent,
		fallback: func(input generationAgentInput) string {
			points := extractKeyPoints(input.Request.Markdown, 3)
			if len(points) == 0 {
				points = []string{"来源笔记"}
			}
			var items []string
			for _, point := range points {
				items = append(items, fmt.Sprintf(`{"type":"short_answer","question":"%s 的核心观点是什么？","options":[],"answer":"%s","explanation":"该答案来自提供的笔记上下文。"}`, jsonEscape(point), jsonEscape(point)))
			}
			return `{"questions":[` + strings.Join(items, ",") + `]}`
		},
	}
}

func newNoteAgent(model GenerationModel) generationAgent {
	return &baseGenerationAgent{
		name:      "note",
		typ:       GenerationTypeNote,
		model:     model,
		validator: validateNoteContent,
		fallback: func(input generationAgentInput) string {
			title := extractTitle(input.Request.Markdown, "生成笔记")
			points := extractKeyPoints(input.Request.Markdown, 8)
			var b strings.Builder
			b.WriteString("# ")
			b.WriteString(title)
			b.WriteString("\n\n## 摘要\n")
			if len(points) > 0 {
				b.WriteString(points[0])
			} else {
				b.WriteString("未能从笔记中提取出简明摘要。")
			}
			b.WriteString("\n\n## 关键点\n")
			for _, point := range points {
				b.WriteString("- ")
				b.WriteString(point)
				b.WriteString("\n")
			}
			return strings.TrimSpace(b.String())
		},
	}
}

func extractTitle(markdown, fallback string) string {
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			title := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if title != "" {
				return title
			}
		}
	}
	return fallback
}

func extractKeyPoints(markdown string, limit int) []string {
	var points []string
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "-*#0123456789. "))
		if len([]rune(line)) < 4 {
			continue
		}
		points = append(points, line)
		if len(points) >= limit {
			return points
		}
	}
	return points
}

func appendReferenceSection(b *strings.Builder, refs []GenerationReference) {
	if len(refs) == 0 {
		return
	}
	b.WriteString("\n\n## 参考资料\n")
	for i, ref := range refs {
		label := generationReferenceLabel(ref)
		b.WriteString(fmt.Sprintf("- [%d] %s: %s\n", i+1, label, summarizeLine(ref.Content, 120)))
	}
}

func summarizeLine(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}

func htmlEscape(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return replacer.Replace(value)
}

func jsonEscape(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", "")
	return replacer.Replace(value)
}
