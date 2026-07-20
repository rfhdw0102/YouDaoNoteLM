// domain_aliases.go 是领域类型桥接层。
//
// 将各子包（mindmap/note/quiz）的领域类型别名为 generation 包类型，
// 并提供 learningContentAnalysis → 子包 Analysis 的转换函数。
// 这样父包代码可以统一引用本地类型名，屏蔽子包差异。
package generation

import (
	"YoudaoNoteLm/internal/service/generation/mindmap"
	"YoudaoNoteLm/internal/service/generation/note"
	"YoudaoNoteLm/internal/service/generation/quiz"
)

// 领域规划类型在根包保留别名，避免改动原有生成流程。
type mindmapPlan = mindmap.Plan
type mindmapBranchPlan = mindmap.BranchPlan
type mindmapNodePlan = mindmap.NodePlan

type noteOutlinePlan = note.OutlinePlan
type noteSectionPlan = note.SectionPlan

type quizQuestionPlan = quiz.QuestionPlan
type quizQuestionItem = quiz.QuestionItem

// 将通用学习分析转换为思维导图子包入参。
func mindmapAnalysisFromLearning(analysis learningContentAnalysis) mindmap.Analysis {
	return mindmap.Analysis{
		Topic:       analysis.Topic,
		KeyConcepts: append([]string{}, analysis.KeyConcepts...),
		Processes:   append([]string{}, analysis.Processes...),
		Examples:    append([]string{}, analysis.Examples...),
		Evidence:    mindmapEvidenceFromLearning(analysis.Evidence),
		Sections:    mindmapSectionsFromPPT(analysis.Sections),
		Sparse:      analysis.Sparse,
	}
}

// 将通用学习分析转换为笔记子包入参。
func noteAnalysisFromLearning(analysis learningContentAnalysis) note.Analysis {
	return note.Analysis{
		Topic:       analysis.Topic,
		KeyConcepts: append([]string{}, analysis.KeyConcepts...),
		Processes:   append([]string{}, analysis.Processes...),
		Examples:    append([]string{}, analysis.Examples...),
		Evidence:    noteEvidenceFromLearning(analysis.Evidence),
		Sparse:      analysis.Sparse,
	}
}

// 将通用学习分析转换为测验子包入参。
func quizAnalysisFromLearning(analysis learningContentAnalysis) quiz.Analysis {
	return quiz.Analysis{
		Topic:       analysis.Topic,
		KeyConcepts: append([]string{}, analysis.KeyConcepts...),
		Processes:   append([]string{}, analysis.Processes...),
		Examples:    append([]string{}, analysis.Examples...),
		Evidence:    quizEvidenceFromLearning(analysis.Evidence),
		Sparse:      analysis.Sparse,
	}
}

// mindmapEvidenceFromLearning 将通用学习证据转换为思维导图子包证据。
func mindmapEvidenceFromLearning(values []learningEvidence) []mindmap.Evidence {
	result := make([]mindmap.Evidence, 0, len(values))
	for _, value := range values {
		result = append(result, mindmap.Evidence{Text: value.Text, Source: value.Source})
	}
	return result
}

func mindmapSectionsFromPPT(values []pptSourceSection) []mindmap.SourceSection {
	result := make([]mindmap.SourceSection, 0, len(values))
	for _, value := range values {
		result = append(result, mindmap.SourceSection{
			Title:  value.Title,
			Points: append([]string{}, value.Points...),
		})
	}
	return result
}

func noteEvidenceFromLearning(values []learningEvidence) []note.Evidence {
	result := make([]note.Evidence, 0, len(values))
	for _, value := range values {
		result = append(result, note.Evidence{Text: value.Text, Source: value.Source})
	}
	return result
}

// quizEvidenceFromLearning 将通用学习证据转换为测验子包证据。
func quizEvidenceFromLearning(values []learningEvidence) []quiz.Evidence {
	result := make([]quiz.Evidence, 0, len(values))
	for _, value := range values {
		result = append(result, quiz.Evidence{Text: value.Text, Source: value.Source})
	}
	return result
}
