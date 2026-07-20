// note_agent_steps.go 实现笔记 Agent 的链式步骤方法。
//
// noteGenerationAgent 通过重写 baseGenerationAgent 的步骤方法实现笔记特有的：
//   - analyzeNoteContent：内容分析
//   - planNoteChainOutline：大纲规划
//   - expandNoteChainContent：内容扩展
//   - generateNoteDraft：初稿生成
//   - repairNoteStructure：结构修复
package generation

import (
	"context"
	"strings"
)

// analyzeNoteContent 分析学习内容并初始化笔记链状态。
func (a *noteGenerationAgent) analyzeNoteContent(ctx context.Context, input generationAgentInput) (noteChainState, error) {
	return noteChainState{
		input:    input,
		analysis: analyzeLearningContent(input),
	}, nil
}

// planNoteOutline 基于分析结果规划笔记大纲。
func (a *noteGenerationAgent) planNoteOutline(ctx context.Context, state noteChainState) (noteChainState, error) {
	state.plan = planNoteOutline(state.analysis)
	return state, nil
}

// expandNoteChainContent 扩展笔记大纲内容。
func (a *noteGenerationAgent) expandNoteChainContent(ctx context.Context, state noteChainState) (noteChainState, error) {
	state.expanded = expandNoteContent(state.plan, state.analysis)
	return state, nil
}

// generateNoteDraft 生成笔记初稿并附带修复方案。
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

// repairNoteStructure 必要时使用修复方案或 fallback 修复笔记结构。
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
