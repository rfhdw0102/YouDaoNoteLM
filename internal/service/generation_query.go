package service

import (
	"strings"
	"unicode"
)

type generationQueryPlan struct {
	LocalQuery string
	WebQuery   string
	Topic      string
	Keywords   []string
}

func buildGenerationQueryPlan(req *GenerationRequest) generationQueryPlan {
	if req == nil {
		return generationQueryPlan{}
	}

	topic := extractTitle(req.Markdown, "")
	headings := extractMarkdownHeadings(req.Markdown, 8)
	keywords := extractGenerationKeywords(req.Prompt, req.Markdown, 10)
	intent := generationTypeIntent(req.Type)

	localParts := append([]string{}, strings.TrimSpace(req.Prompt))
	localParts = append(localParts, headings...)
	localParts = append(localParts, intent...)
	localParts = append(localParts, keywords...)

	webParts := []string{topic, strings.TrimSpace(req.Prompt)}
	webParts = append(webParts, keywords...)

	return generationQueryPlan{
		LocalQuery: compactQuery(localParts, 360),
		WebQuery:   compactQuery(webParts, 240),
		Topic:      topic,
		Keywords:   keywords,
	}
}

func extractMarkdownHeadings(markdown string, limit int) []string {
	var headings []string
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimLeft(line, "#"))
		if heading == "" {
			continue
		}
		headings = append(headings, heading)
		if len(headings) >= limit {
			break
		}
	}
	return headings
}

func extractGenerationKeywords(prompt, markdown string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	candidates := append([]string{}, splitKeywordCandidates(prompt)...)

	var headingLines []string
	var contentLines []string
	for _, rawLine := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(rawLine)
		line := strings.TrimSpace(strings.TrimLeft(trimmed, "#-*0123456789. "))
		if line == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			headingLines = append(headingLines, line)
		}
		contentLines = append(contentLines, line)
	}
	candidates = appendKeywordLineCandidates(candidates, headingLines, limit)
	candidates = appendKeywordLineCandidates(candidates, contentLines, limit*2)

	seen := map[string]struct{}{}
	keywords := make([]string, 0, limit)
	for _, item := range candidates {
		item = strings.TrimSpace(item)
		if len([]rune(item)) < 2 {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		keywords = append(keywords, item)
		if len(keywords) >= limit {
			break
		}
	}
	return keywords
}

func appendKeywordLineCandidates(candidates []string, lines []string, limit int) []string {
	for _, index := range balancedSampleIndexes(len(lines), limit) {
		candidates = append(candidates, splitKeywordCandidates(lines[index])...)
	}
	return candidates
}

func balancedSampleIndexes(count, limit int) []int {
	if count <= 0 || limit <= 0 {
		return nil
	}
	if limit > count {
		limit = count
	}
	indexes := make([]int, 0, limit)
	seen := map[int]struct{}{}
	for step := 1; len(indexes) < limit && step <= count*2; step *= 2 {
		for i := 0; i <= step && len(indexes) < limit; i++ {
			index := (i*(count-1) + step/2) / step
			if _, ok := seen[index]; ok {
				continue
			}
			seen[index] = struct{}{}
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func splitKeywordCandidates(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var result []string
	var current strings.Builder
	flush := func() {
		token := strings.TrimSpace(current.String())
		if token != "" {
			result = append(result, token)
		}
		current.Reset()
	}
	for _, r := range value {
		if unicode.IsSpace(r) || strings.ContainsRune("，。；、,.!?！？:：；（）()[]【】", r) {
			flush()
			continue
		}
		current.WriteRune(r)
	}
	flush()
	return result
}

func generationTypeIntent(typ GenerationType) []string {
	switch typ {
	case GenerationTypeMindmap:
		return []string{"结构", "层级", "概念关系"}
	case GenerationTypePPT:
		return []string{"主题", "论点", "案例", "结构"}
	case GenerationTypeQuiz:
		return []string{"关键概念", "定义", "易错点", "应用题"}
	case GenerationTypeNote:
		return []string{"总结", "知识框架", "重点"}
	default:
		return nil
	}
}

func compactQuery(parts []string, maxRunes int) string {
	seen := map[string]struct{}{}
	var compacted []string
	for _, part := range parts {
		part = strings.Join(strings.Fields(strings.TrimSpace(part)), " ")
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		compacted = append(compacted, part)
	}
	query := strings.Join(compacted, " ")
	if maxRunes <= 0 {
		return query
	}
	runes := []rune(query)
	if len(runes) <= maxRunes {
		return query
	}
	return string(runes[:maxRunes])
}
