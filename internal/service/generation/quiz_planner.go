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
