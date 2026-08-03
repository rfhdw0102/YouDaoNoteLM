package note

// 表示笔记生成时可引用的资料要点。
type Evidence struct {
	Text   string
	Source string
}

// 笔记规划需要的学习内容摘要。
type Analysis struct {
	Topic       string
	KeyConcepts []string
	Processes   []string
	Examples    []string
	Evidence    []Evidence
	Sparse      bool
}

// 描述笔记的大纲规划。
type OutlinePlan struct {
	Title    string
	Summary  string
	Sections []SectionPlan
}

// 描述笔记中的一个章节。
type SectionPlan struct {
	Title   string
	Purpose string
	Points  []string
}
