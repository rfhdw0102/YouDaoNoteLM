// note_planner.go 笔记生成器的规划层。
//
// 本文件将笔记的规划、扩展、渲染、结构修复等能力委托给 note 子包实现，
// 并提供模型不可用时的降级 fallback 内容生成。
// Agent 构造函数 newNoteAgent/newQuizAgent 已统一迁移到 agent_factory.go。
package generation

import "YoudaoNoteLm/internal/service/generation/note"

// 在模型不可用时生成基础笔记内容。
func fallbackNoteContent(input generationAgentInput) string {
	analysis := analyzeLearningContent(input)
	return renderNote(expandNoteContent(planNoteOutline(analysis), analysis))
}

// 在模型不可用时生成基础测验内容。
func fallbackQuizContent(input generationAgentInput) string {
	analysis := analyzeLearningContent(input)
	return renderQuiz(expandQuizContent(planQuizQuestions(analysis), analysis))
}

// 委托笔记子包生成大纲规划。
func planNoteOutline(analysis learningContentAnalysis) noteOutlinePlan {
	return note.PlanOutline(noteAnalysisFromLearning(analysis))
}

// 委托笔记子包补全大纲内容。
func expandNoteContent(plan noteOutlinePlan, analysis learningContentAnalysis) noteOutlinePlan {
	return note.ExpandContent(plan, noteAnalysisFromLearning(analysis))
}

// 委托笔记子包渲染标记文本。
func renderNote(plan noteOutlinePlan) string {
	return note.Render(plan)
}

// 委托笔记子包拼接内部规划上下文。
func appendNotePlansToContext(contextValue string, plan, expanded noteOutlinePlan) string {
	return note.AppendPlansToContext(contextValue, plan, expanded)
}

// 委托笔记子包判断是否需要结构修复。
func noteNeedsStructureRepair(content string) bool {
	return note.NeedsStructureRepair(content)
}
