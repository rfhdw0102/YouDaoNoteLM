package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"YoudaoNoteLm/internal/rag"
)

func buildInlineMarkdownReferences(ctx context.Context, markdown string, plan generationQueryPlan, limit int) ([]GenerationReference, error) {
	parser := rag.NewMarkdownParser()
	docs, err := parser.Parse(ctx, strings.NewReader(markdown))
	if err != nil {
		return nil, err
	}
	parentDocs, err := rag.NewParentTransformer(1000).Transform(ctx, docs)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 6
	}

	type scoredRef struct {
		ref   GenerationReference
		index int
	}
	scored := make([]scoredRef, 0, len(parentDocs))
	for i, doc := range parentDocs {
		content := strings.TrimSpace(doc.Content)
		if content == "" {
			continue
		}
		heading, _ := doc.MetaData["heading"].(string)
		chapterPath, _ := doc.MetaData["chapter_path"].(string)
		score := scoreInlineReference(content, heading, plan)
		scored = append(scored, scoredRef{
			ref: GenerationReference{
				SourceName:  "input_markdown",
				Content:     content,
				Heading:     heading,
				ChapterPath: chapterPath,
				Score:       float32(score),
			},
			index: i,
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].ref.Score == scored[j].ref.Score {
			return scored[i].index < scored[j].index
		}
		return scored[i].ref.Score > scored[j].ref.Score
	})

	refs := make([]GenerationReference, 0, min(limit, len(scored)))
	for _, item := range scored {
		refs = append(refs, item.ref)
		if len(refs) >= limit {
			break
		}
	}
	return refs, nil
}

func scoreInlineReference(content, heading string, plan generationQueryPlan) float64 {
	score := 1.0
	haystack := strings.ToLower(strings.Join([]string{heading, content}, "\n"))
	query := strings.ToLower(strings.Join([]string{plan.LocalQuery, plan.WebQuery}, " "))

	for _, term := range relevantGenerationTerms(plan) {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" {
			continue
		}
		if strings.Contains(haystack, term) {
			score += 2.0
		}
		if heading != "" && (strings.Contains(strings.ToLower(heading), term) || strings.Contains(term, strings.ToLower(heading))) {
			score += 1.5
		}
	}
	if heading != "" && strings.Contains(query, strings.ToLower(strings.TrimSpace(heading))) {
		score += 2.0
	}
	if looksDefinitionLike(content) || looksListRich(content) {
		score += 1.0
	}
	if len([]rune(strings.TrimSpace(content))) < 20 {
		score -= 1.0
	}
	return score
}

func relevantGenerationTerms(plan generationQueryPlan) []string {
	terms := append([]string{}, plan.Keywords...)
	terms = append(terms, splitKeywordCandidates(plan.LocalQuery)...)
	terms = append(terms, splitKeywordCandidates(plan.WebQuery)...)
	return uniqueNonEmpty(terms)
}

func looksDefinitionLike(content string) bool {
	return strings.Contains(content, " is ") ||
		strings.Contains(content, " are ") ||
		strings.Contains(content, "定义") ||
		strings.Contains(content, "是") ||
		strings.Contains(content, "指")
}

func looksListRich(content string) bool {
	count := 0
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "1.") {
			count++
		}
	}
	return count >= 2
}

func mergeGenerationReferences(inlineRefs, ragRefs []GenerationReference, limit int) []GenerationReference {
	if limit <= 0 {
		limit = len(inlineRefs) + len(ragRefs)
	}
	merged := make([]GenerationReference, 0, len(inlineRefs)+len(ragRefs))
	seen := map[string]int{}

	add := func(ref GenerationReference) {
		key := normalizeReferenceContent(ref.Content)
		if key == "" {
			return
		}
		if existing, ok := seen[key]; ok {
			if isInlineReference(ref) && !isInlineReference(merged[existing]) {
				merged[existing] = ref
			}
			return
		}
		seen[key] = len(merged)
		merged = append(merged, ref)
	}

	for _, ref := range inlineRefs {
		add(ref)
	}
	for _, ref := range ragRefs {
		add(ref)
	}

	sort.SliceStable(merged, func(i, j int) bool {
		leftInline := isInlineReference(merged[i])
		rightInline := isInlineReference(merged[j])
		if leftInline != rightInline {
			return leftInline
		}
		return merged[i].Score > merged[j].Score
	})
	if len(merged) > limit {
		return append([]GenerationReference{}, merged[:limit]...)
	}
	return merged
}

func normalizeReferenceContent(content string) string {
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	var b strings.Builder
	for _, r := range content {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

func isInlineReference(ref GenerationReference) bool {
	return ref.SourceName == "input_markdown"
}

func generationReferenceLabel(ref GenerationReference) string {
	if isInlineReference(ref) {
		return "输入 Markdown"
	}
	return firstNonEmpty(ref.SourceName, fmt.Sprintf("来源-%d", ref.SourceID))
}

func pruneGenerationSearchResults(results []SearchResult, limit int) []SearchResult {
	if limit <= 0 {
		limit = len(results)
	}
	kept := make([]SearchResult, 0, min(limit, len(results)))
	for _, result := range results {
		if strings.TrimSpace(result.Title) == "" || strings.TrimSpace(result.URL) == "" {
			continue
		}
		if strings.TrimSpace(result.Snippet) == "" && strings.TrimSpace(result.Content) == "" {
			continue
		}
		kept = append(kept, result)
	}
	sort.SliceStable(kept, func(i, j int) bool {
		return kept[i].Score > kept[j].Score
	})
	if len(kept) > limit {
		return append([]SearchResult{}, kept[:limit]...)
	}
	return kept
}

func buildGenerationContext(req *GenerationRequest, refs []GenerationReference, searchSummary string, searchResults []SearchResult) string {
	var b strings.Builder
	b.WriteString("用户要求：\n")
	b.WriteString(strings.TrimSpace(req.Prompt))
	b.WriteString("\n\n原始 Markdown：\n")
	b.WriteString(strings.TrimSpace(req.Markdown))

	if len(refs) > 0 {
		b.WriteString("\n\n本地 RAG 参考：\n")
		for i, ref := range refs {
			b.WriteString(fmt.Sprintf("[%d] %s %s\n%s\n", i+1, generationReferenceLabel(ref), ref.ChapterPath, ref.Content))
		}
	}
	if strings.TrimSpace(searchSummary) != "" {
		b.WriteString("\n\n联网搜索摘要：\n")
		b.WriteString(strings.TrimSpace(searchSummary))
	}
	if len(searchResults) > 0 {
		b.WriteString("\n\n联网搜索结果：\n")
		for i, result := range searchResults {
			b.WriteString(fmt.Sprintf("[%d] %s - %s\n%s\n", i+1, result.Title, result.URL, firstNonEmpty(result.Snippet, result.Content)))
		}
	}
	return b.String()
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]struct{}{}
	var result []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
