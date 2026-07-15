// generation_validators.go 实现各生成类型的内容校验。
//
// 每种生成类型有对应的校验函数（validateNoteContent / validateMindmapContent /
// validatePPTContent / validateQuizContent），在 Agent 链的 formatValidate 步骤调用，
// 确保生成内容符合该类型的最小结构要求。
package generation

import (
	"strings"

	"YoudaoNoteLm/internal/service/generation/mindmap"
	"YoudaoNoteLm/internal/service/generation/note"
	"YoudaoNoteLm/internal/service/generation/ppt"
	"YoudaoNoteLm/internal/service/generation/quiz"
)

func validateMindmapContent(content string) bool {
	return mindmap.ValidateContent(content)
}

func validatePPTContent(content string) bool {
	return ppt.ValidateContent(content)
}

func validateQuizContent(content string) bool {
	return quiz.ValidateContent(content)
}

func validateNoteContent(content string) bool {
	return note.ValidateContent(content)
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
