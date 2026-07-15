package quiz

import (
	"encoding/json"
	"strings"
)

// 校验测验输出是否为可用题目集合。
func ValidateContent(content string) bool {
	var payload struct {
		Questions []struct {
			Type        string   `json:"type"`
			Question    string   `json:"question"`
			Options     []string `json:"options"`
			Answer      string   `json:"answer"`
			Explanation string   `json:"explanation"`
		} `json:"questions"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &payload); err != nil {
		return false
	}
	if len(payload.Questions) < 5 {
		return false
	}
	validTypes := map[string]bool{
		"single_choice": true, "true_false": true, "multi_choice": true,
		"fill_blank": true, "short_answer": true,
	}
	typeSet := make(map[string]bool)
	for _, question := range payload.Questions {
		if !validTypes[question.Type] {
			return false
		}
		if strings.TrimSpace(question.Question) == "" || strings.TrimSpace(question.Answer) == "" {
			return false
		}
		typeSet[question.Type] = true
		switch question.Type {
		case "single_choice", "multi_choice":
			if len(question.Options) < 3 {
				return false
			}
		case "true_false":
			if len(question.Options) < 2 {
				return false
			}
		}
	}
	return len(typeSet) >= 2
}
