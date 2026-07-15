package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"YoudaoNoteLm/pkg/logger"

	"github.com/cloudwego/eino/compose"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
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

// enrichedPPTSlide represents a slide with fully expanded prose content.
// Unlike pptSlidePlan which has short bullet points, enrichedPPTSlide
// contains detailed paragraphs suitable for actual presentation slides.
type enrichedPPTSlide struct {
	Title      string   `json:"title"`
	Subtitle   string   `json:"subtitle,omitempty"`
	Paragraphs []string `json:"paragraphs"`
	Bullets    []string `json:"bullets,omitempty"`
	Insights   []string `json:"insights,omitempty"`
}

// pptRichContent holds the enriched content for all slides.
type pptRichContent struct {
	Slides []enrichedPPTSlide `json:"slides"`
}

// pptStyleTheme captures a coherent visual design direction for the deck.
type pptStyleTheme struct {
	Name        string // e.g. "简约商务", "学术清新", "科技深色"
	Primary     string // primary color hex
	Secondary   string // secondary/accent color hex
	Background  string // page background
	Surface     string // card/surface background
	Text        string // body text color
	Heading     string // heading color
	Muted       string // muted text color
	FontHeading string // heading font-family
	FontBody    string // body font-family
	LayoutHints string // free-form layout guidance for the LLM
}

const pptContentEnrichBatchSize = 4

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

type noteOutlinePlan struct {
	Title    string
	Summary  string
	Sections []noteSectionPlan
}

type noteSectionPlan struct {
	Title   string
	Purpose string
	Points  []string
}

type quizChainState struct {
	input    generationAgentInput
	analysis learningContentAnalysis
	plan     quizQuestionPlan
	expanded quizQuestionPlan
}

type quizQuestionPlan struct {
	Topic     string
	Questions []quizQuestionItem
}

type quizQuestionItem struct {
	Type        string
	Topic       string
	Question    string
	Options     []string
	Answer      string
	Explanation string
	Difficulty  string // easy | medium | hard
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
	overallStart := time.Now()
	logger.Info("[PPT] generation started",
		zap.Int("prompt_len", len(input.Request.Prompt)),
		zap.Int("context_len", len(input.Context)),
	)

	var state pptChainState
	var err error
	var stepStart time.Time

	// Step 1: analyzePPTContent
	stepStart = time.Now()
	state, err = a.analyzePPTContent(ctx, input)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 1/14: analyzePPTContent done",
		zap.Duration("elapsed", time.Since(stepStart)),
		zap.Int("sections", len(state.analysis.Sections)),
	)

	// Step 2: planPPTChainOutline
	stepStart = time.Now()
	state, err = a.planPPTChainOutline(ctx, state)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 2/14: planPPTChainOutline done",
		zap.Duration("elapsed", time.Since(stepStart)),
		zap.Int("slides", len(state.outlinePlan.Slides)),
	)

	// Step 3: reviewPPTOutline
	stepStart = time.Now()
	state, err = a.reviewPPTOutline(ctx, state)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 3/14: reviewPPTOutline done",
		zap.Duration("elapsed", time.Since(stepStart)),
		zap.Int("slides_after_review", len(state.outlinePlan.Slides)),
	)

	// Step 4: expandPPTChainContent
	stepStart = time.Now()
	state, err = a.expandPPTChainContent(ctx, state)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 4/14: expandPPTChainContent done",
		zap.Duration("elapsed", time.Since(stepStart)),
	)

	// Step 5: enrichPPTContent
	stepStart = time.Now()
	state, err = a.enrichPPTContent(ctx, state)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 5/14: enrichPPTContent done",
		zap.Duration("elapsed", time.Since(stepStart)),
		zap.Int("rich_slides", len(state.richContent.Slides)),
	)

	// Step 6: designPPTStyle
	stepStart = time.Now()
	state, err = a.designPPTStyle(ctx, state)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 6/14: designPPTStyle done",
		zap.Duration("elapsed", time.Since(stepStart)),
		zap.String("theme", state.styleTheme.Name),
	)

	// Step 7: generatePPTCSS
	stepStart = time.Now()
	state, err = a.generatePPTCSS(ctx, state)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 7/14: generatePPTCSS done",
		zap.Duration("elapsed", time.Since(stepStart)),
		zap.Int("css_len", len(state.cssBlock)),
	)

	// Step 8: generatePPTHTML
	stepStart = time.Now()
	draft, err := a.generatePPTHTML(ctx, state)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 8/14: generatePPTHTML done",
		zap.Duration("elapsed", time.Since(stepStart)),
		zap.Int("html_len", len(draft.content)),
		zap.Bool("fallback_used", draft.fallbackUsed),
	)

	// Step 9: structureCheck
	stepStart = time.Now()
	draft, err = a.structureCheck(ctx, draft)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 9/14: structureCheck done",
		zap.Duration("elapsed", time.Since(stepStart)),
	)

	// Step 10: polishPPTHTML
	stepStart = time.Now()
	draft, err = a.polishPPTHTML(ctx, draft)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 10/14: polishPPTHTML done",
		zap.Duration("elapsed", time.Since(stepStart)),
		zap.Int("html_len_after", len(draft.content)),
	)

	// Step 11: repairPPTStructure
	stepStart = time.Now()
	draft, err = a.repairPPTStructure(ctx, draft)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 11/14: repairPPTStructure done",
		zap.Duration("elapsed", time.Since(stepStart)),
		zap.Bool("fallback_used", draft.fallbackUsed),
	)

	// Step 12: factEnhance
	stepStart = time.Now()
	draft, err = a.factEnhance(ctx, draft)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 12/14: factEnhance done",
		zap.Duration("elapsed", time.Since(stepStart)),
	)

	// Step 13: formatValidate
	stepStart = time.Now()
	draft, err = a.formatValidate(ctx, draft)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 13/14: formatValidate done",
		zap.Duration("elapsed", time.Since(stepStart)),
		zap.Bool("format_valid", draft.formatValid),
	)

	// Step 14: finalize
	stepStart = time.Now()
	output, err := a.finalize(ctx, draft)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 14/14: finalize done",
		zap.Duration("elapsed", time.Since(stepStart)),
		zap.Int("final_len", len(output.Content)),
	)

	logger.Info("[PPT] generation completed",
		zap.Duration("total_elapsed", time.Since(overallStart)),
		zap.Bool("fallback_used", output.FallbackUsed),
	)

	return output, nil
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

type noteGenerationAgent struct {
	baseGenerationAgent
}

func (a *noteGenerationAgent) Generate(ctx context.Context, input generationAgentInput) (generationAgentOutput, error) {
	chain := compose.NewChain[generationAgentInput, generationAgentOutput]().
		AppendLambda(compose.InvokableLambda(a.analyzeNoteContent)).
		AppendLambda(compose.InvokableLambda(a.planNoteOutline)).
		AppendLambda(compose.InvokableLambda(a.expandNoteChainContent)).
		AppendLambda(compose.InvokableLambda(a.generateNoteDraft)).
		AppendLambda(compose.InvokableLambda(a.structureCheck)).
		AppendLambda(compose.InvokableLambda(a.repairNoteStructure)).
		AppendLambda(compose.InvokableLambda(a.factEnhance)).
		AppendLambda(compose.InvokableLambda(a.formatValidate)).
		AppendLambda(compose.InvokableLambda(a.finalize))

	runner, err := chain.Compile(ctx)
	if err != nil {
		return generationAgentOutput{}, err
	}
	return runner.Invoke(ctx, input)
}

type quizGenerationAgent struct {
	baseGenerationAgent
}

func (a *quizGenerationAgent) Generate(ctx context.Context, input generationAgentInput) (generationAgentOutput, error) {
	chain := compose.NewChain[generationAgentInput, generationAgentOutput]().
		AppendLambda(compose.InvokableLambda(a.analyzeQuizContent)).
		AppendLambda(compose.InvokableLambda(a.planQuizQuestions)).
		AppendLambda(compose.InvokableLambda(a.expandQuizChainContent)).
		AppendLambda(compose.InvokableLambda(a.generateQuizDraft)).
		AppendLambda(compose.InvokableLambda(a.structureCheck)).
		AppendLambda(compose.InvokableLambda(a.repairQuizStructure)).
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
	if parsed, ok := parsePPTOutlineMarkdown(outline); ok {
		state.outlinePlan = parsed
	} else {
		state.outlinePlan = planPPTOutline(state.analysis)
	}
	return state, nil
}

// reviewPPTOutline calls the LLM to review the generated outline against the
// source material and fix structural issues: missing topics, redundant slides,
// question-style titles, planning label leakage, and sparse content.
// If the review fails or returns an unparseable result, the original outline
// is kept.
func (a *pptGenerationAgent) reviewPPTOutline(ctx context.Context, state pptChainState) (pptChainState, error) {
	if a.model == nil {
		return state, nil
	}
	llmStart := time.Now()
	strategy := pptOutlineReviewPromptStrategy()
	reviewed, err := a.model.Generate(ctx, GenerationPrompt{
		AgentName:    a.name + "_outline_review",
		System:       strategy.System,
		User:         strings.TrimSpace(state.input.Request.Prompt),
		Context:      appendPPTOutlineToContext(state.input.Context, state.outline),
		OutputFormat: strategy.OutputFormat,
	})
	logger.Info("[PPT] LLM call: reviewPPTOutline done",
		zap.Duration("llm_elapsed", time.Since(llmStart)),
		zap.Int("reviewed_len", len(reviewed)),
		zap.Error(err),
	)
	if err != nil || strings.TrimSpace(reviewed) == "" {
		return state, nil
	}
	reviewed = strings.TrimSpace(reviewed)
	if parsed, ok := parsePPTOutlineMarkdown(reviewed); ok && len(parsed.Slides) > 0 {
		state.outline = reviewed
		state.outlinePlan = parsed
	}
	return state, nil
}

func (a *pptGenerationAgent) expandPPTChainContent(ctx context.Context, state pptChainState) (pptChainState, error) {
	state.expanded = expandPPTContent(state.outlinePlan, state.analysis)
	return state, nil
}

func (a *pptGenerationAgent) designPPTStyle(ctx context.Context, state pptChainState) (pptChainState, error) {
	styleHint := ""
	if state.input.Request != nil {
		styleHint = optionString(state.input.Request.Options, "ppt_style", "")
		if styleHint == "" {
			styleHint = optionString(state.input.Request.Options, "pptStyle", "")
		}
		if styleHint == "" {
			styleHint = optionString(state.input.Request.Options, "style", "")
		}
	}
	state.styleTheme = designPPTStyleTheme(state.analysis, state.expanded, state.input.Request.Prompt, styleHint)
	return state, nil
}

// generatePPTCSS is the first LLM call: it focuses solely on producing a
// high-quality <style> block based on the style theme and slide plan.
// Splitting CSS design from HTML structure reduces the cognitive load on
// the model and yields better visual design.
func (a *pptGenerationAgent) generatePPTCSS(ctx context.Context, state pptChainState) (pptChainState, error) {
	if a.model == nil {
		state.cssBlock = fallbackPPTCSS(state.styleTheme)
		return state, nil
	}
	llmStart := time.Now()
	strategy := pptCSSPromptStrategy()
	generated, err := a.model.Generate(ctx, GenerationPrompt{
		AgentName:    a.name + "_css",
		System:       strategy.System,
		User:         strings.TrimSpace(state.input.Request.Prompt),
		Context:      appendPPTStyleToContext(appendPPTPlansToContext(state.input.Context, state.outline, state.expanded), state.styleTheme),
		OutputFormat: strategy.OutputFormat,
	})
	logger.Info("[PPT] LLM call: generatePPTCSS done",
		zap.Duration("llm_elapsed", time.Since(llmStart)),
		zap.Int("css_len", len(generated)),
		zap.Error(err),
	)
	if err != nil || strings.TrimSpace(generated) == "" {
		state.cssBlock = fallbackPPTCSS(state.styleTheme)
		return state, nil
	}
	state.cssBlock = extractCSSBlock(strings.TrimSpace(generated))
	if !pptCSSHasCanvasSize(state.cssBlock) {
		state.cssBlock = injectPPTCanvasSizeIntoCSS(state.cssBlock)
	}
	return state, nil
}

// generatePPTHTML is the second LLM call: it focuses on HTML structure and
// content, reusing the CSS block produced by generatePPTCSS. The model no
// longer needs to design CSS, so it can concentrate on slide structure,
// layout variety, and content quality.
func (a *pptGenerationAgent) generatePPTHTML(ctx context.Context, state pptChainState) (generationDraft, error) {
	content := ""
	fallbackUsed := false
	if a.model != nil {
		llmStart := time.Now()
		strategy := promptStrategyFor(GenerationTypePPT)
		// Build context with enriched content if available
		contextValue := appendPPTCSSToContext(appendPPTStyleToContext(appendPPTPlansToContext(state.input.Context, state.outline, state.expanded), state.styleTheme), state.cssBlock)
		if len(state.richContent.Slides) > 0 {
			contextValue = appendPPTRichContentToContext(contextValue, state.richContent)
		}
		generated, err := a.model.Generate(ctx, GenerationPrompt{
			AgentName:    a.name,
			System:       strategy.System,
			User:         strings.TrimSpace(state.input.Request.Prompt),
			Context:      contextValue,
			OutputFormat: strategy.OutputFormat,
		})
		logger.Info("[PPT] LLM call: generatePPTHTML done",
			zap.Duration("llm_elapsed", time.Since(llmStart)),
			zap.Int("html_len", len(generated)),
			zap.Error(err),
		)
		if err != nil {
			return generationDraft{}, err
		}
		content = strings.TrimSpace(generated)
	}
	if content == "" {
		content = renderStyledPPTSlides(state.expanded, state.styleTheme)
		fallbackUsed = true
	}
	if state.cssBlock != "" && !strings.Contains(strings.ToLower(content), "<style") {
		content = state.cssBlock + "\n" + content
	}
	repairPlan := state.expanded
	return generationDraft{input: state.input, content: content, fallbackUsed: fallbackUsed, pptRepairPlan: &repairPlan, pptStyleTheme: state.styleTheme}, nil
}

// polishPPTHTML attempts to fix minor HTML quality issues without discarding
// the LLM's creative output. Only issues that can be safely patched are handled
// here; fundamental structural problems are left to repairPPTStructure.
func (a *pptGenerationAgent) polishPPTHTML(ctx context.Context, draft generationDraft) (generationDraft, error) {
	content := draft.content
	if strings.TrimSpace(content) == "" {
		return draft, nil
	}
	content = stripPPTExportPlaceholders(content)
	content = sanitizePPTReferenceSections(content)
	content = stripPPTPlanningArtifacts(content)
	content = stripPPTReferenceMetadata(content)
	content = stripPPTHTMLRepeatedTitlePrefix(content)
	content = deduplicatePPTCardTitles(content)
	content = ensurePPTSlideAttributes(content)
	content = ensurePPTCanvasSize(content)
	content = ensurePPTStyleBlock(content)
	draft.content = content
	return draft, nil
}

func (a *pptGenerationAgent) repairPPTStructure(ctx context.Context, draft generationDraft) (generationDraft, error) {
	draft.content = stripPPTExportPlaceholders(draft.content)
	draft.content = sanitizePPTReferenceSections(draft.content)
	draft.content = stripPPTPlanningArtifacts(draft.content)
	draft.content = stripPPTReferenceMetadata(draft.content)
	if pptCanPatchCanvas(draft.content) {
		draft.content = patchPPTCanvasHTML(draft.content)
	}

	// 逐个检查触发条件，记录具体是哪个条件触发了 fallback
	var repairReasons []string
	if pptNeedsStructureRepair(draft.content) {
		repairReasons = append(repairReasons, "structure_repair")
	}
	if pptContainsInternalPromptLeak(draft.content) {
		repairReasons = append(repairReasons, "internal_prompt_leak")
	}
	if pptContainsVisiblePlaceholderText(draft.content) {
		repairReasons = append(repairReasons, "visible_placeholder_text")
	}
	if pptContainsUnrelatedBoilerplate(draft.content, draft.input) {
		repairReasons = append(repairReasons, "unrelated_boilerplate")
	}
	if pptContainsRepetitiveText(draft.content) {
		repairReasons = append(repairReasons, "repetitive_text")
	}
	if pptContainsExcessivePromptWords(draft.content) {
		repairReasons = append(repairReasons, "excessive_prompt_words")
	}
	if pptHasSparseSlides(draft.content) {
		repairReasons = append(repairReasons, "sparse_slides")
	}
	if pptHasDuplicatedSlideTitles(draft.content) {
		repairReasons = append(repairReasons, "duplicated_slide_titles")
	}
	if pptHasMismatchedCardContent(draft.content) {
		repairReasons = append(repairReasons, "mismatched_card_content")
	}
	if pptNeedsHTMLQualityRepair(draft.content) {
		repairReasons = append(repairReasons, "html_quality_repair")
	}
	if pptNeedsPlanCoverageRepair(draft.content, draft.pptRepairPlan) {
		repairReasons = append(repairReasons, "plan_coverage_repair")
	}

	if len(repairReasons) > 0 {
		logger.Warn("[PPT] repairPPTStructure triggered, replacing LLM output with fallback",
			zap.Strings("reasons", repairReasons),
			zap.Int("content_len_before", len(draft.content)),
		)
		if draft.pptRepairPlan != nil {
			draft.content = renderStyledPPTSlides(*draft.pptRepairPlan, draft.pptStyleTheme)
		} else {
			draft.content = a.fallback(draft.input)
		}
		draft.fallbackUsed = true
		logger.Warn("[PPT] repairPPTStructure fallback applied",
			zap.Int("content_len_after", len(draft.content)),
			zap.Bool("used_renderStyledPPTSlides", draft.pptRepairPlan != nil),
		)
	} else {
		logger.Info("[PPT] repairPPTStructure skipped, LLM output kept")
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

func (a *noteGenerationAgent) analyzeNoteContent(ctx context.Context, input generationAgentInput) (noteChainState, error) {
	return noteChainState{
		input:    input,
		analysis: analyzeLearningContent(input),
	}, nil
}

func (a *noteGenerationAgent) planNoteOutline(ctx context.Context, state noteChainState) (noteChainState, error) {
	state.plan = planNoteOutline(state.analysis)
	return state, nil
}

func (a *noteGenerationAgent) expandNoteChainContent(ctx context.Context, state noteChainState) (noteChainState, error) {
	state.expanded = expandNoteContent(state.plan, state.analysis)
	return state, nil
}

func (a *noteGenerationAgent) generateNoteDraft(ctx context.Context, state noteChainState) (generationDraft, error) {
	input := state.input
	input.Context = appendNotePlansToContext(state.input.Context, state.plan, state.expanded)
	draft, err := a.generateDraft(ctx, input)
	if err != nil {
		return generationDraft{}, err
	}
	repairPlan := state.expanded
	if strings.TrimSpace(repairPlan.Title) == "" {
		repairPlan = state.plan
	}
	draft.noteRepairPlan = &repairPlan
	return draft, nil
}

func (a *noteGenerationAgent) repairNoteStructure(ctx context.Context, draft generationDraft) (generationDraft, error) {
	if noteNeedsStructureRepair(draft.content) {
		if draft.noteRepairPlan != nil {
			draft.content = renderNote(*draft.noteRepairPlan)
		} else {
			draft.content = a.fallback(draft.input)
		}
		draft.fallbackUsed = true
	}
	return draft, nil
}

func (a *quizGenerationAgent) analyzeQuizContent(ctx context.Context, input generationAgentInput) (quizChainState, error) {
	return quizChainState{
		input:    input,
		analysis: analyzeLearningContent(input),
	}, nil
}

func (a *quizGenerationAgent) planQuizQuestions(ctx context.Context, state quizChainState) (quizChainState, error) {
	state.plan = planQuizQuestions(state.analysis)
	return state, nil
}

func (a *quizGenerationAgent) expandQuizChainContent(ctx context.Context, state quizChainState) (quizChainState, error) {
	state.expanded = expandQuizContent(state.plan, state.analysis)
	return state, nil
}

func (a *quizGenerationAgent) generateQuizDraft(ctx context.Context, state quizChainState) (generationDraft, error) {
	input := state.input
	input.Context = appendQuizPlansToContext(state.input.Context, state.plan, state.expanded)
	draft, err := a.generateDraft(ctx, input)
	if err != nil {
		return generationDraft{}, err
	}
	repairPlan := state.expanded
	if strings.TrimSpace(repairPlan.Topic) == "" {
		repairPlan = state.plan
	}
	draft.quizRepairPlan = &repairPlan
	return draft, nil
}

func (a *quizGenerationAgent) repairQuizStructure(ctx context.Context, draft generationDraft) (generationDraft, error) {
	if quizNeedsStructureRepair(draft.content) {
		if draft.quizRepairPlan != nil {
			draft.content = renderQuiz(*draft.quizRepairPlan)
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
	llmStart := time.Now()
	strategy := pptOutlinePromptStrategy()
	outline, err := a.model.Generate(ctx, GenerationPrompt{
		AgentName:    "ppt_outline",
		System:       strategy.System,
		User:         strings.TrimSpace(input.Request.Prompt),
		Context:      input.Context,
		OutputFormat: strategy.OutputFormat,
	})
	logger.Info("[PPT] LLM call: generateOutline done",
		zap.Duration("llm_elapsed", time.Since(llmStart)),
		zap.Int("outline_len", len(outline)),
		zap.Error(err),
	)
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
	styleHint := ""
	userPrompt := ""
	if input.Request != nil {
		userPrompt = input.Request.Prompt
		styleHint = optionString(input.Request.Options, "ppt_style", "")
		if styleHint == "" {
			styleHint = optionString(input.Request.Options, "pptStyle", "")
		}
	}
	theme := designPPTStyleTheme(analysis, expandPPTContent(planPPTOutline(analysis), analysis), userPrompt, styleHint)
	return renderStyledPPTSlides(expandPPTContent(planPPTOutline(analysis), analysis), theme)
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

// isPPTPlanningLabel returns true if a line is an internal planning label
// (页面目的, 核心论点, etc.) that should not be treated as slide content.
func isPPTPlanningLabel(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	labels := []string{
		"页面目的", "页面目标", "页面意图", "本页目的", "本页目标", "本页意图",
		"设计目的", "设计目标", "设计意图",
		"slide purpose", "page purpose", "writing brief",
		"核心论点", "可用证据", "内容展开", "关键要点",
		"source-topic", "source topic", "source_topic",
	}
	for _, label := range labels {
		if strings.HasPrefix(lower, strings.ToLower(label)) {
			// Check if followed by : or ：or ** to avoid false positives
			rest := strings.TrimPrefix(lower, strings.ToLower(label))
			rest = strings.TrimSpace(rest)
			if rest == "" || strings.HasPrefix(rest, ":") || strings.HasPrefix(rest, "：") || strings.HasPrefix(rest, "**") {
				return true
			}
		}
	}
	return false
}

// normalizePPTSlideTitle cleans up slide titles: removes question marks,
// trailing colons, and converts question-style titles to concise noun phrases.
func normalizePPTSlideTitle(title string) string {
	title = strings.TrimSpace(title)
	// Remove trailing question marks (Chinese and English)
	title = strings.TrimRight(title, "?？")
	// Remove trailing colons
	title = strings.TrimRight(title, ":：")
	// If title contains a colon, take the part after the colon if it's shorter
	// and more descriptive (e.g., "影响因素：光照强度如何影响光合作用" -> "光照影响因素")
	if idx := strings.IndexAny(title, ":："); idx > 0 {
		before := strings.TrimSpace(title[:idx])
		after := strings.TrimSpace(title[idx+1:])
		// If the part after colon is a question, prefer the part before
		if strings.ContainsAny(after, "?？") {
			title = before
		} else if len([]rune(after)) > len([]rune(before)) && len([]rune(before)) <= 8 {
			// Before is concise, use it
			title = before
		}
	}
	title = strings.TrimSpace(title)
	title = strings.TrimRight(title, ":：?？")
	if title == "" {
		title = "内容页"
	}
	return title
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
		// Skip internal planning labels that the LLM may still output despite
		// instructions not to. These are meta-text, not slide content.
		if isPPTPlanningLabel(text) {
			continue
		}
		if indent == 0 {
			plan.Slides = append(plan.Slides, pptSlidePlan{Title: normalizePPTSlideTitle(text)})
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
	// Deduplicate bullets across slides: if a point already appeared on an
	// earlier slide, remove it from later slides to avoid repetition.
	plan = deduplicatePPTPlanBullets(plan)
	plan = ensurePPTPlanFrame(plan)
	return plan, true
}

// deduplicatePPTPlanBullets removes bullets that appear on multiple slides,
// keeping only the first occurrence. This prevents the LLM from generating
// slides with repeated content.
func deduplicatePPTPlanBullets(plan pptOutlinePlan) pptOutlinePlan {
	seen := make(map[string]bool)
	for i := range plan.Slides {
		filtered := make([]string, 0, len(plan.Slides[i].Bullets))
		for _, bullet := range plan.Slides[i].Bullets {
			key := strings.ToLower(strings.TrimSpace(bullet))
			// Normalize for comparison: remove punctuation differences
			key = strings.ReplaceAll(key, "：", ":")
			key = strings.ReplaceAll(key, "，", ",")
			key = strings.ReplaceAll(key, "。", ".")
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			filtered = append(filtered, bullet)
		}
		plan.Slides[i].Bullets = filtered
	}
	return plan
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

// stripPPTVisibleText extracts visible text from HTML content, also removing
// markdown artifacts (**, *, #, _) that stripSimpleHTML does not handle.
// This ensures that detection functions can reliably match planning keywords
// even when the LLM wraps them in markdown bold/italic markers.
func stripPPTVisibleText(content string) string {
	text := stripSimpleHTML(content)
	// Remove markdown bold/italic markers
	text = strings.ReplaceAll(text, "**", "")
	text = strings.ReplaceAll(text, "__", "")
	text = strings.ReplaceAll(text, "*", "")
	text = strings.ReplaceAll(text, "_", "")
	// Remove markdown heading markers
	text = strings.ReplaceAll(text, "###", "")
	text = strings.ReplaceAll(text, "##", "")
	text = strings.ReplaceAll(text, "#", "")
	return text
}

func stripPPTExportPlaceholders(content string) string {
	cleaned := stripTaggedBlock(content, "PPT_FILE")
	cleaned = stripTaggedBlock(cleaned, "PREVIEW_LINK")
	return strings.TrimSpace(cleaned)
}

// stripPPTPlanningArtifacts removes internal planning labels and meta-text
// that the LLM may accidentally leak into visible slide content. This is a
// best-effort cleanup — if too much is removed, repairPPTStructure will
// catch the resulting sparse slides and fall back to the deterministic render.
func stripPPTPlanningArtifacts(content string) string {
	// Patterns like "页面目的：...", "Purpose: ...", "Slide purpose: ..."
	// followed by planning text. Remove the entire line.
	planLinePrefixes := []string{
		"页面目的", "页面目标", "页面意图", "本页目的", "本页目标", "本页意图",
		"设计目的", "设计目标", "设计意图",
		"slide purpose", "page purpose", "writing brief",
		"source-topic", "source topic", "source_topic",
		"核心论点", "可用证据", "内容展开", "关键要点",
	}
	lines := strings.Split(content, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		skip := false
		for _, prefix := range planLinePrefixes {
			if strings.HasPrefix(lower, strings.ToLower(prefix)) ||
				strings.HasPrefix(lower, strings.ToLower(prefix)+"**") ||
				strings.HasPrefix(lower, strings.ToLower(prefix)+"：") ||
				strings.HasPrefix(lower, strings.ToLower(prefix)+":") ||
				strings.HasPrefix(lower, "**"+strings.ToLower(prefix)) {
				skip = true
				break
			}
		}
		if !skip {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

// deduplicatePPTCardTitles removes duplicate card-title elements within each
// <section>. When the LLM uses the page title as the heading for every card,
// only the first occurrence is kept; subsequent duplicates are emptied to
// avoid visual repetition. The card body content is preserved.
// stripPPTReferenceMetadata removes reference metadata lines that the LLM
// may leak into visible slide content. These include document labels
// ("文档列表", "文档介绍"), chapter paths ("专题五", "第一章"), and
// knowledge-point brackets ("【重点知识...】"). The content within these
// sections is preserved, only the metadata labels are removed.
func stripPPTReferenceMetadata(content string) string {
	refLinePrefixes := []string{
		"文档列表", "文档介绍", "文档概述", "文档目录", "文档内容",
		"专题一", "专题二", "专题三", "专题四", "专题五", "专题六",
		"专题七", "专题八", "专题九", "专题十",
		"第一章", "第二章", "第三章", "第四章", "第五章", "第六章",
		"第七章", "第八章", "第九章", "第十章",
		"第1章", "第2章", "第3章", "第4章", "第5章",
		"第一节", "第二节", "第三节", "第四节", "第五节",
		"【重点知识", "【知识要点", "【核心知识", "【基础知识",
		"【考点分析", "【考点",
		"local references:", "web results:", "web summary:",
		"original markdown:", "input markdown",
	}
	lines := strings.Split(content, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		skip := false
		for _, prefix := range refLinePrefixes {
			if strings.HasPrefix(lower, strings.ToLower(prefix)) ||
				strings.HasPrefix(lower, strings.ToLower(prefix)+"**") ||
				strings.HasPrefix(lower, strings.ToLower(prefix)+"：") ||
				strings.HasPrefix(lower, strings.ToLower(prefix)+":") ||
				strings.HasPrefix(lower, "**"+strings.ToLower(prefix)) {
				skip = true
				break
			}
		}
		if !skip {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

func deduplicatePPTCardTitles(content string) string {
	sections := pptExtractSections(content)
	if len(sections) == 0 {
		return content
	}
	modified := false
	for i, sec := range sections {
		newSec, changed := deduplicateCardTitlesInSection(sec)
		if changed {
			sections[i] = newSec
			modified = true
		}
	}
	if !modified {
		return content
	}
	// Rebuild content: replace each section in original content
	result := content
	for i := len(sections) - 1; i >= 0; i-- {
		// Find the section in result and replace
		lower := strings.ToLower(result)
		start := strings.Index(lower, "<section")
		if start < 0 {
			break
		}
		end := strings.Index(lower[start:], "</section>")
		if end < 0 {
			result = result[:start] + sections[i] + result[start+len(sections[i]):]
			break
		}
		end += start + len("</section>")
		result = result[:start] + sections[i] + result[end:]
	}
	return result
}

// deduplicateCardTitlesInSection processes a single <section> and removes
// duplicate card-title text. Returns the modified section and whether changes
// were made.
func deduplicateCardTitlesInSection(section string) (string, bool) {
	lower := strings.ToLower(section)
	// Collect all card-title texts
	titles := extractClassText(section, lower, "card-title")
	if len(titles) < 3 {
		return section, false
	}
	// Find duplicated titles (appearing 3+ times)
	freq := make(map[string]int)
	for _, t := range titles {
		freq[strings.ToLower(strings.TrimSpace(t))]++
	}
	hasDup := false
	for _, c := range freq {
		if c >= 3 {
			hasDup = true
			break
		}
	}
	if !hasDup {
		return section, false
	}
	// Replace duplicate card-title content with empty or generic text
	// We keep the first occurrence of each title and clear subsequent ones
	seen := make(map[string]bool)
	result := section
	searchFrom := 0
	for {
		idx := strings.Index(strings.ToLower(result[searchFrom:]), "card-title")
		if idx < 0 {
			break
		}
		idx += searchFrom
		// Find end of opening tag
		gtIdx := strings.Index(strings.ToLower(result[idx:]), ">")
		if gtIdx < 0 {
			break
		}
		textStart := idx + gtIdx + 1
		closeIdx := strings.Index(strings.ToLower(result[textStart:]), "<")
		if closeIdx < 0 {
			break
		}
		textEnd := textStart + closeIdx
		titleText := strings.TrimSpace(stripPPTVisibleText(result[textStart:textEnd]))
		titleKey := strings.ToLower(titleText)
		if titleKey == "" {
			searchFrom = textEnd
			continue
		}
		if freq[titleKey] >= 3 {
			if seen[titleKey] {
				// Replace duplicate with empty content
				result = result[:textStart] + result[textEnd:]
				searchFrom = textStart
			} else {
				seen[titleKey] = true
				searchFrom = textEnd
			}
		} else {
			searchFrom = textEnd
		}
	}
	return result, true
}

// stripPPTHTMLRepeatedTitlePrefix removes redundant slide-title prefixes
// that the LLM occasionally injects into every <li>/<p> on a page. For
// example a slide titled "卡尔文循环" may emit
//
//	<li>卡尔文循环：场所：叶绿体基质</li>
//	<li>卡尔文循环：CO₂固定：与RuBP结合</li>
//
// for every bullet; this rewrites them to
//
//	<li>场所：叶绿体基质</li>
//	<li>CO₂固定：与RuBP结合</li>
//
// The pass walks each <section> independently and uses the first heading
// (h1/h2/h3) inside the section as the title. Text nodes inside heading,
// <style>, and <script> tags are skipped — only body text is rewritten,
// so the heading itself never has its title stripped to nothing.
func stripPPTHTMLRepeatedTitlePrefix(content string) string {
	sections := pptExtractSections(content)
	if len(sections) == 0 {
		return content
	}
	for _, sec := range sections {
		candidates := pptCollectSectionTitleCandidates(sec)
		if len(candidates) == 0 {
			continue
		}
		rewritten := pptStripBodyTextTitlePrefix(sec, candidates)
		if rewritten == sec {
			continue
		}
		content = strings.Replace(content, sec, rewritten, 1)
	}
	return content
}

// pptCollectSectionTitleCandidates gathers every heading (h1–h6) text and
// every card-title text inside a section. Any of these can be repeated as a
// redundant prefix on body text — not just the section's main heading. A
// slide titled "卡尔文循环" may have a card titled "暗反应" whose body reads
// "暗反应：场所：叶绿体基质"; we need "暗反应" as a candidate, not only
// "卡尔文循环". Short (<2 rune) and duplicate entries are dropped. The
// returned slice is unsorted here; stripPPTBulletTitlePrefixes sorts it
// longest-first before use.
func pptCollectSectionTitleCandidates(section string) []string {
	var candidates []string
	for _, name := range []string{"h1", "h2", "h3", "h4", "h5", "h6"} {
		candidates = append(candidates, pptCollectHeadingTexts(section, name)...)
	}
	lower := strings.ToLower(section)
	candidates = append(candidates, extractClassText(section, lower, "card-title")...)
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		c = strings.TrimSpace(stripPPTVisibleText(c))
		if len([]rune(c)) < 2 {
			continue
		}
		out = append(out, c)
	}
	return out
}

// pptCollectHeadingTexts returns the visible text of every <name> element
// inside section. Markdown markers and nested tags are stripped so the text
// matches the form bullets repeat.
func pptCollectHeadingTexts(section, name string) []string {
	lower := strings.ToLower(section)
	open := "<" + name
	var texts []string
	searchFrom := 0
	for {
		idx := strings.Index(lower[searchFrom:], open)
		if idx < 0 {
			break
		}
		idx += searchFrom
		// ensure the tag name is not a prefix of a longer tag (h2 vs h2x is
		// impossible in HTML, but guard against "<h2" matching inside "<h2..." ok)
		gtRel := strings.IndexByte(lower[idx:], '>')
		if gtRel < 0 {
			break
		}
		textStart := idx + gtRel + 1
		closeTag := "</" + name + ">"
		closeRel := strings.Index(lower[textStart:], closeTag)
		if closeRel < 0 {
			searchFrom = textStart
			continue
		}
		raw := section[textStart : textStart+closeRel]
		text := strings.TrimSpace(stripPPTVisibleText(raw))
		if text != "" {
			texts = append(texts, text)
		}
		searchFrom = textStart + closeRel + len(closeTag)
	}
	return texts
}

// pptStripBodyTextTitlePrefix rewrites text nodes inside the given section
// using stripPPTBulletTitlePrefixes, while leaving text inside heading,
// <style>, and <script> tags untouched. A small depth counter tracks
// whether the cursor is currently inside a skip-tag.
func pptStripBodyTextTitlePrefix(section string, titles []string) string {
	skipTags := map[string]bool{
		"h1": true, "h2": true, "h3": true,
		"h4": true, "h5": true, "h6": true,
		"style": true, "script": true,
	}
	var b strings.Builder
	b.Grow(len(section))
	lower := strings.ToLower(section)
	pos := 0
	skipDepth := 0
	for pos < len(section) {
		ltRel := strings.IndexByte(lower[pos:], '<')
		if ltRel < 0 {
			b.WriteString(pptApplyTitlePrefixStrip(section[pos:], titles, skipDepth > 0))
			break
		}
		lt := pos + ltRel
		b.WriteString(pptApplyTitlePrefixStrip(section[pos:lt], titles, skipDepth > 0))
		gtRel := strings.IndexByte(lower[lt:], '>')
		if gtRel < 0 {
			b.WriteString(section[lt:])
			break
		}
		gt := lt + gtRel
		tag := section[lt : gt+1]
		b.WriteString(tag)
		name, isClose, selfClose := pptParseTagName(tag)
		if name != "" && skipTags[name] && !selfClose {
			if isClose {
				if skipDepth > 0 {
					skipDepth--
				}
			} else {
				skipDepth++
			}
		}
		pos = gt + 1
	}
	return b.String()
}

func pptApplyTitlePrefixStrip(text string, titles []string, skip bool) string {
	if skip || strings.TrimSpace(text) == "" {
		return text
	}
	leading := text[:len(text)-len(strings.TrimLeft(text, " \t\r\n"))]
	body := strings.TrimLeft(text, " \t\r\n")
	stripped := stripPPTBulletTitlePrefixes(body, titles)
	if stripped == body || stripped == "" {
		// Leave text unchanged when nothing matched, or when stripping would
		// blank the node — a bare <li></li> is worse than the duplicated text.
		return text
	}
	return leading + stripped
}

func pptParseTagName(tag string) (name string, isClose bool, selfClose bool) {
	if len(tag) < 2 || tag[0] != '<' {
		return "", false, false
	}
	inner := strings.TrimSpace(tag[1 : len(tag)-1])
	if strings.HasSuffix(inner, "/") {
		selfClose = true
		inner = strings.TrimSpace(strings.TrimSuffix(inner, "/"))
	}
	if strings.HasPrefix(inner, "/") {
		isClose = true
		inner = strings.TrimSpace(inner[1:])
	}
	end := len(inner)
	for i, r := range inner {
		if r == ' ' || r == '\t' || r == '\n' {
			end = i
			break
		}
	}
	return strings.ToLower(inner[:end]), isClose, selfClose
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

// appendPPTOutlineToContext injects the generated outline into the context
// for the outline review LLM call. It presents the outline as a reviewable
// draft alongside the original source material.
func appendPPTOutlineToContext(contextValue, outline string) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(contextValue))
	if strings.TrimSpace(outline) != "" {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("PPT_OUTLINE_DRAFT\n")
		b.WriteString("以下是需要审查的 PPT 大纲草案：\n")
		b.WriteString(outline)
		b.WriteString("\n\n请审查上述大纲，修正问题后返回完整的大纲。")
	}
	return strings.TrimSpace(b.String())
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
		b.WriteString("- CRITICAL: The 'source-topic' lines are TOPICS TO EXPAND, not final slide text. You must NOT copy them verbatim into the HTML. Instead, use each topic as a starting point and generate richer, more detailed presentation content.\n")
		b.WriteString("- For each source-topic, generate 1-3 sentences of actual presentation content that: explains the concept, provides context, gives a concrete example, or states a clear conclusion. The output should read like a real slide a presenter would show, not an outline.\n")
		b.WriteString("- Keep every planned slide title visible as h1/h2/h3, then add 3-5 substantial content blocks for that page.\n")
		b.WriteString("- CRITICAL: Never output internal plan labels or meta-text as visible slide content. Words like 'Purpose', '页面目的', 'Slide purpose', 'writing brief', 'source-topic', or any planning instruction must NOT appear on the slide. Only output real presentation content.\n")
		b.WriteString("- CRITICAL: Every content slide MUST have substantial body text. Do not output slides with only a title and empty or near-empty content. Each content slide needs at least 5-7 concrete points, each expanded into a full sentence or paragraph.\n")
		b.WriteString("- CRITICAL: 去文字化——不要输出大段连续叙述文本。将内容拆解为：定义句→原理句→证据/数据句→应用/例子句。每个要点自成一体，读出来像一句完整的演讲词。\n")
		b.WriteString("- CRITICAL: 每个内容页的信息密度要高。一个 slide 至少包含 5-7 个实质性的内容块（列表项、卡片、洞察标签等），不要让页面看起来只有标题和两行字。\n")
		b.WriteString("- CRITICAL: 当 source-topic 看起来笼统（如'核心概念'、'基本原理'）时，你必须将其拆解为 3-5 个具体的子论点分别展开，而不是输出一个笼统的概括。\n")
		b.WriteString("- The agenda slide must match the later content sections one-to-one; if the agenda lists a chapter, a later section with the same chapter title must exist.\n")
		b.WriteString("- Design every section as a real 16:9 PPT canvas: width: 1920px; height: 1080px; overflow: hidden; use px font sizes, not web-card rem layouts.\n")
		b.WriteString("- Make each HTML section production-ready as a standalone slide: include a clear visual hierarchy, title area, content grouping, and a small progress or section marker.\n")
		b.WriteString("- Page copy must be grounded in Original Markdown, Local References, Web Results, or the user's explicit prompt. Do not add generic business, market, team, investor, or motivational boilerplate unless it appears in the source.\n")
		b.WriteString("- Expand terse bullets into concise slide-ready statements only by clarifying the source meaning; do not introduce unrelated examples, claims, or slogans.\n")
		b.WriteString("- Finish all planned sections before returning. If the plan is long, make each section concise instead of truncating the deck.\n")
		b.WriteString("\nCONTENT_EXPANSION_GUIDE\n")
		b.WriteString("The source-topic lines are writing prompts, NOT final content. Here is how to expand them:\n\n")
		b.WriteString("BAD (copying outline verbatim):\n")
		b.WriteString("  <li>Go语言是静态类型</li>\n")
		b.WriteString("  <li>Go语言有垃圾回收</li>\n\n")
		b.WriteString("GOOD (expanding topics into real content):\n")
		b.WriteString("  <li>Go 语言采用静态类型系统，在编译期即可发现类型错误，大幅减少运行时崩溃风险。与动态语言相比，这牺牲了一些灵活性，但换来了更高的执行效率和代码可维护性。</li>\n")
		b.WriteString("  <li>Go 内置并发原语 goroutine 和 channel，配合运行时垃圾回收器，开发者无需手动管理内存即可构建高并发服务。GC 采用三色标记算法，暂停时间通常在毫秒级。</li>\n\n")
		b.WriteString("Rule: each source-topic must become at least one full sentence (30+ characters) with explanation, context, or example from the source material.\n")
	}
	return strings.TrimSpace(b.String())
}

// appendPPTRichContentToContext injects the enriched content into the context
// for the HTML generation LLM call. This provides the model with fully expanded
// paragraphs instead of just short bullet points.
func appendPPTRichContentToContext(contextValue string, richContent pptRichContent) string {
	if len(richContent.Slides) == 0 {
		return contextValue
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(contextValue))
	b.WriteString("\n\nENRICHED_PPT_CONTENT\n")
	b.WriteString("以下是已经充实完成的幻灯片内容，请直接使用这些段落和要点生成 HTML：\n\n")
	for i, slide := range richContent.Slides {
		b.WriteString(fmt.Sprintf("Slide %02d: %s\n", i+1, slide.Title))
		if slide.Subtitle != "" {
			b.WriteString("  副标题: ")
			b.WriteString(slide.Subtitle)
			b.WriteString("\n")
		}
		if len(slide.Paragraphs) > 0 {
			b.WriteString("  正文段落:\n")
			for _, p := range slide.Paragraphs {
				b.WriteString("    - ")
				b.WriteString(p)
				b.WriteString("\n")
			}
		}
		if len(slide.Bullets) > 0 {
			b.WriteString("  要点:\n")
			for _, bullet := range slide.Bullets {
				b.WriteString("    - ")
				b.WriteString(bullet)
				b.WriteString("\n")
			}
		}
		if len(slide.Insights) > 0 {
			b.WriteString("  洞察:\n")
			for _, insight := range slide.Insights {
				b.WriteString("    - ")
				b.WriteString(insight)
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("IMPORTANT: 使用上述充实后的内容生成 HTML，不要使用 STRUCTURED_PPT_PLAN 中的 source-topic。\n")
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
		// Present bullets as "source topics to expand", not final slide text.
		// This signals to the LLM that it should generate richer content based
		// on these topics, not copy them verbatim into the HTML.
		for _, bullet := range slide.Bullets {
			bullet = strings.TrimSpace(bullet)
			if bullet == "" {
				continue
			}
			b.WriteString("  source-topic: ")
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

	// Pre-process: merge fenced code blocks into single lines so they survive
	// the line-by-line extraction below. A code block like:
	//   ```go
	//   func main() { ... }
	//   ```
	// becomes a single point: "```go\nfunc main() { ... }\n```"
	lines := strings.Split(markdown, "\n")
	mergedLines := mergeCodeBlockLines(lines)

	for _, line := range mergedLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "```") {
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
		// Code blocks are already merged into single lines starting with ```
		point := strings.TrimSpace(strings.TrimLeft(line, "-*0123456789. "))
		if len([]rune(point)) < 3 {
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

// mergeCodeBlockLines merges fenced code block lines (```...```) into single
// logical lines so that downstream line-by-line extractors treat each code
// block as an atomic unit. Lines outside code blocks are passed through
// unchanged. The opening ``` line (with optional language tag) and closing ```
// are preserved in the merged line, separated by \n.
func mergeCodeBlockLines(lines []string) []string {
	var result []string
	var codeBuf strings.Builder
	inCode := false
	var codeLang string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inCode {
			// Detect opening fence: line starts with ``` (possibly followed by language)
			if strings.HasPrefix(trimmed, "```") {
				inCode = true
				codeLang = strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
				codeBuf.Reset()
				codeBuf.WriteString(trimmed) // opening ``` with language
				codeBuf.WriteByte('\n')
				continue
			}
			result = append(result, line)
		} else {
			// Inside a code block: detect closing ```
			if trimmed == "```" || trimmed == "```{" {
				codeBuf.WriteString(trimmed) // closing ```
				merged := codeBuf.String()
				// Only keep if the merged block has meaningful content
				// (at least 4 runes after stripping the fences)
				inner := strings.TrimPrefix(merged, "```"+codeLang+"\n")
				inner = strings.TrimSuffix(inner, "```")
				inner = strings.TrimSpace(inner)
				if len([]rune(inner)) >= 4 {
					result = append(result, merged)
				}
				inCode = false
				codeLang = ""
				continue
			}
			// Code content line
			codeBuf.WriteString(line)
			codeBuf.WriteByte('\n')
		}
	}
	// Handle unclosed code block: emit what we have
	if inCode {
		merged := codeBuf.String()
		inner := strings.TrimPrefix(merged, "```"+codeLang+"\n")
		inner = strings.TrimSpace(inner)
		if len([]rune(inner)) >= 4 {
			result = append(result, merged)
		}
	}
	return result
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
			slide.Bullets = append(slide.Bullets, supplementBullet(title, 1))
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
	minBullets := 4
	if analysis.Sparse {
		minBullets = 5
	}
	// Build a pool of evidence with their topic keywords for matching
	type evidenceItem struct {
		text     string
		keywords []string
		used     bool
	}
	pool := make([]evidenceItem, 0, len(analysis.Evidence))
	for _, ev := range analysis.Evidence {
		pool = append(pool, evidenceItem{
			text:     ev.Text,
			keywords: extractSignificantWords(ev.Text),
		})
	}
	// Also build pools from key concepts and processes
	conceptPool := make([]string, 0, len(analysis.KeyConcepts))
	conceptPool = append(conceptPool, analysis.KeyConcepts...)
	processPool := make([]string, 0, len(analysis.Processes))
	processPool = append(processPool, analysis.Processes...)

	for i := range plan.Slides {
		slide := &plan.Slides[i]
		if slide.Title == "封面" || slide.Title == "目录" || slide.Title == "封面页" || slide.Title == "目录页" {
			normalizePPTBullets(slide)
			continue
		}
		// Extract keywords from slide title and existing bullets for matching
		slideKeywords := extractSignificantWords(slide.Title)
		for _, b := range slide.Bullets {
			slideKeywords = append(slideKeywords, extractSignificantWords(b)...)
		}
		slideKeywordSet := make(map[string]bool)
		for _, kw := range slideKeywords {
			slideKeywordSet[strings.ToLower(kw)] = true
		}

		// Match evidence by keyword overlap with slide topic
		for len(slide.Bullets) < minBullets {
			bestIdx := -1
			bestScore := 0
			for j := range pool {
				if pool[j].used {
					continue
				}
				score := 0
				for _, kw := range pool[j].keywords {
					if slideKeywordSet[strings.ToLower(kw)] {
						score++
					}
				}
				if score > bestScore {
					bestScore = score
					bestIdx = j
				}
			}
			if bestIdx >= 0 {
				slide.Bullets = append(slide.Bullets, pool[bestIdx].text)
				pool[bestIdx].used = true
			} else {
				break
			}
		}

		// If still not enough, try concepts with keyword overlap
		if len(slide.Bullets) < minBullets {
			for j, concept := range conceptPool {
				if len(slide.Bullets) >= minBullets {
					break
				}
				conceptKeywords := extractSignificantWords(concept)
				matched := false
				for _, kw := range conceptKeywords {
					if slideKeywordSet[strings.ToLower(kw)] {
						matched = true
						break
					}
				}
				if matched {
					slide.Bullets = append(slide.Bullets, concept)
					conceptPool = append(conceptPool[:j], conceptPool[j+1:]...)
				}
			}
		}

		// If still not enough, try processes with keyword overlap
		if len(slide.Bullets) < minBullets {
			for j, proc := range processPool {
				if len(slide.Bullets) >= minBullets {
					break
				}
				procKeywords := extractSignificantWords(proc)
				matched := false
				for _, kw := range procKeywords {
					if slideKeywordSet[strings.ToLower(kw)] {
						matched = true
						break
					}
				}
				if matched {
					slide.Bullets = append(slide.Bullets, proc)
					processPool = append(processPool[:j], processPool[j+1:]...)
				}
			}
		}

		// If still not enough, use remaining concepts and processes
		if len(slide.Bullets) < minBullets {
			for _, concept := range conceptPool {
				if len(slide.Bullets) >= minBullets {
					break
				}
				slide.Bullets = append(slide.Bullets, concept)
			}
		}
		if len(slide.Bullets) < minBullets {
			for _, proc := range processPool {
				if len(slide.Bullets) >= minBullets {
					break
				}
				slide.Bullets = append(slide.Bullets, proc)
			}
		}

		// Last resort: add a contextual supplement
		for len(slide.Bullets) < minBullets {
			slide.Bullets = append(slide.Bullets, supplementBullet(slide.Title, len(slide.Bullets)+1))
		}
		normalizePPTBullets(slide)
	}
	return plan
}

// enrichPPTContent calls the LLM to expand short bullet points into full
// paragraphs suitable for presentation slides. It batches the slide plan so
// each model call returns a small JSON object instead of one oversized deck.
// Batches are processed concurrently via a fixed-size worker pool to reduce
// wall-clock time while keeping the per-call retry logic intact.
func (a *pptGenerationAgent) enrichPPTContent(ctx context.Context, state pptChainState) (pptChainState, error) {
	if a.model == nil {
		return state, nil
	}

	strategy := pptContentEnrichPromptStrategy()
	batches := batchPPTOutlinePlan(state.expanded, pptContentEnrichBatchSize)
	if len(batches) == 0 {
		return state, nil
	}

	logger.Info("[PPT] enrichPPTContent: starting batch enrichment",
		zap.Int("total_slides", len(state.expanded.Slides)),
		zap.Int("batch_count", len(batches)),
		zap.Int("batch_size", pptContentEnrichBatchSize),
	)

	// Job and result types for the concurrent worker pool.
	type enrichJob struct {
		index int
		batch pptOutlinePlan
	}
	type enrichResult struct {
		index int
		rich  pptRichContent
		ok    bool
	}

	// Determine worker count: cap at the effective batch count.
	workerCount := len(batches)
	if workerCount > pptContentEnrichBatchSize {
		workerCount = pptContentEnrichBatchSize
	}

	jobChan := make(chan enrichJob, len(batches))
	resultChan := make(chan enrichResult, len(batches))

	g, ctx := errgroup.WithContext(ctx)

	// Producer: send all batches into jobChan.
	g.Go(func() error {
		defer close(jobChan)
		for i, batch := range batches {
			select {
			case jobChan <- enrichJob{index: i, batch: batch}:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})

	// Fixed-size worker pool. Each worker picks up jobs, runs the enrichment
	// (with retry), and sends the result to resultChan.
	var mu sync.Mutex
	workerID := 0
	for range workerCount {
		g.Go(func() error {
			mu.Lock()
			wid := workerID
			workerID++
			mu.Unlock()

			for job := range jobChan {
				batchStart := time.Now()
				batchState := state
				batchState.expanded = job.batch

				contextValue := state.input.Context
				contextValue += "\n\nPPT_OUTLINE_FOR_ENRICHMENT\n"
				contextValue += renderPPTPlanForPrompt(job.batch)

				rich, ok := a.tryEnrichPPTContent(ctx, strategy, batchState, contextValue, false)
				if !ok {
					rich, ok = a.tryEnrichPPTContent(ctx, strategy, batchState, contextValue, true)
				}

				// Build common log fields
				logFields := []zap.Field{
					zap.Int("worker_id", wid),
					zap.Int("batch_index", job.index),
					zap.Int("batch_count", len(batches)),
					zap.Duration("elapsed", time.Since(batchStart)),
				}

				if !ok {
					logger.Warn("[PPT] enrichPPTContent: batch skipped",
						append(logFields,
							zap.Int("slides_in_batch", len(job.batch.Slides)),
						)...)
					select {
					case resultChan <- enrichResult{index: job.index, ok: false}:
					case <-ctx.Done():
						return ctx.Err()
					}
					continue
				}

				logger.Info("[PPT] enrichPPTContent: batch done",
					append(logFields,
						zap.Int("slides_in_batch", len(job.batch.Slides)),
						zap.Int("rich_slides", len(rich.Slides)),
					)...)
				select {
				case resultChan <- enrichResult{index: job.index, rich: rich, ok: true}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			logger.Debug("[PPT] enrichPPTContent: worker finished",
				zap.Int("worker_id", wid),
			)
			return nil
		})
	}

	// Producer and workers are all part of the errgroup; wait for them to finish,
	// then close resultChan. This is done in a separate goroutine (NOT inside the
	// errgroup) to avoid the deadlock of calling g.Wait() from within g.Go().
	done := make(chan struct{})
	var gErr error
	go func() {
		gErr = g.Wait()
		close(resultChan)
		close(done)
	}()

	// Collect all results and merge by original index order.
	results := make([]enrichResult, len(batches))
	for res := range resultChan {
		results[res.index] = res
	}

	// Wait for the errgroup to complete.
	<-done
	if gErr != nil {
		if ctx.Err() != nil {
			return state, gErr
		}
		logger.Warn("[PPT] enrichPPTContent: non-fatal worker error",
			zap.Error(gErr),
		)
	}

	merged := pptRichContent{}
	for _, res := range results {
		if res.ok {
			merged.Slides = append(merged.Slides, res.rich.Slides...)
		}
	}
	if pptRichContentHasUsableSlides(merged) {
		state.richContent = merged
	}
	return state, nil
}

func batchPPTOutlinePlan(plan pptOutlinePlan, batchSize int) []pptOutlinePlan {
	if batchSize <= 0 {
		batchSize = pptContentEnrichBatchSize
	}
	if len(plan.Slides) == 0 {
		return nil
	}
	batches := make([]pptOutlinePlan, 0, (len(plan.Slides)+batchSize-1)/batchSize)
	for start := 0; start < len(plan.Slides); start += batchSize {
		end := start + batchSize
		if end > len(plan.Slides) {
			end = len(plan.Slides)
		}
		batches = append(batches, pptOutlinePlan{
			Title:  plan.Title,
			Slides: append([]pptSlidePlan(nil), plan.Slides[start:end]...),
		})
	}
	return batches
}

// tryEnrichPPTContent issues one enrichment call and returns the parsed
// rich content. On retry the OutputFormat is appended with a stricter
// JSON-only reminder; the system prompt itself is left unchanged so the
// model still knows what to write.
func (a *pptGenerationAgent) tryEnrichPPTContent(ctx context.Context, strategy generationPromptStrategy, state pptChainState, contextValue string, retry bool) (pptRichContent, bool) {
	outputFormat := strategy.OutputFormat
	if retry {
		outputFormat += "\n\n严格要求：上一次输出无法被解析为 JSON。本次回复必须只包含一个 JSON 对象，第一个字符必须是 '{'，最后一个字符必须是 '}'。不要输出 ```、不要任何解释、不要前后空行说明。"
	}
	llmStart := time.Now()
	generated, err := a.model.Generate(ctx, GenerationPrompt{
		AgentName:    a.name + "_content_enrich",
		System:       strategy.System,
		User:         strings.TrimSpace(state.input.Request.Prompt),
		Context:      contextValue,
		OutputFormat: outputFormat,
		MaxTokens:    pptContentEnrichMaxTokens,
	})
	logger.Info("[PPT] LLM call: tryEnrichPPTContent done",
		zap.Duration("llm_elapsed", time.Since(llmStart)),
		zap.Int("generated_len", len(generated)),
		zap.Bool("retry", retry),
		zap.Error(err),
	)
	logFields := pptEnrichLogFields(state, retry)
	if err != nil {
		logger.Warn("enrich ppt content: model generate failed",
			append(logFields, zap.Error(err))...)
		return pptRichContent{}, false
	}
	generated = strings.TrimSpace(generated)
	if generated == "" {
		logger.Warn("enrich ppt content: model returned empty", logFields...)
		return pptRichContent{}, false
	}
	jsonStr := extractFirstJSONObject(generated)
	if jsonStr == "" {
		logger.Warn("enrich ppt content: no json object in output",
			append(logFields,
				zap.Int("output_len", len(generated)),
				zap.String("output_head", truncate(generated, 200)),
			)...)
		return pptRichContent{}, false
	}
	var rich pptRichContent
	if err := json.Unmarshal([]byte(jsonStr), &rich); err != nil {
		logger.Warn("enrich ppt content: json unmarshal failed",
			append(logFields,
				zap.Int("json_len", len(jsonStr)),
				zap.String("json_head", truncate(jsonStr, 200)),
				zap.Error(err),
			)...)
		return pptRichContent{}, false
	}
	if !pptRichContentHasUsableSlides(rich) {
		paragraphTotal := 0
		for _, slide := range rich.Slides {
			paragraphTotal += len(slide.Paragraphs)
		}
		logger.Warn("enrich ppt content: parsed but no usable slides",
			append(logFields,
				zap.Int("slide_count", len(rich.Slides)),
				zap.Int("paragraph_total", paragraphTotal),
			)...)
		return pptRichContent{}, false
	}
	return rich, true
}

// pptEnrichLogFields builds the common log field set used by every drop
// point inside tryEnrichPPTContent so failures can be grouped per user /
// notebook / retry-attempt without copy-pasting the same boilerplate.
func pptEnrichLogFields(state pptChainState, retry bool) []zap.Field {
	var userID, notebookID uint
	if state.input.Request != nil {
		userID = state.input.Request.UserID
		notebookID = state.input.Request.NotebookID
	}
	return []zap.Field{
		zap.Uint("user_id", userID),
		zap.Uint("notebook_id", notebookID),
		zap.Bool("retry", retry),
		zap.Int("plan_slide_count", len(state.expanded.Slides)),
	}
}

// extractFirstJSONObject locates the first '{' and the last '}' in the
// model output and returns the slice between them. This tolerates the
// most common LLM artifacts that the previous code missed: ```json fences
// that are not at the very start/end, leading "Here is the JSON:" prose,
// trailing "希望对你有帮助" tails, and stray whitespace inside fences.
// Bracket pairing is intentionally not validated here — json.Unmarshal in
// the caller is the source of truth, and the retry path picks up any
// pathological cases (e.g. multiple top-level objects).
func extractFirstJSONObject(s string) string {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return ""
	}
	end := strings.LastIndexByte(s, '}')
	if end <= start {
		return ""
	}
	return s[start : end+1]
}

// pptRichContentHasUsableSlides verifies that at least one slide carries
// a non-empty paragraph. A response with slides whose paragraphs are all
// empty is worse than no enrichment at all — it would let the HTML stage
// believe enrichment succeeded and skip the fallback to source-topics.
func pptRichContentHasUsableSlides(rich pptRichContent) bool {
	if len(rich.Slides) == 0 {
		return false
	}
	for _, slide := range rich.Slides {
		for _, p := range slide.Paragraphs {
			if strings.TrimSpace(p) != "" {
				return true
			}
		}
	}
	return false
}

func normalizePPTBullets(slide *pptSlidePlan) {
	// Strip a leading repetition of the slide title from each bullet before
	// dedup/truncation. The section extractor and the LLM frequently prefix
	// every point with the page title (e.g. a slide titled "卡尔文循环" gets
	// "卡尔文循环：场所：叶绿体基质"), which reads redundantly under the heading.
	// Code blocks are skipped — stripping a title prefix from code would corrupt
	// the code content.
	for i, bullet := range slide.Bullets {
		if isPPTCodeBlockBullet(bullet) {
			continue
		}
		slide.Bullets[i] = stripPPTBulletSlideTitlePrefix(bullet, slide.Title)
	}
	slide.Bullets = uniqueNonEmpty(slide.Bullets)
	switch slide.Title {
	case "封面", "目录":
		return
	}
	if len(slide.Bullets) > 9 {
		slide.Bullets = append([]string{}, slide.Bullets[:7]...)
	}
}

// stripPPTBulletSlideTitlePrefix removes a leading repetition of the slide
// title from a single bullet. It only strips when the title is immediately
// followed by a separator (：: - — ~ | / 、 or whitespace), so a short title
// like "封面" never trims "封面页" and a title never trims a bullet that just
// happens to start with the same characters as prose. Stripping repeats up to
// 3 times to handle "卡尔文循环：卡尔文循环：场所：叶绿体基质".
func stripPPTBulletSlideTitlePrefix(bullet, title string) string {
	return stripPPTBulletTitlePrefixes(bullet, []string{title})
}

// stripPPTBulletTitlePrefixes is the multi-title variant used by the HTML
// pass: a slide may have a section heading (h2) plus one or more sub-headings
// (h3/card-title), and any of them can be repeated as a redundant prefix on
// body text. Candidates are tried from longest to shortest each iteration so
// the most specific match wins; iteration repeats up to 3 times to peel
// stacked prefixes like "卡尔文循环：暗反应：场所" → "场所".
func stripPPTBulletTitlePrefixes(bullet string, candidates []string) string {
	bullet = strings.TrimSpace(bullet)
	if bullet == "" || len(candidates) == 0 {
		return bullet
	}
	usable := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if len([]rune(c)) < 2 {
			continue
		}
		key := strings.ToLower(c)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		usable = append(usable, c)
	}
	if len(usable) == 0 {
		return bullet
	}
	// Longest first so "卡尔文循环：暗反应" wins before "卡尔文循环".
	for i := 1; i < len(usable); i++ {
		for j := i; j > 0 && len([]rune(usable[j])) > len([]rune(usable[j-1])); j-- {
			usable[j], usable[j-1] = usable[j-1], usable[j]
		}
	}
	for iter := 0; iter < 3; iter++ {
		stripped, ok := stripPPTBulletPrefixesOnce(bullet, usable)
		if !ok {
			break
		}
		bullet = stripped
	}
	return bullet
}

func stripPPTBulletPrefixesOnce(bullet string, candidates []string) (string, bool) {
	for _, prefix := range candidates {
		for _, variant := range pptSlideTitlePrefixCandidates(prefix) {
			rest, ok := cutPPTTitlePrefix(bullet, variant)
			if ok {
				return rest, true
			}
		}
	}
	return bullet, false
}

func stripPPTBulletTitlePrefixOnce(bullet, title string) (string, bool) {
	return stripPPTBulletPrefixesOnce(bullet, []string{title})
}

// pptSlideTitlePrefixCandidates returns the title variants that should be
// stripped when they appear at the start of a bullet. Besides the raw title,
// this includes the core part before any bracketed suffix ("卡尔文循环（一）"
// -> "卡尔文循环") so numbered chapter titles still match their plain form.
func pptSlideTitlePrefixCandidates(title string) []string {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil
	}
	candidates := []string{title}
	if idx := strings.IndexAny(title, "（([【"); idx > 0 {
		core := strings.TrimSpace(title[:idx])
		if len([]rune(core)) >= 2 && core != title {
			candidates = append(candidates, core)
		}
	}
	return candidates
}

// cutPPTTitlePrefix checks whether bullet starts with prefix (case-insensitive)
// immediately followed by a separator, and if so returns the remainder after
// the separator. Returns ok=false when there is no such prefix+separator,
// which prevents trimming a prefix that is merely the start of a longer word.
// Whitespace is intentionally NOT trimmed before the separator check, so a
// space-separated form like "光合作用 光反应" is also recognized.
func cutPPTTitlePrefix(bullet, prefix string) (string, bool) {
	if prefix == "" {
		return bullet, false
	}
	if !strings.HasPrefix(strings.ToLower(bullet), strings.ToLower(prefix)) {
		return bullet, false
	}
	rest := bullet[len(prefix):]
	if rest == "" {
		return bullet, false
	}
	r := []rune(rest)[0]
	if !isPPTTitleSeparatorRune(r) && r != ' ' && r != '\t' {
		return bullet, false
	}
	after := strings.TrimSpace(string([]rune(rest)[1:]))
	if after == "" {
		return bullet, false
	}
	return after, true
}

func isPPTTitleSeparatorRune(r rune) bool {
	switch r {
	case '：', ':', '-', '—', '–', '~', '|', '/', '、', '·':
		return true
	}
	return false
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

// styleHintMap: keyword patterns mapped to theme indices and descriptions
// This lets users express style preferences in natural language.
var styleHintMap = []struct {
	patterns []string
	themeIdx int
	desc     string
}{
	// 科技深色
	{[]string{"深色", "dark", "科技", "tech", "极客", "geek", "赛博", "cyber", "霓虹", "neon", "暗黑", "midnight", "黑客", "hacker", "渐变蓝", "渐变", "glow", "发光", "脉冲", "pulse"}, 2, "用户要求深色/科技风格"},
	// 学术清新
	{[]string{"商务", "business", "简约", "minimal", "专业", "professional", "经典", "classic", "古典", "antique", "庄重", "solemn", "古风", "传统", "traditional", "莫兰迪", "morandi", "低饱和", "低度饱和"}, 0, "用户要求商务/简约风格"},
	{[]string{"学术", "academic", "论文", "paper", "研究", "research", "论文风格", "学院", "institute", "知网", "期刊", "journal"}, 1, "用户要求学术风格"},
	// 暖色叙事
	{[]string{"暖色", "warm", "叙事", "narrative", "故事", "story", "温暖", "温暖色调", "治愈", "healing", "柔和", "soft", "温馨", "cozy", "橙", "orange", "粉", "pink", "日落", "sunset", " autobiograph"}, 3, "用户要求暖色/叙事风格"},
}

// extractStyleHintFromPrompt scans the user prompt for style-related keywords
// and returns the matching theme index and description, or (-1, "") if no match.
func extractStyleHintFromPrompt(prompt string) (int, string) {
	promptLower := strings.ToLower(prompt)
	// Also split the prompt into individual words for broader matching
	words := splitKeywordCandidates(promptLower)
	for _, entry := range styleHintMap {
		for _, pattern := range entry.patterns {
			patternLower := strings.ToLower(pattern)
			// Check if the prompt contains the pattern as a substring
			if strings.Contains(promptLower, patternLower) {
				return entry.themeIdx, entry.desc
			}
			// Also check individual words
			for _, word := range words {
				if word == patternLower || strings.Contains(word, patternLower) {
					return entry.themeIdx, entry.desc
				}
			}
		}
	}
	return -1, ""
}

// designPPTStyleTheme selects a coherent visual theme based on content analysis.
// styleHint is an explicit style preference string (e.g. from options["ppt_style"] or prompt keywords).
// If styleHint is one of the theme names, it's used directly. Otherwise, keyword matching applies.
func designPPTStyleTheme(analysis learningContentAnalysis, plan pptOutlinePlan, userPrompt string, styleHint string) pptStyleTheme {
	themes := []pptStyleTheme{
		{
			Name: "简约商务", Primary: "#0f766e", Secondary: "#c2410c",
			Background: "#f8fafc", Surface: "#ffffff", Text: "#111827",
			Heading: "#0f172a", Muted: "#4b5563",
			FontHeading: "'Segoe UI', 'Microsoft YaHei', sans-serif",
			FontBody:    "system-ui, 'Segoe UI', 'Microsoft YaHei', sans-serif",
			LayoutHints: "Use clean grids, generous whitespace, and subtle accent borders. " +
				"Content slides should use 2-column layouts with main points on the left and insight cards on the right.",
		},
		{
			Name: "学术清新", Primary: "#1e40af", Secondary: "#7c3aed",
			Background: "#fefce8", Surface: "#ffffff", Text: "#1e293b",
			Heading: "#0c1e3e", Muted: "#64748b",
			FontHeading: "'Georgia', 'Times New Roman', serif",
			FontBody:    "system-ui, 'Segoe UI', sans-serif",
			LayoutHints: "Use serif headings for an academic feel. " +
				"Slides should have numbered sections, definition cards, and comparison tables.",
		},
		{
			Name: "科技深色", Primary: "#06b6d4", Secondary: "#f59e0b",
			Background: "#0f172a", Surface: "#1e293b", Text: "#e2e8f0",
			Heading: "#f1f5f9", Muted: "#94a3b8",
			FontHeading: "'Segoe UI', sans-serif",
			FontBody:    "system-ui, 'Segoe UI', sans-serif",
			LayoutHints: "Dark background with bright accent text. " +
				"Use glowing borders, monospace code blocks, and data-driven charts/diagrams.",
		},
		{
			Name: "暖色叙事", Primary: "#be123c", Secondary: "#0369a1",
			Background: "#fff7ed", Surface: "#ffffff", Text: "#1c1917",
			Heading: "#451a03", Muted: "#78716c",
			FontHeading: "'Segoe UI', 'Microsoft YaHei', sans-serif",
			FontBody:    "system-ui, 'Segoe UI', 'Microsoft YaHei', sans-serif",
			LayoutHints: "Warm palette for storytelling. " +
				"Use large quote blocks, timeline layouts, and photo-placeholder cards.",
		},
	}

	// 1. 优先使用显式风格 hint（来自 options["ppt_style"]）
	userStyleHint := ""
	themeIdx := -1
	styleHint = strings.TrimSpace(styleHint)
	if styleHint != "" {
		if idx, ok := matchThemeByName(styleHint); ok {
			themeIdx = idx
			userStyleHint = "用户显式指定风格: " + styleHint
		} else if idx, desc := extractStyleHintFromPrompt(styleHint); idx >= 0 {
			themeIdx = idx
			userStyleHint = desc
		}
	}

	// 2. 其次解析用户提示词中的风格关键词
	if themeIdx < 0 {
		if idx, desc := extractStyleHintFromPrompt(userPrompt); idx >= 0 {
			themeIdx = idx
			userStyleHint = desc
		}
	}

	// 3. 如果用户没有指定风格，根据内容主题自动选择
	if themeIdx < 0 {
		topic := strings.ToLower(analysis.Topic)
		themeIdx = 0
		switch {
		case containsAnyFold(topic, "代码", "编程", "算法", "系统", "架构", "数据", "code", "programming", "algorithm", "system", "architecture", "data"):
			themeIdx = 2
		case containsAnyFold(topic, "论文", "研究", "理论", "学术", "paper", "research", "theory", "academic"):
			themeIdx = 1
		case containsAnyFold(topic, "故事", "案例", "历史", "叙事", "story", "case", "history", "narrative"):
			themeIdx = 3
		}
	}

	theme := themes[themeIdx%len(themes)]
	if len(plan.Slides) > 10 {
		theme.LayoutHints += " With many slides, keep each slide focused on one key idea."
	}
	// 把用户的风格偏好附加到 LayoutHints，让 CSS 生成 LLM 能看到
	if userStyleHint != "" {
		theme.LayoutHints = userStyleHint + ". " + theme.LayoutHints
	}
	return theme
}

// matchThemeByName maps an explicit style string (e.g. "tech", "minimal",
// "科技深色", "简约商务") to a theme index. Returns ok=false when the string
// does not correspond to any known theme.
func matchThemeByName(style string) (int, bool) {
	style = strings.ToLower(strings.TrimSpace(style))
	if style == "" {
		return 0, false
	}
	// short aliases
	switch style {
	case "auto", "automatic", "自动":
		return -1, true // -1 means "let the auto path decide"
	case "minimal", "business", "简约", "简约商务", "商务", "minimal-business":
		return 0, true
	case "academic", "清新", "学术", "学术清新", "scholarly":
		return 1, true
	case "tech", "dark", "科技", "深色", "科技深色", "technology", "geek":
		return 2, true
	case "warm", "narrative", "暖色", "叙事", "暖色叙事", "story":
		return 3, true
	}
	// fuzzy: style string contains a theme name fragment
	for i, name := range []string{"简约商务", "学术清新", "科技深色", "暖色叙事"} {
		if strings.Contains(style, strings.ToLower(name)) {
			return i, true
		}
	}
	return 0, false
}

// pptThemeVisualDescription returns a natural-language description of each
// theme's visual characteristics. The CSS-generation LLM uses this to faithfully
// reproduce the intended look instead of defaulting to a generic style.
func pptThemeVisualDescription(name string) string {
	switch name {
	case "简约商务":
		return "浅色背景、大面积留白、细线分隔、青绿色主色调点缀橙色强调；扁平卡片、双栏布局；整体克制、专业、商务感强。"
	case "学术清新":
		return "米黄底色、衬线标题（Georgia/宋体感）、蓝紫色配色、编号小节、定义卡片与对比表格；严谨、学院风、文献感。"
	case "科技深色":
		return "深色背景（深蓝/近黑）、亮色文字、青色与琥珀色高对比强调、发光描边、等宽代码块、数据图表感；未来、科技、极客风。"
	case "暖色叙事":
		return "暖白底色、酒红与深蓝配色、大号引语块、时间线布局、图片占位卡；温暖、叙事、故事感。"
	}
	return ""
}

// appendPPTStyleToContext injects the visual style theme into the LLM context
// so the model has concrete CSS variables and layout guidance.
func appendPPTStyleToContext(contextValue string, theme pptStyleTheme) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(contextValue))
	b.WriteString("\n\nPPT_STYLE_THEME\n")
	b.WriteString(fmt.Sprintf("Theme: %s\n", theme.Name))
	b.WriteString(fmt.Sprintf("Primary: %s\n", theme.Primary))
	b.WriteString(fmt.Sprintf("Secondary: %s\n", theme.Secondary))
	b.WriteString(fmt.Sprintf("Background: %s\n", theme.Background))
	b.WriteString(fmt.Sprintf("Surface: %s\n", theme.Surface))
	b.WriteString(fmt.Sprintf("Text: %s\n", theme.Text))
	b.WriteString(fmt.Sprintf("Heading: %s\n", theme.Heading))
	b.WriteString(fmt.Sprintf("Muted: %s\n", theme.Muted))
	b.WriteString(fmt.Sprintf("FontHeading: %s\n", theme.FontHeading))
	b.WriteString(fmt.Sprintf("FontBody: %s\n", theme.FontBody))
	b.WriteString("VisualDescription: ")
	b.WriteString(pptThemeVisualDescription(theme.Name))
	b.WriteString("\n")
	b.WriteString("LayoutGuidance: ")
	b.WriteString(theme.LayoutHints)
	b.WriteString("\n\nSTYLE_USAGE_RULES\n")
	b.WriteString("- 你必须严格使用上方 PPT_STYLE_THEME 中的配色与字体作为 :root 的 CSS 自定义属性（--bg、--surface、--accent、--accent-2、--accent-soft、--text、--muted、--heading、--border、--panel 等），不要自行替换为其他颜色。\n")
	b.WriteString("- VisualDescription 描述了该风格的视觉特征，你的 CSS 必须忠实还原这些特征（如深色背景、衬线标题、暖色渐变等），而不是回归通用简约样式。\n")
	b.WriteString("- 背景色必须使用 Background 值（深色主题尤其重要：背景必须是深色，文字必须是亮色），不得用浅色背景覆盖深色主题。\n")
	b.WriteString("- Use the CSS variables above as :root custom properties in your <style> block.\n")
	b.WriteString("- Vary slide layouts: not every content slide should look the same.\n")
	b.WriteString("- Use at least 3 different layout patterns across the deck (e.g. two-column, full-width, card-grid, comparison, timeline).\n")
	b.WriteString("- Each slide must be visually distinct while sharing the same color palette and typography.\n")
	return strings.TrimSpace(b.String())
}

// ensurePPTSlideAttributes adds the ppt-slide class and data-ppt-slide attribute
// to <section> tags that are missing them.
func ensurePPTSlideAttributes(content string) string {
	lower := strings.ToLower(content)
	if !strings.Contains(lower, "<section") {
		return content
	}
	var b strings.Builder
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
		tagLower := strings.ToLower(tag)
		if strings.Contains(tagLower, "class=") {
			if !strings.Contains(tagLower, "ppt-slide") {
				tag = strings.Replace(tag, "class=\"", "class=\"ppt-slide ", 1)
				tag = strings.Replace(strings.ToLower(tag), "class='", "class='ppt-slide ", 1)
			}
		} else {
			tag = strings.TrimSuffix(tag, ">") + ` class="ppt-slide" data-ppt-slide="true">`
		}
		if !strings.Contains(strings.ToLower(tag), "data-ppt-slide") {
			tag = strings.TrimSuffix(tag, ">") + ` data-ppt-slide="true">`
		}
		b.WriteString(tag)
		pos = end + 1
	}
	return b.String()
}

// ensurePPTCanvasSize injects 1920x1080 canvas dimensions into the CSS if missing.
func ensurePPTCanvasSize(content string) string {
	lower := strings.ToLower(content)
	if strings.Contains(lower, "width:1920px") || strings.Contains(lower, "width: 1920px") {
		if strings.Contains(lower, "height:1080px") || strings.Contains(lower, "height: 1080px") {
			return content
		}
	}
	if !strings.Contains(lower, "<style") {
		return content
	}
	canvasCSS := `.ppt-slide, section.ppt-slide {
  width: 1920px;
  height: 1080px;
  overflow: hidden;
  position: relative;
  box-sizing: border-box;
}`
	styleClose := strings.Index(lower, "</style>")
	if styleClose < 0 {
		return content
	}
	insertPos := styleClose
	return content[:insertPos] + canvasCSS + "\n" + content[insertPos:]
}

// ensurePPTStyleBlock adds a minimal <style> block if the LLM output has none.
func ensurePPTStyleBlock(content string) string {
	lower := strings.ToLower(content)
	if strings.Contains(lower, "<style") {
		return content
	}
	if !strings.Contains(lower, "<section") {
		return content
	}
	minimalCSS := `<style>
.ppt-slide { width: 1920px; height: 1080px; overflow: hidden; position: relative; box-sizing: border-box; padding: 80px 100px; background: #ffffff; color: #111827; font-family: system-ui, 'Segoe UI', 'Microsoft YaHei', sans-serif; }
.ppt-slide h1 { font-size: 72px; font-weight: 800; }
.ppt-slide h2 { font-size: 48px; font-weight: 700; }
.ppt-slide p, .ppt-slide li { font-size: 30px; line-height: 1.4; }
</style>`
	return minimalCSS + "\n" + content
}

// extractCSSBlock extracts the <style>...</style> block from LLM output.
// If no style tag is found, it wraps the raw output in a style block.
func extractCSSBlock(output string) string {
	lower := strings.ToLower(output)
	start := strings.Index(lower, "<style")
	if start < 0 {
		return "<style>\n" + output + "\n</style>"
	}
	end := strings.Index(lower, "</style>")
	if end < 0 {
		return output + "\n</style>"
	}
	return output[start : end+len("</style>")]
}

// fallbackPPTCSS returns a complete CSS block derived from the style theme,
// used when the LLM CSS call fails or the model is nil.
func fallbackPPTCSS(theme pptStyleTheme) string {
	return fmt.Sprintf(`<style>
:root {
  --bg: %s; --surface: %s; --surface-soft: %s;
  --accent: %s; --accent-2: %s; --accent-soft: %s;
  --text: %s; --muted: %s; --heading: %s;
  --border: %s; --panel: %s;
}
* { margin: 0; padding: 0; box-sizing: border-box; }
.ppt-slide {
  position: relative;
  width: 1920px;
  height: 1080px;
  overflow: hidden;
  background: var(--surface);
  color: var(--text);
  font-family: %s;
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
  background: %s;
  padding: 120px 150px;
}
h1 { font-size: 76px; font-weight: 850; color: var(--heading); line-height: 1.12; }
h2 { font-size: 52px; font-weight: 800; color: var(--heading); line-height: 1.18; }
h3 { font-size: 34px; font-weight: 700; color: var(--heading); }
p, li { font-size: 32px; line-height: 1.36; }
ul { list-style: none; padding-left: 0; }
.content-grid { display: grid; grid-template-columns: minmax(0, 1.3fr) minmax(420px, .7fr); gap: 54px; }
.main-points { background: var(--surface); border-top: 6px solid var(--accent); padding: 12px 0 0; }
.insight-panel { min-height: 360px; background: linear-gradient(180deg, var(--accent-soft) 0%%, var(--panel) 100%%); border: 1px solid var(--border); border-radius: 8px; padding: 34px; }
.card-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 32px; }
.content-card { min-height: 280px; background: var(--surface); border: 1px solid var(--border); border-left: 6px solid var(--accent); border-radius: 8px; padding: 32px 36px; }
.full-width-list { background: var(--surface); border-top: 6px solid var(--accent); padding: 12px 0 0; }
.comparison-layout { display: grid; grid-template-columns: 1fr 1fr; gap: 42px; }
.comparison-col { min-height: 400px; border-radius: 8px; padding: 36px; }
.comparison-col.left { background: var(--accent-soft); border-left: 6px solid var(--accent); }
.comparison-col.right { background: var(--panel); border-left: 6px solid var(--accent-2); }
.quote-block { background: linear-gradient(135deg, var(--accent-soft) 0%%, var(--panel) 100%%); border-left: 8px solid var(--accent); border-radius: 8px; padding: 48px 56px; min-height: 400px; }
.ppt-code-block { background: #1e293b; color: #e2e8f0; border-radius: 8px; padding: 28px 32px; margin-top: 18px; font-family: 'Cascadia Code', 'Fira Code', 'Consolas', 'Courier New', monospace; font-size: 24px; line-height: 1.5; white-space: pre-wrap; word-break: break-all; overflow-x: auto; border-left: 6px solid var(--accent-2); }
.ppt-code-block code { font-family: inherit; color: inherit; }
.slide-progress { position: absolute; left: 112px; right: 112px; bottom: 58px; height: 10px; border-radius: 999px; background: #e5e7eb; overflow: hidden; }
.slide-progress span { display: block; height: 100%%; border-radius: inherit; background: linear-gradient(90deg, var(--accent), var(--accent-2)); }
</style>`,
		theme.Background, theme.Surface, theme.Surface,
		theme.Primary, theme.Secondary, pptAccentSoft(theme),
		theme.Text, theme.Muted, theme.Heading,
		pptThemeBorder(theme), pptThemePanel(theme),
		pptThemeCoverGradient(theme),
		theme.FontBody)
}

// pptAccentSoft returns a soft tint of the primary accent suitable for
// panel backgrounds. Dark themes use a darker accent overlay.
func pptAccentSoft(theme pptStyleTheme) string {
	switch theme.Name {
	case "科技深色":
		return "#1e3a5f"
	case "学术清新":
		return "#eef2ff"
	case "暖色叙事":
		return theme.Primary + "1a"
	default:
		return theme.Primary + "22"
	}
}

// pptThemeBorder returns a border color derived from the theme. Light themes
// use a subtle gray-green border; the dark theme uses a darker slate border.
func pptThemeBorder(theme pptStyleTheme) string {
	switch theme.Name {
	case "科技深色":
		return "#334155"
	case "学术清新":
		return "#dbe4f0"
	case "暖色叙事":
		return "#f0d9c4"
	default:
		return "#d7e3df"
	}
}

// pptThemePanel returns a panel/surface-soft background color derived from
// the theme.
func pptThemePanel(theme pptStyleTheme) string {
	switch theme.Name {
	case "科技深色":
		return "#1e293b"
	case "学术清新":
		return "#eef2ff"
	case "暖色叙事":
		return "#fff1e6"
	default:
		return "#f3f7f6"
	}
}

// pptThemeCoverGradient returns the cover-page gradient for a given theme.
func pptThemeCoverGradient(theme pptStyleTheme) string {
	switch theme.Name {
	case "科技深色":
		return "linear-gradient(135deg, #0f172a 0%, #1e293b 58%, #0c2438 100%)"
	case "学术清新":
		return "linear-gradient(135deg, #fefce8 0%, #eef2ff 58%, #faf5ff 100%)"
	case "暖色叙事":
		return "linear-gradient(135deg, #fff7ed 0%, #ffe4e6 58%, #ffedd5 100%)"
	default:
		return "linear-gradient(135deg, #f7fbfa 0%, #e8f2ef 58%, #fff7ed 100%)"
	}
}

// pptCSSHasCanvasSize checks if the CSS block defines 1920x1080 canvas.
func pptCSSHasCanvasSize(css string) bool {
	lower := strings.ToLower(css)
	hasWidth := strings.Contains(lower, "width:1920px") || strings.Contains(lower, "width: 1920px")
	hasHeight := strings.Contains(lower, "height:1080px") || strings.Contains(lower, "height: 1080px")
	return hasWidth && hasHeight
}

// injectPPTCanvasSizeIntoCSS inserts canvas dimensions into the CSS block.
func injectPPTCanvasSizeIntoCSS(css string) string {
	lower := strings.ToLower(css)
	styleClose := strings.Index(lower, "</style>")
	if styleClose < 0 {
		return css
	}
	canvasCSS := "\n.ppt-slide, section.ppt-slide {\n  width: 1920px;\n  height: 1080px;\n  overflow: hidden;\n  position: relative;\n  box-sizing: border-box;\n}\n"
	return css[:styleClose] + canvasCSS + css[styleClose:]
}

// appendPPTCSSToContext injects the pre-generated CSS block into the LLM
// context so the HTML generation call can reuse it without redesigning CSS.
func appendPPTCSSToContext(contextValue string, cssBlock string) string {
	if strings.TrimSpace(cssBlock) == "" {
		return contextValue
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(contextValue))
	b.WriteString("\n\nPRE_GENERATED_CSS\n")
	b.WriteString("以下 CSS 已由设计步骤生成，请直接复用，不要重新设计样式：\n")
	b.WriteString(cssBlock)
	b.WriteString("\n\nCSS_REUSE_RULES\n")
	b.WriteString("- 上方 <style> 块已包含完整的视觉设计，你的 HTML 输出必须复用其中的 CSS 类名。\n")
	b.WriteString("- 不要在 HTML 中再输出 <style> 块，直接使用已有的 CSS 类。\n")
	b.WriteString("- 如果需要额外样式，可以用内联 style 属性补充，但不要覆盖已有的画布尺寸。\n")
	b.WriteString("- 专注生成 <section> HTML 结构和内容，让每页布局有变化。\n")
	return strings.TrimSpace(b.String())
}

func renderStyledPPTSlides(plan pptOutlinePlan, theme pptStyleTheme) string {
	plan = sanitizePPTPlanVisibleText(plan)
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<style>
:root {
  --bg: %s; --surface: %s; --surface-soft: %s;
  --accent: %s; --accent-2: %s; --accent-soft: %s;
  --text: %s; --muted: %s; --heading: %s;
  --border: %s; --panel: %s;
}
`, theme.Background, theme.Surface, theme.Surface, theme.Primary, theme.Secondary, pptAccentSoft(theme), theme.Text, theme.Muted, theme.Heading, pptThemeBorder(theme), pptThemePanel(theme)))
	b.WriteString(`* { margin: 0; padding: 0; box-sizing: border-box; }
.ppt-slide {
  position: relative;
  width: 1920px;
  height: 1080px;
  overflow: hidden;
  background: var(--surface);
  color: var(--text);
  font-family: ` + theme.FontBody + `;
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
  background: ` + pptThemeCoverGradient(theme) + `;
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
.ppt-agenda { background: var(--bg); }
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
  background: linear-gradient(180deg, var(--accent-soft) 0%, var(--panel) 100%);
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
  background: var(--surface);
  border-left: 8px solid var(--accent);
}
.summary-actions {
  background: var(--panel);
  border: 1px solid var(--border);
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
.card-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 32px;
}
.content-card {
  min-height: 280px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-left: 6px solid var(--accent);
  border-radius: 8px;
  padding: 32px 36px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.content-card .card-title {
  color: var(--heading);
  font-size: 34px;
  font-weight: 800;
  line-height: 1.2;
}
.content-card .card-body {
  color: var(--text);
  font-size: 30px;
  line-height: 1.34;
}
.full-width-list {
  background: var(--surface);
  border-top: 6px solid var(--accent);
  padding: 12px 0 0;
}
.full-width-list ul { list-style: none; padding-left: 0; }
.full-width-list li {
  display: flex;
  align-items: flex-start;
  gap: 18px;
  padding: 20px 0;
  color: var(--text);
  font-size: 32px;
  line-height: 1.36;
  border-bottom: 1px solid var(--border);
}
.full-width-list li::before {
  content: "";
  width: 12px;
  height: 12px;
  margin-top: 16px;
  border-radius: 999px;
  background: var(--accent-2);
  flex-shrink: 0;
}
.full-width-list li:last-child { border-bottom: none; }
.comparison-layout {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 42px;
}
.comparison-col {
  min-height: 400px;
  border-radius: 8px;
  padding: 36px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.comparison-col.left {
  background: var(--accent-soft);
  border-left: 6px solid var(--accent);
}
.comparison-col.right {
  background: var(--panel);
  border-left: 6px solid var(--accent-2);
}
.comparison-col h3 {
  font-size: 34px;
  font-weight: 800;
  color: var(--heading);
  margin-bottom: 8px;
}
.comparison-col p {
  font-size: 30px;
  line-height: 1.34;
  color: var(--text);
}
.quote-block {
  background: linear-gradient(135deg, var(--accent-soft) 0%, var(--panel) 100%);
  border-left: 8px solid var(--accent);
  border-radius: 8px;
  padding: 48px 56px;
  min-height: 400px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 24px;
}
.quote-block .quote-text {
  font-size: 38px;
  line-height: 1.3;
  font-weight: 700;
  color: var(--heading);
}
.quote-block .quote-source {
  font-size: 28px;
  color: var(--muted);
  font-weight: 600;
}
.ppt-code-block {
  background: #1e293b;
  color: #e2e8f0;
  border-radius: 8px;
  padding: 28px 32px;
  margin-top: 18px;
  font-family: 'Cascadia Code', 'Fira Code', 'Consolas', 'Courier New', monospace;
  font-size: 24px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
  overflow-x: auto;
  border-left: 6px solid var(--accent-2);
}
.ppt-code-block code {
  font-family: inherit;
  color: inherit;
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
			writePPTSummarySlideBodyForPlan(&b, plan, slide)
		} else if i > 0 && len(slide.Bullets) > 0 {
			writePPTContentSlideBody(&b, slide, i)
		}
		b.WriteString(`<div class="slide-progress"><span style="width: `)
		b.WriteString(fmt.Sprintf("%d%%", pptSlideProgressPercent(i+1, len(plan.Slides))))
		b.WriteString(`"></span></div>`)
		b.WriteString("</section>\n")
	}
	return strings.TrimSpace(b.String())
}

// writePPTContentSlideBody renders a content slide using a layout pattern
// rotated based on slide index to add visual variety across the deck.
func writePPTContentSlideBody(b *strings.Builder, slide pptSlidePlan, index int) {
	bullets := slide.Bullets
	if len(bullets) == 0 {
		return
	}
	layout := pptSlideLayoutForIndex(index, slide)
	switch layout {
	case "card-grid":
		writePPTCardGridSlide(b, slide)
	case "full-width":
		writePPTFullWidthListSlide(b, slide)
	case "comparison":
		writePPTComparisonSlide(b, slide)
	case "quote":
		writePPTQuoteSlide(b, slide)
	case "code":
		writePPTCodeSlide(b, slide)
	default:
		writePPTTwoColumnSlide(b, slide)
	}
}

// pptSlideLayoutForIndex picks a layout pattern for a content slide.
// It rotates through patterns to ensure visual variety.
func pptSlideLayoutForIndex(index int, slide pptSlidePlan) string {
	title := strings.ToLower(slide.Title)
	// Code layout takes priority when bullets contain code blocks
	if slideHasCodeBlock(slide) {
		return "code"
	}
	if containsAnyFold(title, "对比", "比较", "vs", "compare", "区别", "差异") {
		return "comparison"
	}
	if containsAnyFold(title, "引用", "名言", "观点", "quote", "观点摘录") {
		return "quote"
	}
	if containsAnyFold(title, "列表", "要点", "清单", "list", "checklist") {
		return "full-width"
	}
	layouts := []string{"two-column", "card-grid", "full-width", "two-column", "comparison", "card-grid"}
	if index < 0 {
		index = 0
	}
	return layouts[index%len(layouts)]
}

// slideHasCodeBlock returns true if any bullet in the slide is a fenced code block.
func slideHasCodeBlock(slide pptSlidePlan) bool {
	for _, bullet := range slide.Bullets {
		if isPPTCodeBlockBullet(bullet) {
			return true
		}
	}
	return false
}

func writePPTTwoColumnSlide(b *strings.Builder, slide pptSlidePlan) {
	b.WriteString(`<div class="content-grid"><ul class="main-points">`)
	for _, bullet := range slide.Bullets {
		if isPPTCodeBlockBullet(bullet) {
			continue // code blocks rendered separately below
		}
		b.WriteString("<li>")
		b.WriteString(htmlEscape(pptExpandBullet(bullet, slide.Title)))
		b.WriteString("</li>")
	}
	b.WriteString("</ul>")
	tokens := pptInsightTokens(slide)
	if len(tokens) > 0 {
		b.WriteString(`<div class="insight-panel">`)
		for _, token := range tokens {
			b.WriteString(`<div class="insight-token">`)
			b.WriteString(htmlEscape(token))
			b.WriteString(`</div>`)
		}
		b.WriteString(`</div>`)
	}
	b.WriteString("</div>")
	// Append code blocks as separate <pre> elements after the main content
	writePPTCodeBlocks(b, slide.Bullets)
}

func writePPTCardGridSlide(b *strings.Builder, slide pptSlidePlan) {
	b.WriteString(`<div class="card-grid">`)
	cardIdx := 0
	for _, bullet := range slide.Bullets {
		if isPPTCodeBlockBullet(bullet) {
			continue // code blocks rendered separately
		}
		if cardIdx >= 4 {
			break
		}
		b.WriteString(`<div class="content-card"><div class="card-title">`)
		b.WriteString(htmlEscape(pptCardTitleFromBullet(bullet, cardIdx)))
		b.WriteString(`</div><div class="card-body">`)
		b.WriteString(htmlEscape(pptExpandBullet(bullet, slide.Title)))
		b.WriteString(`</div></div>`)
		cardIdx++
	}
	b.WriteString(`</div>`)
	writePPTCodeBlocks(b, slide.Bullets)
}

// pptCardTitleFromBullet derives a concise card title from the bullet content
// itself, rather than reusing the slide title. It extracts the first meaningful
// phrase (up to 10 characters) from the bullet text.
func pptCardTitleFromBullet(bullet string, index int) string {
	bullet = strings.TrimSpace(bullet)
	if bullet == "" {
		return fmt.Sprintf("要点 %d", index+1)
	}
	// Remove common prefixes
	bullet = strings.TrimLeft(bullet, "-*•0123456789. ")
	// If bullet contains a colon, take the part before the colon as title
	if idx := strings.IndexAny(bullet, ":："); idx > 2 && idx < 20 {
		title := strings.TrimSpace(bullet[:idx])
		if utf8RuneCount(title) >= 2 {
			return title
		}
	}
	// Take first N characters as title (up to 10 runes)
	runes := []rune(bullet)
	limit := 10
	if len(runes) < limit {
		limit = len(runes)
	}
	// Find a natural break point (space, comma, period) within limit
	title := string(runes[:limit])
	for i := limit - 1; i > 2; i-- {
		ch := runes[i]
		if ch == ' ' || ch == '，' || ch == '。' || ch == '、' || ch == '：' || ch == ':' {
			title = string(runes[:i])
			break
		}
	}
	title = strings.TrimSpace(title)
	if utf8RuneCount(title) < 2 {
		return fmt.Sprintf("要点 %d", index+1)
	}
	return title
}

// pptExpandBullet expands a short bullet into a fuller sentence for the
// fallback template. If the bullet is already a full sentence, it's returned
// as-is. If it's a short phrase, it's expanded with context from the slide title.
func pptExpandBullet(bullet, slideTitle string) string {
	bullet = strings.TrimSpace(bullet)
	if bullet == "" {
		return ""
	}
	// If already a long sentence (30+ chars), return as-is
	if utf8RuneCount(bullet) >= 30 {
		return bullet
	}
	// If it looks like a full sentence (ends with period), return as-is
	if strings.HasSuffix(bullet, "。") || strings.HasSuffix(bullet, ".") {
		return bullet
	}
	// Expand short phrase into a fuller sentence
	slideTitle = strings.TrimSpace(slideTitle)
	if slideTitle != "" {
		return fmt.Sprintf("%s：%s。", slideTitle, bullet)
	}
	return bullet + "。"
}

func writePPTFullWidthListSlide(b *strings.Builder, slide pptSlidePlan) {
	b.WriteString(`<div class="full-width-list"><ul>`)
	for _, bullet := range slide.Bullets {
		if isPPTCodeBlockBullet(bullet) {
			continue // code blocks rendered separately below
		}
		b.WriteString("<li>")
		b.WriteString(htmlEscape(pptExpandBullet(bullet, slide.Title)))
		b.WriteString("</li>")
	}
	b.WriteString("</ul></div>")
	writePPTCodeBlocks(b, slide.Bullets)
}

func writePPTComparisonSlide(b *strings.Builder, slide pptSlidePlan) {
	var textBullets []string
	for _, bullet := range slide.Bullets {
		if !isPPTCodeBlockBullet(bullet) {
			textBullets = append(textBullets, bullet)
		}
	}
	mid := len(textBullets) / 2
	if mid == 0 {
		mid = 1
	}
	left := textBullets[:mid]
	right := textBullets[mid:]
	if len(right) == 0 {
		right = left
	}
	leftTitle := pptComparisonTitleFromBullets(left, slide.Title, "左")
	rightTitle := pptComparisonTitleFromBullets(right, slide.Title, "右")
	b.WriteString(`<div class="comparison-layout"><div class="comparison-col left"><h3>`)
	b.WriteString(htmlEscape(leftTitle))
	b.WriteString(`</h3>`)
	for _, bullet := range left {
		b.WriteString("<p>")
		b.WriteString(htmlEscape(pptExpandBullet(bullet, slide.Title)))
		b.WriteString("</p>")
	}
	b.WriteString(`</div><div class="comparison-col right"><h3>`)
	b.WriteString(htmlEscape(rightTitle))
	b.WriteString(`</h3>`)
	for _, bullet := range right {
		b.WriteString("<p>")
		b.WriteString(htmlEscape(pptExpandBullet(bullet, slide.Title)))
		b.WriteString("</p>")
	}
	b.WriteString(`</div></div>`)
	writePPTCodeBlocks(b, slide.Bullets)
}

// pptComparisonTitleFromBullets derives a meaningful comparison column title
// from the bullets in that column, rather than just appending "· A" / "· B".
func pptComparisonTitleFromBullets(bullets []string, slideTitle, side string) string {
	if len(bullets) == 0 {
		if slideTitle != "" {
			return slideTitle
		}
		return side + "侧内容"
	}
	// Try to find a common theme from the first bullet
	first := strings.TrimSpace(bullets[0])
	if first == "" {
		return slideTitle
	}
	// If first bullet has a colon, use the part before colon
	if idx := strings.IndexAny(first, ":："); idx > 2 && idx < 20 {
		return strings.TrimSpace(first[:idx])
	}
	// Use first few words of the first bullet
	runes := []rune(first)
	limit := 8
	if len(runes) < limit {
		limit = len(runes)
	}
	return string(runes[:limit])
}

func writePPTQuoteSlide(b *strings.Builder, slide pptSlidePlan) {
	quote := slide.Title
	if len(slide.Bullets) > 0 {
		quote = pptExpandBullet(slide.Bullets[0], slide.Title)
	}
	source := ""
	if len(slide.Bullets) > 1 {
		source = slide.Bullets[1]
	}
	b.WriteString(`<div class="quote-block"><div class="quote-text">`)
	b.WriteString(htmlEscape(quote))
	b.WriteString(`</div>`)
	if source != "" {
		b.WriteString(`<div class="quote-source">`)
		b.WriteString(htmlEscape(source))
		b.WriteString(`</div>`)
	}
	b.WriteString(`</div>`)
	writePPTCodeBlocks(b, slide.Bullets)
}

// writePPTCodeBlocks renders code block bullets as <pre><code> elements
// with the ppt-code-block class. Non-code-block bullets are skipped.
func writePPTCodeBlocks(b *strings.Builder, bullets []string) {
	for _, bullet := range bullets {
		if !isPPTCodeBlockBullet(bullet) {
			continue
		}
		code, ok := stripFencedCodeBlockForPPT(bullet)
		if !ok {
			continue
		}
		b.WriteString(`<pre class="ppt-code-block"><code>`)
		b.WriteString(htmlEscape(code))
		b.WriteString(`</code></pre>`)
	}
}

// writePPTCodeSlide renders a slide optimized for code display. It shows
// explanatory text bullets first, then renders all code blocks as <pre><code>
// elements below. This layout is used when the slide contains fenced code blocks.
func writePPTCodeSlide(b *strings.Builder, slide pptSlidePlan) {
	// Render non-code bullets as explanatory text
	var textBullets []string
	for _, bullet := range slide.Bullets {
		if !isPPTCodeBlockBullet(bullet) {
			textBullets = append(textBullets, bullet)
		}
	}
	if len(textBullets) > 0 {
		b.WriteString(`<div class="full-width-list"><ul>`)
		for _, bullet := range textBullets {
			b.WriteString("<li>")
			b.WriteString(htmlEscape(pptExpandBullet(bullet, slide.Title)))
			b.WriteString("</li>")
		}
		b.WriteString(`</ul></div>`)
	}
	// Render code blocks as <pre><code> elements
	writePPTCodeBlocks(b, slide.Bullets)
}

func pptCardTitle(slideTitle string, index int) string {
	slideTitle = strings.TrimSpace(slideTitle)
	if slideTitle == "" {
		return fmt.Sprintf("要点 %d", index+1)
	}
	return slideTitle
}

func pptComparisonLeftTitle(slideTitle string) string {
	slideTitle = strings.TrimSpace(slideTitle)
	if slideTitle == "" {
		return "特征 A"
	}
	return slideTitle + " · A"
}

func pptComparisonRightTitle(slideTitle string) string {
	slideTitle = strings.TrimSpace(slideTitle)
	if slideTitle == "" {
		return "特征 B"
	}
	return slideTitle + " · B"
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

	// Preserve code blocks: if the value contains a fenced code block (```...```),
	// keep the fences intact so that downstream code (isPPTCodeBlockBullet,
	// slideHasCodeBlock, writePPTCodeBlocks) can still identify and correctly
	// render it. Only clean the text inside the fences (e.g. &nbsp;).
	// The &nbsp; replacement already happened above; return the code block
	// with its fences preserved.
	if isPPTCodeBlockBullet(value) {
		return value
	}

	// Single-line code fences (```inline```) without a newline are also code
	// content — preserve them rather than stripping to plain text.
	if cleaned, ok := stripFencedCodeBlockForPPT(value); ok {
		return cleaned
	}

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

// stripFencedCodeBlockForPPT detects a fenced code block in value (```lang\n...\n```)
// and returns the inner code with the language label removed, preserving line breaks
// and indentation. Returns ("", false) if value does not contain a code block.
func stripFencedCodeBlockForPPT(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "```") {
		return "", false
	}
	// Find the language tag line end
	firstNewline := strings.Index(trimmed, "\n")
	if firstNewline < 0 {
		// Single-line code fence like ```x``` — treat as code
		inner := strings.TrimPrefix(trimmed, "```")
		inner = strings.TrimSuffix(inner, "```")
		inner = strings.TrimSpace(inner)
		if inner == "" {
			return "", false
		}
		return inner, true
	}
	// Remove opening ```lang\n
	inner := trimmed[firstNewline+1:]
	// Remove closing \n```
	if strings.HasSuffix(inner, "\n```") {
		inner = inner[:len(inner)-4]
	} else if strings.HasSuffix(inner, "```") {
		inner = inner[:len(inner)-3]
	}
	inner = strings.TrimRight(inner, "\n")
	if strings.TrimSpace(inner) == "" {
		return "", false
	}
	return inner, true
}

// isPPTCodeBlockBullet returns true if the bullet text originated from a
// fenced code block (```...```).
func isPPTCodeBlockBullet(bullet string) bool {
	trimmed := strings.TrimSpace(bullet)
	return strings.HasPrefix(trimmed, "```") && strings.Contains(trimmed, "\n")
}

func writePPTCoverSlide(b *strings.Builder, plan pptOutlinePlan, slide pptSlidePlan, index int) {
	b.WriteString(`<div class="cover-meta"><span class="section-number">`)
	b.WriteString(fmt.Sprintf("%02d", index+1))
	b.WriteString(`</span><span>演示文稿</span></div>`)
	b.WriteString("<h1>")
	b.WriteString(htmlEscape(firstNonEmpty(plan.Title, slide.Title, "演示文稿")))
	b.WriteString("</h1>")
	b.WriteString(`<p class="cover-subtitle">`)
	b.WriteString(htmlEscape(pptCoverSubtitle(plan, slide)))
	b.WriteString(`</p>`)
	tags := uniqueNonEmpty(append([]string{}, slide.Bullets...))
	tags = append(tags, pptContentSlideTitles(plan)...)
	if len(tags) == 0 && strings.TrimSpace(slide.Title) != "" && !isCoverSlideTitle(slide.Title) {
		tags = append(tags, slide.Title)
	}
	tags = uniqueNonEmpty(tags)
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

func pptCoverSubtitle(plan pptOutlinePlan, slide pptSlidePlan) string {
	candidates := uniqueNonEmpty(append([]string{}, slide.Bullets...))
	if len(candidates) == 0 {
		candidates = pptContentSlideTitles(plan)
	}
	title := firstNonEmpty(plan.Title, slide.Title, "本次主题")
	if len(candidates) == 0 {
		return fmt.Sprintf("围绕 %s 梳理关键内容", title)
	}
	if len(candidates) > 3 {
		candidates = candidates[:3]
	}
	return fmt.Sprintf("围绕 %s，聚焦 %s", title, strings.Join(candidates, "、"))
}

func pptContentSlideTitles(plan pptOutlinePlan) []string {
	var titles []string
	for i, slide := range plan.Slides {
		title := strings.TrimSpace(slide.Title)
		if title == "" {
			continue
		}
		if i == 0 || i == 1 || i == len(plan.Slides)-1 || isCoverSlideTitle(title) || isAgendaSlideTitle(title) || isEndingSlideTitle(title) {
			continue
		}
		titles = append(titles, title)
	}
	return uniqueNonEmpty(titles)
}

func writePPTSummarySlideBody(b *strings.Builder, slide pptSlidePlan) {
	writePPTSummarySlideBodyForPlan(b, pptOutlinePlan{Slides: []pptSlidePlan{slide}}, slide)
}

func writePPTSummarySlideBodyForPlan(b *strings.Builder, plan pptOutlinePlan, slide pptSlidePlan) {
	primary, actions := pptSummaryContent(plan, slide)
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

func pptSummaryContent(plan pptOutlinePlan, slide pptSlidePlan) (string, []string) {
	if len(slide.Bullets) > 0 {
		return slide.Bullets[0], append([]string{}, slide.Bullets[1:]...)
	}
	titles := pptContentSlideTitles(plan)
	if len(titles) == 0 {
		title := firstNonEmpty(plan.Title, slide.Title, "本次主题")
		return fmt.Sprintf("回顾 %s 的关键内容", title), []string{fmt.Sprintf("继续聚焦 %s", title)}
	}
	focus := titles
	if len(focus) > 3 {
		focus = focus[:3]
	}
	primary := fmt.Sprintf("本次内容围绕 %s 展开", strings.Join(focus, "、"))
	actions := make([]string, 0, len(focus))
	for _, title := range focus {
		actions = append(actions, fmt.Sprintf("继续聚焦 %s", title))
	}
	return primary, actions
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
	text := strings.ToLower(strings.Join(strings.Fields(stripPPTVisibleText(content)), " "))
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

func pptContainsVisiblePlaceholderText(content string) bool {
	text := strings.ToLower(strings.Join(strings.Fields(stripPPTVisibleText(content)), " "))
	if text == "" {
		return false
	}
	placeholderTokens := []string{
		"这里填写",
		"请填写",
		"占位",
		"待补充",
		"待完善",
		"todo",
		"placeholder",
		"insert your",
		"fill in",
		"lorem ipsum",
		// Internal plan labels that must never appear as visible slide text
		"页面目的",
		"核心论点",
		"关键要点",
		"可用证据",
		"内容展开",
		"本页用于",
		"slide purpose",
		"page purpose",
		"writing brief",
		"source-topic",
		"source topic",
		"source_topic",
		// Internal plan Purpose values that leak as visible text
		"礼貌地结束",
		"引导思考",
		"建立演示主题",
		"呈现演示路径",
		"收束核心结论",
		"建立学习主题",
		"呈现学习路径",
		"收束学习结论",
		"说明为什么学习",
		"梳理概念之间的关系",
		"连接材料和实际",
		"提示边界和误区",
		"建立演示主题和受众预期",
		"收束核心结论并给出下一步",
		"展开材料开头的核心背景",
		"收束本部分材料并提炼结论",
		"呈现检索或引用资料中的真实要点",
		"说明材料背景和演示目标",
		"连接材料和实际使用场景",
		"this slide introduces",
		"agenda items are organized",
		"treat each slide entry",
		"the outline bullets are source material",
		"keep every planned slide title",
		"design every section as a real",
		"finish all planned sections",
	}
	for _, token := range placeholderTokens {
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
	text := strings.ToLower(strings.Join(strings.Fields(stripPPTVisibleText(content)), " "))
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
		"lorem ipsum",
		"sample text",
		"example content",
		"replace with",
		"your content here",
		"your title here",
		"click to edit",
		"double click to",
		"版权所有", "未经许可", "保留所有权利",
		"all rights reserved",
	}
	source := pptAllowedSourceText(input)
	for _, token := range unrelatedTokens {
		if strings.Contains(text, token) && !strings.Contains(source, token) {
			return true
		}
	}
	// Also check for reference metadata leakage
	if pptContainsReferenceMetadata(text) {
		return true
	}
	return false
}

// pptContainsReferenceMetadata detects when reference metadata (document
// labels, chapter paths, source citations) leaks into the visible slide
// content. The LLM should integrate reference content into the slides, not
// output the reference labels/paths as visible text.
func pptContainsReferenceMetadata(text string) bool {
	refTokens := []string{
		"文档列表",
		"文档介绍",
		"文档概述",
		"文档目录",
		"文档内容",
		"专题一", "专题二", "专题三", "专题四", "专题五", "专题六",
		"专题七", "专题八", "专题九", "专题十",
		"第一章", "第二章", "第三章", "第四章", "第五章", "第六章",
		"第七章", "第八章", "第九章", "第十章",
		"第1章", "第2章", "第3章", "第4章", "第5章",
		"第一节", "第二节", "第三节", "第四节", "第五节",
		"【重点知识",
		"【知识要点",
		"【核心知识",
		"【基础知识",
		"【考点分析",
		"【考点",
		"local references:",
		"web results:",
		"web summary:",
		"original markdown:",
		"input markdown",
		"source-1", "source-2", "source-3",
	}
	for _, token := range refTokens {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

// pptContainsRepetitiveText detects when the LLM output has excessive
// repetition — the same phrase appearing many times, or a single word
// dominating the visible text. This catches degenerate outputs where the
// model gets stuck repeating a phrase.
func pptContainsRepetitiveText(content string) bool {
	text := strings.Join(strings.Fields(stripPPTVisibleText(content)), " ")
	if len([]rune(text)) < 40 {
		return false
	}
	// Split into words and count frequencies.
	words := strings.Fields(strings.ToLower(text))
	if len(words) < 8 {
		return false
	}
	freq := make(map[string]int, len(words))
	for _, w := range words {
		// Skip very short words (articles, prepositions) for the ratio check.
		if len([]rune(w)) < 2 {
			continue
		}
		freq[w]++
	}
	// If any single word appears more than 12 times and accounts for more
	// than 15% of all words, the output is likely degenerate.
	for _, count := range freq {
		if count >= 12 && float64(count)/float64(len(words)) > 0.15 {
			return true
		}
	}
	// Check for repeated n-grams (4-word phrases).
	phraseFreq := make(map[string]int)
	for i := 0; i+4 <= len(words); i++ {
		phrase := strings.Join(words[i:i+4], " ")
		phraseFreq[phrase]++
	}
	for _, count := range phraseFreq {
		if count >= 4 {
			return true
		}
	}
	// Check for repeated sentences (split by punctuation).
	sentences := splitPPTSentences(text)
	if len(sentences) >= 6 {
		seen := make(map[string]int, len(sentences))
		for _, s := range sentences {
			s = strings.ToLower(strings.TrimSpace(s))
			if len([]rune(s)) < 6 {
				continue
			}
			seen[s]++
		}
		for _, count := range seen {
			if count >= 3 {
				return true
			}
		}
	}
	return false
}

// splitPPTSentences splits text into sentences using common punctuation.
func splitPPTSentences(text string) []string {
	text = strings.ReplaceAll(text, "!", ".")
	text = strings.ReplaceAll(text, "?", ".")
	text = strings.ReplaceAll(text, "！", "。")
	text = strings.ReplaceAll(text, "？", "。")
	text = strings.ReplaceAll(text, "。", ".")
	text = strings.ReplaceAll(text, "；", ";")
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == '.' || r == ';' || r == '\n'
	})
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// pptContainsExcessivePromptWords detects when the LLM output contains too
// many meta/prompt-like words that should not appear in a polished PPT.
// Unlike pptContainsVisiblePlaceholderText which checks for exact placeholder
// tokens, this checks for an accumulation of prompt-instruction language.
func pptContainsExcessivePromptWords(content string) bool {
	text := strings.ToLower(strings.Join(strings.Fields(stripPPTVisibleText(content)), " "))
	if text == "" {
		return false
	}
	promptWords := []string{
		"作为", "你需要", "请生成", "请输出", "请确保", "请注意", "请遵循",
		"根据上下文", "根据要求", "按照要求", "按照计划",
		"以下是", "如下所示", "如下",
		"as an ai", "as a language model", "i cannot", "i'm sorry",
		"i am unable", "i don't have", "as requested", "here is",
		"here are", "below is", "based on the context", "according to",
		"please note", "please ensure", "you should", "the output should",
		"生成内容", "生成结果", "输出内容", "输出结果",
		"演示文稿将", "幻灯片将", "本演示", "本次生成",
		"注意事项", "说明事项", "备注事项",
		"步骤一", "步骤二", "步骤三", "步骤四",
		"第一部分", "第二部分", "第三部分", "第四部分",
	}
	hits := 0
	for _, word := range promptWords {
		count := strings.Count(text, word)
		if count > 0 {
			hits += count
		}
	}
	// If prompt-instruction language appears 5+ times, the output is
	// contaminated with meta-text rather than actual slide content.
	return hits >= 5
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

func pptNeedsHTMLQualityRepair(content string) bool {
	lower := strings.ToLower(content)
	if !strings.Contains(lower, "<section") {
		return true
	}
	if !strings.Contains(lower, "width:1920px") && !strings.Contains(lower, "width: 1920px") {
		return true
	}
	if !strings.Contains(lower, "height:1080px") && !strings.Contains(lower, "height: 1080px") {
		return true
	}
	// data-ppt-slide attribute is patched by polishPPTHTML, so don't trigger full fallback for it
	weakTokens := []string{
		"font-size:1rem",
		"font-size: 1rem",
		"font-size:.",
		"font-size: .",
		"max-width:1100px",
		"max-width: 1100px",
		"max-width:960px",
		"max-width: 960px",
		"height:100vh",
		"height: 100vh",
		"width:100%",
		"width: 100%",
	}
	for _, token := range weakTokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

// pptHasSparseSlides detects when one or more <section> blocks have very
// little visible text — i.e. the slide is mostly empty. This catches the
// common failure where the LLM outputs a title but no body content.
func pptHasSparseSlides(content string) bool {
	sections := pptExtractSections(content)
	if len(sections) == 0 {
		return false
	}
	sparseCount := 0
	for _, sec := range sections {
		text := strings.TrimSpace(stripPPTVisibleText(sec))
		text = strings.Join(strings.Fields(text), " ")
		charCount := len([]rune(text))
		// A real content slide should have at least 80 characters of visible
		// text (title + body). Cover/agenda slides can be shorter.
		if charCount < 80 {
			sparseCount++
		}
	}
	// If more than half the slides are sparse, or any 2+ slides are sparse,
	// the output quality is too low.
	return sparseCount >= 2 || (len(sections) > 0 && sparseCount == len(sections))
}

// pptExtractSections splits HTML content into individual <section> blocks.
func pptExtractSections(content string) []string {
	lower := strings.ToLower(content)
	var sections []string
	searchFrom := 0
	for {
		start := strings.Index(lower[searchFrom:], "<section")
		if start < 0 {
			break
		}
		start += searchFrom
		end := strings.Index(lower[start:], "</section>")
		if end < 0 {
			// Take the rest if no closing tag
			sections = append(sections, content[start:])
			break
		}
		end += start + len("</section>")
		sections = append(sections, content[start:end])
		searchFrom = end
	}
	return sections
}

// pptHasDuplicatedSlideTitles detects when the same title text appears
// multiple times within a single <section>. This catches the common failure
// where the LLM uses the page title as the heading for every card/content
// block, e.g. a "感谢观看" slide with 5 cards all titled "感谢观看".
func pptHasDuplicatedSlideTitles(content string) bool {
	sections := pptExtractSections(content)
	if len(sections) == 0 {
		return false
	}
	for _, sec := range sections {
		titles := pptExtractSlideTitles(sec)
		if len(titles) < 3 {
			continue
		}
		// Count title frequency
		freq := make(map[string]int)
		for _, t := range titles {
			key := strings.ToLower(strings.TrimSpace(t))
			if key == "" {
				continue
			}
			freq[key]++
		}
		// If any single title appears 3+ times in one section, it's a
		// duplication problem.
		for _, count := range freq {
			if count >= 3 {
				return true
			}
		}
	}
	return false
}

// pptExtractSlideTitles extracts text content from h1-h3 and card-title
// pptHasMismatchedCardContent detects when card-title and card-body content
// are logically inconsistent — the title doesn't relate to the body text.
// This catches the common failure where the LLM generates a card title that
// has no semantic connection to the body content below it.
func pptHasMismatchedCardContent(content string) bool {
	sections := pptExtractSections(content)
	if len(sections) == 0 {
		return false
	}
	mismatchCount := 0
	for _, sec := range sections {
		cards := pptExtractContentCards(sec)
		for _, card := range cards {
			if !pptCardTitleMatchesBody(card.title, card.body) {
				mismatchCount++
			}
		}
	}
	// If 3+ cards have mismatched title/body, the structure is chaotic
	return mismatchCount >= 3
}

type pptCardPair struct {
	title string
	body  string
}

// pptExtractContentCards extracts card-title and card-body pairs from a section.
func pptExtractContentCards(section string) []pptCardPair {
	var cards []pptCardPair
	lower := strings.ToLower(section)
	searchFrom := 0
	for {
		idx := strings.Index(lower[searchFrom:], "content-card")
		if idx < 0 {
			break
		}
		idx += searchFrom
		// Find the content-card div scope
		cardStart := idx
		// Find end of content-card opening tag
		gtIdx := strings.Index(lower[cardStart:], ">")
		if gtIdx < 0 {
			break
		}
		cardContentStart := cardStart + gtIdx + 1
		// Find matching </div> for content-card (simplified: find next </div></div>)
		// Look for card-title within this card
		titleStart := strings.Index(lower[cardContentStart:], "card-title")
		if titleStart < 0 {
			searchFrom = cardContentStart
			continue
		}
		titleStart += cardContentStart
		titleGt := strings.Index(lower[titleStart:], ">")
		if titleGt < 0 {
			searchFrom = cardContentStart
			continue
		}
		titleTextStart := titleStart + titleGt + 1
		titleEnd := strings.Index(lower[titleTextStart:], "<")
		if titleEnd < 0 {
			searchFrom = cardContentStart
			continue
		}
		title := strings.TrimSpace(stripPPTVisibleText(section[titleTextStart : titleTextStart+titleEnd]))

		// Find card-body
		bodyStart := strings.Index(lower[titleTextStart+titleEnd:], "card-body")
		if bodyStart < 0 {
			searchFrom = titleTextStart + titleEnd
			continue
		}
		bodyStart += titleTextStart + titleEnd
		bodyGt := strings.Index(lower[bodyStart:], ">")
		if bodyGt < 0 {
			searchFrom = bodyStart
			continue
		}
		bodyTextStart := bodyStart + bodyGt + 1
		bodyEnd := strings.Index(lower[bodyTextStart:], "<")
		if bodyEnd < 0 {
			searchFrom = bodyTextStart
			continue
		}
		body := strings.TrimSpace(stripPPTVisibleText(section[bodyTextStart : bodyTextStart+bodyEnd]))

		if title != "" && body != "" {
			cards = append(cards, pptCardPair{title: title, body: body})
		}
		searchFrom = bodyTextStart + bodyEnd
	}
	return cards
}

// pptCardTitleMatchesBody checks if a card title is semantically related to
// its body content. Returns true if they appear related, false if they seem
// mismatched.
func pptCardTitleMatchesBody(title, body string) bool {
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	if title == "" || body == "" {
		return true // Can't determine, don't flag
	}
	// If title and body are identical, it's a duplication problem (caught elsewhere)
	if strings.ToLower(title) == strings.ToLower(body) {
		return false
	}
	// If title is very short (1-2 chars) and body is long, likely mismatched
	if utf8RuneCount(title) <= 2 && utf8RuneCount(body) > 20 {
		return false
	}
	// Check for keyword overlap: extract significant words from title and
	// see if any appear in body
	titleWords := extractSignificantWords(title)
	if len(titleWords) == 0 {
		return true // Can't determine
	}
	bodyLower := strings.ToLower(body)
	overlap := 0
	for _, word := range titleWords {
		if strings.Contains(bodyLower, strings.ToLower(word)) {
			overlap++
		}
	}
	// If none of the title's significant words appear in body, likely mismatched
	if overlap == 0 && utf8RuneCount(body) > 15 {
		return false
	}
	return true
}

// extractSignificantWords extracts meaningful words (length >= 2) from text,
// filtering out common stop words.
func extractSignificantWords(text string) []string {
	// Remove punctuation
	text = strings.Map(func(r rune) rune {
		if r == '，' || r == '。' || r == '、' || r == '：' ||
			r == '（' || r == '）' || r == '(' || r == ')' ||
			r == '"' || r == '\'' || r == ' ' {
			return ' '
		}
		return r
	}, text)
	fields := strings.Fields(text)
	var words []string
	stopWords := map[string]bool{
		"的": true, "了": true, "是": true, "在": true, "和": true,
		"与": true, "或": true, "及": true, "等": true, "为": true,
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"of": true, "to": true, "in": true, "on": true, "for": true,
	}
	for _, f := range fields {
		if utf8RuneCount(f) >= 2 && !stopWords[strings.ToLower(f)] {
			words = append(words, f)
		}
	}
	return words
}

func utf8RuneCount(s string) int {
	return len([]rune(s))
}

// elements within a section, to detect title duplication.
func pptExtractSlideTitles(section string) []string {
	var titles []string
	lower := strings.ToLower(section)
	// Extract from h1, h2, h3 tags
	for _, tag := range []string{"h1", "h2", "h3"} {
		titles = append(titles, extractTagText(section, lower, tag)...)
	}
	// Extract from card-title class
	titles = append(titles, extractClassText(section, lower, "card-title")...)
	// Extract from dir-item class
	titles = append(titles, extractClassText(section, lower, "dir-item")...)
	return titles
}

// extractTagText extracts text content from all occurrences of a given HTML tag.
func extractTagText(content, lowerContent, tag string) []string {
	var texts []string
	openTag := "<" + tag
	closeTag := "</" + tag + ">"
	searchFrom := 0
	for {
		idx := strings.Index(lowerContent[searchFrom:], openTag)
		if idx < 0 {
			break
		}
		idx += searchFrom
		// Find end of opening tag (handle attributes)
		gtIdx := strings.Index(lowerContent[idx:], ">")
		if gtIdx < 0 {
			break
		}
		textStart := idx + gtIdx + 1
		endIdx := strings.Index(lowerContent[textStart:], closeTag)
		if endIdx < 0 {
			break
		}
		textEnd := textStart + endIdx
		text := strings.TrimSpace(stripPPTVisibleText(content[textStart:textEnd]))
		if text != "" {
			texts = append(texts, text)
		}
		searchFrom = textEnd + len(closeTag)
	}
	return texts
}

// extractClassText extracts text content from elements with a given class.
func extractClassText(content, lowerContent, className string) []string {
	var texts []string
	searchFrom := 0
	for {
		idx := strings.Index(lowerContent[searchFrom:], className)
		if idx < 0 {
			break
		}
		idx += searchFrom
		// Find the enclosing tag start
		tagStart := strings.LastIndex(lowerContent[:idx], "<")
		if tagStart < 0 {
			searchFrom = idx + len(className)
			continue
		}
		// Find end of opening tag
		gtIdx := strings.Index(lowerContent[idx:], ">")
		if gtIdx < 0 {
			break
		}
		textStart := idx + gtIdx + 1
		// Find matching closing tag
		closeIdx := strings.Index(lowerContent[textStart:], "<")
		if closeIdx < 0 {
			break
		}
		textEnd := textStart + closeIdx
		text := strings.TrimSpace(stripPPTVisibleText(content[textStart:textEnd]))
		if text != "" {
			texts = append(texts, text)
		}
		searchFrom = textEnd
	}
	return texts
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

// dynamicMindmapBranches 根据笔记内容智能选择思维导图分支。
// 如果笔记有明确的章节结构，用章节标题作为分支；
// 否则按知识点聚类生成3-5个分支。始终保留"总结"分支。
func dynamicMindmapBranches(analysis learningContentAnalysis) []mindmapBranchPlan {
	var branches []mindmapBranchPlan

	// 如果笔记有章节结构，直接用章节标题作为分支
	if len(analysis.Sections) >= 2 {
		for _, section := range analysis.Sections {
			title := strings.TrimSpace(section.Title)
			if title == "" {
				continue
			}
			branch := mindmapBranchPlan{Title: title}
			for _, point := range section.Points {
				point = strings.TrimSpace(point)
				if point == "" {
					continue
				}
				branch.Nodes = append(branch.Nodes, newMindmapNode(
					point,
					mindmapNodeDetailFromEvidence(point, analysis),
				))
			}
			if len(branch.Nodes) == 0 {
				branch.Nodes = append(branch.Nodes, newMindmapNode(
					supplementBullet(title, 1),
					mindmapNodeDetailFromEvidence(title, analysis),
				))
			}
			branches = append(branches, branch)
		}
	} else {
		// 扁平结构：按知识点类型分类
		if len(analysis.KeyConcepts) > 0 {
			branch := mindmapBranchPlan{Title: "核心概念"}
			for _, concept := range analysis.KeyConcepts {
				branch.Nodes = append(branch.Nodes, newMindmapNode(
					concept,
					mindmapNodeDetailFromEvidence(concept, analysis),
				))
			}
			branches = append(branches, branch)
		}
		if len(analysis.Processes) > 0 {
			branch := mindmapBranchPlan{Title: "原理与过程"}
			for _, proc := range analysis.Processes {
				branch.Nodes = append(branch.Nodes, newMindmapNode(
					proc,
					mindmapNodeDetailFromEvidence(proc, analysis),
				))
			}
			branches = append(branches, branch)
		}
		if len(analysis.Examples) > 0 {
			branch := mindmapBranchPlan{Title: "应用与案例"}
			for _, example := range analysis.Examples {
				branch.Nodes = append(branch.Nodes, newMindmapNode(
					example,
					mindmapNodeDetailFromEvidence(example, analysis),
				))
			}
			branches = append(branches, branch)
		}
		if len(branches) == 0 {
			// 极端稀疏：创建一个通用分支
			branch := mindmapBranchPlan{Title: analysis.Topic}
			for _, concept := range analysis.KeyConcepts {
				branch.Nodes = append(branch.Nodes, newMindmapNode(
					concept,
					mindmapNodeDetailFromEvidence(concept, analysis),
				))
			}
			if len(branch.Nodes) == 0 {
				branch.Nodes = append(branch.Nodes, newMindmapNode(
					fmt.Sprintf("围绕“%s”的关键要点", analysis.Topic),
					"结合笔记内容梳理核心知识点。",
				))
			}
			branches = append(branches, branch)
		}
	}

	// 始终保留总结分支
	branches = append(branches, mindmapBranchPlan{
		Title: "总结",
		Nodes: []mindmapNodePlan{
			newMindmapNode(
				fmt.Sprintf("围绕“%s”形成可复习的结构。", analysis.Topic),
				"按概念、机制、过程、应用和误区回顾学习路径。",
			),
		},
	})

	// 确保至少3个分支（含总结）
	if len(branches) < 3 {
		branchTitle := "补充内容"
		if strings.TrimSpace(analysis.Topic) != "" {
			branchTitle = analysis.Topic
		}
		for len(branches) < 3 {
			branches = append(branches, mindmapBranchPlan{
				Title: branchTitle,
				Nodes: []mindmapNodePlan{
					newMindmapNode(
						supplementBullet(branchTitle, len(branches)+1),
						"该节点为解释补充，用于补足学习结构。",
					),
				},
			})
		}
	}

	// 限制分支数量在8以内（含总结）
	if len(branches) > 8 {
		// 保留前7个分支和最后的总结分支
		branches = append(branches[:7], branches[len(branches)-1])
	}

	return branches
}

func planMindmap(analysis learningContentAnalysis) mindmapPlan {
	plan := mindmapPlan{Title: analysis.Topic}
	branches := dynamicMindmapBranches(analysis)

	// 确保每个分支至少有 minNodes 个节点
	minNodes := 3
	if analysis.Sparse {
		minNodes = 4
	}
	for i := range branches {
		branch := &branches[i]
		for len(branch.Nodes) < minNodes {
			branch.Nodes = append(branch.Nodes, newMindmapNode(
				supplementBullet(branch.Title, len(branch.Nodes)+1),
				mindmapNodeDetailFromEvidence(branch.Title, analysis),
				mindmapBranchExpansionDetail(branch.Title, analysis),
			))
			branch.Nodes = uniqueMindmapNodes(branch.Nodes)
		}
		branch.Nodes = uniqueMindmapNodes(branch.Nodes)
	}

	plan.Branches = branches
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

// mindmapNodeDetailFromEvidence 从笔记证据中提取具体内容作为节点细节，
// 替代原有的模板化描述。
func mindmapNodeDetailFromEvidence(value string, analysis learningContentAnalysis) string {
	// 如果值包含解释补充标记，返回通用提示
	if strings.Contains(value, "解释补充") {
		return "该节点为解释补充，用于补足学习结构。"
	}

	// 从证据中找与 value 最相关的具体内容
	valueKeywords := extractSignificantWords(value)
	for _, ev := range analysis.Evidence {
		evKeywords := extractSignificantWords(ev.Text)
		overlap := 0
		for _, kw := range valueKeywords {
			for _, ekw := range evKeywords {
				if strings.EqualFold(kw, ekw) {
					overlap++
					break
				}
			}
		}
		if overlap >= 2 {
			return summarizeLine(ev.Text, 90)
		}
	}

	// 如果没有直接匹配的证据，返回通用但更有针对性的描述
	switch {
	case containsAnyFold(value, "定义", "概念", "是什么", "含义"):
		return "明确该概念的精确定义、适用范围和与相关概念的区别。"
	case containsAnyFold(value, "原理", "机制", "原因", "为什么"):
		return "理解该原理成立的条件、因果链条和关键变量。"
	case containsAnyFold(value, "步骤", "流程", "过程", "方法"):
		return "按顺序理解每个步骤的输入、变化和产出。"
	case containsAnyFold(value, "应用", "例子", "场景", "案例"):
		return "结合具体场景判断该知识点如何迁移使用。"
	default:
		return fmt.Sprintf("围绕“%s”展开具体内容和关键要点。", strings.TrimSpace(value))
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
	return supplementBullet(branchTitle, position)
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
	if len([]rune(strings.ReplaceAll(trimmed, "#", ""))) < 20 {
		return true
	}
	// 至少3个 ## 分支
	if strings.Count(trimmed, "\n## ") < 3 {
		return true
	}
	// 至少有 ### 节点层级
	if !strings.Contains(trimmed, "\n### ") {
		return true
	}
	return false
}

func supplementBullet(title string, detail int) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Sprintf("补充要点 %d：结合材料进一步分析该主题的关键内容。", detail)
	}
	// Generate concrete, knowledge-driven supplements rather than vague boilerplate.
	// Each template provides a specific angle for expanding the topic with real content.
	templates := []string{
		fmt.Sprintf("%s的核心定义：用一句话精确概括%s是什么，区分它与其他相近概念的本质差异。", title, title),
		fmt.Sprintf("%s的运作原理：解释%s背后的因果机制或数学/逻辑基础，说明为什么它这样工作而非那样工作。", title, title),
		fmt.Sprintf("%s的关键特征：列出%s的3个核心特征，每个特征给出一个具体例子或量化数据支撑。", title, title),
		fmt.Sprintf("%s的应用场景：描述%s在真实场景中的典型用法，给出一个具体的操作步骤或数值案例。", title, title),
		fmt.Sprintf("%s的常见误区：指出学习%s时最容易犯的2-3个错误，说明正确理解应该是什么。", title, title),
		fmt.Sprintf("%s与其他概念的关系：说明%s在整体知识体系中的位置，它依赖什么前置知识，又是什么后续知识的基础。", title, title),
	}
	idx := (detail - 1) % len(templates)
	return templates[idx]
}

func hasSupplementBullet(values []string) bool {
	for _, value := range values {
		if strings.Contains(value, "解释补充") || strings.Contains(value, "补充要点") {
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
	return &quizGenerationAgent{
		baseGenerationAgent: baseGenerationAgent{
			name:      "quiz",
			typ:       GenerationTypeQuiz,
			model:     model,
			validator: validateQuizContent,
			fallback:  fallbackQuizContent,
		},
	}
}

func newNoteAgent(model GenerationModel) generationAgent {
	return &noteGenerationAgent{
		baseGenerationAgent: baseGenerationAgent{
			name:      "note",
			typ:       GenerationTypeNote,
			model:     model,
			validator: validateNoteContent,
			fallback:  fallbackNoteContent,
		},
	}
}

func fallbackNoteContent(input generationAgentInput) string {
	analysis := analyzeLearningContent(input)
	return renderNote(expandNoteContent(planNoteOutline(analysis), analysis))
}

func fallbackQuizContent(input generationAgentInput) string {
	analysis := analyzeLearningContent(input)
	return renderQuiz(expandQuizContent(planQuizQuestions(analysis), analysis))
}

func requiredNoteSections() []string {
	return []string{"摘要", "关键概念", "原理与机制", "过程与步骤", "应用场景", "易错点", "总结"}
}

func planNoteOutline(analysis learningContentAnalysis) noteOutlinePlan {
	plan := noteOutlinePlan{Title: analysis.Topic}
	summaryParts := append([]string{}, analysis.KeyConcepts...)
	if len(analysis.Processes) > 0 {
		summaryParts = append(summaryParts, analysis.Processes[0])
	}
	if len(summaryParts) == 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("围绕“%s”整理学习要点。", analysis.Topic))
	}
	plan.Summary = summarizeLine(strings.Join(summaryParts, "；"), 120)

	for _, title := range requiredNoteSections() {
		section := noteSectionPlan{Title: title}
		switch title {
		case "摘要":
			section.Purpose = "概括主题与核心结论"
			section.Points = append(section.Points, plan.Summary)
		case "关键概念":
			section.Purpose = "梳理定义与术语边界"
			section.Points = appendNotePoints(section.Points, analysis.KeyConcepts, 6)
		case "原理与机制", "过程与步骤":
			section.Purpose = "说明条件、因果与执行顺序"
			section.Points = appendNotePoints(section.Points, analysis.Processes, 6)
		case "应用场景":
			section.Purpose = "结合例子说明迁移使用"
			section.Points = appendNotePoints(section.Points, analysis.Examples, 6)
		case "易错点":
			section.Purpose = "辨析常见误解与修正线索"
			section.Points = append(section.Points, "注意概念边界、条件范围和常见混淆。")
		case "总结":
			section.Purpose = "串联知识路径与复习方向"
			section.Points = append(section.Points, fmt.Sprintf("围绕“%s”形成可复习的结构。", analysis.Topic))
		}
		if len(section.Points) == 0 {
			section.Points = append(section.Points, supplementBullet(title, 1))
		}
		if analysis.Sparse && !hasSupplementBullet(section.Points) {
			section.Points = append(section.Points, supplementBullet(title, 2))
		}
		minPoints := 3
		if analysis.Sparse {
			minPoints = 4
		}
		for len(section.Points) < minPoints {
			section.Points = append(section.Points, supplementBullet(title, len(section.Points)+1))
		}
		section.Points = uniqueNonEmpty(section.Points)
		plan.Sections = append(plan.Sections, section)
	}
	return plan
}

func appendNotePoints(points []string, values []string, limit int) []string {
	for i, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if limit > 0 && i >= limit {
			break
		}
		points = append(points, value)
	}
	return points
}

func expandNoteContent(plan noteOutlinePlan, analysis learningContentAnalysis) noteOutlinePlan {
	expanded := plan
	evidenceIndex := 0
	minPoints := 4
	if analysis.Sparse {
		minPoints = 5
	}

	for i := range expanded.Sections {
		section := &expanded.Sections[i]
		for j := range section.Points {
			section.Points[j] = expandNotePoint(section.Title, section.Points[j], analysis, nextNoteEvidence(analysis.Evidence, &evidenceIndex))
		}
		for len(section.Points) < minPoints {
			title := noteExpansionPointTitle(section.Title, len(section.Points)+1)
			section.Points = append(section.Points, expandNotePoint(section.Title, title, analysis, nextNoteEvidence(analysis.Evidence, &evidenceIndex)))
			section.Points = uniqueNonEmpty(section.Points)
		}
		for j := range section.Points {
			section.Points[j] = expandNotePoint(section.Title, section.Points[j], analysis, nextNoteEvidence(analysis.Evidence, &evidenceIndex))
		}
		section.Points = uniqueNonEmpty(section.Points)
	}
	return expanded
}

func expandNotePoint(sectionTitle, point string, analysis learningContentAnalysis, evidence string) string {
	point = strings.TrimSpace(point)
	if point == "" {
		return ""
	}
	if evidence != "" && !strings.Contains(point, "资料要点：") {
		return point + "（资料要点：" + summarizeLine(evidence, 80) + "）"
	}
	return point
}

func nextNoteEvidence(evidence []learningEvidence, index *int) string {
	if len(evidence) == 0 {
		return ""
	}
	if index == nil {
		return summarizeLine(strings.TrimSpace(evidence[0].Text), 80)
	}
	ev := evidence[*index%len(evidence)]
	*index++
	return summarizeLine(strings.TrimSpace(ev.Text), 80)
}

func noteExpansionPointTitle(sectionTitle string, position int) string {
	return supplementBullet(sectionTitle, position)
}

func renderNote(plan noteOutlinePlan) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(strings.TrimSpace(plan.Title))
	b.WriteString("\n")
	if strings.TrimSpace(plan.Summary) != "" {
		b.WriteString("\n## 摘要\n")
		b.WriteString(strings.TrimSpace(plan.Summary))
		b.WriteString("\n")
	}
	for _, section := range plan.Sections {
		if strings.TrimSpace(section.Title) == "摘要" {
			continue
		}
		b.WriteString("\n## ")
		b.WriteString(strings.TrimSpace(section.Title))
		b.WriteString("\n")
		for _, point := range section.Points {
			point = strings.TrimSpace(point)
			if point == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(point)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func renderNotePlan(plan noteOutlinePlan) string {
	var b strings.Builder
	if strings.TrimSpace(plan.Title) != "" {
		b.WriteString("# ")
		b.WriteString(strings.TrimSpace(plan.Title))
		b.WriteString("\n")
	}
	if strings.TrimSpace(plan.Summary) != "" {
		b.WriteString("Summary: ")
		b.WriteString(strings.TrimSpace(plan.Summary))
		b.WriteString("\n")
	}
	for i, section := range plan.Sections {
		b.WriteString(fmt.Sprintf("Section %02d: %s\n", i+1, strings.TrimSpace(section.Title)))
		if strings.TrimSpace(section.Purpose) != "" {
			b.WriteString("Purpose: ")
			b.WriteString(strings.TrimSpace(section.Purpose))
			b.WriteString("\n")
		}
		for _, point := range section.Points {
			point = strings.TrimSpace(point)
			if point == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(point)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func appendNotePlansToContext(contextValue string, plan, expanded noteOutlinePlan) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(contextValue))
	if strings.TrimSpace(plan.Title) != "" {
		b.WriteString("\n\nINTERNAL_NOTE_PLAN\n")
		b.WriteString("内部笔记规划：\n")
		b.WriteString(renderNotePlan(plan))
	}
	if strings.TrimSpace(expanded.Title) != "" {
		b.WriteString("\n\nINTERNAL_NOTE_EXPANDED_PLAN\n")
		b.WriteString("内部笔记扩展：\n")
		b.WriteString(renderNote(expanded))
		b.WriteString("\n\nNOTE_GENERATION_RULES\n")
		b.WriteString("- Treat each Section entry as the writing brief for exactly one ## section; do not merge, omit, or reorder sections.\n")
		b.WriteString("- The planned points are source material, not the final wording. Expand each point into polished note content with concrete explanations grounded in the provided Markdown.\n")
		b.WriteString("- Keep every planned section title visible as ## heading, then add 3-5 substantial points for that section.\n")
		b.WriteString("- Content must be grounded in Original Markdown, Local References, Web Results, or the user's explicit prompt. Do not add generic boilerplate unless it appears in the source.\n")
		b.WriteString("- Finish all planned sections before returning. If the plan is long, make each section concise instead of truncating the note.\n")
	}
	return strings.TrimSpace(b.String())
}

func noteNeedsStructureRepair(content string) bool {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "#") {
		return true
	}
	if len([]rune(strings.ReplaceAll(trimmed, "#", ""))) < 30 {
		return true
	}
	if strings.Count(trimmed, "\n## ") < 2 {
		return true
	}
	return false
}

func requiredQuizQuestionTypes(analysis learningContentAnalysis) []string {
	conceptCount := len(analysis.KeyConcepts)
	processCount := len(analysis.Processes)
	exampleCount := len(analysis.Examples)
	totalPoints := conceptCount + processCount + exampleCount

	// 根据材料丰富度决定题目数量
	targetCount := 5
	if totalPoints >= 6 {
		targetCount = 6
	}
	if totalPoints >= 10 {
		targetCount = 7
	}
	if totalPoints >= 15 {
		targetCount = 8
	}

	// 题型分配：至少1道 single_choice + 1道 true_false
	types := []string{"single_choice", "true_false"}

	// 如果有对比性知识点，加多选题
	if conceptCount >= 3 {
		types = append(types, "multi_choice")
	}

	// 如果有过程性知识点，加填空题
	if processCount >= 1 {
		types = append(types, "fill_blank")
	}

	// 补充 short_answer 直到达到目标数量
	for len(types) < targetCount {
		types = append(types, "short_answer")
	}

	// 如果超了就截断
	if len(types) > targetCount {
		types = types[:targetCount]
	}

	return types
}

func planQuizQuestions(analysis learningContentAnalysis) quizQuestionPlan {
	plan := quizQuestionPlan{Topic: analysis.Topic}
	types := requiredQuizQuestionTypes(analysis)
	concepts := append([]string{}, analysis.KeyConcepts...)
	processes := append([]string{}, analysis.Processes...)
	examples := append([]string{}, analysis.Examples...)
	if len(concepts) == 0 {
		concepts = append(concepts, analysis.Topic)
	}

	for i, qType := range types {
		item := quizQuestionItem{Type: qType}
		switch qType {
		case "single_choice":
			topic := pickPoint(concepts, i)
			item.Topic = topic
			item.Question = fmt.Sprintf("关于“%s”，下列说法正确的是？", topic)
			item.Options = []string{
				topic + " 的基本定义如上所述。",
				"与原文相反的描述。",
				"无关的干扰项。",
				"概念混淆的选项。",
			}
			item.Answer = item.Options[0]
			item.Explanation = fmt.Sprintf("根据笔记，“%s”的定义如原文所述。", topic)
			item.Difficulty = "easy"
		case "true_false":
			topic := pickPoint(concepts, i+1)
			item.Topic = topic
			item.Question = fmt.Sprintf("判断：%s。", topic)
			item.Options = []string{"正确", "错误"}
			item.Answer = "正确"
			item.Explanation = fmt.Sprintf("根据笔记内容，该说法是正确的。“%s”的定义和描述如原文所述。", topic)
			item.Difficulty = "easy"
		case "multi_choice":
			topic := pickPoint(concepts, i+2)
			if topic == "" {
				topic = pickPoint(concepts, 0)
			}
			item.Topic = topic
			item.Question = fmt.Sprintf("关于“%s”，以下哪些说法是正确的？（多选）", topic)
			item.Options = []string{
				topic + " 的基本定义。",
				topic + " 的关键特征。",
				"与原文矛盾的描述。",
				topic + " 的适用条件。",
			}
			item.Answer = item.Options[0] + "；" + item.Options[1] + "；" + item.Options[3]
			item.Explanation = fmt.Sprintf("选项A、B、D正确。选项C与原文矛盾。关于“%s”的详细说明见笔记原文。", topic)
			item.Difficulty = "medium"
		case "fill_blank":
			if len(processes) > 0 {
				topic := pickPoint(processes, i)
				item.Topic = topic
				item.Question = fmt.Sprintf("“%s”的关键步骤是____。", topic)
				item.Answer = summarizeLine(topic, 100)
				item.Explanation = "该答案来自提供的笔记上下文。"
			} else {
				topic := pickPoint(concepts, i)
				item.Topic = topic
				item.Question = fmt.Sprintf("请填写“%s”的核心定义中的关键词：____。", topic)
				item.Answer = summarizeLine(topic, 100)
				item.Explanation = "该答案来自提供的笔记上下文。"
			}
			item.Difficulty = "medium"
		case "short_answer":
			if len(processes) > 0 {
				topic := pickPoint(processes, i)
				item.Topic = topic
				item.Question = fmt.Sprintf("简述“%s”的关键步骤或机制。", topic)
				item.Answer = summarizeLine(topic, 100)
				item.Explanation = "该答案来自提供的笔记上下文。"
			} else {
				topic := pickPoint(examples, i)
				if topic == "" {
					topic = pickPoint(concepts, i)
				}
				item.Topic = topic
				item.Question = fmt.Sprintf("说明“%s”的应用场景或例子。", topic)
				item.Answer = summarizeLine(topic, 100)
				item.Explanation = "该答案来自提供的笔记上下文。"
			}
			item.Difficulty = "hard"
		}
		plan.Questions = append(plan.Questions, item)
	}
	return plan
}

func expandQuizContent(plan quizQuestionPlan, analysis learningContentAnalysis) quizQuestionPlan {
	expanded := plan
	evidenceIndex := 0
	for i := range expanded.Questions {
		q := &expanded.Questions[i]
		evidence := nextQuizEvidence(analysis.Evidence, &evidenceIndex)
		if evidence != "" && q.Explanation != "" && !strings.Contains(q.Explanation, "资料要点：") {
			q.Explanation = q.Explanation + "（资料要点：" + summarizeLine(evidence, 80) + "）"
		}
		if strings.TrimSpace(q.Answer) == "" {
			q.Answer = summarizeLine(q.Topic, 100)
		}
		if strings.TrimSpace(q.Explanation) == "" {
			q.Explanation = "该答案来自提供的笔记上下文。"
		}
	}
	for len(expanded.Questions) < 5 {
		topic := analysis.Topic
		if len(analysis.KeyConcepts) > len(expanded.Questions) {
			topic = analysis.KeyConcepts[len(expanded.Questions)]
		}
		expanded.Questions = append(expanded.Questions, quizQuestionItem{
			Type:        "short_answer",
			Topic:       topic,
			Question:    fmt.Sprintf("简述“%s”的核心观点。", topic),
			Answer:      summarizeLine(topic, 100),
			Explanation: "该答案来自提供的笔记上下文。",
		})
	}
	return expanded
}

func nextQuizEvidence(evidence []learningEvidence, index *int) string {
	if len(evidence) == 0 {
		return ""
	}
	if index == nil {
		return summarizeLine(strings.TrimSpace(evidence[0].Text), 80)
	}
	ev := evidence[*index%len(evidence)]
	*index++
	return summarizeLine(strings.TrimSpace(ev.Text), 80)
}

func renderQuiz(plan quizQuestionPlan) string {
	items := make([]string, 0, len(plan.Questions))
	for _, q := range plan.Questions {
		options := make([]string, 0, len(q.Options))
		for _, opt := range q.Options {
			options = append(options, fmt.Sprintf("%q", opt))
		}
		item := fmt.Sprintf(`{"type":%q,"question":%q,"options":[%s],"answer":%q,"explanation":%q,"difficulty":%q}`,
			q.Type, q.Question, strings.Join(options, ","), q.Answer, q.Explanation, q.Difficulty)
		items = append(items, item)
	}
	return `{"questions":[` + strings.Join(items, ",") + `]}`
}

func renderQuizPlan(plan quizQuestionPlan) string {
	var b strings.Builder
	if strings.TrimSpace(plan.Topic) != "" {
		b.WriteString("Topic: ")
		b.WriteString(strings.TrimSpace(plan.Topic))
		b.WriteString("\n")
	}
	for i, q := range plan.Questions {
		b.WriteString(fmt.Sprintf("Question %02d: [%s] %s\n", i+1, q.Type, strings.TrimSpace(q.Question)))
		if strings.TrimSpace(q.Topic) != "" {
			b.WriteString("Focus: ")
			b.WriteString(strings.TrimSpace(q.Topic))
			b.WriteString("\n")
		}
		for j, opt := range q.Options {
			b.WriteString(fmt.Sprintf("  Option %d: %s\n", j+1, opt))
		}
		if strings.TrimSpace(q.Answer) != "" {
			b.WriteString("Answer: ")
			b.WriteString(strings.TrimSpace(q.Answer))
			b.WriteString("\n")
		}
		if strings.TrimSpace(q.Explanation) != "" {
			b.WriteString("Explanation: ")
			b.WriteString(strings.TrimSpace(q.Explanation))
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func appendQuizPlansToContext(contextValue string, plan, expanded quizQuestionPlan) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(contextValue))
	if strings.TrimSpace(plan.Topic) != "" {
		b.WriteString("\n\nINTERNAL_QUIZ_PLAN\n")
		b.WriteString("内部测验规划：\n")
		b.WriteString(renderQuizPlan(plan))
	}
	if strings.TrimSpace(expanded.Topic) != "" {
		b.WriteString("\n\nINTERNAL_QUIZ_EXPANDED_PLAN\n")
		b.WriteString("内部测验扩展：\n")
		b.WriteString(renderQuiz(expanded))
		b.WriteString("\n\nQUIZ_GENERATION_RULES\n")
		b.WriteString("- Follow the planned question types and topics; do not omit or merge questions.\n")
		b.WriteString("- Every question must include a non-empty answer and explanation.\n")
		b.WriteString("- For single_choice, provide 3-4 options and mark the correct one in the answer field with the exact option text.\n")
		b.WriteString("- For true_false, options must be [\"正确\",\"错误\"], answer must be \"正确\" or \"错误\".\n")
		b.WriteString("- For multi_choice, provide 4-5 options, answer must be all correct option texts joined by semicolons (；).\n")
		b.WriteString("- For fill_blank, options must be an empty array [], answer must be the key term or phrase to fill in.\n")
		b.WriteString("- For short_answer, options must be an empty array [], answer must be a reference answer.\n")
		b.WriteString("- Generate at least 5 questions, covering at least 2 different question types.\n")
		b.WriteString("- Distribute difficulty levels: roughly 40% easy, 40% medium, 20% hard.\n")
		b.WriteString("- Content must be grounded in Original Markdown, Local References, Web Results, or the user's explicit prompt.\n")
		b.WriteString("- Return only the JSON object, no markdown fences or extra text.\n")
	}
	return strings.TrimSpace(b.String())
}

func quizNeedsStructureRepair(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return true
	}
	if !validateQuizContent(trimmed) {
		return true
	}
	return false
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
	// Pre-process: merge fenced code blocks into single lines so they are
	// extracted as atomic units instead of being scattered as individual lines.
	lines := strings.Split(markdown, "\n")
	mergedLines := mergeCodeBlockLines(lines)

	for _, line := range mergedLines {
		// Code blocks are already merged into single lines starting with ```;
		// preserve them as-is without TrimLeft which would strip the fence markers.
		if isPPTCodeBlockBullet(line) {
			points = append(points, line)
			if len(points) >= limit {
				return points
			}
			continue
		}
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
