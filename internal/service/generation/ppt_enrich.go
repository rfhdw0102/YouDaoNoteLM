// ppt_enrich.go 实现 PPT 内容增强。
//
// 在 LLM 生成初稿后，对 PPT 大纲进行内容增强：
//   - batchPPTOutlinePlan：批量补全各页面的详细内容
//   - normalizePPTBullets：规范化要点格式
//   - extractFirstJSONObject：从 LLM 响应中提取首个 JSON 对象（容错）
package generation

import (
	"YoudaoNoteLm/pkg/logger"
	"context"
	"encoding/json"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"strings"
	"sync"
	"time"
)

// enrichPPTContent 并发批量调用 LLM 补全各幻灯片的详细内容。
func (a *pptGenerationAgent) enrichPPTContent(ctx context.Context, state pptChainState) (pptChainState, error) {
	if a.model == nil {
		return state, nil
	}

	strategy := pptContentEnrichPromptStrategy()
	batches := batchPPTOutlinePlan(state.expanded, pptContentEnrichBatchSize)
	if len(batches) == 0 {
		return state, nil
	}

	logger.Info("[PPT] enrichPPTContent: starting batch enrichment",
		zap.Int("total_slides", len(state.expanded.Slides)),
		zap.Int("batch_count", len(batches)),
		zap.Int("batch_size", pptContentEnrichBatchSize),
	)

	type enrichJob struct {
		index int
		batch pptOutlinePlan
	}
	type enrichResult struct {
		index int
		rich  pptRichContent
		ok    bool
	}

	workerCount := len(batches)
	if workerCount > pptContentEnrichBatchSize {
		workerCount = pptContentEnrichBatchSize
	}

	jobChan := make(chan enrichJob, len(batches))
	resultChan := make(chan enrichResult, len(batches))

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		defer close(jobChan)
		for i, batch := range batches {
			select {
			case jobChan <- enrichJob{index: i, batch: batch}:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})

	var mu sync.Mutex
	workerID := 0
	for range workerCount {
		g.Go(func() error {
			mu.Lock()
			wid := workerID
			workerID++
			mu.Unlock()

			for job := range jobChan {
				batchStart := time.Now()
				batchState := state
				batchState.expanded = job.batch

				contextValue := state.input.Context
				contextValue += "\n\nPPT_OUTLINE_FOR_ENRICHMENT\n"
				contextValue += renderPPTPlanForPrompt(job.batch)

				rich, ok := a.tryEnrichPPTContent(ctx, strategy, batchState, contextValue, false)
				if !ok {
					rich, ok = a.tryEnrichPPTContent(ctx, strategy, batchState, contextValue, true)
				}

				logFields := []zap.Field{
					zap.Int("worker_id", wid),
					zap.Int("batch_index", job.index),
					zap.Int("batch_count", len(batches)),
					zap.Duration("elapsed", time.Since(batchStart)),
				}

				if !ok {
					logger.Warn("[PPT] enrichPPTContent: batch skipped",
						append(logFields,
							zap.Int("slides_in_batch", len(job.batch.Slides)),
						)...)
					select {
					case resultChan <- enrichResult{index: job.index, ok: false}:
					case <-ctx.Done():
						return ctx.Err()
					}
					continue
				}

				logger.Info("[PPT] enrichPPTContent: batch done",
					append(logFields,
						zap.Int("slides_in_batch", len(job.batch.Slides)),
						zap.Int("rich_slides", len(rich.Slides)),
					)...)
				select {
				case resultChan <- enrichResult{index: job.index, rich: rich, ok: true}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			logger.Debug("[PPT] enrichPPTContent: worker finished",
				zap.Int("worker_id", wid),
			)
			return nil
		})
	}

	done := make(chan struct{})
	var gErr error
	go func() {
		gErr = g.Wait()
		close(resultChan)
		close(done)
	}()

	results := make([]enrichResult, len(batches))
	for res := range resultChan {
		results[res.index] = res
	}

	<-done
	if gErr != nil {
		if ctx.Err() != nil {
			return state, gErr
		}
		logger.Warn("[PPT] enrichPPTContent: non-fatal worker error",
			zap.Error(gErr),
		)
	}

	merged := pptRichContent{}
	for _, res := range results {
		if res.ok {
			merged.Slides = append(merged.Slides, res.rich.Slides...)
		}
	}
	if pptRichContentHasUsableSlides(merged) {
		state.richContent = merged
	}
	return state, nil
}

// batchPPTOutlinePlan 将大纲计划按批次大小切分成多个子计划。
func batchPPTOutlinePlan(plan pptOutlinePlan, batchSize int) []pptOutlinePlan {
	if batchSize <= 0 {
		batchSize = pptContentEnrichBatchSize
	}
	if len(plan.Slides) == 0 {
		return nil
	}
	batches := make([]pptOutlinePlan, 0, (len(plan.Slides)+batchSize-1)/batchSize)
	for start := 0; start < len(plan.Slides); start += batchSize {
		end := start + batchSize
		if end > len(plan.Slides) {
			end = len(plan.Slides)
		}
		batches = append(batches, pptOutlinePlan{
			Title:  plan.Title,
			Slides: append([]pptSlidePlan(nil), plan.Slides[start:end]...),
		})
	}
	return batches
}

// tryEnrichPPTContent 调用 LLM 增强一批幻灯片内容，失败时按重试策略加固输出格式。
func (a *pptGenerationAgent) tryEnrichPPTContent(ctx context.Context, strategy generationPromptStrategy, state pptChainState, contextValue string, retry bool) (pptRichContent, bool) {
	outputFormat := strategy.OutputFormat
	if retry {
		outputFormat += "\n\n严格要求：上一次输出无法被解析为 JSON。本次回复必须只包含一个 JSON 对象，第一个字符必须是 '{'，最后一个字符必须是 '}'。不要输出 ```、不要任何解释、不要前后空行说明。"
	}
	llmStart := time.Now()
	generated, err := a.model.Generate(ctx, GenerationPrompt{
		AgentName:    a.name + "_content_enrich",
		System:       strategy.System,
		User:         strings.TrimSpace(state.input.Request.Prompt),
		Context:      contextValue,
		OutputFormat: outputFormat,
		MaxTokens:    pptContentEnrichMaxTokens,
	})
	logger.Info("[PPT] LLM call: tryEnrichPPTContent done",
		zap.Duration("llm_elapsed", time.Since(llmStart)),
		zap.Int("generated_len", len(generated)),
		zap.Bool("retry", retry),
		zap.Error(err),
	)
	logFields := pptEnrichLogFields(state, retry)
	if err != nil {
		logger.Warn("enrich ppt content: model generate failed",
			append(logFields, zap.Error(err))...)
		return pptRichContent{}, false
	}
	generated = strings.TrimSpace(generated)
	if generated == "" {
		logger.Warn("enrich ppt content: model returned empty", logFields...)
		return pptRichContent{}, false
	}
	jsonStr := extractFirstJSONObject(generated)
	if jsonStr == "" {
		logger.Warn("enrich ppt content: no json object in output",
			append(logFields,
				zap.Int("output_len", len(generated)),
				zap.String("output_head", truncate(generated, 200)),
			)...)
		return pptRichContent{}, false
	}
	var rich pptRichContent
	if err := json.Unmarshal([]byte(jsonStr), &rich); err != nil {
		logger.Warn("enrich ppt content: json unmarshal failed",
			append(logFields,
				zap.Int("json_len", len(jsonStr)),
				zap.String("json_head", truncate(jsonStr, 200)),
				zap.Error(err),
			)...)
		return pptRichContent{}, false
	}
	if !pptRichContentHasUsableSlides(rich) {
		paragraphTotal := 0
		for _, slide := range rich.Slides {
			paragraphTotal += len(slide.Paragraphs)
		}
		logger.Warn("enrich ppt content: parsed but no usable slides",
			append(logFields,
				zap.Int("slide_count", len(rich.Slides)),
				zap.Int("paragraph_total", paragraphTotal),
			)...)
		return pptRichContent{}, false
	}
	return rich, true
}

// pptEnrichLogFields 构造内容增强日志的公共字段。
func pptEnrichLogFields(state pptChainState, retry bool) []zap.Field {
	var userID, notebookID uint
	if state.input.Request != nil {
		userID = state.input.Request.UserID
		notebookID = state.input.Request.NotebookID
	}
	return []zap.Field{
		zap.Uint("user_id", userID),
		zap.Uint("notebook_id", notebookID),
		zap.Bool("retry", retry),
		zap.Int("plan_slide_count", len(state.expanded.Slides)),
	}
}

// extractFirstJSONObject 从字符串中提取首个 JSON 对象片段。
func extractFirstJSONObject(s string) string {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return ""
	}
	end := strings.LastIndexByte(s, '}')
	if end <= start {
		return ""
	}
	return s[start : end+1]
}

// pptRichContentHasUsableSlides 判断富内容中是否存在带有效段落的幻灯片。
func pptRichContentHasUsableSlides(rich pptRichContent) bool {
	if len(rich.Slides) == 0 {
		return false
	}
	for _, slide := range rich.Slides {
		for _, p := range slide.Paragraphs {
			if strings.TrimSpace(p) != "" {
				return true
			}
		}
	}
	return false
}

// normalizePPTBullets 规整幻灯片要点：去除标题前缀、去重并控制数量上限。
func normalizePPTBullets(slide *pptSlidePlan) {
	for i, bullet := range slide.Bullets {
		if isPPTCodeBlockBullet(bullet) {
			continue
		}
		slide.Bullets[i] = stripPPTBulletSlideTitlePrefix(bullet, slide.Title)
	}
	slide.Bullets = uniqueNonEmpty(slide.Bullets)
	switch slide.Title {
	case "封面", "目录":
		return
	}
	if len(slide.Bullets) > 9 {
		slide.Bullets = append([]string{}, slide.Bullets[:7]...)
	}
}

// stripPPTBulletSlideTitlePrefix 去除要点开头的幻灯片标题前缀。
func stripPPTBulletSlideTitlePrefix(bullet, title string) string {
	return stripPPTBulletTitlePrefixes(bullet, []string{title})
}

// stripPPTBulletTitlePrefixes 去除要点开头匹配任意候选标题的前缀。
func stripPPTBulletTitlePrefixes(bullet string, candidates []string) string {
	bullet = strings.TrimSpace(bullet)
	if bullet == "" || len(candidates) == 0 {
		return bullet
	}
	usable := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if len([]rune(c)) < 2 {
			continue
		}
		key := strings.ToLower(c)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		usable = append(usable, c)
	}
	if len(usable) == 0 {
		return bullet
	}
	for i := 1; i < len(usable); i++ {
		for j := i; j > 0 && len([]rune(usable[j])) > len([]rune(usable[j-1])); j-- {
			usable[j], usable[j-1] = usable[j-1], usable[j]
		}
	}
	for iter := 0; iter < 3; iter++ {
		stripped, ok := stripPPTBulletPrefixesOnce(bullet, usable)
		if !ok {
			break
		}
		bullet = stripped
	}
	return bullet
}

// stripPPTBulletPrefixesOnce 尝试一次性去除要点匹配候选前缀的部分。
func stripPPTBulletPrefixesOnce(bullet string, candidates []string) (string, bool) {
	for _, prefix := range candidates {
		for _, variant := range pptSlideTitlePrefixCandidates(prefix) {
			rest, ok := cutPPTTitlePrefix(bullet, variant)
			if ok {
				return rest, true
			}
		}
	}
	return bullet, false
}

// stripPPTBulletTitlePrefixOnce 尝试去除要点开头的单个标题前缀。
func stripPPTBulletTitlePrefixOnce(bullet, title string) (string, bool) {
	return stripPPTBulletPrefixesOnce(bullet, []string{title})
}

// pptSlideTitlePrefixCandidates 生成标题的可匹配前缀候选列表。
func pptSlideTitlePrefixCandidates(title string) []string {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil
	}
	candidates := []string{title}
	if idx := strings.IndexAny(title, "（([【"); idx > 0 {
		core := strings.TrimSpace(title[:idx])
		if len([]rune(core)) >= 2 && core != title {
			candidates = append(candidates, core)
		}
	}
	return candidates
}

// cutPPTTitlePrefix 若要点以指定前缀开头且后接分隔符，则去除前缀部分。
func cutPPTTitlePrefix(bullet, prefix string) (string, bool) {
	if prefix == "" {
		return bullet, false
	}
	if !strings.HasPrefix(strings.ToLower(bullet), strings.ToLower(prefix)) {
		return bullet, false
	}
	rest := bullet[len(prefix):]
	if rest == "" {
		return bullet, false
	}
	r := []rune(rest)[0]
	if !isPPTTitleSeparatorRune(r) && r != ' ' && r != '\t' {
		return bullet, false
	}
	after := strings.TrimSpace(string([]rune(rest)[1:]))
	if after == "" {
		return bullet, false
	}
	return after, true
}

// isPPTTitleSeparatorRune 判断 rune 是否为标题分隔符。
func isPPTTitleSeparatorRune(r rune) bool {
	switch r {
	case '：', ':', '-', '—', '–', '~', '|', '/', '、', '·':
		return true
	}
	return false
}

// renderPPTSlides 将大纲计划渲染为基础 HTML 段落。
func renderPPTSlides(plan pptOutlinePlan) string {
	var b strings.Builder
	for _, slide := range plan.Slides {
		b.WriteString("<section>")
		if slide.Title == "封面" {
			b.WriteString("<h2>封面</h2>")
			b.WriteString("<h1>")
			b.WriteString(htmlEscape(plan.Title))
			b.WriteString("</h1>")
		} else {
			b.WriteString("<h2>")
			b.WriteString(htmlEscape(slide.Title))
			b.WriteString("</h2>")
		}
		if len(slide.Bullets) > 0 {
			b.WriteString("<ul>")
			for _, bullet := range slide.Bullets {
				b.WriteString("<li>")
				b.WriteString(htmlEscape(bullet))
				b.WriteString("</li>")
			}
			b.WriteString("</ul>")
		}
		b.WriteString("</section>\n")
	}
	return strings.TrimSpace(b.String())
}
