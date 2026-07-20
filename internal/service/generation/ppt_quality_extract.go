// ppt_quality_extract.go 实现 PPT 质量检查的内容提取函数。
//
// 从 PPT HTML 中提取页面结构、标题层级、要点列表等，
// 供 ppt_quality_basic.go 的校验函数使用。
package generation

import (
	"strings"
)

// pptNeedsHTMLQualityRepair 判断 HTML 是否存在画布缺失或弱样式等问题。
func pptNeedsHTMLQualityRepair(content string) bool {
	lower := strings.ToLower(content)
	if !strings.Contains(lower, "<section") {
		return true
	}
	if !strings.Contains(lower, "width:1920px") && !strings.Contains(lower, "width: 1920px") {
		return true
	}
	if !strings.Contains(lower, "height:1080px") && !strings.Contains(lower, "height: 1080px") {
		return true
	}
	weakTokens := []string{
		"font-size:1rem",
		"font-size: 1rem",
		"font-size:.",
		"font-size: .",
		"max-width:1100px",
		"max-width: 1100px",
		"max-width:960px",
		"max-width: 960px",
		"height:100vh",
		"height: 100vh",
		"width:100%",
		"width: 100%",
	}
	for _, token := range weakTokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

// pptHasSparseSlides 判断是否存在内容过少的稀疏幻灯片。
func pptHasSparseSlides(content string) bool {
	sections := pptExtractSections(content)
	if len(sections) == 0 {
		return false
	}
	sparseCount := 0
	for _, sec := range sections {
		text := strings.TrimSpace(stripPPTVisibleText(sec))
		text = strings.Join(strings.Fields(text), " ")
		charCount := len([]rune(text))
		if charCount < 80 {
			sparseCount++
		}
	}
	return sparseCount >= 2 || (len(sections) > 0 && sparseCount == len(sections))
}

// pptExtractSections 从 HTML 中提取所有 section 块。
func pptExtractSections(content string) []string {
	lower := strings.ToLower(content)
	var sections []string
	searchFrom := 0
	for {
		start := strings.Index(lower[searchFrom:], "<section")
		if start < 0 {
			break
		}
		start += searchFrom
		end := strings.Index(lower[start:], "</section>")
		if end < 0 {
			sections = append(sections, content[start:])
			break
		}
		end += start + len("</section>")
		sections = append(sections, content[start:end])
		searchFrom = end
	}
	return sections
}

// pptHasDuplicatedSlideTitles 判断单张幻灯片内是否存在重复的标题。
func pptHasDuplicatedSlideTitles(content string) bool {
	sections := pptExtractSections(content)
	if len(sections) == 0 {
		return false
	}
	for _, sec := range sections {
		titles := pptExtractSlideTitles(sec)
		if len(titles) < 3 {
			continue
		}
		freq := make(map[string]int)
		for _, t := range titles {
			key := strings.ToLower(strings.TrimSpace(t))
			if key == "" {
				continue
			}
			freq[key]++
		}
		for _, count := range freq {
			if count >= 3 {
				return true
			}
		}
	}
	return false
}

// pptHasMismatchedCardContent 判断卡片标题与正文是否大量不匹配。
func pptHasMismatchedCardContent(content string) bool {
	sections := pptExtractSections(content)
	if len(sections) == 0 {
		return false
	}
	mismatchCount := 0
	for _, sec := range sections {
		cards := pptExtractContentCards(sec)
		for _, card := range cards {
			if !pptCardTitleMatchesBody(card.title, card.body) {
				mismatchCount++
			}
		}
	}
	return mismatchCount >= 3
}

type pptCardPair struct {
	title string
	body  string
}

// pptExtractContentCards 从 section 中提取卡片的标题与正文配对。
func pptExtractContentCards(section string) []pptCardPair {
	var cards []pptCardPair
	lower := strings.ToLower(section)
	searchFrom := 0
	for {
		idx := strings.Index(lower[searchFrom:], "content-card")
		if idx < 0 {
			break
		}
		idx += searchFrom
		cardStart := idx
		gtIdx := strings.Index(lower[cardStart:], ">")
		if gtIdx < 0 {
			break
		}
		cardContentStart := cardStart + gtIdx + 1
		titleStart := strings.Index(lower[cardContentStart:], "card-title")
		if titleStart < 0 {
			searchFrom = cardContentStart
			continue
		}
		titleStart += cardContentStart
		titleGt := strings.Index(lower[titleStart:], ">")
		if titleGt < 0 {
			searchFrom = cardContentStart
			continue
		}
		titleTextStart := titleStart + titleGt + 1
		titleEnd := strings.Index(lower[titleTextStart:], "<")
		if titleEnd < 0 {
			searchFrom = cardContentStart
			continue
		}
		title := strings.TrimSpace(stripPPTVisibleText(section[titleTextStart : titleTextStart+titleEnd]))

		bodyStart := strings.Index(lower[titleTextStart+titleEnd:], "card-body")
		if bodyStart < 0 {
			searchFrom = titleTextStart + titleEnd
			continue
		}
		bodyStart += titleTextStart + titleEnd
		bodyGt := strings.Index(lower[bodyStart:], ">")
		if bodyGt < 0 {
			searchFrom = bodyStart
			continue
		}
		bodyTextStart := bodyStart + bodyGt + 1
		bodyEnd := strings.Index(lower[bodyTextStart:], "<")
		if bodyEnd < 0 {
			searchFrom = bodyTextStart
			continue
		}
		body := strings.TrimSpace(stripPPTVisibleText(section[bodyTextStart : bodyTextStart+bodyEnd]))

		if title != "" && body != "" {
			cards = append(cards, pptCardPair{title: title, body: body})
		}
		searchFrom = bodyTextStart + bodyEnd
	}
	return cards
}

// pptCardTitleMatchesBody 判断卡片标题与正文是否相互匹配。
func pptCardTitleMatchesBody(title, body string) bool {
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	if title == "" || body == "" {
		return true // 无法判断时不标记异常
	}
	if strings.ToLower(title) == strings.ToLower(body) {
		return false
	}
	if utf8RuneCount(title) <= 2 && utf8RuneCount(body) > 20 {
		return false
	}
	titleWords := extractSignificantWords(title)
	if len(titleWords) == 0 {
		return true // 无法判断
	}
	bodyLower := strings.ToLower(body)
	overlap := 0
	for _, word := range titleWords {
		if strings.Contains(bodyLower, strings.ToLower(word)) {
			overlap++
		}
	}
	if overlap == 0 && utf8RuneCount(body) > 15 {
		return false
	}
	return true
}

// extractSignificantWords 从文本中提取去停用词后的有效关键词。
func extractSignificantWords(text string) []string {
	text = strings.Map(func(r rune) rune {
		if r == '，' || r == '。' || r == '、' || r == '：' ||
			r == '（' || r == '）' || r == '(' || r == ')' ||
			r == '"' || r == '\'' || r == ' ' {
			return ' '
		}
		return r
	}, text)
	fields := strings.Fields(text)
	var words []string
	stopWords := map[string]bool{
		"的": true, "了": true, "是": true, "在": true, "和": true,
		"与": true, "或": true, "及": true, "等": true, "为": true,
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"of": true, "to": true, "in": true, "on": true, "for": true,
	}
	for _, f := range fields {
		if utf8RuneCount(f) >= 2 && !stopWords[strings.ToLower(f)] {
			words = append(words, f)
		}
	}
	return words
}

// utf8RuneCount 返回字符串的 rune 数量。
func utf8RuneCount(s string) int {
	return len([]rune(s))
}

// pptExtractSlideTitles 提取 section 内的各级标题与卡片/目录项文本。
func pptExtractSlideTitles(section string) []string {
	var titles []string
	lower := strings.ToLower(section)
	for _, tag := range []string{"h1", "h2", "h3"} {
		titles = append(titles, extractTagText(section, lower, tag)...)
	}
	titles = append(titles, extractClassText(section, lower, "card-title")...)
	titles = append(titles, extractClassText(section, lower, "dir-item")...)
	return titles
}

// extractTagText 提取指定 HTML 标签内的文本内容。
func extractTagText(content, lowerContent, tag string) []string {
	var texts []string
	openTag := "<" + tag
	closeTag := "</" + tag + ">"
	searchFrom := 0
	for {
		idx := strings.Index(lowerContent[searchFrom:], openTag)
		if idx < 0 {
			break
		}
		idx += searchFrom
		gtIdx := strings.Index(lowerContent[idx:], ">")
		if gtIdx < 0 {
			break
		}
		textStart := idx + gtIdx + 1
		endIdx := strings.Index(lowerContent[textStart:], closeTag)
		if endIdx < 0 {
			break
		}
		textEnd := textStart + endIdx
		text := strings.TrimSpace(stripPPTVisibleText(content[textStart:textEnd]))
		if text != "" {
			texts = append(texts, text)
		}
		searchFrom = textEnd + len(closeTag)
	}
	return texts
}

// extractClassText 提取带有指定 class 名的元素文本。
func extractClassText(content, lowerContent, className string) []string {
	var texts []string
	searchFrom := 0
	for {
		idx := strings.Index(lowerContent[searchFrom:], className)
		if idx < 0 {
			break
		}
		idx += searchFrom
		tagStart := strings.LastIndex(lowerContent[:idx], "<")
		if tagStart < 0 {
			searchFrom = idx + len(className)
			continue
		}
		gtIdx := strings.Index(lowerContent[idx:], ">")
		if gtIdx < 0 {
			break
		}
		textStart := idx + gtIdx + 1
		closeIdx := strings.Index(lowerContent[textStart:], "<")
		if closeIdx < 0 {
			break
		}
		textEnd := textStart + closeIdx
		text := strings.TrimSpace(stripPPTVisibleText(content[textStart:textEnd]))
		if text != "" {
			texts = append(texts, text)
		}
		searchFrom = textEnd
	}
	return texts
}

// containsPlannedHeading 判断标题列表中是否包含计划标题（双向包含）。
func containsPlannedHeading(headings []string, title string) bool {
	for _, heading := range headings {
		if strings.Contains(heading, title) || strings.Contains(title, heading) {
			return true
		}
	}
	return false
}

// pptHTMLHeadings 提取 HTML 中所有 h1/h2/h3 标题文本。
func pptHTMLHeadings(content string) []string {
	lower := strings.ToLower(content)
	var headings []string
	for _, tag := range []string{"h1", "h2", "h3"} {
		open := "<" + tag
		close := "</" + tag + ">"
		searchFrom := 0
		for {
			start := strings.Index(lower[searchFrom:], open)
			if start < 0 {
				break
			}
			start += searchFrom
			openEnd := strings.Index(lower[start:], ">")
			if openEnd < 0 {
				break
			}
			textStart := start + openEnd + 1
			end := strings.Index(lower[textStart:], close)
			if end < 0 {
				break
			}
			textEnd := textStart + end
			heading := strings.ToLower(strings.TrimSpace(stripSimpleHTML(content[textStart:textEnd])))
			if heading != "" {
				headings = append(headings, heading)
			}
			searchFrom = textEnd + len(close)
		}
	}
	return headings
}

// isGenericPPTPlanTitle 判断标题是否为通用的封面/目录/结束页等占位标题。
func isGenericPPTPlanTitle(title string) bool {
	return containsAnyFold(title,
		"cover", "agenda", "closing", "finish", "end",
		"封面", "目录", "总结", "结束", "行动",
	)
}
