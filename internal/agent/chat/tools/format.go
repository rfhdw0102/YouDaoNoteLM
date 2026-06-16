package tools

import (
	"fmt"
	"strings"

	"YoudaoNoteLm/internal/rag"
	"YoudaoNoteLm/pkg/cache"
)

// FormatRetrievalResults 格式化检索结果，offset 用于多次检索时编号连续
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
		content := r.ParentContent
		if content == "" {
			content = r.Content
		}
		sb.WriteString(fmt.Sprintf("内容: %s\n", content))
		sb.WriteString(fmt.Sprintf("相关度: %.2f\n\n", r.Score))
	}
	return sb.String()
}

// FormatChatHistoryWithSummary 格式化对话历史（含摘要）
func FormatChatHistoryWithSummary(summary string, hasSummary bool, history []cache.MessagePair) string {
	var sb strings.Builder

	// 先输出摘要
	if hasSummary && summary != "" {
		sb.WriteString("【对话摘要】\n")
		sb.WriteString(summary)
		sb.WriteString("\n\n")
	}

	// 再输出历史消息
	if len(history) == 0 {
		sb.WriteString("暂无对话历史")
	} else {
		sb.WriteString("【最近对话】\n")
		for i, pair := range history {
			sb.WriteString(fmt.Sprintf("第 %d 轮:\n", i+1))
			sb.WriteString(fmt.Sprintf("用户: %s\n", pair.User))
			sb.WriteString(fmt.Sprintf("助手: %s\n\n", pair.Assistant))
		}
	}

	return sb.String()
}
