// content_analysis.go 实现学习内容分析。
//
// analyzeLearningContent 是主入口，对 Markdown 源材料进行结构化分析：
//   - 识别主题、章节、要点
//   - 提取证据（learningEvidence）支持的事实
//   - 生成 learningContentAnalysis 供各类型生成器的规划阶段使用
//
// 还包含 PPT 专用的源材料分节（extractPPTSourceSections）和思维导图 fallback 逻辑。
package generation

import (
	"strings"
)

func fallbackMindmapContent(input generationAgentInput) string {
	analysis := analyzeLearningContent(input)
	return renderMindmap(expandMindmapContent(planMindmap(analysis), analysis))
}

func appendMindmapPlansToContext(contextValue string, plan, expanded mindmapPlan) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(contextValue))
	if strings.TrimSpace(plan.Title) != "" {
		b.WriteString("\n\nINTERNAL_MINDMAP_PLAN\n")
		b.WriteString("内部思维导图规划：\n")
		b.WriteString(renderMindmapPlan(plan))
	}
	if strings.TrimSpace(expanded.Title) != "" {
		b.WriteString("\n\nINTERNAL_MINDMAP_EXPANDED_PLAN\n")
		b.WriteString("内部思维导图扩展：\n")
		b.WriteString(renderMindmap(expanded))
	}
	return strings.TrimSpace(b.String())
}

func renderMindmapPlan(plan mindmapPlan) string {
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

func learningDeckSections() []string {
	return []string{"背景与目标", "概念框架", "机制与流程", "案例与应用", "易错辨析", "总结复盘"}
}

func analyzeLearningContent(input generationAgentInput) learningContentAnalysis {
	markdown := ""
	prompt := ""
	if input.Request != nil {
		markdown = input.Request.Markdown
		prompt = input.Request.Prompt
	}
	sections := extractPPTSourceSections(markdown, 18)
	sections, focused := focusPPTSectionsByPrompt(sections, prompt)
	points := extractKeyPoints(markdown, 48)
	if focused && len(sections) > 0 {
		points = pointsFromPPTSections(sections, 48)
	}
	references := input.References
	if focused || len(sections) > 0 {
		references = focusPPTReferencesByPrompt(references, prompt, sections)
	}
	evidence := append(evidenceFromReferences(references), evidenceFromSearch(input.SearchResults)...)
	if len(sections) == 0 && len(references) > 0 {
		sections = pptSectionsFromReferences(references, 12)
		points = pointsFromPPTSections(sections, 48)
	}
	if focused && len(evidence) > 0 {
		for _, ev := range evidence {
			points = append(points, ev.Text)
		}
		points = uniqueNonEmpty(points)
	}
	analysis := learningContentAnalysis{
		Topic:       extractTitle(markdown, "学习资料"),
		KeyConcepts: points,
		UserIntent:  strings.TrimSpace(prompt),
		Evidence:    evidence,
		Sections:    sections,
		Sparse:      len(points) < 3,
	}
	if focused && len(sections) == 1 {
		analysis.Topic = sections[0].Title
	}
	for _, point := range points {
		switch {
		case containsAnyFold(point, "步骤", "流程", "机制", "反应", "cycle", "process"):
			analysis.Processes = append(analysis.Processes, point)
		case containsAnyFold(point, "例", "应用", "场景", "example", "case"):
			analysis.Examples = append(analysis.Examples, point)
		}
	}
	if len(analysis.KeyConcepts) == 0 {
		analysis.Gaps = append(analysis.Gaps, "核心概念")
	}
	if len(analysis.Processes) == 0 {
		analysis.Gaps = append(analysis.Gaps, "过程机制")
	}
	if len(analysis.Examples) == 0 {
		analysis.Gaps = append(analysis.Gaps, "例子应用")
	}
	return analysis
}

func focusPPTSectionsByPrompt(sections []pptSourceSection, prompt string) ([]pptSourceSection, bool) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" || len(sections) == 0 {
		return sections, false
	}
	focused := make([]pptSourceSection, 0, len(sections))
	for _, section := range sections {
		if pptPromptMatchesFocusText(prompt, section.Title) {
			focused = append(focused, section)
		}
	}
	if len(focused) == 0 {
		return sections, false
	}
	return focused, true
}

func focusPPTReferencesByPrompt(refs []GenerationReference, prompt string, sections []pptSourceSection) []GenerationReference {
	if len(refs) == 0 {
		return refs
	}
	prompt = strings.TrimSpace(prompt)
	sectionTitles := make([]string, 0, len(sections))
	for _, section := range sections {
		if strings.TrimSpace(section.Title) != "" {
			sectionTitles = append(sectionTitles, section.Title)
		}
	}
	focused := make([]GenerationReference, 0, len(refs))
	for _, ref := range refs {
		if pptReferenceMatchesFocus(ref, prompt, sectionTitles) {
			focused = append(focused, ref)
		}
	}
	if len(focused) == 0 {
		return refs
	}
	return focused
}

func pptReferenceMatchesFocus(ref GenerationReference, prompt string, sectionTitles []string) bool {
	for _, title := range sectionTitles {
		if pptPromptMatchesFocusText(ref.Heading, title) ||
			pptPromptMatchesFocusText(ref.ChapterPath, title) ||
			pptPromptMatchesFocusText(ref.Content, title) {
			return true
		}
	}
	if prompt == "" {
		return len(sectionTitles) == 0
	}
	return pptPromptMatchesFocusText(prompt, ref.Heading) ||
		pptPromptMatchesFocusText(prompt, ref.ChapterPath)
}

func pptPromptMatchesFocusText(prompt, text string) bool {
	prompt = strings.ToLower(strings.TrimSpace(prompt))
	text = strings.ToLower(strings.TrimSpace(text))
	if prompt == "" || text == "" {
		return false
	}
	if strings.Contains(prompt, text) || strings.Contains(text, prompt) {
		return true
	}
	for _, token := range splitKeywordCandidates(text) {
		token = strings.ToLower(strings.TrimSpace(token))
		if len([]rune(token)) >= 2 && strings.Contains(prompt, token) {
			return true
		}
	}
	return false
}

func pointsFromPPTSections(sections []pptSourceSection, limit int) []string {
	var points []string
	for _, section := range sections {
		points = append(points, section.Points...)
	}
	points = uniqueNonEmpty(points)
	if limit > 0 && len(points) > limit {
		return append([]string{}, points[:limit]...)
	}
	return points
}

func pptSectionsFromReferences(refs []GenerationReference, limit int) []pptSourceSection {
	sectionsByTitle := map[string]int{}
	sections := make([]pptSourceSection, 0, len(refs))
	for _, ref := range refs {
		title := firstNonEmpty(ref.Heading, ref.ChapterPath, ref.SourceName, "相关资料")
		point := strings.TrimSpace(summarizeLine(ref.Content, 120))
		if point == "" {
			continue
		}
		if idx, ok := sectionsByTitle[title]; ok {
			sections[idx].Points = append(sections[idx].Points, point)
			continue
		}
		sectionsByTitle[title] = len(sections)
		sections = append(sections, pptSourceSection{
			Title:  title,
			Points: []string{point},
		})
		if limit > 0 && len(sections) >= limit {
			break
		}
	}
	return sections
}

func evidenceFromReferences(refs []GenerationReference) []learningEvidence {
	evidence := make([]learningEvidence, 0, len(refs))
	for _, ref := range refs {
		text := strings.TrimSpace(summarizeLine(ref.Content, 120))
		if text == "" {
			continue
		}
		evidence = append(evidence, learningEvidence{Text: text, Source: generationReferenceLabel(ref)})
	}
	return evidence
}

func evidenceFromSearch(results []SearchResult) []learningEvidence {
	evidence := make([]learningEvidence, 0, len(results))
	for _, result := range results {
		text := strings.TrimSpace(summarizeLine(firstNonEmpty(result.Snippet, result.Content), 120))
		if text == "" {
			continue
		}
		evidence = append(evidence, learningEvidence{Text: text, Source: firstNonEmpty(result.Title, result.URL, "web")})
	}
	return evidence
}

func containsAnyFold(value string, terms ...string) bool {
	value = strings.ToLower(value)
	for _, term := range terms {
		if strings.Contains(value, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func extractPPTSourceSections(markdown string, maxSections int) []pptSourceSection {
	if maxSections <= 0 {
		maxSections = 18
	}
	var sections []pptSourceSection
	var overview []string
	var current *pptSourceSection
	flush := func() {
		if current == nil {
			return
		}
		current.Points = uniqueNonEmpty(current.Points)
		if strings.TrimSpace(current.Title) != "" && len(current.Points) > 0 {
			sections = append(sections, *current)
		}
		current = nil
	}

	//   ```go
	//   ```
	lines := strings.Split(markdown, "\n")
	mergedLines := mergeCodeBlockLines(lines)

	for _, line := range mergedLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "```") {
			level := 0
			for level < len(line) && line[level] == '#' {
				level++
			}
			title := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if title == "" {
				continue
			}
			if level == 1 && len(sections) == 0 && current == nil {
				continue
			}
			if level <= 3 {
				flush()
				if len(sections) >= maxSections {
					break
				}
				current = &pptSourceSection{Title: title}
				continue
			}
		}
		point := strings.TrimSpace(strings.TrimLeft(line, "-*0123456789. "))
		if len([]rune(point)) < 3 {
			continue
		}
		if current == nil {
			overview = append(overview, point)
			continue
		}
		current.Points = append(current.Points, point)
	}
	flush()
	if len(overview) > 0 {
		sections = append([]pptSourceSection{{
			Title:  "概述",
			Points: uniqueNonEmpty(overview),
		}}, sections...)
	}
	return sections
}

func mergeCodeBlockLines(lines []string) []string {
	var result []string
	var codeBuf strings.Builder
	inCode := false
	var codeLang string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inCode {
			if strings.HasPrefix(trimmed, "```") {
				inCode = true
				codeLang = strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
				codeBuf.Reset()
				codeBuf.WriteString(trimmed) // 带语言标识的起始围栏
				codeBuf.WriteByte('\n')
				continue
			}
			result = append(result, line)
		} else {
			if trimmed == "```" || trimmed == "```{" {
				codeBuf.WriteString(trimmed) // 结束围栏
				merged := codeBuf.String()
				inner := strings.TrimPrefix(merged, "```"+codeLang+"\n")
				inner = strings.TrimSuffix(inner, "```")
				inner = strings.TrimSpace(inner)
				if len([]rune(inner)) >= 4 {
					result = append(result, merged)
				}
				inCode = false
				codeLang = ""
				continue
			}
			codeBuf.WriteString(line)
			codeBuf.WriteByte('\n')
		}
	}
	if inCode {
		merged := codeBuf.String()
		inner := strings.TrimPrefix(merged, "```"+codeLang+"\n")
		inner = strings.TrimSpace(inner)
		if len([]rune(inner)) >= 4 {
			result = append(result, merged)
		}
	}
	return result
}

func requiredPPTSlideTitles() []string {
	return []string{"封面", "目录", "背景与目标", "概念框架", "机制与流程", "案例与应用", "易错辨析", "总结复盘"}
}
