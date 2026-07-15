// mindmap_agent_steps.go 实现思维导图 Agent 的链式步骤方法。
//
// mindmapGenerationAgent 通过重写 baseGenerationAgent 的步骤方法实现思维导图特有的：
//   - analyzeMindmapContent：内容分析
//   - planMindmapOutline：大纲规划
//   - expandMindmapChainContent：内容扩展
//   - generateMindmapDraft：初稿生成
//   - repairMindmapStructure：结构修复
package generation

import (
	"context"
	"strings"
)

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
