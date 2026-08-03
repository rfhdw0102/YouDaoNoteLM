// quiz_agent_steps.go 实现测验 Agent 的链式步骤方法。
//
// quizGenerationAgent 通过重写 baseGenerationAgent 的步骤方法实现测验特有的：
//   - analyzeQuizContent：内容分析
//   - planQuizChainQuestions：题目规划
//   - expandQuizChainContent：内容扩展
//   - generateQuizDraft：初稿生成
//   - repairQuizStructure：结构修复（只修不丢，保留 LLM 已生成的有效题）
//   - formatValidate：格式校验（不整篇丢弃 LLM 内容，仅兜底完全无法解析的情况）
package generation

import (
	"context"
	"strings"

	"YoudaoNoteLm/pkg/logger"
	"go.uber.org/zap"
)

// analyzeQuizContent 分析学习内容并初始化测验链状态。
func (a *quizGenerationAgent) analyzeQuizContent(ctx context.Context, input generationAgentInput) (quizChainState, error) {
	return quizChainState{
		input:    input,
		analysis: analyzeLearningContent(input),
	}, nil
}

// planQuizQuestions 基于分析结果规划测验题目。
func (a *quizGenerationAgent) planQuizQuestions(ctx context.Context, state quizChainState) (quizChainState, error) {
	state.plan = planQuizQuestions(state.analysis)
	return state, nil
}

// expandQuizChainContent 扩展测验题目内容。
func (a *quizGenerationAgent) expandQuizChainContent(ctx context.Context, state quizChainState) (quizChainState, error) {
	state.expanded = expandQuizContent(state.plan, state.analysis)
	return state, nil
}

// generateQuizDraft 生成测验初稿并附带修复方案。
func (a *quizGenerationAgent) generateQuizDraft(ctx context.Context, state quizChainState) (generationDraft, error) {
	input := state.input
	input.Context = appendQuizPlansToContext(state.input.Context, state.plan, state.expanded)
	draft, err := a.generateDraft(ctx, input)
	if err != nil {
		return generationDraft{}, err
	}
	// 诊断日志：记录 LLM 原始返回，定位"一直降级"的根因。
	// 可能原因：LLM 返回空、返回非 JSON、返回字段名不匹配等。
	preview := draft.content
	if len(preview) > 300 {
		preview = preview[:300]
	}
	logger.Info("[QUIZ] generateQuizDraft done",
		zap.Int("content_len", len(draft.content)),
		zap.Bool("fallback_used", draft.fallbackUsed),
		zap.String("content_preview", preview),
	)
	repairPlan := state.expanded
	if strings.TrimSpace(repairPlan.Topic) == "" {
		repairPlan = state.plan
	}
	draft.quizRepairPlan = &repairPlan
	return draft, nil
}

func quizRepairTarget(draft generationDraft) int {
	if draft.quizRepairPlan != nil && len(draft.quizRepairPlan.Questions) > 0 {
		return len(draft.quizRepairPlan.Questions)
	}
	return 0
}

// repairQuizStructure 修复 LLM 半成品输出，不整篇丢弃。
//
// 策略（按优先级）：
//  1. 已通过校验 → 直接返回，不处理
//  2. 不通过但能修复 → 调 repairQuizContent 规范化题型/补齐选项/补齐题量，
//     保留 LLM 已生成的有效题；fallbackUsed=true 仅作为质量标记，不替换内容
//  3. 完全无法修复（JSON 解析失败/无任何有效题）→ 退回 fallback 整篇替换
func (a *quizGenerationAgent) repairQuizStructure(ctx context.Context, draft generationDraft) (generationDraft, error) {
	targetCount := quizRepairTarget(draft)
	if !quizNeedsStructureRepairWithMin(draft.content, targetCount) {
		logger.Info("[QUIZ] repairQuizStructure: already valid, skip")
		if normalized := normalizeQuizContent(draft.content); normalized != "" {
			draft.content = normalized
		}
		return draft, nil
	}

	repaired := repairQuizContentWithMin(draft.content, targetCount)
	logger.Info("[QUIZ] repairQuizStructure: repair attempted",
		zap.Int("original_len", len(draft.content)),
		zap.Int("repaired_len", len(repaired)),
		zap.Bool("repair_success", repaired != ""),
	)
	if repaired != "" {
		// 修复成功：保留 LLM 内容，仅标记 fallbackUsed 表示经过了修复。
		// 注意不写 draft.fallbackUsed = true，避免前端把"修复过的 LLM 内容"
		// 误判为兜底模板而隐藏质量提示。
		draft.content = repaired
		return draft, nil
	}

	// 修复失败：输入完全无法解析，退回整篇 fallback。
	logger.Warn("[QUIZ] repairQuizStructure: repair failed, falling back to template")
	if draft.quizRepairPlan != nil {
		draft.content = renderQuiz(*draft.quizRepairPlan)
	} else {
		draft.content = a.fallback(draft.input)
	}
	draft.fallbackUsed = true
	return draft, nil
}

// formatValidate 重写基类格式校验：quiz 类型不整篇丢弃 LLM 内容。
//
// 基类的默认行为是 validator 不通过就替换为 fallback，对 quiz 过于激进——
// LLM 生成 4 道有效题但差一道就被整篇换成模板，质量反而下降。
// 此处改为：先尝试 repairQuizContent 修复，仍不通过才走 fallback。
func (a *quizGenerationAgent) formatValidate(ctx context.Context, draft generationDraft) (generationDraft, error) {
	targetCount := quizRepairTarget(draft)
	draft.formatValid = !quizNeedsStructureRepairWithMin(draft.content, targetCount)
	logger.Info("[QUIZ] formatValidate",
		zap.Bool("format_valid", draft.formatValid),
		zap.Bool("fallback_used_before", draft.fallbackUsed),
	)
	if draft.formatValid {
		if normalized := normalizeQuizContent(draft.content); normalized != "" {
			draft.content = normalized
		}
		return draft, nil
	}

	// 校验失败先尝试修复，保留 LLM 已生成的有效题。
	repaired := repairQuizContentWithMin(draft.content, targetCount)
	if repaired != "" && !quizNeedsStructureRepairWithMin(repaired, targetCount) {
		logger.Info("[QUIZ] formatValidate: repaired successfully")
		draft.content = repaired
		draft.formatValid = true
		return draft, nil
	}

	// 修复仍不通过：JSON 完全无法解析，退回 fallback。
	logger.Warn("[QUIZ] formatValidate: repair failed, falling back to template")
	draft.content = a.fallback(draft.input)
	draft.fallbackUsed = true
	draft.formatValid = validateQuizContent(draft.content)
	return draft, nil
}
