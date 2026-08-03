package quiz

import (
	"encoding/json"
	"strings"
)

// quizRawQuestion 是 LLM 输出的原始题目结构，字段宽松以容忍半成品。
// 区别于 QuestionItem：字段为指针/可空，便于识别 LLM 缺失的部分。
type quizRawQuestion struct {
	Type        string   `json:"type"`
	Question    string   `json:"question"`
	Options     []string `json:"options"`
	Answer      string   `json:"answer"`
	Explanation string   `json:"explanation"`
	Difficulty  string   `json:"difficulty"`
}

type quizRawPayload struct {
	Questions []quizRawQuestion `json:"questions"`
}

// validQuizQuestionTypes 合法的测验题目类型集合。
var validQuizQuestionTypes = map[string]bool{
	"single_choice": true,
	"true_false":    true,
	"multi_choice":  true,
	"fill_blank":    true,
	"short_answer":  true,
}

// isRawQuestionValid 判断单道题是否结构完整可用。
// 放宽标准：只要 type 合法、question/answer 非空、选择题选项数达标即算有效。
func isRawQuestionValid(q quizRawQuestion) bool {
	if !validQuizQuestionTypes[q.Type] {
		return false
	}
	if strings.TrimSpace(q.Question) == "" || strings.TrimSpace(q.Answer) == "" {
		return false
	}
	switch q.Type {
	case "single_choice", "multi_choice":
		if len(q.Options) < 3 {
			return false
		}
	case "true_false":
		if len(q.Options) < 2 {
			return false
		}
	}
	return true
}

// parseQuizPayload 解析 LLM 输出为原始题目载荷，容忍前后空白与 markdown 代码块。
func parseQuizPayload(content string) (quizRawPayload, bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return quizRawPayload{}, false
	}
	// 兼容 LLM 偶尔返回的 ```json ... ``` 代码块。
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)
	trimmed = stripJSONComments(trimmed)

	var payload quizRawPayload
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return quizRawPayload{}, false
	}
	return payload, true
}

// stripJSONComments removes JSONC-style comments outside string literals.
func stripJSONComments(content string) string {
	var b strings.Builder
	b.Grow(len(content))
	inString := false
	escaped := false

	for i := 0; i < len(content); i++ {
		ch := content[i]
		if inString {
			b.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			b.WriteByte(ch)
			continue
		}

		if ch == '/' && i+1 < len(content) {
			next := content[i+1]
			if next == '/' {
				i += 2
				for i < len(content) && content[i] != '\n' && content[i] != '\r' {
					i++
				}
				if i < len(content) {
					b.WriteByte(content[i])
				}
				continue
			}
			if next == '*' {
				i += 2
				for i+1 < len(content) && !(content[i] == '*' && content[i+1] == '/') {
					if content[i] == '\n' || content[i] == '\r' {
						b.WriteByte(content[i])
					}
					i++
				}
				if i+1 < len(content) {
					i++
				}
				continue
			}
		}

		b.WriteByte(ch)
	}
	return strings.TrimSpace(b.String())
}

// ValidateContent 校验测验输出是否为完整可用的题目集合。
func ValidateContent(content string) bool {
	return ValidateContentWithMin(content, minQuizQuestionCount)
}

// ValidateContentWithMin 校验测验输出是否至少包含 minQuestions 道有效题。
func ValidateContentWithMin(content string, minQuestions int) bool {
	if minQuestions <= 0 {
		minQuestions = minQuizQuestionCount
	}
	payload, ok := parseQuizPayload(content)
	if !ok {
		return false
	}
	validCount := 0
	for _, q := range payload.Questions {
		if isRawQuestionValid(q) {
			validCount++
		}
	}
	return validCount >= minQuestions
}

// NormalizeContent returns strict JSON with comments/fences removed when content is usable.
func NormalizeContent(content string) string {
	payload, ok := parseQuizPayload(content)
	if !ok || len(payload.Questions) == 0 {
		return ""
	}
	normalized := make([]quizRawQuestion, 0, len(payload.Questions))
	for _, q := range payload.Questions {
		if !isRawQuestionValid(q) {
			return ""
		}
		normalized = append(normalized, normalizeRawQuestion(q))
	}
	return renderRawQuestions(normalized)
}

// normalizeRawQuestion 规范化单道题：
//   - 题型不合法 → 改为 short_answer（最宽松的题型，不需 options）
//   - 选择题选项不足 → 补齐占位选项
//   - 难度为空 → 默认 medium
//   - answer/question 去除首尾空白
func normalizeRawQuestion(q quizRawQuestion) quizRawQuestion {
	q.Question = strings.TrimSpace(q.Question)
	q.Answer = strings.TrimSpace(q.Answer)
	q.Explanation = strings.TrimSpace(q.Explanation)
	q.Difficulty = strings.TrimSpace(q.Difficulty)
	if q.Difficulty == "" {
		q.Difficulty = "medium"
	}
	if !validQuizQuestionTypes[q.Type] {
		q.Type = "short_answer"
		q.Options = nil
	}
	switch q.Type {
	case "single_choice", "multi_choice":
		// 选项不足 3 个时补齐占位项，避免整题作废。
		for len(q.Options) < 3 {
			q.Options = append(q.Options, "（选项待补充）")
		}
	case "true_false":
		// 判断题选项不足时使用标准 ["正确","错误"]。
		if len(q.Options) < 2 {
			q.Options = []string{"正确", "错误"}
		}
	case "fill_blank", "short_answer":
		// 填空题/简答题不需要 options。
		q.Options = nil
	}
	return q
}

// repairInvalidQuestion 将单道结构不完整的题目修复为可用题目。
// 若 question/answer 任一为空无法修复，则返回 false。
func repairInvalidQuestion(q *quizRawQuestion) bool {
	q.Question = strings.TrimSpace(q.Question)
	q.Answer = strings.TrimSpace(q.Answer)
	if q.Question == "" || q.Answer == "" {
		return false
	}
	if !validQuizQuestionTypes[q.Type] {
		q.Type = "short_answer"
	}
	q.Difficulty = strings.TrimSpace(q.Difficulty)
	if q.Difficulty == "" {
		q.Difficulty = "medium"
	}
	switch q.Type {
	case "single_choice", "multi_choice":
		for len(q.Options) < 3 {
			q.Options = append(q.Options, "（选项待补充）")
		}
	case "true_false":
		if len(q.Options) < 2 {
			q.Options = []string{"正确", "错误"}
		}
	case "fill_blank", "short_answer":
		q.Options = nil
	}
	q.Explanation = strings.TrimSpace(q.Explanation)
	if q.Explanation == "" {
		q.Explanation = "该答案来自提供的笔记上下文。"
	}
	return true
}

// RepairContent 修复 LLM 输出的半成品测验内容。
//
// 策略：
//  1. JSON 解析失败 → 返回空字符串，由调用方走 fallback
//  2. 解析成功但无任何有效题 → 返回空字符串，由调用方走 fallback
//  3. 存在部分有效题 → 规范化无效题（type 不合法改 short_answer、选项不足补齐），
//     跳过 question/answer 都为空无法修复的题
//  4. 有效题不足最低题量 → 用 planner 风格的模板题补齐
//
// 返回的 content 必定通过 ValidateContent（除非输入完全无法解析）。
func RepairContent(content string) string {
	return RepairContentWithMin(content, minQuizQuestionCount)
}

// RepairContentWithMin 修复 LLM 输出，并补齐到指定的最低题量。
func RepairContentWithMin(content string, minQuestions int) string {
	if minQuestions <= 0 {
		minQuestions = minQuizQuestionCount
	}
	payload, ok := parseQuizPayload(content)
	if !ok || len(payload.Questions) == 0 {
		return ""
	}

	repaired := make([]quizRawQuestion, 0, len(payload.Questions))
	for _, q := range payload.Questions {
		if isRawQuestionValid(q) {
			repaired = append(repaired, normalizeRawQuestion(q))
			continue
		}
		if repairInvalidQuestion(&q) {
			repaired = append(repaired, q)
		}
	}

	if len(repaired) == 0 {
		return ""
	}

	// 有效题不足最低题量时补齐，避免题量太少。
	for len(repaired) < minQuestions {
		repaired = append(repaired, quizRawQuestion{
			Type:        "short_answer",
			Question:    "请简述本主题的核心要点。",
			Answer:      "（请根据笔记原文补充核心要点）",
			Explanation: "该答案来自提供的笔记上下文。",
			Difficulty:  "medium",
		})
	}

	return renderRawQuestions(repaired)
}

// renderRawQuestions 将原始题目列表渲染为 JSON 字符串。
// 与 Render 的区别：保留 LLM 原始字段（如 difficulty），不依赖 QuestionItem。
func renderRawQuestions(questions []quizRawQuestion) string {
	items := make([]string, 0, len(questions))
	for _, q := range questions {
		options := make([]string, 0, len(q.Options))
		for _, opt := range q.Options {
			options = append(options, jsonQuote(opt))
		}
		difficulty := q.Difficulty
		if difficulty == "" {
			difficulty = "medium"
		}
		item := `{"type":` + jsonQuote(q.Type) +
			`,"question":` + jsonQuote(q.Question) +
			`,"options":[` + strings.Join(options, ",") + `]` +
			`,"answer":` + jsonQuote(q.Answer) +
			`,"explanation":` + jsonQuote(q.Explanation) +
			`,"difficulty":` + jsonQuote(difficulty) +
			`}`
		items = append(items, item)
	}
	return `{"questions":[` + strings.Join(items, ",") + `]}`
}

// jsonQuote 对字符串做 JSON 字符串字面量转义。
func jsonQuote(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}
