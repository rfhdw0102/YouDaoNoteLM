package service

import (
	"encoding/json"
	"strings"
)

func validateMindmapContent(content string) bool {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "#") {
		return false
	}
	return strings.Contains(content, "\n## ") || strings.Contains(content, "\n- ")
}

func validatePPTContent(content string) bool {
	lower := strings.ToLower(strings.TrimSpace(content))
	if !strings.Contains(lower, "<section") || !strings.Contains(lower, "</section>") {
		return false
	}
	text := strings.TrimSpace(stripSimpleHTML(content))
	return len([]rune(text)) >= 4
}

func validateQuizContent(content string) bool {
	var payload struct {
		Questions []struct {
			Question    string   `json:"question"`
			Options     []string `json:"options"`
			Answer      string   `json:"answer"`
			Explanation string   `json:"explanation"`
		} `json:"questions"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &payload); err != nil {
		return false
	}
	if len(payload.Questions) == 0 {
		return false
	}
	for _, question := range payload.Questions {
		if strings.TrimSpace(question.Question) == "" || strings.TrimSpace(question.Answer) == "" {
			return false
		}
	}
	return true
}

func validateNoteContent(content string) bool {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "#") {
		return false
	}
	lines := strings.Split(content, "\n")
	bodyRunes := 0
	hasSection := false
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") {
			hasSection = true
		}
		if line != "" && !strings.HasPrefix(line, "#") {
			bodyRunes += len([]rune(line))
		}
	}
	return hasSection || bodyRunes >= 8
}

func stripSimpleHTML(content string) string {
	var b strings.Builder
	inTag := false
	for _, r := range content {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}
