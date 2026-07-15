package mindmap

// 表示思维导图生成时可引用的资料要点。
type Evidence struct {
	Text   string
	Source string
}

// 表示从标记文本或引用资料中解析出的章节。
type SourceSection struct {
	Title  string
	Points []string
}

// 思维导图规划需要的学习内容摘要。
type Analysis struct {
	Topic       string
	KeyConcepts []string
	Processes   []string
	Examples    []string
	Evidence    []Evidence
	Sections    []SourceSection
	Sparse      bool
}

// 描述思维导图整体规划。
type Plan struct {
	Title    string
	Branches []BranchPlan
}

// 描述思维导图一级分支。
type BranchPlan struct {
	Title string
	Nodes []NodePlan
}

// 描述思维导图节点。
type NodePlan struct {
	Title   string
	Details []string
}
