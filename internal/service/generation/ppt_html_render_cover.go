// ppt_html_render_cover.go 实现 PPT 封面页和总结页的渲染。
//
//   - writePPTCoverSlide：渲染封面页（主标题、副标题、标签）
//   - writePPTSummarySlideBody：渲染总结页正文
//   - pptContentSlideTitles：提取所有内容页标题（供目录页使用）
package generation

import (
	"fmt"
	"strings"
)

// writePPTCoverSlide 渲染封面页：主标题、副标题与标签。
func writePPTCoverSlide(b *strings.Builder, plan pptOutlinePlan, slide pptSlidePlan, index int) {
	b.WriteString(`<div class="cover-meta"><span class="section-number">`)
	b.WriteString(fmt.Sprintf("%02d", index+1))
	b.WriteString(`</span><span>演示文稿</span></div>`)
	b.WriteString("<h1>")
	b.WriteString(htmlEscape(firstNonEmpty(plan.Title, slide.Title, "演示文稿")))
	b.WriteString("</h1>")
	b.WriteString(`<p class="cover-subtitle">`)
	b.WriteString(htmlEscape(pptCoverSubtitle(plan, slide)))
	b.WriteString(`</p>`)
	tags := uniqueNonEmpty(append([]string{}, slide.Bullets...))
	tags = append(tags, pptContentSlideTitles(plan)...)
	if len(tags) == 0 && strings.TrimSpace(slide.Title) != "" && !isCoverSlideTitle(slide.Title) {
		tags = append(tags, slide.Title)
	}
	tags = uniqueNonEmpty(tags)
	if len(tags) > 4 {
		tags = tags[:4]
	}
	if len(tags) > 0 {
		b.WriteString(`<div class="cover-tags">`)
		for _, tag := range tags {
			b.WriteString(`<span class="cover-tag">`)
			b.WriteString(htmlEscape(tag))
			b.WriteString(`</span>`)
		}
		b.WriteString(`</div>`)
	}
}

// pptCoverSubtitle 根据主题和要点组合生成封面副标题。
func pptCoverSubtitle(plan pptOutlinePlan, slide pptSlidePlan) string {
	candidates := uniqueNonEmpty(append([]string{}, slide.Bullets...))
	if len(candidates) == 0 {
		candidates = pptContentSlideTitles(plan)
	}
	title := firstNonEmpty(plan.Title, slide.Title, "本次主题")
	if len(candidates) == 0 {
		return fmt.Sprintf("围绕 %s 梳理关键内容", title)
	}
	if len(candidates) > 3 {
		candidates = candidates[:3]
	}
	return fmt.Sprintf("围绕 %s，聚焦 %s", title, strings.Join(candidates, "、"))
}

// pptContentSlideTitles 提取计划中所有内容页标题，过滤封面、目录与结束页。
func pptContentSlideTitles(plan pptOutlinePlan) []string {
	var titles []string
	for i, slide := range plan.Slides {
		title := strings.TrimSpace(slide.Title)
		if title == "" {
			continue
		}
		if i == 0 || i == 1 || i == len(plan.Slides)-1 || isCoverSlideTitle(title) || isAgendaSlideTitle(title) || isEndingSlideTitle(title) {
			continue
		}
		titles = append(titles, title)
	}
	return uniqueNonEmpty(titles)
}

// writePPTSummarySlideBody 渲染总结页正文（仅基于单张幻灯片）。
func writePPTSummarySlideBody(b *strings.Builder, slide pptSlidePlan) {
	writePPTSummarySlideBodyForPlan(b, pptOutlinePlan{Slides: []pptSlidePlan{slide}}, slide)
}

// writePPTSummarySlideBodyForPlan 基于完整计划渲染总结页的核心结论与后续行动。
func writePPTSummarySlideBodyForPlan(b *strings.Builder, plan pptOutlinePlan, slide pptSlidePlan) {
	primary, actions := pptSummaryContent(plan, slide)
	if len(actions) == 0 {
		actions = []string{primary}
	}
	b.WriteString(`<div class="summary-layout"><div class="summary-card"><h3>核心结论</h3><p>`)
	b.WriteString(htmlEscape(primary))
	b.WriteString(`</p></div><div class="summary-actions"><h3>后续行动</h3><ul>`)
	for _, action := range actions {
		b.WriteString("<li>")
		b.WriteString(htmlEscape(action))
		b.WriteString("</li>")
	}
	b.WriteString(`</ul></div></div>`)
}

// pptSummaryContent 提取总结页的核心结论与后续行动列表。
func pptSummaryContent(plan pptOutlinePlan, slide pptSlidePlan) (string, []string) {
	if len(slide.Bullets) > 0 {
		return slide.Bullets[0], append([]string{}, slide.Bullets[1:]...)
	}
	titles := pptContentSlideTitles(plan)
	if len(titles) == 0 {
		title := firstNonEmpty(plan.Title, slide.Title, "本次主题")
		return fmt.Sprintf("回顾 %s 的关键内容", title), []string{fmt.Sprintf("继续聚焦 %s", title)}
	}
	focus := titles
	if len(focus) > 3 {
		focus = focus[:3]
	}
	primary := fmt.Sprintf("本次内容围绕 %s 展开", strings.Join(focus, "、"))
	actions := make([]string, 0, len(focus))
	for _, title := range focus {
		actions = append(actions, fmt.Sprintf("继续聚焦 %s", title))
	}
	return primary, actions
}

// pptSlideProgressPercent 计算幻灯片进度百分比。
func pptSlideProgressPercent(index, total int) int {
	if total <= 0 {
		return 100
	}
	if index < 1 {
		index = 1
	}
	if index > total {
		index = total
	}
	return int(float64(index)/float64(total)*100 + 0.5)
}

// pptInsightTokens 从幻灯片要点中提取最多 2 条洞察摘要。
func pptInsightTokens(slide pptSlidePlan) []string {
	candidates := append([]string{}, slide.Bullets...)
	if len(candidates) == 0 {
		candidates = append(candidates, slide.Title)
	}
	tokens := make([]string, 0, 2)
	for _, candidate := range candidates {
		token := summarizeLine(candidate, 42)
		if token == "" {
			continue
		}
		tokens = append(tokens, token)
		if len(tokens) >= 2 {
			break
		}
	}
	return uniqueNonEmpty(tokens)
}
