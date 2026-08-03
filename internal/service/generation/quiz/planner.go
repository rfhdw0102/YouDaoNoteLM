package quiz

import (
	"fmt"
	"strings"
)

const minQuizQuestionCount = 10

func targetQuizQuestionCount(analysis Analysis) int {
	totalPoints := len(analysis.KeyConcepts) + len(analysis.Processes) + len(analysis.Examples)
	targetCount := minQuizQuestionCount
	if totalPoints >= 12 {
		targetCount = 12
	}
	if totalPoints >= 24 {
		targetCount = 15
	}
	return targetCount
}

// requiredQuizQuestionTypes 根据材料丰富度决定题目类型与数量。
func requiredQuizQuestionTypes(analysis Analysis) []string {
	conceptCount := len(analysis.KeyConcepts)
	processCount := len(analysis.Processes)

	// 根据材料丰富度决定题目数量
	targetCount := targetQuizQuestionCount(analysis)

	types := []string{"single_choice", "true_false"}

	// 如果有对比性知识点，加多选题
	if conceptCount >= 3 {
		types = append(types, "multi_choice")
	}

	// 如果有过程性知识点，加填空题
	if processCount >= 1 {
		types = append(types, "fill_blank")
	}

	for len(types) < targetCount {
		types = append(types, "short_answer")
	}

	// 如果超了就截断
	if len(types) > targetCount {
		types = types[:targetCount]
	}

	return types
}

// PlanQuestions 根据学习分析生成测验题目规划。
func PlanQuestions(analysis Analysis) QuestionPlan {
	plan := QuestionPlan{Topic: analysis.Topic}
	types := requiredQuizQuestionTypes(analysis)
	concepts := append([]string{}, analysis.KeyConcepts...)
	processes := append([]string{}, analysis.Processes...)
	examples := append([]string{}, analysis.Examples...)
	if len(concepts) == 0 {
		concepts = append(concepts, analysis.Topic)
	}

	for i, qType := range types {
		item := QuestionItem{Type: qType}
		switch qType {
		case "single_choice":
			topic := pickPoint(concepts, i)
			item.Topic = topic
			item.Question = fmt.Sprintf("关于“%s”，下列说法正确的是？", topic)
			item.Options = []string{
				topic + " 的基本定义如上所述。",
				"与原文相反的描述。",
				"无关的干扰项。",
				"概念混淆的选项。",
			}
			item.Answer = item.Options[0]
			item.Explanation = fmt.Sprintf("根据笔记，“%s”的定义如原文所述。", topic)
			item.Difficulty = "easy"
		case "true_false":
			topic := pickPoint(concepts, i+1)
			item.Topic = topic
			item.Question = fmt.Sprintf("判断：%s。", topic)
			item.Options = []string{"正确", "错误"}
			item.Answer = "正确"
			item.Explanation = fmt.Sprintf("根据笔记内容，该说法是正确的。“%s”的定义和描述如原文所述。", topic)
			item.Difficulty = "easy"
		case "multi_choice":
			topic := pickPoint(concepts, i+2)
			if topic == "" {
				topic = pickPoint(concepts, 0)
			}
			item.Topic = topic
			item.Question = fmt.Sprintf("关于“%s”，以下哪些说法是正确的？（多选）", topic)
			item.Options = []string{
				topic + " 的基本定义。",
				topic + " 的关键特征。",
				"与原文矛盾的描述。",
				topic + " 的适用条件。",
			}
			item.Answer = item.Options[0] + "；" + item.Options[1] + "；" + item.Options[3]
			item.Explanation = fmt.Sprintf("选项A、B、D正确。选项C与原文矛盾。关于“%s”的详细说明见笔记原文。", topic)
			item.Difficulty = "medium"
		case "fill_blank":
			if len(processes) > 0 {
				topic := pickPoint(processes, i)
				item.Topic = topic
				item.Question = fmt.Sprintf("“%s”的关键步骤是____。", topic)
				item.Answer = summarizeLine(topic, 100)
				item.Explanation = "该答案来自提供的笔记上下文。"
			} else {
				topic := pickPoint(concepts, i)
				item.Topic = topic
				item.Question = fmt.Sprintf("请填写“%s”的核心定义中的关键词：____。", topic)
				item.Answer = summarizeLine(topic, 100)
				item.Explanation = "该答案来自提供的笔记上下文。"
			}
			item.Difficulty = "medium"
		case "short_answer":
			if len(processes) > 0 {
				topic := pickPoint(processes, i)
				item.Topic = topic
				item.Question = fmt.Sprintf("简述“%s”的关键步骤或机制。", topic)
				item.Answer = summarizeLine(topic, 100)
				item.Explanation = "该答案来自提供的笔记上下文。"
			} else {
				topic := pickPoint(examples, i)
				if topic == "" {
					topic = pickPoint(concepts, i)
				}
				item.Topic = topic
				item.Question = fmt.Sprintf("说明“%s”的应用场景或例子。", topic)
				item.Answer = summarizeLine(topic, 100)
				item.Explanation = "该答案来自提供的笔记上下文。"
			}
			item.Difficulty = "hard"
		}
		plan.Questions = append(plan.Questions, item)
	}
	return plan
}

// ExpandContent 扩展测验题目解析并补足题量。
func ExpandContent(plan QuestionPlan, analysis Analysis) QuestionPlan {
	expanded := plan
	evidenceIndex := 0
	for i := range expanded.Questions {
		q := &expanded.Questions[i]
		evidence := nextQuizEvidence(analysis.Evidence, &evidenceIndex)
		if evidence != "" && q.Explanation != "" && !strings.Contains(q.Explanation, "资料要点：") {
			q.Explanation = q.Explanation + "（资料要点：" + summarizeLine(evidence, 80) + "）"
		}
		if strings.TrimSpace(q.Answer) == "" {
			q.Answer = summarizeLine(q.Topic, 100)
		}
		if strings.TrimSpace(q.Explanation) == "" {
			q.Explanation = "该答案来自提供的笔记上下文。"
		}
	}
	for len(expanded.Questions) < targetQuizQuestionCount(analysis) {
		topic := analysis.Topic
		if len(analysis.KeyConcepts) > len(expanded.Questions) {
			topic = analysis.KeyConcepts[len(expanded.Questions)]
		}
		expanded.Questions = append(expanded.Questions, QuestionItem{
			Type:        "short_answer",
			Topic:       topic,
			Question:    fmt.Sprintf("简述“%s”的核心观点。", topic),
			Answer:      summarizeLine(topic, 100),
			Explanation: "该答案来自提供的笔记上下文。",
		})
	}
	return expanded
}

// nextQuizEvidence 按索引循环返回下一条证据的摘要。
func nextQuizEvidence(evidence []Evidence, index *int) string {
	if len(evidence) == 0 {
		return ""
	}
	if index == nil {
		return summarizeLine(strings.TrimSpace(evidence[0].Text), 80)
	}
	ev := evidence[*index%len(evidence)]
	*index++
	return summarizeLine(strings.TrimSpace(ev.Text), 80)
}

// Render 将测验规划渲染为 JSON 字符串。
func Render(plan QuestionPlan) string {
	items := make([]string, 0, len(plan.Questions))
	for _, q := range plan.Questions {
		options := make([]string, 0, len(q.Options))
		for _, opt := range q.Options {
			options = append(options, fmt.Sprintf("%q", opt))
		}
		item := fmt.Sprintf(`{"type":%q,"question":%q,"options":[%s],"answer":%q,"explanation":%q,"difficulty":%q}`,
			q.Type, q.Question, strings.Join(options, ","), q.Answer, q.Explanation, q.Difficulty)
		items = append(items, item)
	}
	return `{"questions":[` + strings.Join(items, ",") + `]}`
}

// renderPlan 将测验规划渲染为内部上下文用的文本格式。
func renderPlan(plan QuestionPlan) string {
	var b strings.Builder
	if strings.TrimSpace(plan.Topic) != "" {
		b.WriteString("Topic: ")
		b.WriteString(strings.TrimSpace(plan.Topic))
		b.WriteString("\n")
	}
	for i, q := range plan.Questions {
		b.WriteString(fmt.Sprintf("Question %02d: [%s] %s\n", i+1, q.Type, strings.TrimSpace(q.Question)))
		if strings.TrimSpace(q.Topic) != "" {
			b.WriteString("Focus: ")
			b.WriteString(strings.TrimSpace(q.Topic))
			b.WriteString("\n")
		}
		for j, opt := range q.Options {
			b.WriteString(fmt.Sprintf("  Option %d: %s\n", j+1, opt))
		}
		if strings.TrimSpace(q.Answer) != "" {
			b.WriteString("Answer: ")
			b.WriteString(strings.TrimSpace(q.Answer))
			b.WriteString("\n")
		}
		if strings.TrimSpace(q.Explanation) != "" {
			b.WriteString("Explanation: ")
			b.WriteString(strings.TrimSpace(q.Explanation))
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

// AppendPlansToContext 将测验规划与扩展结果及生成规则拼入上下文。
func AppendPlansToContext(contextValue string, plan, expanded QuestionPlan) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(contextValue))
	if strings.TrimSpace(plan.Topic) != "" {
		b.WriteString("\n\nINTERNAL_QUIZ_PLAN\n")
		b.WriteString("内部测验规划：\n")
		b.WriteString(renderPlan(plan))
	}
	if strings.TrimSpace(expanded.Topic) != "" {
		b.WriteString("\n\nINTERNAL_QUIZ_EXPANDED_PLAN\n")
		b.WriteString("内部测验扩展：\n")
		b.WriteString(Render(expanded))
		b.WriteString("\n\nQUIZ_GENERATION_RULES\n")
		b.WriteString("- Follow the planned question types and topics; do not omit or merge questions.\n")
		b.WriteString("- Every question must include a non-empty answer and explanation.\n")
		b.WriteString("- For single_choice, provide 3-4 options and mark the correct one in the answer field with the exact option text.\n")
		b.WriteString("- For true_false, options must be [\"正确\",\"错误\"], answer must be \"正确\" or \"错误\".\n")
		b.WriteString("- For multi_choice, provide 4-5 options, answer must be all correct option texts joined by semicolons (；).\n")
		b.WriteString("- For fill_blank, options must be an empty array [], answer must be the key term or phrase to fill in.\n")
		b.WriteString("- For short_answer, options must be an empty array [], answer must be a reference answer.\n")
		b.WriteString(fmt.Sprintf("- Generate at least %d questions, covering at least 2 different question types.\n", len(expanded.Questions)))
		b.WriteString("- Distribute difficulty levels: roughly 40% easy, 40% medium, 20% hard.\n")
		b.WriteString("- Content must be grounded in Original Markdown, Local References, Web Results, or the user's explicit prompt.\n")
		b.WriteString("- Return only the JSON object, no markdown fences or extra text.\n")
	}
	return strings.TrimSpace(b.String())
}

// NeedsStructureRepair 判断测验输出是否结构不达标需修复。
func NeedsStructureRepair(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return true
	}
	if !ValidateContent(trimmed) {
		return true
	}
	return false
}
