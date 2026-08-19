package generation

import (
	"fmt"
	"strings"
)

func ensurePPTStyleBlockForPlan(content string, plan *pptOutlinePlan, theme pptStyleTheme) string {
	if strings.Contains(strings.ToLower(content), "<style") {
		return content
	}
	var resolved pptOutlinePlan
	if plan != nil {
		resolved = *plan
	}
	return adaptivePPTStyleBlock(resolved, theme) + "\n" + content
}

func adaptivePPTStyleBlock(plan pptOutlinePlan, theme pptStyleTheme) string {
	paddingX, paddingY, gap, bodySize, headingSize := pptAdaptiveMetrics(plan)
	surfaceSoft := pptAccentSoft(theme)
	border := pptThemeBorder(theme)
	panel := pptThemePanel(theme)
	cover := pptThemeCoverGradient(theme)

	return fmt.Sprintf(`<style>
:root {
  --bg: %s;
  --surface: %s;
  --surface-soft: %s;
  --accent: %s;
  --accent-2: %s;
  --accent-soft: %s;
  --text: %s;
  --muted: %s;
  --heading: %s;
  --border: %s;
  --panel: %s;
}
* { box-sizing: border-box; }
.ppt-slide {
  position: relative;
  width: 1920px;
  height: 1080px;
  overflow: hidden;
  padding: %dpx %dpx;
  display: flex;
  flex-direction: column;
  gap: %dpx;
  background: var(--surface);
  color: var(--text);
  font-family: %s;
}
.ppt-slide::after {
  content: "";
  position: absolute;
  right: 0;
  bottom: 0;
  width: %dpx;
  height: 12px;
  background: linear-gradient(90deg, var(--accent), var(--accent-2));
}
.ppt-cover {
  justify-content: center;
  padding: %dpx %dpx;
  background: %s;
}
.ppt-agenda { background: var(--bg); }
.cover-meta, .slide-title-wrap {
  display: flex;
  align-items: flex-start;
  gap: 24px;
}
.slide-title-wrap { justify-content: space-between; }
.section-number {
  flex: 0 0 auto;
  padding: 8px 18px;
  border-radius: 999px;
  background: var(--accent-soft);
  color: var(--accent);
  font-size: 24px;
  font-weight: 800;
}
h1, h2, h3 { color: var(--heading); margin: 0; }
h1 { font-size: %dpx; line-height: 1.08; font-weight: 850; }
h2 { flex: 1; font-size: %dpx; line-height: 1.12; font-weight: 800; }
h3 { font-size: 32px; line-height: 1.18; font-weight: 800; }
p, li { font-size: %dpx; line-height: 1.34; }
ul { list-style: none; padding: 0; margin: 0; }
li {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  padding: 14px 0;
  border-bottom: 1px solid var(--border);
}
li::before {
  content: "";
  width: 11px;
  height: 11px;
  flex: 0 0 auto;
  margin-top: 12px;
  border-radius: 50%%;
  background: var(--accent);
}
li:last-child { border-bottom: 0; }
.content-grid, .summary-layout, .comparison-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.25fr) minmax(0, .75fr);
  gap: %dpx;
  align-items: stretch;
}
.card-grid, .dir-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: %dpx;
}
.main-points, .full-width-list {
  padding-top: 10px;
  border-top: 6px solid var(--accent);
}
.insight-panel, .content-card, .summary-card, .summary-actions, .comparison-col, .quote-block, .dir-item {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel);
  padding: 28px 32px;
}
.insight-panel, .quote-block { display: flex; flex-direction: column; justify-content: center; gap: 18px; }
.insight-token, .card-title, .dir-item { color: var(--heading); font-weight: 760; }
.insight-token { font-size: 28px; line-height: 1.22; }
.content-card { display: flex; flex-direction: column; gap: 14px; border-left: 6px solid var(--accent); }
.content-card .card-title { font-size: 30px; }
.content-card .card-body { font-size: %dpx; line-height: 1.3; }
.comparison-col.left { border-left: 6px solid var(--accent); background: var(--accent-soft); }
.comparison-col.right { border-left: 6px solid var(--accent-2); }
.summary-card { border-left: 8px solid var(--accent); background: var(--surface); }
.summary-actions { background: var(--panel); }
.quote-block { border-left: 8px solid var(--accent); background: linear-gradient(135deg, var(--accent-soft), var(--panel)); }
.quote-text { font-size: 36px; line-height: 1.28; font-weight: 700; color: var(--heading); }
.ppt-code-block {
  margin: 12px 0 0;
  padding: 24px 28px;
  overflow: hidden;
  white-space: pre-wrap;
  word-break: break-word;
  border-left: 6px solid var(--accent-2);
  border-radius: 8px;
  background: %s;
  color: %s;
  font-family: 'Cascadia Code', 'Consolas', monospace;
  font-size: 22px;
  line-height: 1.42;
}
.slide-progress {
  position: absolute;
  left: %dpx;
  right: %dpx;
  bottom: 42px;
  height: 8px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--border);
}
.slide-progress span { display: block; height: 100%%; background: linear-gradient(90deg, var(--accent), var(--accent-2)); }
</style>`,
		theme.Background,
		theme.Surface,
		theme.Surface,
		theme.Primary,
		theme.Secondary,
		surfaceSoft,
		theme.Text,
		theme.Muted,
		theme.Heading,
		border,
		panel,
		paddingY,
		paddingX,
		gap,
		theme.FontBody,
		paddingX*2,
		paddingY+24,
		paddingX+24,
		cover,
		headingSize+22,
		headingSize,
		bodySize,
		gap,
		gap/2+12,
		bodySize-2,
		theme.Background,
		theme.Text,
		paddingX,
		paddingX,
	)
}

func pptAdaptiveMetrics(plan pptOutlinePlan) (paddingX, paddingY, gap, bodySize, headingSize int) {
	maxBullets := 0
	maxTitle := 0
	for _, slide := range plan.Slides {
		if len(slide.Bullets) > maxBullets {
			maxBullets = len(slide.Bullets)
		}
		if n := utf8RuneCount(slide.Title); n > maxTitle {
			maxTitle = n
		}
	}

	paddingX = 112
	paddingY = 86
	gap = 30
	bodySize = 32
	headingSize = 52
	if maxBullets >= 7 {
		paddingY = 66
		gap = 22
		bodySize = 28
		headingSize = 48
	}
	if maxBullets <= 3 {
		paddingY = 104
		gap = 36
		bodySize = 34
	}
	if maxTitle >= 18 {
		headingSize -= 4
	}
	return
}

func replacePPTStyleBlock(content, styleBlock string) string {
	lower := strings.ToLower(content)
	start := strings.Index(lower, "<style")
	if start < 0 {
		return styleBlock + "\n" + content
	}
	end := strings.Index(lower[start:], "</style>")
	if end < 0 {
		return styleBlock + "\n" + content[start:]
	}
	end += start + len("</style>")
	return content[:start] + styleBlock + content[end:]
}
