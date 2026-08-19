// ppt_agent_steps.go 实现 PPT Agent 的链式步骤方法。
//
// pptGenerationAgent 通过重写 baseGenerationAgent 的步骤方法实现 PPT 特有的：
//   - analyzePPTContent：内容分析
//   - planPPTChainOutline：大纲规划（遵循 ppt_outline_spec.go 规范）
//   - enrichPPTContent：内容增强
//   - renderPPTSlides：HTML 渲染
//   - stylePPTTheme：样式主题应用
package generation

import (
	"YoudaoNoteLm/pkg/logger"
	"context"
	"go.uber.org/zap"
	"strings"
	"time"
)

// analyzePPTContent 分析输入学习内容，初始化链式状态。
func (a *pptGenerationAgent) analyzePPTContent(ctx context.Context, input generationAgentInput) (pptChainState, error) {
	return pptChainState{
		input:    input,
		analysis: analyzeLearningContent(input),
	}, nil
}

// planPPTChainOutline 生成并解析 PPT 大纲，写入链式状态。
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

// expandPPTChainContent 基于分析结果扩充大纲要点。
func (a *pptGenerationAgent) expandPPTChainContent(ctx context.Context, state pptChainState) (pptChainState, error) {
	state.expanded = expandPPTContent(state.outlinePlan, state.analysis)
	return state, nil
}

// designPPTStyle 根据分析和用户选项设计 PPT 样式主题。
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

// generatePPTHTML 调用 LLM 生成 PPT HTML 内容并组装最终草稿。
func (a *pptGenerationAgent) generatePPTHTML(ctx context.Context, state pptChainState) (generationDraft, error) {
	content := ""
	fallbackUsed := false
	if a.model != nil {
		llmStart := time.Now()
		strategy := contentDrivenPPTHTMLPromptStrategy()
		contextValue := appendPPTLayoutDirectionToContext(
			appendPPTStyleToContext(
				appendPPTPlansToContext(state.input.Context, state.outline, state.expanded),
				state.styleTheme,
			),
			state.expanded,
		)
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
	repairPlan := state.expanded
	return generationDraft{input: state.input, content: content, fallbackUsed: fallbackUsed, pptRepairPlan: &repairPlan, pptStyleTheme: state.styleTheme}, nil
}

// polishPPTHTML 清理并规整生成的 HTML 内容。
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
	content = ensurePPTStyleBlockForPlan(content, draft.pptRepairPlan, draft.pptStyleTheme)
	draft.content = content
	return draft, nil
}

// repairPPTStructure 检测 HTML 质量问题并在必要时回退到兜底渲染。
func (a *pptGenerationAgent) repairPPTStructure(ctx context.Context, draft generationDraft) (generationDraft, error) {
	draft.content = stripPPTExportPlaceholders(draft.content)
	draft.content = sanitizePPTReferenceSections(draft.content)
	draft.content = stripPPTPlanningArtifacts(draft.content)
	draft.content = stripPPTReferenceMetadata(draft.content)
	if pptCanPatchCanvas(draft.content) {
		draft.content = patchPPTCanvasHTML(draft.content)
	}

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

	hardRepairReasons, softRepairReasons := splitPPTRepairReasons(repairReasons)
	if len(softRepairReasons) > 0 {
		logger.Warn("[PPT] quality warnings kept with LLM output",
			zap.Strings("reasons", softRepairReasons),
			zap.Int("content_len", len(draft.content)),
		)
	}

	if len(hardRepairReasons) > 0 {
		logger.Warn("[PPT] repairPPTStructure triggered, replacing LLM output with fallback",
			zap.Strings("reasons", hardRepairReasons),
			zap.Strings("soft_reasons", softRepairReasons),
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
	} else if len(softRepairReasons) == 0 {
		logger.Info("[PPT] repairPPTStructure skipped, LLM output kept")
	} else {
		logger.Info("[PPT] repairPPTStructure kept LLM output",
			zap.Strings("soft_reasons", softRepairReasons),
		)
	}
	return draft, nil
}

// splitPPTRepairReasons separates rendering blockers from quality warnings.
// Content-quality issues should not erase an otherwise valid, content-driven
// layout because the fallback renderer is intentionally more uniform.
func splitPPTRepairReasons(reasons []string) (hard, soft []string) {
	for _, reason := range reasons {
		switch reason {
		case "structure_repair", "internal_prompt_leak", "visible_placeholder_text":
			hard = append(hard, reason)
		default:
			soft = append(soft, reason)
		}
	}
	return hard, soft
}
