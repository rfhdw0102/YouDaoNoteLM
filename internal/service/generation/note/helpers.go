package note

import (
	"fmt"
	"strings"
)

// 压缩资料片段，避免笔记要点过长。
func summarizeLine(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}

// 保留非空且不重复的要点。
func uniqueNonEmpty(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// 为资料不足的章节补充结构化学习要点。
func supplementBullet(title string, detail int) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Sprintf("补充要点 %d：结合材料进一步分析该主题的关键内容。", detail)
	}
	templates := []string{
		fmt.Sprintf("%s的核心定义：用一句话精确概括%s是什么，区分它与其他相近概念的本质差异。", title, title),
		fmt.Sprintf("%s的运作原理：解释%s背后的因果机制或数学/逻辑基础，说明为什么它这样工作而非那样工作。", title, title),
		fmt.Sprintf("%s的关键特征：列出%s的3个核心特征，每个特征给出一个具体例子或量化数据支撑。", title, title),
		fmt.Sprintf("%s的应用场景：描述%s在真实场景中的典型用法，给出一个具体的操作步骤或数值案例。", title, title),
		fmt.Sprintf("%s的常见误区：指出学习%s时最容易犯的2-3个错误，说明正确理解应该是什么。", title, title),
		fmt.Sprintf("%s与其他概念的关系：说明%s在整体知识体系中的位置，它依赖什么前置知识，又是什么后续知识的基础。", title, title),
	}
	idx := (detail - 1) % len(templates)
	return templates[idx]
}

// 判断当前章节是否已经存在补充要点。
func hasSupplementBullet(values []string) bool {
	for _, value := range values {
		if strings.Contains(value, "解释补充") || strings.Contains(value, "补充要点") {
			return true
		}
	}
	return false
}
