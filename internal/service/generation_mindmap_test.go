package service

import (
	"strings"
	"testing"
)

func TestDynamicMindmapBranchesSectioned(t *testing.T) {
	// 测试有章节结构时的情况
	analysis := learningContentAnalysis{
		Topic: "测试主题",
		Sections: []pptSourceSection{
			{Title: "第一章", Points: []string{"要点1", "要点2", "要点3"}},
			{Title: "第二章", Points: []string{"要点4", "要点5"}},
			{Title: "第三章", Points: []string{"要点6", "要点7"}},
		},
		KeyConcepts: []string{"概念1", "概念2"},
	}
	branches := dynamicMindmapBranches(analysis)
	if len(branches) < 3 {
		t.Errorf("expected at least 3 branches, got %d", len(branches))
	}
	// 最后一个应该是总结
	last := branches[len(branches)-1]
	if last.Title != "总结" {
		t.Errorf("expected last branch to be '总结', got '%s'", last.Title)
	}
	// 至少有一个分支有节点
	hasNodes := false
	for _, b := range branches {
		if len(b.Nodes) > 0 {
			hasNodes = true
			break
		}
	}
	if !hasNodes {
		t.Error("expected at least one branch to have nodes")
	}
}

func TestDynamicMindmapBranchesFlat(t *testing.T) {
	// 测试扁平材料时的情况
	analysis := learningContentAnalysis{
		Topic:       "测试主题",
		KeyConcepts: []string{"概念1", "概念2", "概念3"},
		Processes:   []string{"过程1"},
		Examples:    []string{"例子1"},
		Sparse:      false,
	}
	branches := dynamicMindmapBranches(analysis)
	if len(branches) < 3 {
		t.Errorf("expected at least 3 branches, got %d", len(branches))
	}
	// 应该有总结
	last := branches[len(branches)-1]
	if last.Title != "总结" {
		t.Errorf("expected last branch to be '总结', got '%s'", last.Title)
	}
}

func TestDynamicMindmapBranchesSparse(t *testing.T) {
	// 测试稀疏材料时的情况
	analysis := learningContentAnalysis{
		Topic:  "测试主题",
		Sparse: true,
	}
	branches := dynamicMindmapBranches(analysis)
	if len(branches) < 2 {
		t.Errorf("expected at least 2 branches, got %d", len(branches))
	}
}

func TestDynamicMindmapBranchesEmptySections(t *testing.T) {
	// 测试章节为空的情况
	analysis := learningContentAnalysis{
		Topic:       "测试主题",
		KeyConcepts: []string{},
		Processes:   []string{},
		Examples:    []string{},
	}
	branches := dynamicMindmapBranches(analysis)
	if len(branches) < 3 {
		t.Errorf("expected at least 3 branches, got %d", len(branches))
	}
}

func TestMindmapNeedsStructureRepair(t *testing.T) {
	tests := []struct {
		name    string
		content string
		repair  bool
	}{
		{
			name:    "empty content",
			content: "",
			repair:  true,
		},
		{
			name:    "too few branches",
			content: "# 标题\n## 分支1\n### 节点1",
			repair:  true,
		},
		{
			name: "valid structure",
			content: "# 标题\n## 分支1\n### 节点1\n#### 细节1\n## 分支2\n### 节点2\n## 分支3\n### 节点3",
			repair: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mindmapNeedsStructureRepair(tt.content)
			if result != tt.repair {
				t.Errorf("mindmapNeedsStructureRepair() = %v, want %v", result, tt.repair)
			}
		})
	}
}

func TestPlanMindmap(t *testing.T) {
	analysis := learningContentAnalysis{
		Topic:       "测试主题",
		KeyConcepts: []string{"概念1", "概念2", "概念3"},
		Processes:   []string{"过程1"},
		Examples:    []string{"例子1"},
	}
	plan := planMindmap(analysis)
	if plan.Title == "" {
		t.Error("planMindmap() returned empty title")
	}
	if len(plan.Branches) < 3 {
		t.Errorf("expected at least 3 branches, got %d", len(plan.Branches))
	}
	// 每个分支至少应有节点
	for i, branch := range plan.Branches {
		if len(branch.Nodes) == 0 {
			t.Errorf("branch %d (%s) has no nodes", i, branch.Title)
		}
	}
}

func TestExpandMindmapContent(t *testing.T) {
	plan := mindmapPlan{
		Title: "测试主题",
		Branches: []mindmapBranchPlan{
			{Title: "核心概念", Nodes: []mindmapNodePlan{{Title: "概念1", Details: []string{"细节"}}}},
			{Title: "原理与过程", Nodes: []mindmapNodePlan{{Title: "过程1", Details: []string{"步骤"}}}},
			{Title: "总结", Nodes: []mindmapNodePlan{{Title: "要点", Details: []string{"关键点"}}}},
		},
	}
	analysis := learningContentAnalysis{
		Topic:       "测试主题",
		KeyConcepts: []string{"概念1", "概念2"},
		Evidence:    []learningEvidence{{Text: "补充证据1", Source: "src1"}},
	}
	expanded := expandMindmapContent(plan, analysis)
	if len(expanded.Branches) < 3 {
		t.Errorf("expected at least 3 branches after expansion, got %d", len(expanded.Branches))
	}
}

func TestRenderMindmap(t *testing.T) {
	plan := mindmapPlan{
		Title: "测试主题",
		Branches: []mindmapBranchPlan{
			{Title: "核心概念", Nodes: []mindmapNodePlan{{Title: "概念1", Details: []string{"概念1的详细说明"}}}},
			{Title: "总结", Nodes: []mindmapNodePlan{{Title: "知识结构", Details: []string{"结构化复习"}}}},
		},
	}
	rendered := renderMindmap(plan)
	if rendered == "" {
		t.Error("renderMindmap() returned empty string")
	}
	if !strings.Contains(rendered, "#") {
		t.Error("renderMindmap() output does not contain markdown headings")
	}
}

func TestMindmapNodeDetailFromEvidence(t *testing.T) {
	analysis := learningContentAnalysis{
		Topic:       "测试主题",
		KeyConcepts: []string{"概念1", "概念2"},
		Evidence:    []learningEvidence{{Text: "概念1的详细描述", Source: "src1"}},
	}
	detail := mindmapNodeDetailFromEvidence("概念1", analysis)
	if detail == "" {
		t.Error("mindmapNodeDetailFromEvidence() returned empty string")
	}
	// 测试不存在的主题
	detail2 := mindmapNodeDetailFromEvidence("不存在", analysis)
	if detail2 == "" {
		t.Error("mindmapNodeDetailFromEvidence() returned empty for unknown topic")
	}
}

func TestNewMindmapNode(t *testing.T) {
	node := newMindmapNode("测试节点", "详细说明")
	if node.Title != "测试节点" {
		t.Errorf("expected title '测试节点', got '%s'", node.Title)
	}
	if len(node.Details) != 1 {
		t.Errorf("expected 1 detail, got %d", len(node.Details))
	}
	if node.Details[0] != "详细说明" {
		t.Errorf("expected detail '详细说明', got '%s'", node.Details[0])
	}
	// 测试无详情
	node2 := newMindmapNode("仅标题")
	if node2.Title != "仅标题" {
		t.Errorf("expected title '仅标题', got '%s'", node2.Title)
	}
	if len(node2.Details) != 0 {
		t.Errorf("expected 0 details, got %d", len(node2.Details))
	}
}
