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
	Gaps        []string
	UserIntent  string
	Sparse      bool
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
	outline, err := a.generateOutline(ctx, state.input)
	if err != nil {
		return pptChainState{}, err
	}
	state.outline = outline
	state.outlinePlan = planPPTOutline(state.analysis)
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
			Context:      appendPPTOutlineToContext(state.input.Context, state.outline),
			OutputFormat: strategy.OutputFormat,
		})
		if err != nil {
			return generationDraft{}, err
		}
		content = strings.TrimSpace(generated)
	}
	if content == "" {
		content = renderPPTSlides(state.expanded)
		fallbackUsed = true
	}
	repairPlan := state.expanded
	return generationDraft{input: state.input, content: content, fallbackUsed: fallbackUsed, pptRepairPlan: &repairPlan}, nil
}

func (a *pptGenerationAgent) repairPPTStructure(ctx context.Context, draft generationDraft) (generationDraft, error) {
	if pptNeedsStructureRepair(draft.content) {
		if draft.pptRepairPlan != nil {
			draft.content = renderPPTSlides(*draft.pptRepairPlan)
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
	return renderPPTSlides(expandPPTContent(planPPTOutline(analysis), analysis))
}
func fallbackPPTOutline(input generationAgentInput) string {
	title := extractTitle(input.Request.Markdown, "演示文稿")
	points := buildPPTFallbackPoints(input, 6)
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(title)
	b.WriteString("\n")
	for _, item := range learningDeckSections() {
		b.WriteString("- ")
		b.WriteString(item)
		b.WriteString("\n")
	}
	for _, point := range points {
		b.WriteString("  - ")
		b.WriteString(point)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func appendPPTOutlineToContext(contextValue, outline string) string {
	outline = strings.TrimSpace(outline)
	if outline == "" {
		return contextValue
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(contextValue))
	b.WriteString("\n\n内部演示大纲：\n")
	b.WriteString(outline)
	return b.String()
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
	points := extractKeyPoints(input.Request.Markdown, 12)
	analysis := learningContentAnalysis{
		Topic:       extractTitle(input.Request.Markdown, "学习资料"),
		KeyConcepts: points,
		UserIntent:  strings.TrimSpace(input.Request.Prompt),
		Evidence:    append(evidenceFromReferences(input.References), evidenceFromSearch(input.SearchResults)...),
		Sparse:      len(points) < 3,
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

func requiredPPTSlideTitles() []string {
	return []string{"封面", "目录", "背景与目标", "概念框架", "机制与流程", "案例与应用", "易错辨析", "总结复盘"}
}

func planPPTOutline(analysis learningContentAnalysis) pptOutlinePlan {
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
			for len(slide.Bullets) < 3 {
				slide.Bullets = append(slide.Bullets, supplementBullet(slide.Title, "围绕本页主题补充学习解释。"))
			}
		}
		if analysis.Sparse && slide.Title != "封面" && slide.Title != "目录" && !hasSupplementBullet(slide.Bullets) {
			slide.Bullets = append(slide.Bullets, supplementBullet(slide.Title, "原始材料较少，本页内容为解释性学习补充。"))
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
	for len(slide.Bullets) < 3 {
		slide.Bullets = append(slide.Bullets, supplementBullet(slide.Title, "围绕本页主题补充学习解释。"))
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

func pptNeedsStructureRepair(content string) bool {
	trimmed := strings.TrimSpace(content)
	lower := strings.ToLower(trimmed)
	if strings.Count(lower, "<section") < len(requiredPPTSlideTitles()) || strings.Contains(lower, "<section></section>") {
		return true
	}
	if len([]rune(strings.TrimSpace(stripSimpleHTML(trimmed)))) < 20 {
		return true
	}
	for _, title := range requiredPPTSlideTitles() {
		if !strings.Contains(trimmed, title) {
			return true
		}
	}
	return false
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
	title := extractTitle(input.Request.Markdown, "演示文稿")
	candidates := extractKeyPoints(input.Request.Markdown, limit)
	if prompt := strings.TrimSpace(input.Request.Prompt); prompt != "" {
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
