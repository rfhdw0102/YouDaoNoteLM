// ppt_quality_basic.go 实现 PPT 质量校验基础函数。
//
// 在生成完成后对 PPT 内容进行质量检查：
//   - 页面数量是否充足
//   - 每页内容是否充实（信息密度）
//   - 标题命名是否规范
//   - 要点是否具体可展开
package generation

import (
	"strings"
)

// pptNeedsStructureRepair 判断 HTML 是否缺少必要结构（页数、样式、画布尺寸等）。
func pptNeedsStructureRepair(content string) bool {
	trimmed := strings.TrimSpace(content)
	lower := strings.ToLower(trimmed)
	compact := strings.Join(strings.Fields(lower), "")
	sectionCount := strings.Count(lower, "<section")
	if sectionCount < 4 || strings.Contains(lower, "<section></section>") {
		return true
	}
	for _, required := range []string{"<style", "width:1920px", "height:1080px", "overflow:hidden"} {
		if !strings.Contains(compact, required) {
			return true
		}
	}
	if len([]rune(strings.TrimSpace(stripSimpleHTML(trimmed)))) < 20 {
		return true
	}
	return false
}

// pptContainsInternalPromptLeak 判断内容是否泄露内部提示词。
func pptContainsInternalPromptLeak(content string) bool {
	text := strings.ToLower(strings.Join(strings.Fields(stripPPTVisibleText(content)), " "))
	if text == "" {
		return false
	}
	leakTokens := []string{
		"structured_ppt_plan",
		"ppt_html_generation_rules",
		"treat each slide entry",
		"the outline bullets are source material",
		"keep every planned slide title visible",
		"design every section as a real 16:9 ppt canvas",
		"finish all planned sections before returning",
		"internal slide outline",
		"generation constraints",
		"original markdown:",
		"local references:",
	}
	for _, token := range leakTokens {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

// pptContainsVisiblePlaceholderText 判断内容中是否含有占位符或规划标签文本。
func pptContainsVisiblePlaceholderText(content string) bool {
	text := strings.ToLower(strings.Join(strings.Fields(stripPPTVisibleText(content)), " "))
	if text == "" {
		return false
	}
	placeholderTokens := []string{
		"这里填写",
		"请填写",
		"占位",
		"待补充",
		"待完善",
		"todo",
		"placeholder",
		"insert your",
		"fill in",
		"lorem ipsum",
		"slide purpose",
		"page purpose",
		"writing brief",
		"source-topic",
		"source topic",
		"source_topic",
		"this slide introduces",
		"agenda items are organized",
		"treat each slide entry",
		"the outline bullets are source material",
		"keep every planned slide title",
		"design every section as a real",
		"finish all planned sections",
	}
	for _, token := range placeholderTokens {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

// pptCanPatchCanvas 判断内容是否可通过补丁方式修复画布尺寸。
func pptCanPatchCanvas(content string) bool {
	lower := strings.ToLower(strings.TrimSpace(content))
	if strings.Count(lower, "<section") < 4 {
		return false
	}
	if strings.Contains(lower, "<style") && strings.Contains(lower, "width: 1920px") && strings.Contains(lower, "height: 1080px") {
		return false
	}
	if len([]rune(strings.TrimSpace(stripSimpleHTML(content)))) < 20 {
		return false
	}
	return true
}

// patchPPTCanvasHTML 为内容补齐画布 CSS 并为 section 添加 ppt-slide 类。
func patchPPTCanvasHTML(content string) string {
	css := `<style>
.ppt-slide {
  position: relative;
  width: 1920px;
  height: 1080px;
  overflow: hidden;
  box-sizing: border-box;
  padding: 90px 116px;
  background: #ffffff;
  color: #111827;
  font-family: system-ui, 'Segoe UI', 'Microsoft YaHei', sans-serif;
}
.ppt-slide h1 { font-size: 78px; line-height: 1.12; font-weight: 850; margin-bottom: 36px; }
.ppt-slide h2 { font-size: 54px; line-height: 1.18; font-weight: 800; margin-bottom: 32px; }
.ppt-slide p, .ppt-slide li { font-size: 32px; line-height: 1.36; }
</style>`
	patched := strings.TrimSpace(content)
	if !strings.Contains(strings.ToLower(patched), "<style") {
		patched = css + "\n" + patched
	}
	return addPPTSlideClassToSections(patched)
}

// addPPTSlideClassToSections 为缺少 class 的 section 标签补充 ppt-slide 类名。
func addPPTSlideClassToSections(content string) string {
	var b strings.Builder
	lower := strings.ToLower(content)
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
		if strings.Contains(strings.ToLower(tag), "class=") {
			b.WriteString(tag)
		} else {
			b.WriteString(strings.TrimSuffix(tag, ">"))
			b.WriteString(` class="ppt-slide" data-ppt-slide="true">`)
		}
		pos = end + 1
	}
	return b.String()
}

// pptContainsUnrelatedBoilerplate 判断内容是否包含与源材料无关的样板文本。
func pptContainsUnrelatedBoilerplate(content string, input generationAgentInput) bool {
	text := strings.ToLower(strings.Join(strings.Fields(stripPPTVisibleText(content)), " "))
	if text == "" {
		return false
	}
	unrelatedTokens := []string{
		"market opportunity",
		"competitive advantage",
		"business model",
		"go-to-market",
		"revenue growth",
		"investor",
		"stakeholder alignment",
		"team introduction",
		"company overview",
		"lorem ipsum",
		"sample text",
		"example content",
		"replace with",
		"your content here",
		"your title here",
		"click to edit",
		"double click to",
		"版权所有", "未经许可", "保留所有权利",
		"all rights reserved",
	}
	source := pptAllowedSourceText(input)
	for _, token := range unrelatedTokens {
		if strings.Contains(text, token) && !strings.Contains(source, token) {
			return true
		}
	}
	if pptContainsReferenceMetadata(text) {
		return true
	}
	return false
}

// pptContainsReferenceMetadata 判断文本中是否包含参考资料元信息标签。
func pptContainsReferenceMetadata(text string) bool {
	refTokens := []string{
		"文档列表",
		"文档介绍",
		"文档概述",
		"文档目录",
		"文档内容",
		"专题一", "专题二", "专题三", "专题四", "专题五", "专题六",
		"专题七", "专题八", "专题九", "专题十",
		"第一章", "第二章", "第三章", "第四章", "第五章", "第六章",
		"第七章", "第八章", "第九章", "第十章",
		"第1章", "第2章", "第3章", "第4章", "第5章",
		"第一节", "第二节", "第三节", "第四节", "第五节",
		"【重点知识",
		"【知识要点",
		"【核心知识",
		"【基础知识",
		"【考点分析",
		"【考点",
		"local references:",
		"web results:",
		"web summary:",
		"original markdown:",
		"input markdown",
		"source-1", "source-2", "source-3",
	}
	for _, token := range refTokens {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

// pptContainsRepetitiveText 判断内容是否存在高频重复词、短语或句子。
func pptContainsRepetitiveText(content string) bool {
	text := strings.Join(strings.Fields(stripPPTVisibleText(content)), " ")
	if len([]rune(text)) < 40 {
		return false
	}
	words := strings.Fields(strings.ToLower(text))
	if len(words) < 8 {
		return false
	}
	freq := make(map[string]int, len(words))
	for _, w := range words {
		if len([]rune(w)) < 2 {
			continue
		}
		freq[w]++
	}
	for _, count := range freq {
		if count >= 12 && float64(count)/float64(len(words)) > 0.15 {
			return true
		}
	}
	phraseFreq := make(map[string]int)
	for i := 0; i+4 <= len(words); i++ {
		phrase := strings.Join(words[i:i+4], " ")
		phraseFreq[phrase]++
	}
	for _, count := range phraseFreq {
		if count >= 4 {
			return true
		}
	}
	sentences := splitPPTSentences(text)
	if len(sentences) >= 6 {
		seen := make(map[string]int, len(sentences))
		for _, s := range sentences {
			s = strings.ToLower(strings.TrimSpace(s))
			if len([]rune(s)) < 6 {
				continue
			}
			seen[s]++
		}
		for _, count := range seen {
			if count >= 3 {
				return true
			}
		}
	}
	return false
}

// splitPPTSentences 将文本按中英文句末标点切分为句子。
func splitPPTSentences(text string) []string {
	text = strings.ReplaceAll(text, "!", ".")
	text = strings.ReplaceAll(text, "?", ".")
	text = strings.ReplaceAll(text, "！", "。")
	text = strings.ReplaceAll(text, "？", "。")
	text = strings.ReplaceAll(text, "。", ".")
	text = strings.ReplaceAll(text, "；", ";")
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == '.' || r == ';' || r == '\n'
	})
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// pptContainsExcessivePromptWords 判断内容是否含有过多提示词用语。
func pptContainsExcessivePromptWords(content string) bool {
	text := strings.ToLower(strings.Join(strings.Fields(stripPPTVisibleText(content)), " "))
	if text == "" {
		return false
	}
	promptWords := []string{
		"作为", "你需要", "请生成", "请输出", "请确保", "请注意", "请遵循",
		"根据上下文", "根据要求", "按照要求", "按照计划",
		"以下是", "如下所示", "如下",
		"as an ai", "as a language model", "i cannot", "i'm sorry",
		"i am unable", "i don't have", "as requested", "here is",
		"here are", "below is", "based on the context", "according to",
		"please note", "please ensure", "you should", "the output should",
		"生成内容", "生成结果", "输出内容", "输出结果",
		"演示文稿将", "幻灯片将", "本演示", "本次生成",
		"注意事项", "说明事项", "备注事项",
		"步骤一", "步骤二", "步骤三", "步骤四",
		"第一部分", "第二部分", "第三部分", "第四部分",
	}
	hits := 0
	for _, word := range promptWords {
		count := strings.Count(text, word)
		if count > 0 {
			hits += count
		}
	}
	return hits >= 5
}

// pptAllowedSourceText 汇总输入中允许出现的源材料文本。
func pptAllowedSourceText(input generationAgentInput) string {
	var parts []string
	if input.Request != nil {
		parts = append(parts, input.Request.Markdown, input.Request.Prompt)
	}
	for _, ref := range input.References {
		parts = append(parts, ref.Content, ref.Heading, ref.ChapterPath, ref.SourceName)
	}
	for _, result := range input.SearchResults {
		parts = append(parts, result.Title, result.Snippet, result.Content)
	}
	return strings.ToLower(strings.Join(strings.Fields(strings.Join(parts, "\n")), " "))
}

// pptNeedsPlanCoverageRepair 判断 HTML 是否遗漏了计划中的标题或页数。
func pptNeedsPlanCoverageRepair(content string, plan *pptOutlinePlan) bool {
	if plan == nil || len(plan.Slides) == 0 {
		return false
	}
	if strings.TrimSpace(plan.Title) != "" && !containsPlannedHeading(pptHTMLHeadings(content), strings.ToLower(strings.TrimSpace(plan.Title))) {
		return true
	}
	actual := pptSectionCount(content)
	expected := len(plan.Slides)
	if expected <= 4 {
		return actual < expected
	}
	return actual < expected || actual < 6 || pptMissingPlannedTitles(content, plan)
}

// pptSectionCount 统计 HTML 中 section 标签的数量。
func pptSectionCount(content string) int {
	return strings.Count(strings.ToLower(content), "<section")
}

// pptMissingPlannedTitles 判断 HTML 中是否缺少计划中的非通用标题。
func pptMissingPlannedTitles(content string, plan *pptOutlinePlan) bool {
	headings := pptHTMLHeadings(content)
	required := 0
	missing := 0
	for _, slide := range plan.Slides {
		title := strings.ToLower(strings.TrimSpace(slide.Title))
		if title == "" || isGenericPPTPlanTitle(title) {
			continue
		}
		required++
		if !containsPlannedHeading(headings, title) {
			missing++
		}
	}
	if required == 0 {
		return false
	}
	return missing > 0
}
