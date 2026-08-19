// ppt_html_render_base.go 实现 PPT 的 HTML 渲染基础。
//
// renderStyledPPTSlides 是主入口，将 pptOutlinePlan 渲染为带样式的 HTML 幻灯片。
// 每张幻灯片用 <section> 标签包裹，配合 ppt_style.go 的主题样式生成最终 HTML。
package generation

import (
	"fmt"
	"strings"
)

// renderStyledPPTSlides 将大纲计划渲染为带主题样式的完整 HTML 幻灯片。
func renderStyledPPTSlides(plan pptOutlinePlan, theme pptStyleTheme) string {
	plan = sanitizePPTPlanVisibleText(plan)
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<style>
:root {
  --bg: %s; --surface: %s; --surface-soft: %s;
  --accent: %s; --accent-2: %s; --accent-soft: %s;
  --text: %s; --muted: %s; --heading: %s;
  --border: %s; --panel: %s;
}
`, theme.Background, theme.Surface, theme.Surface, theme.Primary, theme.Secondary, pptAccentSoft(theme), theme.Text, theme.Muted, theme.Heading, pptThemeBorder(theme), pptThemePanel(theme)))
	b.WriteString(`* { margin: 0; padding: 0; box-sizing: border-box; }
.ppt-slide {
  position: relative;
  width: 1920px;
  height: 1080px;
  overflow: hidden;
  background: var(--surface);
  color: var(--text);
  font-family: ` + theme.FontBody + `;
  padding: 86px 112px;
  display: flex;
  flex-direction: column;
  gap: 30px;
}
.ppt-slide::after {
  content: "";
  position: absolute;
  right: 0;
  bottom: 0;
  width: 520px;
  height: 16px;
  background: linear-gradient(90deg, var(--accent), var(--accent-2));
}
.ppt-cover {
  justify-content: center;
  background: ` + pptThemeCoverGradient(theme) + `;
  padding: 120px 150px;
}
.cover-meta {
  display: flex;
  align-items: center;
  gap: 18px;
  color: var(--accent);
  font-size: 28px;
  font-weight: 800;
}
.cover-subtitle {
  max-width: 1180px;
  color: var(--muted);
  font-size: 34px;
  line-height: 1.34;
  font-weight: 650;
}
.cover-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 18px;
  max-width: 1320px;
}
.cover-tag {
  background: rgba(255, 255, 255, .78);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 16px 24px;
  color: var(--heading);
  font-size: 28px;
  font-weight: 760;
}
.ppt-agenda { background: var(--bg); }
.section-number {
  width: fit-content;
  background: var(--accent-soft);
  color: var(--accent);
  border-radius: 999px;
  padding: 10px 24px;
  font-size: 26px;
  font-weight: 800;
}
.slide-title-wrap {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 42px;
}
.slide-title-wrap h2 { flex: 1; }
.slide-progress {
  position: absolute;
  left: 112px;
  right: 112px;
  bottom: 58px;
  height: 10px;
  border-radius: 999px;
  background: #e5e7eb;
  overflow: hidden;
}
.slide-progress span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, var(--accent), var(--accent-2));
}
h1 { max-width: 1360px; font-size: 76px; font-weight: 850; color: var(--heading); line-height: 1.12; }
h2 { max-width: 1480px; font-size: 52px; font-weight: 800; color: var(--heading); line-height: 1.18; }
.ppt-cover h2 { font-size: 34px; color: var(--accent); font-weight: 750; }
ul { list-style: none; padding-left: 0; }
li {
  display: flex;
  align-items: flex-start;
  gap: 18px;
  padding: 18px 0;
  color: var(--text);
  font-size: 32px;
  line-height: 1.36;
  border-bottom: 1px solid var(--border);
}
li::before { content: ""; width: 12px; height: 12px; margin-top: 16px; border-radius: 999px; background: var(--accent); flex-shrink: 0; }
li:last-child { border-bottom: none; }
.content-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.3fr) minmax(420px, .7fr);
  gap: 54px;
  align-items: start;
}
.main-points {
  background: var(--surface);
  border-top: 6px solid var(--accent);
  padding: 12px 0 0;
}
.insight-panel {
  min-height: 360px;
  background: linear-gradient(180deg, var(--accent-soft) 0%, var(--panel) 100%);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 34px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 22px;
}
.insight-token {
  color: var(--heading);
  font-size: 30px;
  line-height: 1.25;
  font-weight: 760;
}
.dir-list { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 28px; }
.dir-item {
  min-height: 118px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 28px 32px;
  color: var(--heading);
  font-weight: 750;
  font-size: 31px;
  line-height: 1.25;
}
.summary-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.05fr) minmax(0, .95fr);
  gap: 42px;
  align-items: stretch;
}
.summary-card,
.summary-actions {
  min-height: 460px;
  border-radius: 8px;
  padding: 38px 42px;
}
.summary-card {
  background: var(--surface);
  border-left: 8px solid var(--accent);
}
.summary-actions {
  background: var(--panel);
  border: 1px solid var(--border);
}
.summary-card h3,
.summary-actions h3 {
  color: var(--heading);
  font-size: 34px;
  line-height: 1.2;
  margin-bottom: 18px;
}
.summary-card p {
  color: var(--text);
  font-size: 32px;
  line-height: 1.34;
  font-weight: 700;
}
.card-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 32px;
}
.content-card {
  min-height: 280px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-left: 6px solid var(--accent);
  border-radius: 8px;
  padding: 32px 36px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.content-card .card-title {
  color: var(--heading);
  font-size: 34px;
  font-weight: 800;
  line-height: 1.2;
}
.content-card .card-body {
  color: var(--text);
  font-size: 30px;
  line-height: 1.34;
}
.full-width-list {
  background: var(--surface);
  border-top: 6px solid var(--accent);
  padding: 12px 0 0;
}
.full-width-list ul { list-style: none; padding-left: 0; }
.full-width-list li {
  display: flex;
  align-items: flex-start;
  gap: 18px;
  padding: 20px 0;
  color: var(--text);
  font-size: 32px;
  line-height: 1.36;
  border-bottom: 1px solid var(--border);
}
.full-width-list li::before {
  content: "";
  width: 12px;
  height: 12px;
  margin-top: 16px;
  border-radius: 999px;
  background: var(--accent-2);
  flex-shrink: 0;
}
.full-width-list li:last-child { border-bottom: none; }
.comparison-layout {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 42px;
}
.comparison-col {
  min-height: 400px;
  border-radius: 8px;
  padding: 36px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.comparison-col.left {
  background: var(--accent-soft);
  border-left: 6px solid var(--accent);
}
.comparison-col.right {
  background: var(--panel);
  border-left: 6px solid var(--accent-2);
}
.comparison-col h3 {
  font-size: 34px;
  font-weight: 800;
  color: var(--heading);
  margin-bottom: 8px;
}
.comparison-col p {
  font-size: 30px;
  line-height: 1.34;
  color: var(--text);
}
.quote-block {
  background: linear-gradient(135deg, var(--accent-soft) 0%, var(--panel) 100%);
  border-left: 8px solid var(--accent);
  border-radius: 8px;
  padding: 48px 56px;
  min-height: 400px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 24px;
}
.quote-block .quote-text {
  font-size: 38px;
  line-height: 1.3;
  font-weight: 700;
  color: var(--heading);
}
.quote-block .quote-source {
  font-size: 28px;
  color: var(--muted);
  font-weight: 600;
}
.ppt-code-block {
  background: #1e293b;
  color: #e2e8f0;
  border-radius: 8px;
  padding: 28px 32px;
  margin-top: 18px;
  font-family: 'Cascadia Code', 'Fira Code', 'Consolas', 'Courier New', monospace;
  font-size: 24px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
  overflow-x: auto;
  border-left: 6px solid var(--accent-2);
}
.ppt-code-block code {
  font-family: inherit;
  color: inherit;
}
</style>`)
	for i, slide := range plan.Slides {
		className := "ppt-slide"
		if i == 0 {
			className += " ppt-cover"
		} else if i == 1 {
			className += " ppt-agenda"
		}
		b.WriteString(`<section class="`)
		b.WriteString(className)
		b.WriteString(`" data-ppt-slide="true">`)
		if i == 0 {
			writePPTCoverSlide(&b, plan, slide, i)
		} else {
			b.WriteString(`<div class="slide-title-wrap"><span class="section-number">`)
			b.WriteString(fmt.Sprintf("%02d", i+1))
			b.WriteString(`</span>`)
			b.WriteString("<h2>")
			b.WriteString(htmlEscape(slide.Title))
			b.WriteString("</h2>")
			b.WriteString(`</div>`)
		}
		if i == 1 && len(slide.Bullets) > 0 {
			b.WriteString(`<div class="dir-list">`)
			for _, bullet := range slide.Bullets {
				b.WriteString(`<div class="dir-item">`)
				b.WriteString(htmlEscape(bullet))
				b.WriteString(`</div>`)
			}
			b.WriteString(`</div>`)
		} else if i == len(plan.Slides)-1 && isEndingSlideTitle(slide.Title) {
			writePPTSummarySlideBodyForPlan(&b, plan, slide)
		} else if i > 0 && len(slide.Bullets) > 0 {
			writePPTContentSlideBody(&b, slide, i)
		}
		b.WriteString(`<div class="slide-progress"><span style="width: `)
		b.WriteString(fmt.Sprintf("%d%%", pptSlideProgressPercent(i+1, len(plan.Slides))))
		b.WriteString(`"></span></div>`)
		b.WriteString("</section>\n")
	}
	rendered := strings.TrimSpace(b.String())
	return replacePPTStyleBlock(rendered, adaptivePPTStyleBlock(plan, theme))
}

// writePPTContentSlideBody 根据布局类型渲染内容页主体。
func writePPTContentSlideBody(b *strings.Builder, slide pptSlidePlan, index int) {
	bullets := slide.Bullets
	if len(bullets) == 0 {
		return
	}
	layout := pptSlideLayoutForIndex(index, slide)
	switch layout {
	case "card-grid":
		writePPTCardGridSlide(b, slide)
	case "full-width":
		writePPTFullWidthListSlide(b, slide)
	case "comparison":
		writePPTComparisonSlide(b, slide)
	case "quote":
		writePPTQuoteSlide(b, slide)
	case "code":
		writePPTCodeSlide(b, slide)
	default:
		writePPTTwoColumnSlide(b, slide)
	}
}
