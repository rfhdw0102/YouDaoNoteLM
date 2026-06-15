package service

import "context"

// StructureMeta 结构化元信息
type StructureMeta struct {
	Title      string // 原始标题（如有）
	SourceType string // "youdao" / "url" / "file" / "audio"
}

// MarkdownStructurer markdown 结构化服务接口
type MarkdownStructurer interface {
	// Structure 给 markdown 内容补充结构
	// - 已有结构（检测到 heading ≥ 2）→ 跳过
	// - 无结构 → 调用 LLM 补充标题/段落
	Structure(ctx context.Context, userID uint, content string, meta StructureMeta) (string, error)
}
