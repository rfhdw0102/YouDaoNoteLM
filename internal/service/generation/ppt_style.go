// ppt_style.go 实现 PPT 样式主题生成。
//
// 根据内容特征选择配色方案、字体、布局间距等样式参数，
// 生成 CSS 样式表供 ppt_html_render_*.go 使用。
package generation

import (
	"fmt"
	"strings"
)

var styleHintMap = []struct {
	patterns []string
	themeIdx int
	desc     string
}{
	// 科技深色
	{[]string{"深色", "dark", "科技", "tech", "极客", "geek", "赛博", "cyber", "霓虹", "neon", "暗黑", "midnight", "黑客", "hacker", "渐变蓝", "渐变", "glow", "发光", "脉冲", "pulse"}, 2, "用户要求深色/科技风格"},
	// 学术清新
	{[]string{"商务", "business", "简约", "minimal", "专业", "professional", "经典", "classic", "古典", "antique", "庄重", "solemn", "古风", "传统", "traditional", "莫兰迪", "morandi", "低饱和", "低度饱和"}, 0, "用户要求商务/简约风格"},
	{[]string{"学术", "academic", "论文", "paper", "研究", "research", "论文风格", "学院", "institute", "知网", "期刊", "journal"}, 1, "用户要求学术风格"},
	// 暖色叙事
	{[]string{"暖色", "warm", "叙事", "narrative", "故事", "story", "温暖", "温暖色调", "治愈", "healing", "柔和", "soft", "温馨", "cozy", "橙", "orange", "粉", "pink", "日落", "sunset", " autobiograph"}, 3, "用户要求暖色/叙事风格"},
}

func extractStyleHintFromPrompt(prompt string) (int, string) {
	promptLower := strings.ToLower(prompt)
	words := splitKeywordCandidates(promptLower)
	for _, entry := range styleHintMap {
		for _, pattern := range entry.patterns {
			patternLower := strings.ToLower(pattern)
			if strings.Contains(promptLower, patternLower) {
				return entry.themeIdx, entry.desc
			}
			for _, word := range words {
				if word == patternLower || strings.Contains(word, patternLower) {
					return entry.themeIdx, entry.desc
				}
			}
		}
	}
	return -1, ""
}

func designPPTStyleTheme(analysis learningContentAnalysis, plan pptOutlinePlan, userPrompt string, styleHint string) pptStyleTheme {
	themes := []pptStyleTheme{
		{
			Name: "简约商务", Primary: "#0f766e", Secondary: "#c2410c",
			Background: "#f8fafc", Surface: "#ffffff", Text: "#111827",
			Heading: "#0f172a", Muted: "#4b5563",
			FontHeading: "'Segoe UI', 'Microsoft YaHei', sans-serif",
			FontBody:    "system-ui, 'Segoe UI', 'Microsoft YaHei', sans-serif",
			LayoutHints: "Use clean grids, generous whitespace, and subtle accent borders. " +
				"Content slides should use 2-column layouts with main points on the left and insight cards on the right.",
		},
		{
			Name: "学术清新", Primary: "#1e40af", Secondary: "#7c3aed",
			Background: "#fefce8", Surface: "#ffffff", Text: "#1e293b",
			Heading: "#0c1e3e", Muted: "#64748b",
			FontHeading: "'Georgia', 'Times New Roman', serif",
			FontBody:    "system-ui, 'Segoe UI', sans-serif",
			LayoutHints: "Use serif headings for an academic feel. " +
				"Slides should have numbered sections, definition cards, and comparison tables.",
		},
		{
			Name: "科技深色", Primary: "#06b6d4", Secondary: "#f59e0b",
			Background: "#0f172a", Surface: "#1e293b", Text: "#e2e8f0",
			Heading: "#f1f5f9", Muted: "#94a3b8",
			FontHeading: "'Segoe UI', sans-serif",
			FontBody:    "system-ui, 'Segoe UI', sans-serif",
			LayoutHints: "Dark background with bright accent text. " +
				"Use glowing borders, monospace code blocks, and data-driven charts/diagrams.",
		},
		{
			Name: "暖色叙事", Primary: "#be123c", Secondary: "#0369a1",
			Background: "#fff7ed", Surface: "#ffffff", Text: "#1c1917",
			Heading: "#451a03", Muted: "#78716c",
			FontHeading: "'Segoe UI', 'Microsoft YaHei', sans-serif",
			FontBody:    "system-ui, 'Segoe UI', 'Microsoft YaHei', sans-serif",
			LayoutHints: "Warm palette for storytelling. " +
				"Use large quote blocks, timeline layouts, and photo-placeholder cards.",
		},
	}

	userStyleHint := ""
	themeIdx := -1
	styleHint = strings.TrimSpace(styleHint)
	if styleHint != "" {
		if idx, ok := matchThemeByName(styleHint); ok {
			themeIdx = idx
			userStyleHint = "用户显式指定风格: " + styleHint
		} else if idx, desc := extractStyleHintFromPrompt(styleHint); idx >= 0 {
			themeIdx = idx
			userStyleHint = desc
		}
	}

	// 2. 其次解析用户提示词中的风格关键词
	if themeIdx < 0 {
		if idx, desc := extractStyleHintFromPrompt(userPrompt); idx >= 0 {
			themeIdx = idx
			userStyleHint = desc
		}
	}

	// 3. 如果用户没有指定风格，根据内容主题自动选择
	if themeIdx < 0 {
		topic := strings.ToLower(analysis.Topic)
		themeIdx = 0
		switch {
		case containsAnyFold(topic, "代码", "编程", "算法", "系统", "架构", "数据", "code", "programming", "algorithm", "system", "architecture", "data"):
			themeIdx = 2
		case containsAnyFold(topic, "论文", "研究", "理论", "学术", "paper", "research", "theory", "academic"):
			themeIdx = 1
		case containsAnyFold(topic, "故事", "案例", "历史", "叙事", "story", "case", "history", "narrative"):
			themeIdx = 3
		}
	}

	theme := themes[themeIdx%len(themes)]
	if len(plan.Slides) > 10 {
		theme.LayoutHints += " With many slides, keep each slide focused on one key idea."
	}
	if userStyleHint != "" {
		theme.LayoutHints = userStyleHint + ". " + theme.LayoutHints
	}
	return theme
}

func matchThemeByName(style string) (int, bool) {
	style = strings.ToLower(strings.TrimSpace(style))
	if style == "" {
		return 0, false
	}
	switch style {
	case "auto", "automatic", "自动":
		return -1, true // -1 表示交给自动路径决定
	case "minimal", "business", "简约", "简约商务", "商务", "minimal-business":
		return 0, true
	case "academic", "清新", "学术", "学术清新", "scholarly":
		return 1, true
	case "tech", "dark", "科技", "深色", "科技深色", "technology", "geek":
		return 2, true
	case "warm", "narrative", "暖色", "叙事", "暖色叙事", "story":
		return 3, true
	}
	for i, name := range []string{"简约商务", "学术清新", "科技深色", "暖色叙事"} {
		if strings.Contains(style, strings.ToLower(name)) {
			return i, true
		}
	}
	return 0, false
}

func pptThemeVisualDescription(name string) string {
	switch name {
	case "简约商务":
		return "浅色背景、大面积留白、细线分隔、青绿色主色调点缀橙色强调；扁平卡片、双栏布局；整体克制、专业、商务感强。"
	case "学术清新":
		return "米黄底色、衬线标题（Georgia/宋体感）、蓝紫色配色、编号小节、定义卡片与对比表格；严谨、学院风、文献感。"
	case "科技深色":
		return "深色背景（深蓝/近黑）、亮色文字、青色与琥珀色高对比强调、发光描边、等宽代码块、数据图表感；未来、科技、极客风。"
	case "暖色叙事":
		return "暖白底色、酒红与深蓝配色、大号引语块、时间线布局、图片占位卡；温暖、叙事、故事感。"
	}
	return ""
}

func appendPPTStyleToContext(contextValue string, theme pptStyleTheme) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(contextValue))
	b.WriteString("\n\nPPT_STYLE_THEME\n")
	b.WriteString(fmt.Sprintf("Theme: %s\n", theme.Name))
	b.WriteString(fmt.Sprintf("Primary: %s\n", theme.Primary))
	b.WriteString(fmt.Sprintf("Secondary: %s\n", theme.Secondary))
	b.WriteString(fmt.Sprintf("Background: %s\n", theme.Background))
	b.WriteString(fmt.Sprintf("Surface: %s\n", theme.Surface))
	b.WriteString(fmt.Sprintf("Text: %s\n", theme.Text))
	b.WriteString(fmt.Sprintf("Heading: %s\n", theme.Heading))
	b.WriteString(fmt.Sprintf("Muted: %s\n", theme.Muted))
	b.WriteString(fmt.Sprintf("FontHeading: %s\n", theme.FontHeading))
	b.WriteString(fmt.Sprintf("FontBody: %s\n", theme.FontBody))
	b.WriteString("VisualDescription: ")
	b.WriteString(pptThemeVisualDescription(theme.Name))
	b.WriteString("\n")
	b.WriteString("LayoutGuidance: ")
	b.WriteString(theme.LayoutHints)
	b.WriteString("\n\nSTYLE_USAGE_RULES\n")
	b.WriteString("- 你必须严格使用上方 PPT_STYLE_THEME 中的配色与字体作为 :root 的 CSS 自定义属性（--bg、--surface、--accent、--accent-2、--accent-soft、--text、--muted、--heading、--border、--panel 等），不要自行替换为其他颜色。\n")
	b.WriteString("- VisualDescription 描述了该风格的视觉特征，你的 CSS 必须忠实还原这些特征（如深色背景、衬线标题、暖色渐变等），而不是回归通用简约样式。\n")
	b.WriteString("- 背景色必须使用 Background 值（深色主题尤其重要：背景必须是深色，文字必须是亮色），不得用浅色背景覆盖深色主题。\n")
	b.WriteString("- Use the CSS variables above as :root custom properties in your <style> block.\n")
	b.WriteString("- Vary slide layouts: not every content slide should look the same.\n")
	b.WriteString("- Use at least 3 different layout patterns across the deck (e.g. two-column, full-width, card-grid, comparison, timeline).\n")
	b.WriteString("- Each slide must be visually distinct while sharing the same color palette and typography.\n")
	return strings.TrimSpace(b.String())
}

func ensurePPTSlideAttributes(content string) string {
	lower := strings.ToLower(content)
	if !strings.Contains(lower, "<section") {
		return content
	}
	var b strings.Builder
	pos := 0
	for {
		start := strings.Index(lower[pos:], "<section")
		if start < 0 {
			b.WriteString(content[pos:])
			break
		}
		start += pos
		end := strings.Index(content[start:], ">")
		if end < 0 {
			b.WriteString(content[pos:])
			break
		}
		end += start
		tag := content[start : end+1]
		b.WriteString(content[pos:start])
		tagLower := strings.ToLower(tag)
		if strings.Contains(tagLower, "class=") {
			if !strings.Contains(tagLower, "ppt-slide") {
				tag = strings.Replace(tag, "class=\"", "class=\"ppt-slide ", 1)
				tag = strings.Replace(strings.ToLower(tag), "class='", "class='ppt-slide ", 1)
			}
		} else {
			tag = strings.TrimSuffix(tag, ">") + ` class="ppt-slide" data-ppt-slide="true">`
		}
		if !strings.Contains(strings.ToLower(tag), "data-ppt-slide") {
			tag = strings.TrimSuffix(tag, ">") + ` data-ppt-slide="true">`
		}
		b.WriteString(tag)
		pos = end + 1
	}
	return b.String()
}

func ensurePPTCanvasSize(content string) string {
	lower := strings.ToLower(content)
	if strings.Contains(lower, "width:1920px") || strings.Contains(lower, "width: 1920px") {
		if strings.Contains(lower, "height:1080px") || strings.Contains(lower, "height: 1080px") {
			return content
		}
	}
	if !strings.Contains(lower, "<style") {
		return content
	}
	canvasCSS := `.ppt-slide, section.ppt-slide {
  width: 1920px;
  height: 1080px;
  overflow: hidden;
  position: relative;
  box-sizing: border-box;
}`
	styleClose := strings.Index(lower, "</style>")
	if styleClose < 0 {
		return content
	}
	insertPos := styleClose
	return content[:insertPos] + canvasCSS + "\n" + content[insertPos:]
}

func ensurePPTStyleBlock(content string) string {
	lower := strings.ToLower(content)
	if strings.Contains(lower, "<style") {
		return content
	}
	if !strings.Contains(lower, "<section") {
		return content
	}
	minimalCSS := `<style>
.ppt-slide { width: 1920px; height: 1080px; overflow: hidden; position: relative; box-sizing: border-box; padding: 80px 100px; background: #ffffff; color: #111827; font-family: system-ui, 'Segoe UI', 'Microsoft YaHei', sans-serif; }
.ppt-slide h1 { font-size: 72px; font-weight: 800; }
.ppt-slide h2 { font-size: 48px; font-weight: 700; }
.ppt-slide p, .ppt-slide li { font-size: 30px; line-height: 1.4; }
</style>`
	return minimalCSS + "\n" + content
}

func extractCSSBlock(output string) string {
	lower := strings.ToLower(output)
	start := strings.Index(lower, "<style")
	if start < 0 {
		return "<style>\n" + output + "\n</style>"
	}
	end := strings.Index(lower, "</style>")
	if end < 0 {
		return output + "\n</style>"
	}
	return output[start : end+len("</style>")]
}

func fallbackPPTCSS(theme pptStyleTheme) string {
	return fmt.Sprintf(`<style>
:root {
  --bg: %s; --surface: %s; --surface-soft: %s;
  --accent: %s; --accent-2: %s; --accent-soft: %s;
  --text: %s; --muted: %s; --heading: %s;
  --border: %s; --panel: %s;
}
* { margin: 0; padding: 0; box-sizing: border-box; }
.ppt-slide {
  position: relative;
  width: 1920px;
  height: 1080px;
  overflow: hidden;
  background: var(--surface);
  color: var(--text);
  font-family: %s;
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
  background: %s;
  padding: 120px 150px;
}
h1 { font-size: 76px; font-weight: 850; color: var(--heading); line-height: 1.12; }
h2 { font-size: 52px; font-weight: 800; color: var(--heading); line-height: 1.18; }
h3 { font-size: 34px; font-weight: 700; color: var(--heading); }
p, li { font-size: 32px; line-height: 1.36; }
ul { list-style: none; padding-left: 0; }
.content-grid { display: grid; grid-template-columns: minmax(0, 1.3fr) minmax(420px, .7fr); gap: 54px; }
.main-points { background: var(--surface); border-top: 6px solid var(--accent); padding: 12px 0 0; }
.insight-panel { min-height: 360px; background: linear-gradient(180deg, var(--accent-soft) 0%%, var(--panel) 100%%); border: 1px solid var(--border); border-radius: 8px; padding: 34px; }
.card-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 32px; }
.content-card { min-height: 280px; background: var(--surface); border: 1px solid var(--border); border-left: 6px solid var(--accent); border-radius: 8px; padding: 32px 36px; }
.full-width-list { background: var(--surface); border-top: 6px solid var(--accent); padding: 12px 0 0; }
.comparison-layout { display: grid; grid-template-columns: 1fr 1fr; gap: 42px; }
.comparison-col { min-height: 400px; border-radius: 8px; padding: 36px; }
.comparison-col.left { background: var(--accent-soft); border-left: 6px solid var(--accent); }
.comparison-col.right { background: var(--panel); border-left: 6px solid var(--accent-2); }
.quote-block { background: linear-gradient(135deg, var(--accent-soft) 0%%, var(--panel) 100%%); border-left: 8px solid var(--accent); border-radius: 8px; padding: 48px 56px; min-height: 400px; }
.ppt-code-block { background: #1e293b; color: #e2e8f0; border-radius: 8px; padding: 28px 32px; margin-top: 18px; font-family: 'Cascadia Code', 'Fira Code', 'Consolas', 'Courier New', monospace; font-size: 24px; line-height: 1.5; white-space: pre-wrap; word-break: break-all; overflow-x: auto; border-left: 6px solid var(--accent-2); }
.ppt-code-block code { font-family: inherit; color: inherit; }
.slide-progress { position: absolute; left: 112px; right: 112px; bottom: 58px; height: 10px; border-radius: 999px; background: #e5e7eb; overflow: hidden; }
.slide-progress span { display: block; height: 100%%; border-radius: inherit; background: linear-gradient(90deg, var(--accent), var(--accent-2)); }
</style>`,
		theme.Background, theme.Surface, theme.Surface,
		theme.Primary, theme.Secondary, pptAccentSoft(theme),
		theme.Text, theme.Muted, theme.Heading,
		pptThemeBorder(theme), pptThemePanel(theme),
		pptThemeCoverGradient(theme),
		theme.FontBody)
}

func pptAccentSoft(theme pptStyleTheme) string {
	switch theme.Name {
	case "科技深色":
		return "#1e3a5f"
	case "学术清新":
		return "#eef2ff"
	case "暖色叙事":
		return theme.Primary + "1a"
	default:
		return theme.Primary + "22"
	}
}

func pptThemeBorder(theme pptStyleTheme) string {
	switch theme.Name {
	case "科技深色":
		return "#334155"
	case "学术清新":
		return "#dbe4f0"
	case "暖色叙事":
		return "#f0d9c4"
	default:
		return "#d7e3df"
	}
}

func pptThemePanel(theme pptStyleTheme) string {
	switch theme.Name {
	case "科技深色":
		return "#1e293b"
	case "学术清新":
		return "#eef2ff"
	case "暖色叙事":
		return "#fff1e6"
	default:
		return "#f3f7f6"
	}
}

func pptThemeCoverGradient(theme pptStyleTheme) string {
	switch theme.Name {
	case "科技深色":
		return "linear-gradient(135deg, #0f172a 0%, #1e293b 58%, #0c2438 100%)"
	case "学术清新":
		return "linear-gradient(135deg, #fefce8 0%, #eef2ff 58%, #faf5ff 100%)"
	case "暖色叙事":
		return "linear-gradient(135deg, #fff7ed 0%, #ffe4e6 58%, #ffedd5 100%)"
	default:
		return "linear-gradient(135deg, #f7fbfa 0%, #e8f2ef 58%, #fff7ed 100%)"
	}
}

func pptCSSHasCanvasSize(css string) bool {
	lower := strings.ToLower(css)
	hasWidth := strings.Contains(lower, "width:1920px") || strings.Contains(lower, "width: 1920px")
	hasHeight := strings.Contains(lower, "height:1080px") || strings.Contains(lower, "height: 1080px")
	return hasWidth && hasHeight
}

func injectPPTCanvasSizeIntoCSS(css string) string {
	lower := strings.ToLower(css)
	styleClose := strings.Index(lower, "</style>")
	if styleClose < 0 {
		return css
	}
	canvasCSS := "\n.ppt-slide, section.ppt-slide {\n  width: 1920px;\n  height: 1080px;\n  overflow: hidden;\n  position: relative;\n  box-sizing: border-box;\n}\n"
	return css[:styleClose] + canvasCSS + css[styleClose:]
}

func appendPPTCSSToContext(contextValue string, cssBlock string) string {
	if strings.TrimSpace(cssBlock) == "" {
		return contextValue
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(contextValue))
	b.WriteString("\n\nPRE_GENERATED_CSS\n")
	b.WriteString("以下 CSS 已由设计步骤生成，请直接复用，不要重新设计样式：\n")
	b.WriteString(cssBlock)
	b.WriteString("\n\nCSS_REUSE_RULES\n")
	b.WriteString("- 上方 <style> 块已包含完整的视觉设计，你的 HTML 输出必须复用其中的 CSS 类名。\n")
	b.WriteString("- 不要在 HTML 中再输出 <style> 块，直接使用已有的 CSS 类。\n")
	b.WriteString("- 如果需要额外样式，可以用内联 style 属性补充，但不要覆盖已有的画布尺寸。\n")
	b.WriteString("- 专注生成 <section> HTML 结构和内容，让每页布局有变化。\n")
	return strings.TrimSpace(b.String())
}
