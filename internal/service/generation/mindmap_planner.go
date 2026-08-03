// mindmap_planner.go 思维导图生成器的规划层。
//
// 将思维导图的规划、扩展、渲染、结构修复等能力委托给 mindmap 子包实现，
// 并提供模型不可用时的降级 fallback 内容生成。
package generation

import (
	"YoudaoNoteLm/internal/service/generation/mindmap"
	"fmt"
	"strings"
)

// 委托思维导图子包生成导图规划。
func planMindmap(analysis learningContentAnalysis) mindmapPlan {
	return mindmap.PlanMindmap(mindmapAnalysisFromLearning(analysis))
}

// 委托思维导图子包补全节点内容。
func expandMindmapContent(plan mindmapPlan, analysis learningContentAnalysis) mindmapPlan {
	return mindmap.ExpandContent(plan, mindmapAnalysisFromLearning(analysis))
}

// 委托思维导图子包渲染标记文本。
func renderMindmap(plan mindmapPlan) string {
	return mindmap.Render(plan)
}

// 委托思维导图子包判断是否需要结构修复。
func mindmapNeedsStructureRepair(content string) bool {
	return mindmap.NeedsStructureRepair(content)
}

// 为资料不足的页面或章节补充结构化要点。
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

// pickPoint 按索引从要点列表取一条，越界则循环取。
func pickPoint(points []string, index int) string {
	if len(points) == 0 {
		return ""
	}
	if index < len(points) {
		return points[index]
	}
	return points[index%len(points)]
}

// buildPPTFallbackPoints 从输入材料中提炼 PPT fallback 要点列表。
func buildPPTFallbackPoints(input generationAgentInput, limit int) []string {
	if limit <= 0 {
		limit = 9
	}
	markdown := ""
	prompt := ""
	if input.Request != nil {
		markdown = input.Request.Markdown
		prompt = input.Request.Prompt
	}
	title := extractTitle(markdown, "演示文稿")
	candidates := extractKeyPoints(markdown, limit)
	if prompt := strings.TrimSpace(prompt); prompt != "" {
		candidates = append(candidates, prompt)
	}
	for _, ref := range input.References {
		candidates = append(candidates, summarizeLine(ref.Content, 90))
	}
	for _, result := range input.SearchResults {
		candidates = append(candidates, summarizeLine(firstNonEmpty(result.Snippet, result.Content), 90))
	}
	candidates = append(candidates,
		fmt.Sprintf("围绕“%s”说明背景、问题和目标。", title),
		"提炼现有材料中的核心观点，并补充必要解释。",
		"使用来源材料、示例或数据支撑关键结论。",
		"将内容组织成适合演讲的开场、展开和收束。",
		"给出听众可以理解或执行的总结。",
	)
	points := uniqueNonEmpty(candidates)
	if len(points) > limit {
		return append([]string{}, points[:limit]...)
	}
	return points
}
