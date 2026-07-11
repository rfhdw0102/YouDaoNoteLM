package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"YoudaoNoteLm/internal/rag"
)

type generationReferenceSelection struct {
	References           []GenerationReference
	StrongLocalCount     int
	WeakLocalCount       int
	IrrelevantLocalCount int
	NeedsWebSupplement   bool
	WebSupplementReason  string
}

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
	if looksDefinitionLike(content) || looksListRich(content) || looksCodeBlock(content) {
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
	return filterGenerationTerms(terms)
}

func looksDefinitionLike(content string) bool {
	content = strings.ToLower(content)
	return strings.Contains(content, " is ") ||
		strings.Contains(content, " are ") ||
		strings.Contains(content, " means ") ||
		strings.Contains(content, "definition")
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

// looksCodeBlock returns true if the content contains a fenced code block
// (```...```), indicating it carries code that should be preserved as-is
// in generation references.
func looksCodeBlock(content string) bool {
	return strings.Contains(content, "```")
}

// refContentLimit returns the character limit for summarizing a reference's
// content. Code blocks need more space to remain meaningful, so they get a
// larger limit than regular text.
func refContentLimit(ref GenerationReference) int {
	if strings.Contains(ref.Content, "```") {
		return 500
	}
	return 120
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

func selectGenerationReferences(inlineRefs, ragRefs []GenerationReference, plan generationQueryPlan, limit int) generationReferenceSelection {
	if limit <= 0 {
		limit = len(inlineRefs) + len(ragRefs)
	}

	strongRefs := make([]GenerationReference, 0, len(ragRefs))
	strongCoverage := map[string]struct{}{}
	weakCount := 0
	irrelevantCount := 0

	for _, ref := range ragRefs {
		classification, coverageTerms := classifyGenerationLocalReference(ref, plan)
		switch classification {
		case "strong":
			strongRefs = append(strongRefs, ref)
			for _, term := range coverageTerms {
				strongCoverage[term] = struct{}{}
			}
		case "weak":
			weakCount++
		default:
			irrelevantCount++
		}
	}

	sort.SliceStable(strongRefs, func(i, j int) bool {
		return strongRefs[i].Score > strongRefs[j].Score
	})

	reason := generationWebSupplementReason(len(strongRefs), len(strongCoverage))

	return generationReferenceSelection{
		References:           mergeGenerationReferences(inlineRefs, strongRefs, limit),
		StrongLocalCount:     len(strongRefs),
		WeakLocalCount:       weakCount,
		IrrelevantLocalCount: irrelevantCount,
		NeedsWebSupplement:   reason != "",
		WebSupplementReason:  reason,
	}
}

func classifyGenerationLocalReference(ref GenerationReference, plan generationQueryPlan) (string, []string) {
	headingText := strings.ToLower(strings.TrimSpace(strings.Join([]string{ref.Heading, ref.ChapterPath}, "\n")))
	contentText := strings.ToLower(strings.TrimSpace(ref.Content))

	primaryTerms := filterGenerationTerms(append([]string{plan.Topic}, plan.Keywords...))
	secondaryTerms := filterGenerationTerms(append(splitKeywordCandidates(plan.LocalQuery), splitKeywordCandidates(plan.WebQuery)...))

	coverage := make([]string, 0, len(primaryTerms))
	primaryScore := 0.0
	secondaryScore := 0.0

	for _, term := range primaryTerms {
		switch {
		case strings.Contains(headingText, term):
			primaryScore += 3.0
			coverage = append(coverage, term)
		case strings.Contains(contentText, term):
			primaryScore += 1.75
			coverage = append(coverage, term)
		}
	}

	for _, term := range secondaryTerms {
		if containsString(coverage, term) {
			continue
		}
		switch {
		case strings.Contains(headingText, term):
			secondaryScore += 1.5
		case strings.Contains(contentText, term):
			secondaryScore += 0.75
		}
	}

	if len([]rune(contentText)) > 700 && primaryScore == 0 {
		secondaryScore -= 1.0
	}

	totalScore := primaryScore + secondaryScore
	coverage = uniqueNonEmpty(coverage)

	switch {
	case primaryScore >= 3.0:
		return "strong", coverage
	case primaryScore >= 1.75 && totalScore >= 4.0:
		return "strong", coverage
	case primaryScore >= 1.75:
		return "weak", coverage
	case totalScore >= 2.5:
		return "weak", coverage
	default:
		return "irrelevant", nil
	}
}

func filterGenerationTerms(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || isGenericGenerationTerm(value) {
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

func isGenericGenerationTerm(value string) bool {
	if isASCIIStopword(value) {
		return true
	}
	if isASCIIAlphaToken(value) && len(value) < 4 {
		return true
	}
	switch value {
	case "ppt", "slides", "slide", "markdown", "mindmap", "quiz", "note", "notes",
		"generate", "generated", "summary", "outline", "structure", "topic", "content",
		"生成", "总结", "大纲", "结构", "主题", "内容", "脑图", "思维导图", "幻灯片", "笔记", "题目":
		return true
	default:
		return len([]rune(value)) < 2
	}
}

func generationWebSupplementReason(strongLocalCount, strongCoverageCount int) string {
	switch {
	case strongLocalCount == 0:
		return "no_strong_local_references"
	case strongCoverageCount < 2:
		return "insufficient_local_topic_coverage"
	default:
		return ""
	}
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
		return "Input Markdown"
	}
	return firstNonEmpty(ref.SourceName, fmt.Sprintf("source-%d", ref.SourceID))
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
	b.WriteString("User Request:\n")
	b.WriteString(strings.TrimSpace(req.Prompt))
	b.WriteString("\n\nOriginal Markdown:\n")
	b.WriteString(strings.TrimSpace(req.Markdown))

	if len(refs) > 0 {
		b.WriteString("\n\nLocal References:\n")
		for i, ref := range refs {
			b.WriteString(fmt.Sprintf("[%d] %s %s\n%s\n", i+1, generationReferenceLabel(ref), ref.ChapterPath, summarizeLine(ref.Content, refContentLimit(ref))))
		}
	}
	if strings.TrimSpace(searchSummary) != "" {
		b.WriteString("\n\nWeb Summary:\n")
		b.WriteString(strings.TrimSpace(searchSummary))
	}
	if len(searchResults) > 0 {
		b.WriteString("\n\nWeb Results:\n")
		for i, result := range searchResults {
			b.WriteString(fmt.Sprintf("[%d] %s - %s\n%s\n", i+1, result.Title, result.URL, summarizeLine(firstNonEmpty(result.Snippet, result.Content), 120)))
		}
	}

	b.WriteString("\n\nGeneration Constraints:\n")
	b.WriteString("Use only references directly relevant to the current markdown and prompt topic. Ignore unrelated recalled material.\n")
	if req != nil && req.UseWeb {
		b.WriteString("Treat the original markdown and local references as primary evidence. Use web information only as supplementary background. Do not output references, source lists, or appendices in the final body.\n")
	} else {
		b.WriteString("Web search is disabled. Generate only from the original markdown and directly relevant local references. When source material is sparse, expand with careful explanatory synthesis instead of inventing external facts. Do not output references, source lists, or appendices in the final body.\n")
		if len(refs) == 0 {
			b.WriteString("If there are no usable local references, structure the result directly from the original markdown.\n")
		}
	}
	return b.String()
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
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

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func isASCIIAlphaToken(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r > unicode.MaxASCII || !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func isASCIIStopword(value string) bool {
	switch value {
	case "a", "an", "and", "are", "as", "at", "be", "by", "for", "from", "in", "into", "is",
		"it", "of", "on", "or", "that", "the", "their", "this", "to", "uses", "with":
		return true
	default:
		return false
	}
}
