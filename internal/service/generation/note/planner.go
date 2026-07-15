package note

import (
	"fmt"
	"strings"
)

func requiredNoteSections() []string {
	return []string{"摘要", "关键概念", "原理与机制", "过程与步骤", "应用场景", "易错点", "总结"}
}

func PlanOutline(analysis Analysis) OutlinePlan {
	plan := OutlinePlan{Title: analysis.Topic}
	summaryParts := append([]string{}, analysis.KeyConcepts...)
	if len(analysis.Processes) > 0 {
		summaryParts = append(summaryParts, analysis.Processes[0])
	}
	if len(summaryParts) == 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("围绕“%s”整理学习要点。", analysis.Topic))
	}
	plan.Summary = summarizeLine(strings.Join(summaryParts, "；"), 120)

	for _, title := range requiredNoteSections() {
		section := SectionPlan{Title: title}
		switch title {
		case "摘要":
			section.Purpose = "概括主题与核心结论"
			section.Points = append(section.Points, plan.Summary)
		case "关键概念":
			section.Purpose = "梳理定义与术语边界"
			section.Points = appendNotePoints(section.Points, analysis.KeyConcepts, 6)
		case "原理与机制", "过程与步骤":
			section.Purpose = "说明条件、因果与执行顺序"
			section.Points = appendNotePoints(section.Points, analysis.Processes, 6)
		case "应用场景":
			section.Purpose = "结合例子说明迁移使用"
			section.Points = appendNotePoints(section.Points, analysis.Examples, 6)
		case "易错点":
			section.Purpose = "辨析常见误解与修正线索"
			section.Points = append(section.Points, "注意概念边界、条件范围和常见混淆。")
		case "总结":
			section.Purpose = "串联知识路径与复习方向"
			section.Points = append(section.Points, fmt.Sprintf("围绕“%s”形成可复习的结构。", analysis.Topic))
		}
		if len(section.Points) == 0 {
			section.Points = append(section.Points, supplementBullet(title, 1))
		}
		if analysis.Sparse && !hasSupplementBullet(section.Points) {
			section.Points = append(section.Points, supplementBullet(title, 2))
		}
		minPoints := 3
		if analysis.Sparse {
			minPoints = 4
		}
		for len(section.Points) < minPoints {
			section.Points = append(section.Points, supplementBullet(title, len(section.Points)+1))
		}
		section.Points = uniqueNonEmpty(section.Points)
		plan.Sections = append(plan.Sections, section)
	}
	return plan
}

func appendNotePoints(points []string, values []string, limit int) []string {
	for i, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if limit > 0 && i >= limit {
			break
		}
		points = append(points, value)
	}
	return points
}

func ExpandContent(plan OutlinePlan, analysis Analysis) OutlinePlan {
	expanded := plan
	evidenceIndex := 0
	minPoints := 4
	if analysis.Sparse {
		minPoints = 5
	}

	for i := range expanded.Sections {
		section := &expanded.Sections[i]
		for j := range section.Points {
			section.Points[j] = expandNotePoint(section.Title, section.Points[j], analysis, nextNoteEvidence(analysis.Evidence, &evidenceIndex))
		}
		for len(section.Points) < minPoints {
			title := noteExpansionPointTitle(section.Title, len(section.Points)+1)
			section.Points = append(section.Points, expandNotePoint(section.Title, title, analysis, nextNoteEvidence(analysis.Evidence, &evidenceIndex)))
			section.Points = uniqueNonEmpty(section.Points)
		}
		for j := range section.Points {
			section.Points[j] = expandNotePoint(section.Title, section.Points[j], analysis, nextNoteEvidence(analysis.Evidence, &evidenceIndex))
		}
		section.Points = uniqueNonEmpty(section.Points)
	}
	return expanded
}

func expandNotePoint(sectionTitle, point string, analysis Analysis, evidence string) string {
	point = strings.TrimSpace(point)
	if point == "" {
		return ""
	}
	if evidence != "" && !strings.Contains(point, "资料要点：") {
		return point + "（资料要点：" + summarizeLine(evidence, 80) + "）"
	}
	return point
}

func nextNoteEvidence(evidence []Evidence, index *int) string {
	if len(evidence) == 0 {
		return ""
	}
	if index == nil {
		return summarizeLine(strings.TrimSpace(evidence[0].Text), 80)
	}
	ev := evidence[*index%len(evidence)]
	*index++
	return summarizeLine(strings.TrimSpace(ev.Text), 80)
}

func noteExpansionPointTitle(sectionTitle string, position int) string {
	return supplementBullet(sectionTitle, position)
}

func Render(plan OutlinePlan) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(strings.TrimSpace(plan.Title))
	b.WriteString("\n")
	if strings.TrimSpace(plan.Summary) != "" {
		b.WriteString("\n## 摘要\n")
		b.WriteString(strings.TrimSpace(plan.Summary))
		b.WriteString("\n")
	}
	for _, section := range plan.Sections {
		if strings.TrimSpace(section.Title) == "摘要" {
			continue
		}
		b.WriteString("\n## ")
		b.WriteString(strings.TrimSpace(section.Title))
		b.WriteString("\n")
		for _, point := range section.Points {
			point = strings.TrimSpace(point)
			if point == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(point)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func renderPlan(plan OutlinePlan) string {
	var b strings.Builder
	if strings.TrimSpace(plan.Title) != "" {
		b.WriteString("# ")
		b.WriteString(strings.TrimSpace(plan.Title))
		b.WriteString("\n")
	}
	if strings.TrimSpace(plan.Summary) != "" {
		b.WriteString("Summary: ")
		b.WriteString(strings.TrimSpace(plan.Summary))
		b.WriteString("\n")
	}
	for i, section := range plan.Sections {
		b.WriteString(fmt.Sprintf("Section %02d: %s\n", i+1, strings.TrimSpace(section.Title)))
		if strings.TrimSpace(section.Purpose) != "" {
			b.WriteString("Purpose: ")
			b.WriteString(strings.TrimSpace(section.Purpose))
			b.WriteString("\n")
		}
		for _, point := range section.Points {
			point = strings.TrimSpace(point)
			if point == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(point)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func AppendPlansToContext(contextValue string, plan, expanded OutlinePlan) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(contextValue))
	if strings.TrimSpace(plan.Title) != "" {
		b.WriteString("\n\nINTERNAL_NOTE_PLAN\n")
		b.WriteString("内部笔记规划：\n")
		b.WriteString(renderPlan(plan))
	}
	if strings.TrimSpace(expanded.Title) != "" {
		b.WriteString("\n\nINTERNAL_NOTE_EXPANDED_PLAN\n")
		b.WriteString("内部笔记扩展：\n")
		b.WriteString(Render(expanded))
		b.WriteString("\n\nNOTE_GENERATION_RULES\n")
		b.WriteString("- Treat each Section entry as the writing brief for exactly one ## section; do not merge, omit, or reorder sections.\n")
		b.WriteString("- The planned points are source material, not the final wording. Expand each point into polished note content with concrete explanations grounded in the provided Markdown.\n")
		b.WriteString("- Keep every planned section title visible as ## heading, then add 3-5 substantial points for that section.\n")
		b.WriteString("- Content must be grounded in Original Markdown, Local References, Web Results, or the user's explicit prompt. Do not add generic boilerplate unless it appears in the source.\n")
		b.WriteString("- Finish all planned sections before returning. If the plan is long, make each section concise instead of truncating the note.\n")
	}
	return strings.TrimSpace(b.String())
}

func NeedsStructureRepair(content string) bool {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "#") {
		return true
	}
	if len([]rune(strings.ReplaceAll(trimmed, "#", ""))) < 30 {
		return true
	}
	if strings.Count(trimmed, "\n## ") < 2 {
		return true
	}
	return false
}
