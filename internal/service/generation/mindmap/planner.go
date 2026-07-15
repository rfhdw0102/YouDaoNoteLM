package mindmap

import (
	"fmt"
	"strings"
)

func dynamicMindmapBranches(analysis Analysis) []BranchPlan {
	var branches []BranchPlan

	// 如果笔记有章节结构，直接用章节标题作为分支
	if len(analysis.Sections) >= 2 {
		for _, section := range analysis.Sections {
			title := strings.TrimSpace(section.Title)
			if title == "" {
				continue
			}
			branch := BranchPlan{Title: title}
			for _, point := range section.Points {
				point = strings.TrimSpace(point)
				if point == "" {
					continue
				}
				branch.Nodes = append(branch.Nodes, newMindmapNode(
					point,
					mindmapNodeDetailFromEvidence(point, analysis),
				))
			}
			if len(branch.Nodes) == 0 {
				branch.Nodes = append(branch.Nodes, newMindmapNode(
					supplementBullet(title, 1),
					mindmapNodeDetailFromEvidence(title, analysis),
				))
			}
			branches = append(branches, branch)
		}
	} else {
		// 扁平结构：按知识点类型分类
		if len(analysis.KeyConcepts) > 0 {
			branch := BranchPlan{Title: "核心概念"}
			for _, concept := range analysis.KeyConcepts {
				branch.Nodes = append(branch.Nodes, newMindmapNode(
					concept,
					mindmapNodeDetailFromEvidence(concept, analysis),
				))
			}
			branches = append(branches, branch)
		}
		if len(analysis.Processes) > 0 {
			branch := BranchPlan{Title: "原理与过程"}
			for _, proc := range analysis.Processes {
				branch.Nodes = append(branch.Nodes, newMindmapNode(
					proc,
					mindmapNodeDetailFromEvidence(proc, analysis),
				))
			}
			branches = append(branches, branch)
		}
		if len(analysis.Examples) > 0 {
			branch := BranchPlan{Title: "应用与案例"}
			for _, example := range analysis.Examples {
				branch.Nodes = append(branch.Nodes, newMindmapNode(
					example,
					mindmapNodeDetailFromEvidence(example, analysis),
				))
			}
			branches = append(branches, branch)
		}
		if len(branches) == 0 {
			// 极端稀疏：创建一个通用分支
			branch := BranchPlan{Title: analysis.Topic}
			for _, concept := range analysis.KeyConcepts {
				branch.Nodes = append(branch.Nodes, newMindmapNode(
					concept,
					mindmapNodeDetailFromEvidence(concept, analysis),
				))
			}
			if len(branch.Nodes) == 0 {
				branch.Nodes = append(branch.Nodes, newMindmapNode(
					fmt.Sprintf("围绕“%s”的关键要点", analysis.Topic),
					"结合笔记内容梳理核心知识点。",
				))
			}
			branches = append(branches, branch)
		}
	}

	// 始终保留总结分支
	branches = append(branches, BranchPlan{
		Title: "总结",
		Nodes: []NodePlan{
			newMindmapNode(
				fmt.Sprintf("围绕“%s”形成可复习的结构。", analysis.Topic),
				"按概念、机制、过程、应用和误区回顾学习路径。",
			),
		},
	})

	// 确保至少3个分支（含总结）
	if len(branches) < 3 {
		branchTitle := "补充内容"
		if strings.TrimSpace(analysis.Topic) != "" {
			branchTitle = analysis.Topic
		}
		for len(branches) < 3 {
			branches = append(branches, BranchPlan{
				Title: branchTitle,
				Nodes: []NodePlan{
					newMindmapNode(
						supplementBullet(branchTitle, len(branches)+1),
						"该节点为解释补充，用于补足学习结构。",
					),
				},
			})
		}
	}

	// 限制分支数量在8以内（含总结）
	if len(branches) > 8 {
		// 保留前7个分支和最后的总结分支
		branches = append(branches[:7], branches[len(branches)-1])
	}

	return branches
}

func PlanMindmap(analysis Analysis) Plan {
	plan := Plan{Title: analysis.Topic}
	branches := dynamicMindmapBranches(analysis)

	minNodes := 3
	if analysis.Sparse {
		minNodes = 4
	}
	for i := range branches {
		branch := &branches[i]
		for len(branch.Nodes) < minNodes {
			branch.Nodes = append(branch.Nodes, newMindmapNode(
				supplementBullet(branch.Title, len(branch.Nodes)+1),
				mindmapNodeDetailFromEvidence(branch.Title, analysis),
				mindmapBranchExpansionDetail(branch.Title, analysis),
			))
			branch.Nodes = uniqueMindmapNodes(branch.Nodes)
		}
		branch.Nodes = uniqueMindmapNodes(branch.Nodes)
	}

	plan.Branches = branches
	return plan
}

func ExpandContent(plan Plan, analysis Analysis) Plan {
	expanded := plan
	evidenceIndex := 0
	minNodes := 4
	if analysis.Sparse {
		minNodes = 5
	}

	for i := range expanded.Branches {
		branch := &expanded.Branches[i]
		for j := range branch.Nodes {
			branch.Nodes[j].Details = expandMindmapNodeDetails(branch.Title, branch.Nodes[j], analysis, nextMindmapEvidence(analysis.Evidence, &evidenceIndex))
		}
		for len(branch.Nodes) < minNodes {
			title := mindmapExpansionNodeTitle(branch.Title, len(branch.Nodes)+1)
			branch.Nodes = append(branch.Nodes, NodePlan{
				Title:   title,
				Details: expandMindmapNodeDetails(branch.Title, NodePlan{Title: title}, analysis, nextMindmapEvidence(analysis.Evidence, &evidenceIndex)),
			})
			branch.Nodes = uniqueMindmapNodes(branch.Nodes)
		}
		for j := range branch.Nodes {
			branch.Nodes[j].Details = expandMindmapNodeDetails(branch.Title, branch.Nodes[j], analysis, nextMindmapEvidence(analysis.Evidence, &evidenceIndex))
		}
		branch.Nodes = uniqueMindmapNodes(branch.Nodes)
	}
	return expanded
}

func appendMindmapNodes(nodes []NodePlan, branchTitle string, values []string, analysis Analysis) []NodePlan {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		nodes = append(nodes, newMindmapNode(value, mindmapNodeDetail(branchTitle, value, analysis)))
	}
	return nodes
}

func newMindmapNode(title string, details ...string) NodePlan {
	node := NodePlan{Title: strings.TrimSpace(title)}
	for _, detail := range details {
		detail = strings.TrimSpace(detail)
		if detail != "" {
			node.Details = append(node.Details, detail)
		}
	}
	return node
}

func expandMindmapNodeDetails(branchTitle string, node NodePlan, analysis Analysis, evidence string) []string {
	details := uniqueNonEmpty(node.Details)
	if len(details) == 0 {
		details = append(details, mindmapNodeDetail(branchTitle, node.Title, analysis))
	}
	if len(details) < 2 {
		details = append(details, mindmapNodeReviewDetail(branchTitle, node.Title))
	}
	if len(details) < 3 && evidence != "" {
		details = append(details, "资料要点："+summarizeLine(evidence, 90))
	}
	if len(details) < 3 {
		details = append(details, mindmapBranchExpansionDetail(branchTitle, analysis))
	}
	details = uniqueNonEmpty(details)
	if len(details) > 3 {
		details = append([]string{}, details[:3]...)
	}
	for len(details) < 2 {
		details = append(details, fmt.Sprintf("围绕“%s”继续补足复习说明。", strings.TrimSpace(node.Title)))
		details = uniqueNonEmpty(details)
	}
	return details
}

func mindmapNodeReviewDetail(branchTitle, nodeTitle string) string {
	switch branchTitle {
	case "核心概念":
		return "继续说明该概念的定义边界、关联概念和典型辨析。"
	case "原理机制":
		return "继续说明触发条件、关键变量和因果链条如何变化。"
	case "过程步骤":
		return "继续说明前置条件、执行顺序和每一步产出。"
	case "应用场景":
		return "继续说明适用场景、迁移方式和判断依据。"
	case "易错点":
		return "继续说明常见误解、错误推断和修正线索。"
	case "总结":
		return "继续串联概念、机制、过程和应用结论。"
	default:
		return fmt.Sprintf("继续围绕“%s”补足复习说明和展开方向。", strings.TrimSpace(nodeTitle))
	}
}

func nextMindmapEvidence(evidence []Evidence, index *int) string {
	if len(evidence) == 0 {
		return ""
	}
	if index == nil {
		return summarizeLine(strings.TrimSpace(evidence[0].Text), 90)
	}
	ev := evidence[*index%len(evidence)]
	*index++
	return summarizeLine(strings.TrimSpace(ev.Text), 90)
}

func mindmapNodeDetail(branchTitle, value string, analysis Analysis) string {
	if strings.Contains(value, "解释补充") {
		return "该节点为解释补充，用于补足学习结构。"
	}
	switch branchTitle {
	case "核心概念":
		return "先明确含义，再和相关概念建立联系。"
	case "原理机制":
		return "关注该机制成立的条件、因果关系和边界。"
	case "过程步骤":
		return "按先后顺序理解输入、变化和结果。"
	case "应用场景":
		return "结合具体情境判断该知识点如何迁移使用。"
	default:
		if len(analysis.Evidence) > 0 {
			return fmt.Sprintf("可参考：%s", analysis.Evidence[0].Source)
		}
		return "用于复习时展开说明和自我检查。"
	}
}

func mindmapNodeDetailFromEvidence(value string, analysis Analysis) string {
	// 如果值包含解释补充标记，返回通用提示
	if strings.Contains(value, "解释补充") {
		return "该节点为解释补充，用于补足学习结构。"
	}

	valueKeywords := extractSignificantWords(value)
	for _, ev := range analysis.Evidence {
		evKeywords := extractSignificantWords(ev.Text)
		overlap := 0
		for _, kw := range valueKeywords {
			for _, ekw := range evKeywords {
				if strings.EqualFold(kw, ekw) {
					overlap++
					break
				}
			}
		}
		if overlap >= 2 {
			return summarizeLine(ev.Text, 90)
		}
	}

	// 如果没有直接匹配的证据，返回通用但更有针对性的描述
	switch {
	case containsAnyFold(value, "定义", "概念", "是什么", "含义"):
		return "明确该概念的精确定义、适用范围和与相关概念的区别。"
	case containsAnyFold(value, "原理", "机制", "原因", "为什么"):
		return "理解该原理成立的条件、因果链条和关键变量。"
	case containsAnyFold(value, "步骤", "流程", "过程", "方法"):
		return "按顺序理解每个步骤的输入、变化和产出。"
	case containsAnyFold(value, "应用", "例子", "场景", "案例"):
		return "结合具体场景判断该知识点如何迁移使用。"
	default:
		return fmt.Sprintf("围绕“%s”展开具体内容和关键要点。", strings.TrimSpace(value))
	}
}

func mindmapBranchExpansionDetail(branchTitle string, analysis Analysis) string {
	switch branchTitle {
	case "核心概念":
		return "补充概念之间的联系、边界和典型辨析。"
	case "原理机制":
		return "补充触发条件、因果链条和关键变量。"
	case "过程步骤":
		return "补充前后顺序、输入输出和阶段性结果。"
	case "应用场景":
		if len(analysis.Examples) > 0 {
			return "结合已有例子扩展到相近场景和迁移使用。"
		}
		return "补充典型应用场景、判断方式和迁移思路。"
	case "易错点":
		return "补充常见混淆、错误推断和修正线索。"
	case "总结":
		return "补充复习路径、串联方式和回顾问题。"
	default:
		return "补充该分支下仍然缺失的学习展开。"
	}
}

func mindmapExpansionNodeTitle(branchTitle string, position int) string {
	return supplementBullet(branchTitle, position)
}

func uniqueMindmapNodes(nodes []NodePlan) []NodePlan {
	seen := map[string]struct{}{}
	result := make([]NodePlan, 0, len(nodes))
	for _, node := range nodes {
		node.Title = strings.TrimSpace(node.Title)
		if node.Title == "" {
			continue
		}
		if _, ok := seen[node.Title]; ok {
			continue
		}
		seen[node.Title] = struct{}{}
		node.Details = uniqueNonEmpty(node.Details)
		result = append(result, node)
	}
	return result
}

func Render(plan Plan) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(plan.Title)
	b.WriteString("\n")
	for _, branch := range plan.Branches {
		b.WriteString("## ")
		b.WriteString(branch.Title)
		b.WriteString("\n")
		for _, node := range branch.Nodes {
			b.WriteString("### ")
			b.WriteString(node.Title)
			b.WriteString("\n")
			for _, detail := range node.Details {
				b.WriteString("#### ")
				b.WriteString(detail)
				b.WriteString("\n")
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func NeedsStructureRepair(content string) bool {
	trimmed := strings.TrimSpace(content)
	if len([]rune(strings.ReplaceAll(trimmed, "#", ""))) < 20 {
		return true
	}
	// 至少3个 ## 分支
	if strings.Count(trimmed, "\n## ") < 3 {
		return true
	}
	// 至少有 ### 节点层级
	if !strings.Contains(trimmed, "\n### ") {
		return true
	}
	return false
}

func supplementBullet(title string, detail int) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Sprintf("补充要点 %d：结合材料进一步分析该主题的关键内容。", detail)
	}
	templates := []string{
		fmt.Sprintf("%s的核心定义：用一句话精确概括%s是什么，区分它与其他相近概念的本质差异。", title, title),
		fmt.Sprintf("%s的运作原理：解释%s背后的因果机制或数学/逻辑基础，说明为什么它这样工作而非那样工作。", title, title),
		fmt.Sprintf("%s的关键特征：列出%s的3个核心特征，每个特征给出一个具体例子或量化数据支撑。", title, title),
		fmt.Sprintf("%s的应用场景：描述%s在真实场景中的典型用法，给出一个具体的操作步骤或数值案例。", title, title),
		fmt.Sprintf("%s的常见误区：指出学习%s时最容易犯的2-3个错误，说明正确理解应该是什么。", title, title),
		fmt.Sprintf("%s与其他概念的关系：说明%s在整体知识体系中的位置，它依赖什么前置知识，又是什么后续知识的基础。", title, title),
	}
	idx := (detail - 1) % len(templates)
	return templates[idx]
}

func hasSupplementBullet(values []string) bool {
	for _, value := range values {
		if strings.Contains(value, "解释补充") || strings.Contains(value, "补充要点") {
			return true
		}
	}
	return false
}

func hasSupplementMindmapNode(nodes []NodePlan) bool {
	for _, node := range nodes {
		if strings.Contains(node.Title, "解释补充") {
			return true
		}
	}
	return false
}

// 渲染思维导图内部规划，供提示词上下文使用。
func RenderPlan(plan Plan) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(plan.Title)
	b.WriteString("\n")
	for _, branch := range plan.Branches {
		b.WriteString("## ")
		b.WriteString(branch.Title)
		b.WriteString("\n")
		for _, node := range branch.Nodes {
			b.WriteString("- ")
			b.WriteString(node.Title)
			b.WriteString("\n")
			for _, detail := range node.Details {
				b.WriteString("  - ")
				b.WriteString(detail)
				b.WriteString("\n")
			}
		}
	}
	return strings.TrimSpace(b.String())
}
