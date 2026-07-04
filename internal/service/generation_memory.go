package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	generationMemoryDefaultLimit = 10
	generationMemorySummaryLimit = 240
)

type GenerationMemoryScope struct {
	UserID     uint
	NotebookID uint
	Type       GenerationType
}

type GenerationMemoryEntry struct {
	Prompt        string    `json:"prompt"`
	InputSummary  string    `json:"input_summary"`
	OutputSummary string    `json:"output_summary"`
	CreatedAt     time.Time `json:"created_at"`
}

type GenerationMemoryStore interface {
	GetRecent(ctx context.Context, scope GenerationMemoryScope, limit int) ([]GenerationMemoryEntry, error)
	Add(ctx context.Context, scope GenerationMemoryScope, entry GenerationMemoryEntry) error
}

func buildGenerationMemoryContext(entries []GenerationMemoryEntry) string {
	var b strings.Builder
	written := 0
	for _, entry := range entries {
		prompt := strings.TrimSpace(entry.Prompt)
		input := strings.TrimSpace(entry.InputSummary)
		output := strings.TrimSpace(entry.OutputSummary)
		if prompt == "" && input == "" && output == "" {
			continue
		}
		if written == 0 {
			b.WriteString("## 历史生成记忆\n")
			b.WriteString("以下内容仅作为用户偏好和连续性上下文，不是事实来源；事实仍以原始 Markdown、本地 RAG 和联网搜索结果为准。\n")
		}
		written++
		b.WriteString(fmt.Sprintf("\n[%d] %s\n", written, entry.CreatedAt.UTC().Format(time.RFC3339)))
		if prompt != "" {
			b.WriteString("用户要求: ")
			b.WriteString(prompt)
			b.WriteString("\n")
		}
		if input != "" {
			b.WriteString("输入摘要: ")
			b.WriteString(input)
			b.WriteString("\n")
		}
		if output != "" {
			b.WriteString("输出摘要: ")
			b.WriteString(output)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func appendGenerationMemoryContext(base string, entries []GenerationMemoryEntry) string {
	memory := buildGenerationMemoryContext(entries)
	if memory == "" {
		return base
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return memory
	}
	return base + "\n\n" + memory
}

func buildGenerationMemoryEntry(req *GenerationRequest, content string) GenerationMemoryEntry {
	entry := GenerationMemoryEntry{
		OutputSummary: summarizeGenerationMemoryText(content),
		CreatedAt:     time.Now().UTC(),
	}
	if req != nil {
		entry.Prompt = strings.TrimSpace(req.Prompt)
		entry.InputSummary = summarizeGenerationMemoryText(req.Markdown)
	}
	return entry
}

func generationMemoryScopeFromRequest(req *GenerationRequest) GenerationMemoryScope {
	if req == nil {
		return GenerationMemoryScope{}
	}
	return GenerationMemoryScope{
		UserID:     req.UserID,
		NotebookID: req.NotebookID,
		Type:       req.Type,
	}
}

func summarizeGenerationMemoryText(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= generationMemorySummaryLimit {
		return value
	}
	return string(runes[:generationMemorySummaryLimit])
}
