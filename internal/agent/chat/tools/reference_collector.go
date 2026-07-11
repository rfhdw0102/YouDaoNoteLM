package tools

import "YoudaoNoteLm/internal/model/dto/response"

// ReferenceCollector 跨多次检索调用累积引用，保证编号与 LLM 看到的连续一致。
type ReferenceCollector struct {
	references []response.Reference
}

// NewReferenceCollector 创建引用收集器
func NewReferenceCollector() *ReferenceCollector {
	return &ReferenceCollector{references: make([]response.Reference, 0)}
}

// Add 追加一批引用，返回这批在全局列表中的起始编号（从 1 开始）。
func (c *ReferenceCollector) Add(refs []response.Reference) int {
	start := len(c.references) + 1
	c.references = append(c.references, refs...)
	return start
}

// All 返回累积的全部引用
func (c *ReferenceCollector) All() []response.Reference {
	return c.references
}
