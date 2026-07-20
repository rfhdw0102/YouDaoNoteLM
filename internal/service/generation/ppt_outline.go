// ppt_outline.go 实现 PPT 大纲的生成与解析。
//
// 负责将 LLM 输出的 PPT 大纲文本解析为结构化的 pptOutlinePlan，
// 包括标题提取、章节切分、要点归整、JSON 解析容错等。
package generation

import (
	"YoudaoNoteLm/pkg/logger"
	"context"
	"go.uber.org/zap"
	"strings"
	"time"
)

// generateOutline 调用 LLM 生成 PPT 大纲文本，失败时回退到静态版本。
func (a *pptGenerationAgent) generateOutline(ctx context.Context, input generationAgentInput) (string, error) {
	if a.model == nil {
		return fallbackPPTOutline(input), nil
	}
	llmStart := time.Now()
	strategy := pptOutlinePromptStrategy()
	outline, err := a.model.Generate(ctx, GenerationPrompt{
		AgentName:    "ppt_outline",
		System:       strategy.System,
		User:         strings.TrimSpace(input.Request.Prompt),
		Context:      input.Context,
		OutputFormat: strategy.OutputFormat,
	})
	logger.Info("[PPT] LLM call: generateOutline done",
		zap.Duration("llm_elapsed", time.Since(llmStart)),
		zap.Int("outline_len", len(outline)),
		zap.Error(err),
	)
	if err != nil {
		return "", err
	}
	outline = strings.TrimSpace(outline)
	if outline == "" {
		return fallbackPPTOutline(input), nil
	}
	return outline, nil
}

// fallbackPPTContent 在 LLM 不可用时生成本地兜底的 PPT 内容。
func fallbackPPTContent(input generationAgentInput) string {
	analysis := analyzeLearningContent(input)
	styleHint := ""
	userPrompt := ""
	if input.Request != nil {
		userPrompt = input.Request.Prompt
		styleHint = optionString(input.Request.Options, "ppt_style", "")
		if styleHint == "" {
			styleHint = optionString(input.Request.Options, "pptStyle", "")
		}
	}
	theme := designPPTStyleTheme(analysis, expandPPTContent(planPPTOutline(analysis), analysis), userPrompt, styleHint)
	return renderStyledPPTSlides(expandPPTContent(planPPTOutline(analysis), analysis), theme)
}

// fallbackPPTOutline 将本地静态大纲计划渲染为 Markdown 文本。
func fallbackPPTOutline(input generationAgentInput) string {
	plan := planPPTOutline(analyzeLearningContent(input))
	var outline strings.Builder
	outline.WriteString("# ")
	outline.WriteString(plan.Title)
	outline.WriteString("\n")
	for _, slide := range plan.Slides {
		outline.WriteString("- ")
		outline.WriteString(strings.TrimSpace(slide.Title))
		outline.WriteString("\n")
		for _, point := range slide.Bullets {
			point = strings.TrimSpace(point)
			if point == "" {
				continue
			}
			outline.WriteString("  - ")
			outline.WriteString(point)
			outline.WriteString("\n")
		}
	}
	return strings.TrimSpace(outline.String())
}

// isPPTPlanningLabel 判断文本是否为规划用途的标签行（如页面目的等）。
func isPPTPlanningLabel(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	labels := []string{
		"页面目的", "页面目标", "页面意图", "本页目的", "本页目标", "本页意图",
		"设计目的", "设计目标", "设计意图",
		"slide purpose", "page purpose", "writing brief",
		"核心论点", "可用证据", "内容展开", "关键要点",
		"source-topic", "source topic", "source_topic",
	}
	for _, label := range labels {
		if strings.HasPrefix(lower, strings.ToLower(label)) {
			rest := strings.TrimPrefix(lower, strings.ToLower(label))
			rest = strings.TrimSpace(rest)
			if rest == "" || strings.HasPrefix(rest, ":") || strings.HasPrefix(rest, "：") || strings.HasPrefix(rest, "**") {
				return true
			}
		}
	}
	return false
}

// normalizePPTSlideTitle 规整幻灯片标题，去除多余分隔符和后缀。
func normalizePPTSlideTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.TrimRight(title, "?？")
	title = strings.TrimRight(title, ":：")
	if idx := strings.IndexAny(title, ":："); idx > 0 {
		before := strings.TrimSpace(title[:idx])
		after := strings.TrimSpace(title[idx+1:])
		if strings.ContainsAny(after, "?？") {
			title = before
		} else if len([]rune(after)) > len([]rune(before)) && len([]rune(before)) <= 8 {
			title = before
		}
	}
	title = strings.TrimSpace(title)
	title = strings.TrimRight(title, ":：?？")
	if title == "" {
		title = "内容页"
	}
	return title
}

// parsePPTOutlineMarkdown 将大纲 Markdown 文本解析为结构化计划。
func parsePPTOutlineMarkdown(outline string) (pptOutlinePlan, bool) {
	lines := strings.Split(strings.TrimSpace(outline), "\n")
	plan := pptOutlinePlan{}
	var current *pptSlidePlan
	for _, line := range lines {
		raw := strings.TrimRight(line, " \t\r")
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			title := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			if title != "" && plan.Title == "" {
				plan.Title = title
			}
			continue
		}

		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))
		text := strings.TrimSpace(strings.TrimLeft(trimmed, "-*0123456789. "))
		if text == "" {
			continue
		}
		if isPPTPlanningLabel(text) {
			continue
		}
		if indent == 0 {
			plan.Slides = append(plan.Slides, pptSlidePlan{Title: normalizePPTSlideTitle(text)})
			current = &plan.Slides[len(plan.Slides)-1]
			continue
		}
		if current == nil {
			plan.Slides = append(plan.Slides, pptSlidePlan{Title: firstNonEmpty(plan.Title, "Slide")})
			current = &plan.Slides[len(plan.Slides)-1]
		}
		current.Bullets = append(current.Bullets, text)
	}
	if plan.Title == "" && len(plan.Slides) > 0 {
		plan.Title = plan.Slides[0].Title
	}
	if strings.TrimSpace(plan.Title) == "" || len(plan.Slides) == 0 {
		return pptOutlinePlan{}, false
	}
	for i := range plan.Slides {
		normalizePPTBullets(&plan.Slides[i])
		if strings.TrimSpace(plan.Slides[i].Purpose) == "" {
			plan.Slides[i].Purpose = purposeForPPTSlide(plan.Slides[i].Title, i, len(plan.Slides))
		}
	}
	plan = deduplicatePPTPlanBullets(plan)
	plan = ensurePPTPlanFrame(plan)
	return plan, true
}

// deduplicatePPTPlanBullets 去除幻灯片要点中的重复项。
func deduplicatePPTPlanBullets(plan pptOutlinePlan) pptOutlinePlan {
	seen := make(map[string]bool)
	for i := range plan.Slides {
		filtered := make([]string, 0, len(plan.Slides[i].Bullets))
		for _, bullet := range plan.Slides[i].Bullets {
			key := strings.ToLower(strings.TrimSpace(bullet))
			key = strings.ReplaceAll(key, "：", ":")
			key = strings.ReplaceAll(key, "，", ",")
			key = strings.ReplaceAll(key, "。", ".")
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			filtered = append(filtered, bullet)
		}
		plan.Slides[i].Bullets = filtered
	}
	return plan
}

// ensurePPTPlanFrame 确保大纲包含封面、目录和结束页的完整结构。
func ensurePPTPlanFrame(plan pptOutlinePlan) pptOutlinePlan {
	if strings.TrimSpace(plan.Title) == "" {
		plan.Title = "演示文稿"
	}
	if len(plan.Slides) == 0 {
		return planPPTOutline(learningContentAnalysis{Topic: plan.Title, Sparse: true})
	}
	if !isCoverSlideTitle(plan.Slides[0].Title) {
		cover := pptSlidePlan{
			Title:   "封面页",
			Purpose: "建立演示主题",
			Bullets: []string{plan.Title},
		}
		plan.Slides = append([]pptSlidePlan{cover}, plan.Slides...)
	}
	if len(plan.Slides) < 2 || !isAgendaSlideTitle(plan.Slides[1].Title) {
		agenda := pptSlidePlan{
			Title:   "目录页",
			Purpose: "呈现演示路径",
		}
		for _, slide := range plan.Slides[1:] {
			if !isEndingSlideTitle(slide.Title) {
				agenda.Bullets = append(agenda.Bullets, slide.Title)
			}
		}
		if len(agenda.Bullets) == 0 {
			agenda.Bullets = append(agenda.Bullets, "内容页")
		}
		plan.Slides = append([]pptSlidePlan{plan.Slides[0], agenda}, plan.Slides[1:]...)
	}
	last := plan.Slides[len(plan.Slides)-1]
	if !isEndingSlideTitle(last.Title) {
		plan.Slides = append(plan.Slides, pptSlidePlan{
			Title:   "结束页",
			Purpose: "收束结论与下一步",
			Bullets: []string{"总结核心观点", "明确下一步行动"},
		})
	}
	for i := range plan.Slides {
		if strings.TrimSpace(plan.Slides[i].Purpose) == "" {
			plan.Slides[i].Purpose = purposeForPPTSlide(plan.Slides[i].Title, i, len(plan.Slides))
		}
		normalizePPTBullets(&plan.Slides[i])
	}
	return plan
}

// purposeForPPTSlide 根据幻灯片位置和标题推断其用途说明。
func purposeForPPTSlide(title string, index, total int) string {
	switch {
	case index == 0 || isCoverSlideTitle(title):
		return "建立演示主题"
	case index == 1 || isAgendaSlideTitle(title):
		return "呈现演示路径"
	case index == total-1 || isEndingSlideTitle(title):
		return "收束结论与下一步"
	default:
		return "展开核心内容"
	}
}

// isCoverSlideTitle 判断标题是否为封面页。
func isCoverSlideTitle(title string) bool {
	return containsAnyFold(title, "封面", "cover", "title")
}

// isAgendaSlideTitle 判断标题是否为目录页。
func isAgendaSlideTitle(title string) bool {
	return containsAnyFold(title, "目录", "agenda", "outline")
}

// isEndingSlideTitle 判断标题是否为结束页。
func isEndingSlideTitle(title string) bool {
	return containsAnyFold(title, "结束", "总结", "closing", "end", "finish")
}

// stripPPTVisibleText 去除内容中的 Markdown 标记，仅保留可见文本。
func stripPPTVisibleText(content string) string {
	text := stripSimpleHTML(content)
	text = strings.ReplaceAll(text, "**", "")
	text = strings.ReplaceAll(text, "__", "")
	text = strings.ReplaceAll(text, "*", "")
	text = strings.ReplaceAll(text, "_", "")
	text = strings.ReplaceAll(text, "###", "")
	text = strings.ReplaceAll(text, "##", "")
	text = strings.ReplaceAll(text, "#", "")
	return text
}

// stripPPTExportPlaceholders 移除导出占位符块（如文件路径、预览链接）。
func stripPPTExportPlaceholders(content string) string {
	cleaned := stripTaggedBlock(content, "PPT_FILE")
	cleaned = stripTaggedBlock(cleaned, "PREVIEW_LINK")
	return strings.TrimSpace(cleaned)
}

// stripPPTPlanningArtifacts 移除规划用的标签行，保留有效内容。
func stripPPTPlanningArtifacts(content string) string {
	planLinePrefixes := []string{
		"页面目的", "页面目标", "页面意图", "本页目的", "本页目标", "本页意图",
		"设计目的", "设计目标", "设计意图",
		"slide purpose", "page purpose", "writing brief",
		"source-topic", "source topic", "source_topic",
		"核心论点", "可用证据", "内容展开", "关键要点",
	}
	lines := strings.Split(content, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		skip := false
		for _, prefix := range planLinePrefixes {
			if strings.HasPrefix(lower, strings.ToLower(prefix)) ||
				strings.HasPrefix(lower, strings.ToLower(prefix)+"**") ||
				strings.HasPrefix(lower, strings.ToLower(prefix)+"：") ||
				strings.HasPrefix(lower, strings.ToLower(prefix)+":") ||
				strings.HasPrefix(lower, "**"+strings.ToLower(prefix)) {
				skip = true
				break
			}
		}
		if !skip {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}
