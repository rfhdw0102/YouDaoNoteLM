// quiz_agent_steps.go 实现测验 Agent 的链式步骤方法。
//
// quizGenerationAgent 通过重写 baseGenerationAgent 的步骤方法实现测验特有的：
//   - analyzeQuizContent：内容分析
//   - planQuizChainQuestions：题目规划
//   - expandQuizChainContent：内容扩展
//   - generateQuizDraft：初稿生成
//   - repairQuizStructure：结构修复
package generation

import (
	"context"
	"strings"
)

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
