package tools

import (
	"fmt"
	"strings"

	"YoudaoNoteLm/internal/rag"
	"YoudaoNoteLm/pkg/cache"
)

// FormatRetrievalResults 格式化检索结果
func FormatRetrievalResults(results []*rag.RetrieveResult) string {
	if len(results) == 0 {
		return "未找到相关资料"
	}

	var sb strings.Builder
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("[%d] 来源: %s\n", i+1, r.SourceName))
		if r.Heading != "" {
			sb.WriteString(fmt.Sprintf("章节: %s\n", r.Heading))
		}
		sb.WriteString(fmt.Sprintf("内容: %s\n", r.Content))
		sb.WriteString(fmt.Sprintf("相关度: %.2f\n\n", r.Score))
	}
	return sb.String()
}

// FormatChatHistory 格式化对话历史
func FormatChatHistory(history []cache.MessagePair) string {
	if len(history) == 0 {
		return "暂无对话历史"
	}

	var sb strings.Builder
	for i, pair := range history {
		sb.WriteString(fmt.Sprintf("第 %d 轮:\n", i+1))
		sb.WriteString(fmt.Sprintf("用户: %s\n", pair.User))
		sb.WriteString(fmt.Sprintf("助手: %s\n\n", pair.Assistant))
	}
	return sb.String()
}
