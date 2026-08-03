// quiz_planner.go 测验生成器的规划层。
//
// 将测验的规划、扩展、渲染、结构修复等能力委托给 quiz 子包实现。
package generation

import "YoudaoNoteLm/internal/service/generation/quiz"

// 委托测验子包生成题目规划。
func planQuizQuestions(analysis learningContentAnalysis) quizQuestionPlan {
	return quiz.PlanQuestions(quizAnalysisFromLearning(analysis))
}

// 委托测验子包补全题目内容。
func expandQuizContent(plan quizQuestionPlan, analysis learningContentAnalysis) quizQuestionPlan {
	return quiz.ExpandContent(plan, quizAnalysisFromLearning(analysis))
}

// 委托测验子包渲染题目结构化数据。
func renderQuiz(plan quizQuestionPlan) string {
	return quiz.Render(plan)
}

// 委托测验子包拼接内部规划上下文。
func appendQuizPlansToContext(contextValue string, plan, expanded quizQuestionPlan) string {
	return quiz.AppendPlansToContext(contextValue, plan, expanded)
}

// 委托测验子包判断是否需要结构修复。
func quizNeedsStructureRepair(content string) bool {
	return quiz.NeedsStructureRepair(content)
}

// 委托测验子包按指定最低题量判断是否需要结构修复。
func quizNeedsStructureRepairWithMin(content string, minQuestions int) bool {
	return !quiz.ValidateContentWithMin(content, minQuestions)
}

// 委托测验子包规范化可用内容，返回前端可直接 JSON.parse 的严格 JSON。
func normalizeQuizContent(content string) string {
	return quiz.NormalizeContent(content)
}

// 委托测验子包修复半成品内容，保留 LLM 已生成的有效题。
// 修复失败（如 JSON 完全无法解析）返回空字符串，由调用方走 fallback。
func repairQuizContent(content string) string {
	return quiz.RepairContent(content)
}

// 委托测验子包修复半成品内容，并补齐到指定最低题量。
func repairQuizContentWithMin(content string, minQuestions int) string {
	return quiz.RepairContentWithMin(content, minQuestions)
}
