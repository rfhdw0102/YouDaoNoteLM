// ppt_html_render_blocks.go 实现 PPT 内容页的多种布局渲染。
//
// 根据内容特征选择布局：
//   - writePPTTwoColumnSlide：双栏布局（对比类内容）
//   - writePPTCardGridSlide：卡片网格布局（要点罗列）
//   - pptSlideLayoutForIndex：按页面序号自动选择布局
package generation

import (
	"fmt"
	"strings"
)

func pptSlideLayoutForIndex(index int, slide pptSlidePlan) string {
	title := strings.ToLower(slide.Title)
	if slideHasCodeBlock(slide) {
		return "code"
	}
	if containsAnyFold(title, "对比", "比较", "vs", "compare", "区别", "差异") {
		return "comparison"
	}
	if containsAnyFold(title, "引用", "名言", "观点", "quote", "观点摘录") {
		return "quote"
	}
	if containsAnyFold(title, "列表", "要点", "清单", "list", "checklist") {
		return "full-width"
	}
	layouts := []string{"two-column", "card-grid", "full-width", "two-column", "comparison", "card-grid"}
	if index < 0 {
		index = 0
	}
	return layouts[index%len(layouts)]
}

func slideHasCodeBlock(slide pptSlidePlan) bool {
	for _, bullet := range slide.Bullets {
		if isPPTCodeBlockBullet(bullet) {
			return true
		}
	}
	return false
}

func writePPTTwoColumnSlide(b *strings.Builder, slide pptSlidePlan) {
	b.WriteString(`<div class="content-grid"><ul class="main-points">`)
	for _, bullet := range slide.Bullets {
		if isPPTCodeBlockBullet(bullet) {
			continue // 代码块在下方单独渲染
		}
		b.WriteString("<li>")
		b.WriteString(htmlEscape(pptExpandBullet(bullet, slide.Title)))
		b.WriteString("</li>")
	}
	b.WriteString("</ul>")
	tokens := pptInsightTokens(slide)
	if len(tokens) > 0 {
		b.WriteString(`<div class="insight-panel">`)
		for _, token := range tokens {
			b.WriteString(`<div class="insight-token">`)
			b.WriteString(htmlEscape(token))
			b.WriteString(`</div>`)
		}
		b.WriteString(`</div>`)
	}
	b.WriteString("</div>")
	writePPTCodeBlocks(b, slide.Bullets)
}

func writePPTCardGridSlide(b *strings.Builder, slide pptSlidePlan) {
	b.WriteString(`<div class="card-grid">`)
	cardIdx := 0
	for _, bullet := range slide.Bullets {
		if isPPTCodeBlockBullet(bullet) {
			continue // 代码块单独渲染
		}
		if cardIdx >= 4 {
			break
		}
		b.WriteString(`<div class="content-card"><div class="card-title">`)
		b.WriteString(htmlEscape(pptCardTitleFromBullet(bullet, cardIdx)))
		b.WriteString(`</div><div class="card-body">`)
		b.WriteString(htmlEscape(pptExpandBullet(bullet, slide.Title)))
		b.WriteString(`</div></div>`)
		cardIdx++
	}
	b.WriteString(`</div>`)
	writePPTCodeBlocks(b, slide.Bullets)
}

func pptCardTitleFromBullet(bullet string, index int) string {
	bullet = strings.TrimSpace(bullet)
	if bullet == "" {
		return fmt.Sprintf("要点 %d", index+1)
	}
	bullet = strings.TrimLeft(bullet, "-*•0123456789. ")
	if idx := strings.IndexAny(bullet, ":："); idx > 2 && idx < 20 {
		title := strings.TrimSpace(bullet[:idx])
		if utf8RuneCount(title) >= 2 {
			return title
		}
	}
	runes := []rune(bullet)
	limit := 10
	if len(runes) < limit {
		limit = len(runes)
	}
	title := string(runes[:limit])
	for i := limit - 1; i > 2; i-- {
		ch := runes[i]
		if ch == ' ' || ch == '，' || ch == '。' || ch == '、' || ch == '：' || ch == ':' {
			title = string(runes[:i])
			break
		}
	}
	title = strings.TrimSpace(title)
	if utf8RuneCount(title) < 2 {
		return fmt.Sprintf("要点 %d", index+1)
	}
	return title
}

func pptExpandBullet(bullet, slideTitle string) string {
	bullet = strings.TrimSpace(bullet)
	if bullet == "" {
		return ""
	}
	if utf8RuneCount(bullet) >= 30 {
		return bullet
	}
	if strings.HasSuffix(bullet, "。") || strings.HasSuffix(bullet, ".") {
		return bullet
	}
	slideTitle = strings.TrimSpace(slideTitle)
	if slideTitle != "" {
		return fmt.Sprintf("%s：%s。", slideTitle, bullet)
	}
	return bullet + "。"
}

func writePPTFullWidthListSlide(b *strings.Builder, slide pptSlidePlan) {
	b.WriteString(`<div class="full-width-list"><ul>`)
	for _, bullet := range slide.Bullets {
		if isPPTCodeBlockBullet(bullet) {
			continue // 代码块在下方单独渲染
		}
		b.WriteString("<li>")
		b.WriteString(htmlEscape(pptExpandBullet(bullet, slide.Title)))
		b.WriteString("</li>")
	}
	b.WriteString("</ul></div>")
	writePPTCodeBlocks(b, slide.Bullets)
}

func writePPTComparisonSlide(b *strings.Builder, slide pptSlidePlan) {
	var textBullets []string
	for _, bullet := range slide.Bullets {
		if !isPPTCodeBlockBullet(bullet) {
			textBullets = append(textBullets, bullet)
		}
	}
	mid := len(textBullets) / 2
	if mid == 0 {
		mid = 1
	}
	left := textBullets[:mid]
	right := textBullets[mid:]
	if len(right) == 0 {
		right = left
	}
	leftTitle := pptComparisonTitleFromBullets(left, slide.Title, "左")
	rightTitle := pptComparisonTitleFromBullets(right, slide.Title, "右")
	b.WriteString(`<div class="comparison-layout"><div class="comparison-col left"><h3>`)
	b.WriteString(htmlEscape(leftTitle))
	b.WriteString(`</h3>`)
	for _, bullet := range left {
		b.WriteString("<p>")
		b.WriteString(htmlEscape(pptExpandBullet(bullet, slide.Title)))
		b.WriteString("</p>")
	}
	b.WriteString(`</div><div class="comparison-col right"><h3>`)
	b.WriteString(htmlEscape(rightTitle))
	b.WriteString(`</h3>`)
	for _, bullet := range right {
		b.WriteString("<p>")
		b.WriteString(htmlEscape(pptExpandBullet(bullet, slide.Title)))
		b.WriteString("</p>")
	}
	b.WriteString(`</div></div>`)
	writePPTCodeBlocks(b, slide.Bullets)
}

func pptComparisonTitleFromBullets(bullets []string, slideTitle, side string) string {
	if len(bullets) == 0 {
		if slideTitle != "" {
			return slideTitle
		}
		return side + "侧内容"
	}
	first := strings.TrimSpace(bullets[0])
	if first == "" {
		return slideTitle
	}
	if idx := strings.IndexAny(first, ":："); idx > 2 && idx < 20 {
		return strings.TrimSpace(first[:idx])
	}
	runes := []rune(first)
	limit := 8
	if len(runes) < limit {
		limit = len(runes)
	}
	return string(runes[:limit])
}

func writePPTQuoteSlide(b *strings.Builder, slide pptSlidePlan) {
	quote := slide.Title
	if len(slide.Bullets) > 0 {
		quote = pptExpandBullet(slide.Bullets[0], slide.Title)
	}
	source := ""
	if len(slide.Bullets) > 1 {
		source = slide.Bullets[1]
	}
	b.WriteString(`<div class="quote-block"><div class="quote-text">`)
	b.WriteString(htmlEscape(quote))
	b.WriteString(`</div>`)
	if source != "" {
		b.WriteString(`<div class="quote-source">`)
		b.WriteString(htmlEscape(source))
		b.WriteString(`</div>`)
	}
	b.WriteString(`</div>`)
	writePPTCodeBlocks(b, slide.Bullets)
}

func writePPTCodeBlocks(b *strings.Builder, bullets []string) {
	for _, bullet := range bullets {
		if !isPPTCodeBlockBullet(bullet) {
			continue
		}
		code, ok := stripFencedCodeBlockForPPT(bullet)
		if !ok {
			continue
		}
		b.WriteString(`<pre class="ppt-code-block"><code>`)
		b.WriteString(htmlEscape(code))
		b.WriteString(`</code></pre>`)
	}
}

func writePPTCodeSlide(b *strings.Builder, slide pptSlidePlan) {
	var textBullets []string
	for _, bullet := range slide.Bullets {
		if !isPPTCodeBlockBullet(bullet) {
			textBullets = append(textBullets, bullet)
		}
	}
	if len(textBullets) > 0 {
		b.WriteString(`<div class="full-width-list"><ul>`)
		for _, bullet := range textBullets {
			b.WriteString("<li>")
			b.WriteString(htmlEscape(pptExpandBullet(bullet, slide.Title)))
			b.WriteString("</li>")
		}
		b.WriteString(`</ul></div>`)
	}
	writePPTCodeBlocks(b, slide.Bullets)
}

func pptCardTitle(slideTitle string, index int) string {
	slideTitle = strings.TrimSpace(slideTitle)
	if slideTitle == "" {
		return fmt.Sprintf("要点 %d", index+1)
	}
	return slideTitle
}

func pptComparisonLeftTitle(slideTitle string) string {
	slideTitle = strings.TrimSpace(slideTitle)
	if slideTitle == "" {
		return "特征 A"
	}
	return slideTitle + " · A"
}

func pptComparisonRightTitle(slideTitle string) string {
	slideTitle = strings.TrimSpace(slideTitle)
	if slideTitle == "" {
		return "特征 B"
	}
	return slideTitle + " · B"
}

func sanitizePPTPlanVisibleText(plan pptOutlinePlan) pptOutlinePlan {
	plan.Title = cleanPPTVisibleText(plan.Title)
	for i := range plan.Slides {
		plan.Slides[i].Title = cleanPPTVisibleText(plan.Slides[i].Title)
		plan.Slides[i].Purpose = cleanPPTVisibleText(plan.Slides[i].Purpose)
		for j := range plan.Slides[i].Bullets {
			plan.Slides[i].Bullets[j] = cleanPPTVisibleText(plan.Slides[i].Bullets[j])
		}
		normalizePPTBullets(&plan.Slides[i])
	}
	return plan
}

func cleanPPTVisibleText(value string) string {
	value = strings.ReplaceAll(value, "&nbsp;", " ")

	if isPPTCodeBlockBullet(value) {
		return value
	}

	if cleaned, ok := stripFencedCodeBlockForPPT(value); ok {
		return cleaned
	}

	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	for {
		trimmed := strings.TrimSpace(value)
		cleaned := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		if cleaned != trimmed {
			value = cleaned
			continue
		}
		cleaned = strings.TrimSpace(strings.TrimLeft(trimmed, "-*•"))
		if cleaned != trimmed {
			value = cleaned
			continue
		}
		return trimmed
	}
}

func stripFencedCodeBlockForPPT(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "```") {
		return "", false
	}
	firstNewline := strings.Index(trimmed, "\n")
	if firstNewline < 0 {
		inner := strings.TrimPrefix(trimmed, "```")
		inner = strings.TrimSuffix(inner, "```")
		inner = strings.TrimSpace(inner)
		if inner == "" {
			return "", false
		}
		return inner, true
	}
	inner := trimmed[firstNewline+1:]
	if strings.HasSuffix(inner, "\n```") {
		inner = inner[:len(inner)-4]
	} else if strings.HasSuffix(inner, "```") {
		inner = inner[:len(inner)-3]
	}
	inner = strings.TrimRight(inner, "\n")
	if strings.TrimSpace(inner) == "" {
		return "", false
	}
	return inner, true
}

func isPPTCodeBlockBullet(bullet string) bool {
	trimmed := strings.TrimSpace(bullet)
	return strings.HasPrefix(trimmed, "```") && strings.Contains(trimmed, "\n")
}
