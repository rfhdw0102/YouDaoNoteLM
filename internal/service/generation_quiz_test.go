package service

import (
	"strings"
	"testing"
)

func TestPlanQuizQuestions(t *testing.T) {
	analysis := learningContentAnalysis{
		Topic:       "光合作用",
		KeyConcepts: []string{"光反应", "暗反应", "叶绿素"},
		Processes:   []string{"电子传递链", "卡尔文循环"},
		Examples:    []string{"C3植物", "C4植物"},
	}
	plan := planQuizQuestions(analysis)
	if len(plan.Questions) < 3 {
		t.Errorf("expected at least 3 questions, got %d", len(plan.Questions))
	}
	// 检查题型多样性
	typeSet := make(map[string]bool)
	for _, q := range plan.Questions {
		typeSet[q.Type] = true
		if q.Question == "" {
			t.Error("question must not be empty")
		}
		if q.Answer == "" {
			t.Error("answer must not be empty")
		}
	}
	if len(typeSet) < 2 {
		t.Errorf("expected at least 2 different question types, got %d", len(typeSet))
	}
}

func TestPlanQuizQuestionsSparse(t *testing.T) {
	// 测试材料稀疏时的场景
	analysis := learningContentAnalysis{
		Topic:       "测试主题",
		KeyConcepts: []string{"概念1"},
		Sparse:      true,
	}
	plan := planQuizQuestions(analysis)
	if len(plan.Questions) < 3 {
		t.Errorf("expected at least 3 questions, got %d", len(plan.Questions))
	}
}

func TestPlanQuizQuestionsRich(t *testing.T) {
	// 测试材料丰富时的场景
	analysis := learningContentAnalysis{
		Topic:       "测试主题",
		KeyConcepts: []string{"c1", "c2", "c3", "c4", "c5", "c6", "c7", "c8"},
		Processes:   []string{"p1", "p2", "p3"},
		Examples:    []string{"e1", "e2", "e3", "e4"},
	}
	plan := planQuizQuestions(analysis)
	if len(plan.Questions) < 3 {
		t.Errorf("expected at least 3 questions, got %d", len(plan.Questions))
	}
	typeSet := make(map[string]bool)
	for _, q := range plan.Questions {
		typeSet[q.Type] = true
	}
	if len(typeSet) < 2 {
		t.Errorf("expected at least 2 different question types, got %d", len(typeSet))
	}
}

func TestRequiredQuizQuestionTypes(t *testing.T) {
	types := requiredQuizQuestionTypes(learningContentAnalysis{
		Topic:       "测试主题",
		KeyConcepts: []string{"概念1"},
		Sparse:      true,
	})
	if len(types) < 3 {
		t.Errorf("expected at least 3 question types, got %d", len(types))
	}
	// 检查至少有2种不同题型
	typeSet := make(map[string]bool)
	for _, qt := range types {
		typeSet[qt] = true
	}
	if len(typeSet) < 2 {
		t.Errorf("expected at least 2 different question types, got %d", len(typeSet))
	}
}

func TestValidateQuizContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		valid   bool
	}{
		{
			name:    "empty content",
			content: "",
			valid:   false,
		},
		{
			name:    "too few questions",
			content: `{"questions":[{"type":"single_choice","question":"Q1","options":["A","B","C"],"answer":"A","explanation":"E1"}]}`,
			valid:   false,
		},
		{
			name: "valid mixed types",
			content: `{"questions":[` +
				`{"type":"single_choice","question":"Q1","options":["A","B","C","D"],"answer":"A","explanation":"E1"},` +
				`{"type":"single_choice","question":"Q2","options":["A","B","C","D"],"answer":"B","explanation":"E2"},` +
				`{"type":"short_answer","question":"Q3","options":[],"answer":"关键词","explanation":"E3"},` +
				`{"type":"short_answer","question":"Q4","options":[],"answer":"答案","explanation":"E4"},` +
				`{"type":"short_answer","question":"Q5","options":[],"answer":"答案5","explanation":"E5"}` +
				`]}`,
			valid: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateQuizContent(tt.content)
			if result != tt.valid {
				t.Errorf("validateQuizContent() = %v, want %v", result, tt.valid)
			}
		})
	}
}

func TestExpandQuizContent(t *testing.T) {
	plan := quizQuestionPlan{
		Topic: "测试主题",
		Questions: []quizQuestionItem{
			{Type: "single_choice", Topic: "概念1", Question: "关于概念1的说法？", Options: []string{"A", "B", "C", "D"}, Answer: "A", Explanation: "解释1"},
			{Type: "short_answer", Topic: "过程1", Question: "简述过程1？", Answer: "答案1", Explanation: "解释2"},
			{Type: "short_answer", Topic: "例子1", Question: "说明例子1？", Answer: "答案2", Explanation: "解释3"},
		},
	}
	analysis := learningContentAnalysis{
		Topic:       "测试主题",
		KeyConcepts: []string{"概念1"},
		Evidence:    []learningEvidence{{Text: "资料要点：这是关键信息", Source: "source1"}},
	}
	expanded := expandQuizContent(plan, analysis)
	if len(expanded.Questions) < 3 {
		t.Errorf("expected at least 3 questions after expansion, got %d", len(expanded.Questions))
	}
}

func TestRenderQuiz(t *testing.T) {
	plan := quizQuestionPlan{
		Topic: "测试主题",
		Questions: []quizQuestionItem{
			{Type: "single_choice", Topic: "概念1", Question: "关于概念1的说法？", Options: []string{"A", "B", "C", "D"}, Answer: "A", Explanation: "解释1"},
			{Type: "short_answer", Topic: "过程1", Question: "简述过程1？", Answer: "答案1", Explanation: "解释2"},
		},
	}
	rendered := renderQuiz(plan)
	if rendered == "" {
		t.Error("renderQuiz() returned empty string")
	}
	if !strings.Contains(rendered, "questions") {
		t.Error("renderQuiz() does not contain 'questions' key")
	}
}
