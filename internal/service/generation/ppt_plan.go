// ppt_plan.go 实现 PPT 大纲规划逻辑。
//
// planPPTOutline 是主入口，根据学习内容分析结果生成 PPT 大纲计划。
// planStaticPPTOutline 提供静态规划路径（无 LLM 调用时的 fallback）。
package generation

import (
	"fmt"
	"strings"
)

// planPPTOutline 根据学习内容分析生成 PPT 大纲计划。
func planPPTOutline(analysis learningContentAnalysis) pptOutlinePlan {
	return planDynamicPPTOutline(analysis)
}

// planStaticPPTOutline 静态规划 PPT 大纲，作为无 LLM 调用时的兜底。
func planStaticPPTOutline(analysis learningContentAnalysis) pptOutlinePlan {
	plan := pptOutlinePlan{Title: analysis.Topic}
	for _, title := range requiredPPTSlideTitles() {
		slide := pptSlidePlan{Title: title}
		switch title {
		case "封面":
			slide.Purpose = "建立学习主题"
			slide.Bullets = append(slide.Bullets, analysis.Topic)
			if analysis.UserIntent != "" {
				slide.Bullets = append(slide.Bullets, analysis.UserIntent)
			}
		case "目录":
			slide.Purpose = "呈现学习路径"
			slide.Bullets = append(slide.Bullets, requiredPPTSlideTitles()[2:]...)
		case "背景与目标":
			slide.Purpose = "说明为什么学习"
			slide.Bullets = append(slide.Bullets, fmt.Sprintf("围绕“%s”建立学习背景和目标。", analysis.Topic), "先形成整体问题，再进入概念、机制和应用。")
		case "概念框架":
			slide.Purpose = "提炼关键概念并建立概念关系"
			slide.Bullets = append(slide.Bullets, analysis.KeyConcepts...)
		case "机制与流程":
			slide.Purpose = "组织过程和因果"
			slide.Bullets = append(slide.Bullets, analysis.Processes...)
		case "案例与应用":
			slide.Purpose = "连接例子和迁移"
			slide.Bullets = append(slide.Bullets, analysis.Examples...)
		case "易错辨析":
			slide.Purpose = "提示边界和误区"
			slide.Bullets = append(slide.Bullets, "区分相近概念、条件范围和结论边界。")
		case "总结复盘":
			slide.Purpose = "收束学习结论"
			slide.Bullets = append(slide.Bullets, fmt.Sprintf("用结构化方式回顾“%s”的核心内容。", analysis.Topic))
		}
		if len(slide.Bullets) == 0 {
			slide.Bullets = append(slide.Bullets, supplementBullet(title, 1))
		}
		plan.Slides = append(plan.Slides, slide)
	}
	return plan
}

// planDynamicPPTOutline 动态生成 PPT 大纲，包含封面、目录、内容页和总结页。
func planDynamicPPTOutline(analysis learningContentAnalysis) pptOutlinePlan {
	plan := pptOutlinePlan{Title: analysis.Topic}
	contentSlides := pptContentSlidesFromAnalysis(analysis)
	plan.Slides = append(plan.Slides, pptSlidePlan{
		Title:   "封面",
		Purpose: "建立演示主题和受众预期",
		Bullets: uniqueNonEmpty([]string{analysis.Topic, analysis.UserIntent}),
	})
	agenda := pptSlidePlan{
		Title:   "目录",
		Purpose: "呈现演示路径",
	}
	for _, slide := range contentSlides {
		agenda.Bullets = append(agenda.Bullets, slide.Title)
	}
	normalizePPTBullets(&agenda)
	plan.Slides = append(plan.Slides, agenda)
	plan.Slides = append(plan.Slides, contentSlides...)
	if len(contentSlides) == 0 {
		plan.Slides = append(plan.Slides, supplementalPPTSlides(analysis, 1)...)
	}
	plan.Slides = append(plan.Slides, pptSlidePlan{
		Title:   "总结与行动",
		Purpose: "收束核心结论并给出下一步",
		Bullets: []string{
			fmt.Sprintf("回顾“%s”的核心结构和关键判断。", analysis.Topic),
			"提炼最需要记住的结论、风险点和应用场景。",
			"明确后续复习、讲解或执行时的重点顺序。",
		},
	})
	return plan
}

// pptContentSlidesFromAnalysis 从分析结果的章节派生内容页切片。
func pptContentSlidesFromAnalysis(analysis learningContentAnalysis) []pptSlidePlan {
	sections := analysis.Sections
	if len(sections) == 0 {
		sections = sectionsFromFlatPoints(analysis.KeyConcepts, 12)
	}
	if len(sections) == 0 {
		sections = []pptSourceSection{{
			Title:  "核心内容",
			Points: append([]string{}, analysis.KeyConcepts...),
		}}
	}
	if len(sections) > 16 {
		sections = sections[:16]
	}
	slides := make([]pptSlidePlan, 0, len(sections))
	for i, section := range sections {
		title := strings.TrimSpace(section.Title)
		if title == "" {
			title = fmt.Sprintf("内容展开 %d", i+1)
		}
		points := uniqueNonEmpty(section.Points)
		if len(points) == 0 && i < len(analysis.Evidence) {
			points = append(points, analysis.Evidence[i].Text)
		}
		if len(points) == 0 {
			continue
		}
		for chunkIndex, chunk := range chunkPPTPoints(points, 4) {
			slideTitle := title
			if chunkIndex > 0 {
				slideTitle = fmt.Sprintf("%s (%d)", title, chunkIndex+1)
			}
			slides = append(slides, pptSlidePlan{
				Title:   slideTitle,
				Purpose: pptSectionPurpose(title, i, len(sections)),
				Bullets: chunk,
			})
		}
	}
	return slides
}

// chunkPPTPoints 将要点按指定大小切片成多组。
func chunkPPTPoints(points []string, size int) [][]string {
	points = uniqueNonEmpty(points)
	if len(points) == 0 {
		return nil
	}
	if size <= 0 {
		size = 4
	}
	chunks := make([][]string, 0, (len(points)+size-1)/size)
	for start := 0; start < len(points); start += size {
		end := start + size
		if end > len(points) {
			end = len(points)
		}
		chunks = append(chunks, append([]string{}, points[start:end]...))
	}
	return chunks
}

// sectionsFromFlatPoints 将扁平要点按每组 4 条聚合成章节。
func sectionsFromFlatPoints(points []string, maxSections int) []pptSourceSection {
	points = uniqueNonEmpty(points)
	if len(points) == 0 {
		return nil
	}
	if maxSections <= 0 {
		maxSections = 12
	}
	var sections []pptSourceSection
	for i := 0; i < len(points) && len(sections) < maxSections; i += 4 {
		end := i + 4
		if end > len(points) {
			end = len(points)
		}
		sections = append(sections, pptSourceSection{
			Title:  summarizeLine(points[i], 36),
			Points: append([]string{}, points[i:end]...),
		})
	}
	return sections
}

// supplementalPPTSlides 在缺少内容章节时补充背景、关系、应用等幻灯片。
func supplementalPPTSlides(analysis learningContentAnalysis, count int) []pptSlidePlan {
	if len(analysis.Sections) > 0 {
		return nil
	}
	candidates := []pptSlidePlan{
		{
			Title:   "背景与目标",
			Purpose: "说明材料背景和演示目标",
			Bullets: []string{fmt.Sprintf("围绕“%s”建立背景、问题和目标。", analysis.Topic), "说明为什么需要理解这些内容。", "给出本次演示的学习或行动范围。"},
		},
		{
			Title:   "关键关系",
			Purpose: "梳理概念之间的关系",
			Bullets: append([]string{}, analysis.KeyConcepts...),
		},
		{
			Title:   "应用与案例",
			Purpose: "连接材料和实际使用场景",
			Bullets: append([]string{}, analysis.Examples...),
		},
	}
	if count > len(candidates) {
		count = len(candidates)
	}
	return candidates[:count]
}

// appendRealEvidencePPTSlides 向幻灯片集合追加未使用过的真实证据要点。
func appendRealEvidencePPTSlides(slides []pptSlidePlan, analysis learningContentAnalysis, count int) []pptSlidePlan {
	if count <= 0 || len(analysis.Evidence) == 0 {
		return slides
	}
	used := map[string]struct{}{}
	for _, slide := range slides {
		for _, bullet := range slide.Bullets {
			used[strings.TrimSpace(bullet)] = struct{}{}
		}
	}
	points := make([]string, 0, count*3)
	for _, ev := range analysis.Evidence {
		point := strings.TrimSpace(ev.Text)
		if point == "" {
			continue
		}
		if _, ok := used[point]; ok {
			continue
		}
		points = append(points, point)
		if len(points) >= count*3 {
			break
		}
	}
	for i := 0; i < len(points) && count > 0; i += 3 {
		end := i + 3
		if end > len(points) {
			end = len(points)
		}
		slides = append(slides, pptSlidePlan{
			Title:   fmt.Sprintf("补充资料 %d", len(slides)+1),
			Purpose: "呈现检索或引用资料中的真实要点",
			Bullets: append([]string{}, points[i:end]...),
		})
		count--
	}
	return slides
}

// pptSectionPurpose 根据章节位置返回该章节幻灯片的目的说明。
func pptSectionPurpose(title string, index, total int) string {
	switch {
	case index == 0:
		return "展开材料开头的核心背景和问题"
	case index == total-1:
		return "收束本部分材料并提炼结论"
	default:
		return fmt.Sprintf("解释“%s”的关键论点、证据和推导关系", title)
	}
}

// expandPPTContent 基于证据和概念池扩充每张幻灯片的要点数量。
func expandPPTContent(plan pptOutlinePlan, analysis learningContentAnalysis) pptOutlinePlan {
	minBullets := 4
	if analysis.Sparse {
		minBullets = 5
	}
	type evidenceItem struct {
		text     string
		keywords []string
		used     bool
	}
	pool := make([]evidenceItem, 0, len(analysis.Evidence))
	for _, ev := range analysis.Evidence {
		pool = append(pool, evidenceItem{
			text:     ev.Text,
			keywords: extractSignificantWords(ev.Text),
		})
	}
	conceptPool := make([]string, 0, len(analysis.KeyConcepts))
	conceptPool = append(conceptPool, analysis.KeyConcepts...)
	processPool := make([]string, 0, len(analysis.Processes))
	processPool = append(processPool, analysis.Processes...)

	for i := range plan.Slides {
		slide := &plan.Slides[i]
		if slide.Title == "封面" || slide.Title == "目录" || slide.Title == "封面页" || slide.Title == "目录页" {
			normalizePPTBullets(slide)
			continue
		}
		slideKeywords := extractSignificantWords(slide.Title)
		for _, b := range slide.Bullets {
			slideKeywords = append(slideKeywords, extractSignificantWords(b)...)
		}
		slideKeywordSet := make(map[string]bool)
		for _, kw := range slideKeywords {
			slideKeywordSet[strings.ToLower(kw)] = true
		}

		for len(slide.Bullets) < minBullets {
			bestIdx := -1
			bestScore := 0
			for j := range pool {
				if pool[j].used {
					continue
				}
				score := 0
				for _, kw := range pool[j].keywords {
					if slideKeywordSet[strings.ToLower(kw)] {
						score++
					}
				}
				if score > bestScore {
					bestScore = score
					bestIdx = j
				}
			}
			if bestIdx >= 0 {
				slide.Bullets = append(slide.Bullets, pool[bestIdx].text)
				pool[bestIdx].used = true
			} else {
				break
			}
		}

		if len(slide.Bullets) < minBullets {
			for j, concept := range conceptPool {
				if len(slide.Bullets) >= minBullets {
					break
				}
				conceptKeywords := extractSignificantWords(concept)
				matched := false
				for _, kw := range conceptKeywords {
					if slideKeywordSet[strings.ToLower(kw)] {
						matched = true
						break
					}
				}
				if matched {
					slide.Bullets = append(slide.Bullets, concept)
					conceptPool = append(conceptPool[:j], conceptPool[j+1:]...)
				}
			}
		}

		if len(slide.Bullets) < minBullets {
			for j, proc := range processPool {
				if len(slide.Bullets) >= minBullets {
					break
				}
				procKeywords := extractSignificantWords(proc)
				matched := false
				for _, kw := range procKeywords {
					if slideKeywordSet[strings.ToLower(kw)] {
						matched = true
						break
					}
				}
				if matched {
					slide.Bullets = append(slide.Bullets, proc)
					processPool = append(processPool[:j], processPool[j+1:]...)
				}
			}
		}

		if len(slide.Bullets) < minBullets {
			for _, concept := range conceptPool {
				if len(slide.Bullets) >= minBullets {
					break
				}
				slide.Bullets = append(slide.Bullets, concept)
			}
		}
		if len(slide.Bullets) < minBullets {
			for _, proc := range processPool {
				if len(slide.Bullets) >= minBullets {
					break
				}
				slide.Bullets = append(slide.Bullets, proc)
			}
		}

		for len(slide.Bullets) < minBullets {
			slide.Bullets = append(slide.Bullets, supplementBullet(slide.Title, len(slide.Bullets)+1))
		}
		normalizePPTBullets(slide)
	}
	return plan
}
