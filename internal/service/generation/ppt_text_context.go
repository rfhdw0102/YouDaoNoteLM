// ppt_text_context.go 实现 PPT 文本上下文处理。
//
// 在生成 PPT 前对源材料进行文本预处理：
//   - 提取关键段落和要点
//   - 识别代码块、公式、数据等特殊内容
//   - 构建供 LLM 使用的 PPT 专用上下文
package generation

import (
	"fmt"
	"strings"
)

// stripPPTReferenceMetadata 移除内容中的参考资料、章节编号等元信息行。
func stripPPTReferenceMetadata(content string) string {
	refLinePrefixes := []string{
		"文档列表", "文档介绍", "文档概述", "文档目录", "文档内容",
		"专题一", "专题二", "专题三", "专题四", "专题五", "专题六",
		"专题七", "专题八", "专题九", "专题十",
		"第一章", "第二章", "第三章", "第四章", "第五章", "第六章",
		"第七章", "第八章", "第九章", "第十章",
		"第1章", "第2章", "第3章", "第4章", "第5章",
		"第一节", "第二节", "第三节", "第四节", "第五节",
		"【重点知识", "【知识要点", "【核心知识", "【基础知识",
		"【考点分析", "【考点",
		"local references:", "web results:", "web summary:",
		"original markdown:", "input markdown",
	}
	lines := strings.Split(content, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		skip := false
		for _, prefix := range refLinePrefixes {
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

// deduplicatePPTCardTitles 去除 HTML 中重复出现的卡片标题。
func deduplicatePPTCardTitles(content string) string {
	sections := pptExtractSections(content)
	if len(sections) == 0 {
		return content
	}
	modified := false
	for i, sec := range sections {
		newSec, changed := deduplicateCardTitlesInSection(sec)
		if changed {
			sections[i] = newSec
			modified = true
		}
	}
	if !modified {
		return content
	}
	result := content
	for i := len(sections) - 1; i >= 0; i-- {
		lower := strings.ToLower(result)
		start := strings.Index(lower, "<section")
		if start < 0 {
			break
		}
		end := strings.Index(lower[start:], "</section>")
		if end < 0 {
			result = result[:start] + sections[i] + result[start+len(sections[i]):]
			break
		}
		end += start + len("</section>")
		result = result[:start] + sections[i] + result[end:]
	}
	return result
}

// deduplicateCardTitlesInSection 在单个 section 内去除重复的卡片标题。
func deduplicateCardTitlesInSection(section string) (string, bool) {
	lower := strings.ToLower(section)
	titles := extractClassText(section, lower, "card-title")
	if len(titles) < 3 {
		return section, false
	}
	freq := make(map[string]int)
	for _, t := range titles {
		freq[strings.ToLower(strings.TrimSpace(t))]++
	}
	hasDup := false
	for _, c := range freq {
		if c >= 3 {
			hasDup = true
			break
		}
	}
	if !hasDup {
		return section, false
	}
	seen := make(map[string]bool)
	result := section
	searchFrom := 0
	for {
		idx := strings.Index(strings.ToLower(result[searchFrom:]), "card-title")
		if idx < 0 {
			break
		}
		idx += searchFrom
		gtIdx := strings.Index(strings.ToLower(result[idx:]), ">")
		if gtIdx < 0 {
			break
		}
		textStart := idx + gtIdx + 1
		closeIdx := strings.Index(strings.ToLower(result[textStart:]), "<")
		if closeIdx < 0 {
			break
		}
		textEnd := textStart + closeIdx
		titleText := strings.TrimSpace(stripPPTVisibleText(result[textStart:textEnd]))
		titleKey := strings.ToLower(titleText)
		if titleKey == "" {
			searchFrom = textEnd
			continue
		}
		if freq[titleKey] >= 3 {
			if seen[titleKey] {
				result = result[:textStart] + result[textEnd:]
				searchFrom = textStart
			} else {
				seen[titleKey] = true
				searchFrom = textEnd
			}
		} else {
			searchFrom = textEnd
		}
	}
	return result, true
}

// stripPPTHTMLRepeatedTitlePrefix 去除 HTML 正文中重复出现的标题前缀。
func stripPPTHTMLRepeatedTitlePrefix(content string) string {
	sections := pptExtractSections(content)
	if len(sections) == 0 {
		return content
	}
	for _, sec := range sections {
		candidates := pptCollectSectionTitleCandidates(sec)
		if len(candidates) == 0 {
			continue
		}
		rewritten := pptStripBodyTextTitlePrefix(sec, candidates)
		if rewritten == sec {
			continue
		}
		content = strings.Replace(content, sec, rewritten, 1)
	}
	return content
}

// pptCollectSectionTitleCandidates 收集 section 内的各级标题作为前缀候选。
func pptCollectSectionTitleCandidates(section string) []string {
	var candidates []string
	for _, name := range []string{"h1", "h2", "h3", "h4", "h5", "h6"} {
		candidates = append(candidates, pptCollectHeadingTexts(section, name)...)
	}
	lower := strings.ToLower(section)
	candidates = append(candidates, extractClassText(section, lower, "card-title")...)
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		c = strings.TrimSpace(stripPPTVisibleText(c))
		if len([]rune(c)) < 2 {
			continue
		}
		out = append(out, c)
	}
	return out
}

// pptCollectHeadingTexts 提取 section 中指定级别标题的文本。
func pptCollectHeadingTexts(section, name string) []string {
	lower := strings.ToLower(section)
	open := "<" + name
	var texts []string
	searchFrom := 0
	for {
		idx := strings.Index(lower[searchFrom:], open)
		if idx < 0 {
			break
		}
		idx += searchFrom
		gtRel := strings.IndexByte(lower[idx:], '>')
		if gtRel < 0 {
			break
		}
		textStart := idx + gtRel + 1
		closeTag := "</" + name + ">"
		closeRel := strings.Index(lower[textStart:], closeTag)
		if closeRel < 0 {
			searchFrom = textStart
			continue
		}
		raw := section[textStart : textStart+closeRel]
		text := strings.TrimSpace(stripPPTVisibleText(raw))
		if text != "" {
			texts = append(texts, text)
		}
		searchFrom = textStart + closeRel + len(closeTag)
	}
	return texts
}

// pptStripBodyTextTitlePrefix 去除 section 正文中重复的标题前缀，跳过标题与脚本标签。
func pptStripBodyTextTitlePrefix(section string, titles []string) string {
	skipTags := map[string]bool{
		"h1": true, "h2": true, "h3": true,
		"h4": true, "h5": true, "h6": true,
		"style": true, "script": true,
	}
	var b strings.Builder
	b.Grow(len(section))
	lower := strings.ToLower(section)
	pos := 0
	skipDepth := 0
	for pos < len(section) {
		ltRel := strings.IndexByte(lower[pos:], '<')
		if ltRel < 0 {
			b.WriteString(pptApplyTitlePrefixStrip(section[pos:], titles, skipDepth > 0))
			break
		}
		lt := pos + ltRel
		b.WriteString(pptApplyTitlePrefixStrip(section[pos:lt], titles, skipDepth > 0))
		gtRel := strings.IndexByte(lower[lt:], '>')
		if gtRel < 0 {
			b.WriteString(section[lt:])
			break
		}
		gt := lt + gtRel
		tag := section[lt : gt+1]
		b.WriteString(tag)
		name, isClose, selfClose := pptParseTagName(tag)
		if name != "" && skipTags[name] && !selfClose {
			if isClose {
				if skipDepth > 0 {
					skipDepth--
				}
			} else {
				skipDepth++
			}
		}
		pos = gt + 1
	}
	return b.String()
}

// pptApplyTitlePrefixStrip 在非跳过段中去除文本开头的标题前缀。
func pptApplyTitlePrefixStrip(text string, titles []string, skip bool) string {
	if skip || strings.TrimSpace(text) == "" {
		return text
	}
	leading := text[:len(text)-len(strings.TrimLeft(text, " \t\r\n"))]
	body := strings.TrimLeft(text, " \t\r\n")
	stripped := stripPPTBulletTitlePrefixes(body, titles)
	if stripped == body || stripped == "" {
		return text
	}
	return leading + stripped
}

// pptParseTagName 解析 HTML 标签，返回名称及是否为闭合、自闭合标签。
func pptParseTagName(tag string) (name string, isClose bool, selfClose bool) {
	if len(tag) < 2 || tag[0] != '<' {
		return "", false, false
	}
	inner := strings.TrimSpace(tag[1 : len(tag)-1])
	if strings.HasSuffix(inner, "/") {
		selfClose = true
		inner = strings.TrimSpace(strings.TrimSuffix(inner, "/"))
	}
	if strings.HasPrefix(inner, "/") {
		isClose = true
		inner = strings.TrimSpace(inner[1:])
	}
	end := len(inner)
	for i, r := range inner {
		if r == ' ' || r == '\t' || r == '\n' {
			end = i
			break
		}
	}
	return strings.ToLower(inner[:end]), isClose, selfClose
}

// stripTaggedBlock 移除内容中指定标签包裹的整块内容。
func stripTaggedBlock(content, tag string) string {
	lower := strings.ToLower(content)
	openTag := "<" + strings.ToLower(tag) + ">"
	closeTag := "</" + strings.ToLower(tag) + ">"
	for {
		start := strings.Index(lower, openTag)
		if start < 0 {
			return content
		}
		closeStart := strings.Index(lower[start+len(openTag):], closeTag)
		if closeStart < 0 {
			return content[:start] + content[start+len(openTag):]
		}
		end := start + len(openTag) + closeStart + len(closeTag)
		content = content[:start] + content[end:]
		lower = strings.ToLower(content)
	}
}

// appendPPTOutlineToContext 将大纲草案与审查指令追加到 LLM 上下文。
func appendPPTOutlineToContext(contextValue, outline string) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(contextValue))
	if strings.TrimSpace(outline) != "" {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("PPT_OUTLINE_DRAFT\n")
		b.WriteString("以下是需要审查的 PPT 大纲草案：\n")
		b.WriteString(outline)
		b.WriteString("\n\n请审查上述大纲，修正问题后返回完整的大纲。")
	}
	return strings.TrimSpace(b.String())
}

// appendPPTPlansToContext 将大纲、结构化计划与生成规则追加到 LLM 上下文。
func appendPPTPlansToContext(contextValue, outline string, plan pptOutlinePlan) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(contextValue))
	if strings.TrimSpace(outline) != "" {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("内部演示大纲：\n")
		b.WriteString(outline)
	}
	if strings.TrimSpace(plan.Title) != "" || len(plan.Slides) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("STRUCTURED_PPT_PLAN\n")
		b.WriteString(renderPPTPlanForPrompt(plan))
		b.WriteString("\n\nPPT_HTML_GENERATION_RULES\n")
		b.WriteString("- Treat each Slide entry as the writing brief for exactly one HTML <section>; do not merge, omit, or reorder slides.\n")
		b.WriteString("- CRITICAL: The 'source-topic' lines are TOPICS TO EXPAND, not final slide text. You must NOT copy them verbatim into the HTML. Instead, use each topic as a starting point and generate richer, more detailed presentation content.\n")
		b.WriteString("- For each source-topic, generate 1-3 sentences of actual presentation content that: explains the concept, provides context, gives a concrete example, or states a clear conclusion. The output should read like a real slide a presenter would show, not an outline.\n")
		b.WriteString("- Keep every planned slide title visible as h1/h2/h3, then add 3-5 substantial content blocks for that page.\n")
		b.WriteString("- CRITICAL: Never output internal plan labels or meta-text as visible slide content. Words like 'Purpose', '页面目的', 'Slide purpose', 'writing brief', 'source-topic', or any planning instruction must NOT appear on the slide. Only output real presentation content.\n")
		b.WriteString("- CRITICAL: Every content slide MUST have substantial body text. Do not output slides with only a title and empty or near-empty content. Each content slide needs at least 5-7 concrete points, each expanded into a full sentence or paragraph.\n")
		b.WriteString("- CRITICAL: 去文字化——不要输出大段连续叙述文本。将内容拆解为：定义句→原理句→证据/数据句→应用/例子句。每个要点自成一体，读出来像一句完整的演讲词。\n")
		b.WriteString("- CRITICAL: 每个内容页的信息密度要高。一个 slide 至少包含 5-7 个实质性的内容块（列表项、卡片、洞察标签等），不要让页面看起来只有标题和两行字。\n")
		b.WriteString("- CRITICAL: 当 source-topic 看起来笼统（如'核心概念'、'基本原理'）时，你必须将其拆解为 3-5 个具体的子论点分别展开，而不是输出一个笼统的概括。\n")
		b.WriteString("- The agenda slide must match the later content sections one-to-one; if the agenda lists a chapter, a later section with the same chapter title must exist.\n")
		b.WriteString("- Design every section as a real 16:9 PPT canvas: width: 1920px; height: 1080px; overflow: hidden; use px font sizes, not web-card rem layouts.\n")
		b.WriteString("- Make each HTML section production-ready as a standalone slide: include a clear visual hierarchy, title area, content grouping, and a small progress or section marker.\n")
		b.WriteString("- Page copy must be grounded in Original Markdown, Local References, Web Results, or the user's explicit prompt. Do not add generic business, market, team, investor, or motivational boilerplate unless it appears in the source.\n")
		b.WriteString("- Expand terse bullets into concise slide-ready statements only by clarifying the source meaning; do not introduce unrelated examples, claims, or slogans.\n")
		b.WriteString("- Finish all planned sections before returning. If the plan is long, make each section concise instead of truncating the deck.\n")
		b.WriteString("\nCONTENT_EXPANSION_GUIDE\n")
		b.WriteString("The source-topic lines are writing prompts, NOT final content. Here is how to expand them:\n\n")
		b.WriteString("BAD (copying outline verbatim):\n")
		b.WriteString("  <li>Go语言是静态类型</li>\n")
		b.WriteString("  <li>Go语言有垃圾回收</li>\n\n")
		b.WriteString("GOOD (expanding topics into real content):\n")
		b.WriteString("  <li>Go 语言采用静态类型系统，在编译期即可发现类型错误，大幅减少运行时崩溃风险。与动态语言相比，这牺牲了一些灵活性，但换来了更高的执行效率和代码可维护性。</li>\n")
		b.WriteString("  <li>Go 内置并发原语 goroutine 和 channel，配合运行时垃圾回收器，开发者无需手动管理内存即可构建高并发服务。GC 采用三色标记算法，暂停时间通常在毫秒级。</li>\n\n")
		b.WriteString("Rule: each source-topic must become at least one full sentence (30+ characters) with explanation, context, or example from the source material.\n")
	}
	return strings.TrimSpace(b.String())
}

// appendPPTRichContentToContext 将增强后的幻灯片内容追加到 LLM 上下文。
func appendPPTRichContentToContext(contextValue string, richContent pptRichContent) string {
	if len(richContent.Slides) == 0 {
		return contextValue
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(contextValue))
	b.WriteString("\n\nENRICHED_PPT_CONTENT\n")
	b.WriteString("以下是已经充实完成的幻灯片内容，请直接使用这些段落和要点生成 HTML：\n\n")
	for i, slide := range richContent.Slides {
		b.WriteString(fmt.Sprintf("Slide %02d: %s\n", i+1, slide.Title))
		if slide.Subtitle != "" {
			b.WriteString("  副标题: ")
			b.WriteString(slide.Subtitle)
			b.WriteString("\n")
		}
		if len(slide.Paragraphs) > 0 {
			b.WriteString("  正文段落:\n")
			for _, p := range slide.Paragraphs {
				b.WriteString("    - ")
				b.WriteString(p)
				b.WriteString("\n")
			}
		}
		if len(slide.Bullets) > 0 {
			b.WriteString("  要点:\n")
			for _, bullet := range slide.Bullets {
				b.WriteString("    - ")
				b.WriteString(bullet)
				b.WriteString("\n")
			}
		}
		if len(slide.Insights) > 0 {
			b.WriteString("  洞察:\n")
			for _, insight := range slide.Insights {
				b.WriteString("    - ")
				b.WriteString(insight)
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("IMPORTANT: 使用上述充实后的内容生成 HTML，不要使用 STRUCTURED_PPT_PLAN 中的 source-topic。\n")
	return strings.TrimSpace(b.String())
}

// renderPPTPlanForPrompt 将结构化大纲渲染为供 LLM 阅读的纯文本。
func renderPPTPlanForPrompt(plan pptOutlinePlan) string {
	var b strings.Builder
	if strings.TrimSpace(plan.Title) != "" {
		b.WriteString("Title: ")
		b.WriteString(strings.TrimSpace(plan.Title))
		b.WriteString("\n")
	}
	for i, slide := range plan.Slides {
		b.WriteString(fmt.Sprintf("Slide %02d: %s\n", i+1, strings.TrimSpace(slide.Title)))
		for _, bullet := range slide.Bullets {
			bullet = strings.TrimSpace(bullet)
			if bullet == "" {
				continue
			}
			b.WriteString("  source-topic: ")
			b.WriteString(bullet)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}
