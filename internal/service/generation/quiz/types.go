package quiz

// 表示生成测验时可引用的资料要点。
type Evidence struct {
	Text   string
	Source string
}

// 测验规划需要的学习内容摘要。
type Analysis struct {
	Topic       string
	KeyConcepts []string
	Processes   []string
	Examples    []string
	Evidence    []Evidence
	Sparse      bool
}

// 描述一组测验题的规划结果。
type QuestionPlan struct {
	Topic     string
	Questions []QuestionItem
}

// 描述单道测验题。
type QuestionItem struct {
	Type        string
	Topic       string
	Question    string
	Options     []string
	Answer      string
	Explanation string
	Difficulty  string
}
