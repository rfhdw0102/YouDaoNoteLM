package rag

import (
	"context"
	"io"
	"strings"

	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// astDocument 解析后的文档结构
type astDocument struct {
	Sections []*section
}

// section 文档章节
type section struct {
	Heading     string
	Level       int    // 1=H1, 2=H2, ...
	ChapterPath string // 完整路径，如 "第一章 > 1.1 背景"
	Content     []contentBlock
}

// contentBlock 内容块
type contentBlock struct {
	Type     string // paragraph/code/table/image/mermaid/quote
	Content  string
	Language string // 代码块语言
}

// MarkdownParser Markdown AST 解析器，实现 eino parser.Parser 接口
type MarkdownParser struct {
	md goldmark.Markdown
}

// NewMarkdownParser 创建 Markdown 解析器
func NewMarkdownParser() *MarkdownParser {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.Table,
		),
	)
	return &MarkdownParser{md: md}
}

func (p *MarkdownParser) Parse(ctx context.Context, reader io.Reader, opts ...parser.Option) ([]*schema.Document, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	doc := p.md.Parser().Parse(text.NewReader(content))
	astDoc := p.buildASTDocument(doc, content)

	var docs []*schema.Document
	for i, sec := range astDoc.Sections {
		d := &schema.Document{
			ID:      "",
			Content: sec.buildContent(),
			MetaData: map[string]any{
				"heading":      sec.Heading,
				"level":        sec.Level,
				"chapter_path": sec.ChapterPath,
				"section_idx":  i,
			},
		}
		docs = append(docs, d)
	}
	return docs, nil
}

func (p *MarkdownParser) buildASTDocument(doc ast.Node, source []byte) *astDocument {
	result := &astDocument{}
	var currentSection *section
	var headingStack []string

	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		switch n.Kind() {
		case ast.KindHeading:
			heading := n.(*ast.Heading)
			headingText := string(n.Text(source))
			level := heading.Level

			if level <= len(headingStack) {
				headingStack = headingStack[:level-1]
			}
			headingStack = append(headingStack, headingText)

			currentSection = &section{
				Heading:     headingText,
				Level:       level,
				ChapterPath: joinPath(headingStack),
			}
			result.Sections = append(result.Sections, currentSection)

		default:
			if currentSection == nil {
				currentSection = &section{
					Heading:     "",
					Level:       0,
					ChapterPath: "",
				}
				result.Sections = append(result.Sections, currentSection)
			}
			block := p.extractContentBlock(n, source)
			if block != nil {
				currentSection.Content = append(currentSection.Content, *block)
			}
		}
	}
	return result
}

func (p *MarkdownParser) extractContentBlock(n ast.Node, source []byte) *contentBlock {
	switch n.Kind() {
	case ast.KindParagraph:
		return &contentBlock{Type: "paragraph", Content: string(n.Text(source))}

	case ast.KindFencedCodeBlock:
		lang := ""
		codeBlock := n.(*ast.FencedCodeBlock)
		if codeBlock.Info != nil {
			lang = string(codeBlock.Info.Text(source))
		}
		var code []byte
		lines := codeBlock.Lines()
		for i := 0; i < lines.Len(); i++ {
			seg := lines.At(i)
			code = append(code, seg.Value(source)...)
			code = append(code, '\n')
		}
		if lang == "mermaid" {
			return &contentBlock{Type: "mermaid", Content: string(code)}
		}
		return &contentBlock{Type: "code", Content: string(code), Language: lang}

	case ast.KindBlockquote:
		return &contentBlock{Type: "quote", Content: string(n.Text(source))}

	case ast.KindList:
		return &contentBlock{Type: "paragraph", Content: string(n.Text(source))}

	default:
		tableContent := p.extractTable(n, source)
		if tableContent != "" {
			return &contentBlock{Type: "table", Content: tableContent}
		}

		nodeText := string(n.Text(source))
		if strings.TrimSpace(nodeText) == "" {
			return nil
		}
		return &contentBlock{Type: "paragraph", Content: nodeText}
	}
}

func (p *MarkdownParser) extractTable(n ast.Node, source []byte) string {
	if n.Kind().String() != "Table" {
		return ""
	}

	var rows []string
	var headerRow string
	isHeader := true

	for row := n.FirstChild(); row != nil; row = row.NextSibling() {
		var cells []string
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			cellText := strings.TrimSpace(string(cell.Text(source)))
			cells = append(cells, cellText)
		}

		if len(cells) > 0 {
			rowStr := "| " + strings.Join(cells, " | ") + " |"
			if isHeader {
				headerRow = rowStr
				separator := "| " + strings.Repeat("--- | ", len(cells))
				rows = append(rows, headerRow)
				rows = append(rows, separator)
				isHeader = false
			} else {
				rows = append(rows, rowStr)
			}
		}
	}

	if len(rows) > 0 {
		return strings.Join(rows, "\n")
	}
	return ""
}

func (s *section) buildContent() string {
	var parts []string
	for _, block := range s.Content {
		switch block.Type {
		case "code":
			parts = append(parts, "```"+block.Language+"\n"+block.Content+"```")
		case "mermaid":
			parts = append(parts, "```mermaid\n"+block.Content+"```")
		case "table":
			parts = append(parts, block.Content)
		default:
			parts = append(parts, block.Content)
		}
	}
	return strings.Join(parts, "\n\n")
}

func joinPath(stack []string) string {
	return strings.Join(stack, " > ")
}
