package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const rendererPageSize = maximumPageSize

type markdownRenderer struct {
	fetcher     BlockFetcher
	accessToken string
	limits      RenderLimits
	blockCount  int
}

type renderedBlock struct {
	markdown   string
	meaningful bool
	kind       renderBlockKind
}

type renderBlockKind uint8

const (
	renderBlockOther renderBlockKind = iota
	renderBlockList
	renderBlockTable
)

type richTextBlock struct {
	RichText []notionRichText `json:"rich_text"`
}

type notionRichText struct {
	Type      string `json:"type"`
	PlainText string `json:"plain_text"`
	Href      string `json:"href"`
	Text      struct {
		Content string `json:"content"`
		Link    *struct {
			URL string `json:"url"`
		} `json:"link"`
	} `json:"text"`
	Annotations struct {
		Bold          bool `json:"bold"`
		Italic        bool `json:"italic"`
		Strikethrough bool `json:"strikethrough"`
		Code          bool `json:"code"`
	} `json:"annotations"`
}

// RenderPageMarkdown recursively reads page blocks and converts the supported
// Notion block subset into Markdown without fetching media URLs.
func RenderPageMarkdown(ctx context.Context, fetcher BlockFetcher, accessToken, pageID string, limits RenderLimits) (string, error) {
	if fetcher == nil || strings.TrimSpace(accessToken) == "" || strings.TrimSpace(pageID) == "" {
		return "", ErrInvalidRequest
	}
	if ctx == nil {
		ctx = context.Background()
	}

	renderer := markdownRenderer{
		fetcher:     fetcher,
		accessToken: accessToken,
		limits:      limits.normalized(),
	}
	result, err := renderer.renderChildren(ctx, pageID, 0)
	if err != nil {
		return "", err
	}
	markdown := strings.TrimSpace(result.markdown)
	if markdown == "" || !result.meaningful {
		return "", ErrEmptyMarkdown
	}
	if err := renderer.checkSize(markdown); err != nil {
		return "", err
	}
	return markdown, nil
}

func (r *markdownRenderer) renderChildren(ctx context.Context, blockID string, depth int) (renderedBlock, error) {
	if depth > r.limits.MaxDepth {
		return renderedBlock{}, ErrRenderDepthLimit
	}

	cursor := ""
	var blocks []renderedBlock
	for {
		if err := ctx.Err(); err != nil {
			return renderedBlock{}, err
		}
		page, err := r.fetcher.ListBlockChildren(ctx, r.accessToken, blockID, cursor, rendererPageSize)
		if err != nil {
			return renderedBlock{}, fmt.Errorf("list notion block children: %w", err)
		}
		for _, block := range page.Blocks {
			if err := r.countBlock(); err != nil {
				return renderedBlock{}, err
			}

			current, err := r.renderBlock(block)
			if err != nil {
				return renderedBlock{}, err
			}
			if block.HasChildren && block.Type != "child_page" {
				if block.Type == "table" {
					current, err = r.renderTable(ctx, block.ID, depth+1)
				} else {
					var children renderedBlock
					children, err = r.renderChildren(ctx, block.ID, depth+1)
					if err == nil {
						current = mergeChildren(block.Type, current, children)
					}
				}
				if err != nil {
					return renderedBlock{}, err
				}
			}
			if err := r.checkSize(current.markdown); err != nil {
				return renderedBlock{}, err
			}
			blocks = append(blocks, current)
		}

		if !page.HasMore {
			break
		}
		if page.NextCursor == "" {
			return renderedBlock{}, ErrInvalidResponse
		}
		cursor = page.NextCursor
	}

	result := joinRenderedBlocks(blocks)
	if err := r.checkSize(result.markdown); err != nil {
		return renderedBlock{}, err
	}
	return result, nil
}

func (r *markdownRenderer) renderTable(ctx context.Context, blockID string, depth int) (renderedBlock, error) {
	if depth > r.limits.MaxDepth {
		return renderedBlock{}, ErrRenderDepthLimit
	}

	cursor := ""
	var lines []string
	meaningful := false
	for {
		if err := ctx.Err(); err != nil {
			return renderedBlock{}, err
		}
		page, err := r.fetcher.ListBlockChildren(ctx, r.accessToken, blockID, cursor, rendererPageSize)
		if err != nil {
			return renderedBlock{}, fmt.Errorf("list notion table rows: %w", err)
		}
		for _, block := range page.Blocks {
			if err := r.countBlock(); err != nil {
				return renderedBlock{}, err
			}
			if block.Type != "table_row" {
				unsupported, err := r.renderBlock(block)
				if err != nil {
					return renderedBlock{}, err
				}
				if unsupported.markdown != "" {
					lines = append(lines, unsupported.markdown)
					meaningful = meaningful || unsupported.meaningful
				}
				continue
			}

			cells, rowMeaningful, err := renderTableRow(block)
			if err != nil {
				return renderedBlock{}, err
			}
			if len(cells) == 0 {
				continue
			}
			lines = append(lines, markdownTableRow(cells))
			if len(lines) == 1 {
				separator := make([]string, len(cells))
				for i := range separator {
					separator[i] = "---"
				}
				lines = append(lines, markdownTableRow(separator))
			}
			meaningful = meaningful || rowMeaningful
		}

		if !page.HasMore {
			break
		}
		if page.NextCursor == "" {
			return renderedBlock{}, ErrInvalidResponse
		}
		cursor = page.NextCursor
	}
	result := renderedBlock{markdown: strings.Join(lines, "\n"), meaningful: meaningful, kind: renderBlockTable}
	if err := r.checkSize(result.markdown); err != nil {
		return renderedBlock{}, err
	}
	return result, nil
}

func (r *markdownRenderer) renderBlock(block Block) (renderedBlock, error) {
	switch block.Type {
	case "paragraph":
		return renderRichTextBlock(block, "", renderBlockOther)
	case "heading_1":
		return renderRichTextBlock(block, "# ", renderBlockOther)
	case "heading_2":
		return renderRichTextBlock(block, "## ", renderBlockOther)
	case "heading_3":
		return renderRichTextBlock(block, "### ", renderBlockOther)
	case "bulleted_list_item":
		return renderRichTextBlock(block, "- ", renderBlockList)
	case "numbered_list_item":
		return renderRichTextBlock(block, "1. ", renderBlockList)
	case "to_do":
		var data struct {
			RichText []notionRichText `json:"rich_text"`
			Checked  bool             `json:"checked"`
		}
		if err := decodeBlockPayload(block, &data); err != nil {
			return renderedBlock{}, err
		}
		text := strings.TrimSpace(renderRichText(data.RichText))
		checked := " "
		if data.Checked {
			checked = "x"
		}
		return renderedBlock{markdown: "- [" + checked + "] " + text, meaningful: text != "", kind: renderBlockList}, nil
	case "quote", "callout":
		result, err := renderRichTextBlock(block, "> ", renderBlockOther)
		if err != nil {
			return renderedBlock{}, err
		}
		return result, nil
	case "code":
		var data struct {
			RichText []notionRichText `json:"rich_text"`
			Language string           `json:"language"`
		}
		if err := decodeBlockPayload(block, &data); err != nil {
			return renderedBlock{}, err
		}
		content := strings.TrimSpace(renderCodeRichText(data.RichText))
		language := safeCodeLanguage(data.Language)
		if content == "" {
			return renderedBlock{}, nil
		}
		return renderedBlock{markdown: "```" + language + "\n" + content + "\n```", meaningful: true, kind: renderBlockOther}, nil
	case "divider":
		return renderedBlock{markdown: "---", kind: renderBlockOther}, nil
	case "table":
		return renderedBlock{kind: renderBlockTable}, nil
	case "table_row":
		cells, meaningful, err := renderTableRow(block)
		if err != nil {
			return renderedBlock{}, err
		}
		return renderedBlock{markdown: markdownTableRow(cells), meaningful: meaningful, kind: renderBlockTable}, nil
	case "child_page":
		var data struct {
			Title string `json:"title"`
			URL   string `json:"url"`
		}
		if err := decodeBlockPayload(block, &data); err != nil {
			return renderedBlock{}, err
		}
		title := strings.TrimSpace(data.Title)
		if title == "" {
			title = "未命名子页面"
		}
		link := strings.TrimSpace(data.URL)
		if link == "" {
			link = "https://www.notion.so/" + strings.ReplaceAll(block.ID, "-", "")
		}
		return renderedBlock{markdown: "[" + escapeMarkdownText(title) + "](" +
			escapeMarkdownURL(link) + ")", meaningful: true, kind: renderBlockOther}, nil
	case "bookmark", "link_preview", "embed":
		return renderLinkBlock(block)
	case "image", "file", "video", "audio", "pdf":
		return renderMediaBlock(block)
	default:
		typeName := strings.TrimSpace(block.Type)
		if typeName == "" {
			typeName = "unknown"
		}
		return renderedBlock{
			markdown:   "[Notion 区块暂不支持：" + escapeMarkdownText(typeName) + "]",
			meaningful: true,
			kind:       renderBlockOther,
		}, nil
	}
}

func renderRichTextBlock(block Block, prefix string, kind renderBlockKind) (renderedBlock, error) {
	var data richTextBlock
	if err := decodeBlockPayload(block, &data); err != nil {
		return renderedBlock{}, err
	}
	text := strings.TrimSpace(renderRichText(data.RichText))
	if text == "" {
		return renderedBlock{}, nil
	}
	return renderedBlock{markdown: prefix + text, meaningful: true, kind: kind}, nil
}

func renderTableRow(block Block) ([]string, bool, error) {
	var data struct {
		Cells [][]notionRichText `json:"cells"`
	}
	if err := decodeBlockPayload(block, &data); err != nil {
		return nil, false, err
	}
	cells := make([]string, 0, len(data.Cells))
	meaningful := false
	for _, cell := range data.Cells {
		value := strings.TrimSpace(renderRichText(cell))
		cells = append(cells, value)
		meaningful = meaningful || value != ""
	}
	return cells, meaningful, nil
}

func renderLinkBlock(block Block) (renderedBlock, error) {
	var data struct {
		URL string `json:"url"`
	}
	if err := decodeBlockPayload(block, &data); err != nil {
		return renderedBlock{}, err
	}
	link := strings.TrimSpace(data.URL)
	if link == "" {
		return renderedBlock{markdown: "[链接暂不可用]", meaningful: true, kind: renderBlockOther}, nil
	}
	return renderedBlock{markdown: "[链接](" + escapeMarkdownURL(link) + ")", meaningful: true, kind: renderBlockOther}, nil
}

func renderMediaBlock(block Block) (renderedBlock, error) {
	var data struct {
		URL      string `json:"url"`
		External *struct {
			URL string `json:"url"`
		} `json:"external"`
		File *struct {
			URL string `json:"url"`
		} `json:"file"`
	}
	if err := decodeBlockPayload(block, &data); err != nil {
		return renderedBlock{}, err
	}
	link := strings.TrimSpace(data.URL)
	if link == "" && data.External != nil {
		link = strings.TrimSpace(data.External.URL)
	}
	if link == "" && data.File != nil {
		link = strings.TrimSpace(data.File.URL)
	}
	label := mediaPlaceholder(block.Type)
	if link == "" {
		return renderedBlock{markdown: "[" + label + "]", meaningful: true, kind: renderBlockOther}, nil
	}
	return renderedBlock{markdown: "[" + label + "](" + escapeMarkdownURL(link) + ")", meaningful: true, kind: renderBlockOther}, nil
}

func mediaPlaceholder(blockType string) string {
	switch blockType {
	case "image":
		return "图片未下载"
	case "video":
		return "视频未下载"
	case "audio":
		return "音频未下载"
	case "pdf":
		return "PDF 未下载"
	default:
		return "附件未下载"
	}
}

func mergeChildren(parentType string, parent, children renderedBlock) renderedBlock {
	if children.markdown == "" {
		return parent
	}
	if parent.markdown == "" {
		return children
	}
	if isListBlock(parentType) {
		parent.markdown += "\n" + indentMarkdown(children.markdown, "  ")
	} else {
		parent.markdown += "\n\n" + children.markdown
	}
	parent.meaningful = parent.meaningful || children.meaningful
	return parent
}

func joinRenderedBlocks(blocks []renderedBlock) renderedBlock {
	var output strings.Builder
	meaningful := false
	previousKind := renderBlockOther
	hasPrevious := false
	for _, block := range blocks {
		if strings.TrimSpace(block.markdown) == "" {
			continue
		}
		if hasPrevious {
			if previousKind == renderBlockList && block.kind == renderBlockList {
				output.WriteByte('\n')
			} else {
				output.WriteString("\n\n")
			}
		}
		output.WriteString(block.markdown)
		meaningful = meaningful || block.meaningful
		previousKind = block.kind
		hasPrevious = true
	}
	return renderedBlock{markdown: output.String(), meaningful: meaningful, kind: renderBlockOther}
}

func (r *markdownRenderer) countBlock() error {
	r.blockCount++
	if r.blockCount > r.limits.MaxBlocks {
		return ErrRenderBlockLimit
	}
	return nil
}

func (r *markdownRenderer) checkSize(markdown string) error {
	if len(markdown) > r.limits.MaxBytes {
		return ErrRenderByteLimit
	}
	return nil
}

func decodeBlockPayload(block Block, target any) error {
	payload := bytes.TrimSpace(block.Data)
	if len(payload) == 0 || bytes.Equal(payload, []byte("null")) {
		return ErrInvalidResponse
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("%w: invalid %s block payload", ErrInvalidResponse, block.Type)
	}
	return nil
}

func renderRichText(parts []notionRichText) string {
	var output strings.Builder
	for _, part := range parts {
		text := part.PlainText
		if text == "" {
			text = part.Text.Content
		}
		if text == "" {
			continue
		}
		text = escapeMarkdownText(text)
		if part.Annotations.Code {
			text = "`" + text + "`"
		} else {
			if part.Annotations.Bold {
				text = "**" + text + "**"
			}
			if part.Annotations.Italic {
				text = "_" + text + "_"
			}
			if part.Annotations.Strikethrough {
				text = "~~" + text + "~~"
			}
		}
		link := part.Href
		if part.Text.Link != nil && part.Text.Link.URL != "" {
			link = part.Text.Link.URL
		}
		if link != "" {
			text = "[" + text + "](" + escapeMarkdownURL(link) + ")"
		}
		output.WriteString(text)
	}
	return output.String()
}

func renderCodeRichText(parts []notionRichText) string {
	var output strings.Builder
	for _, part := range parts {
		text := part.PlainText
		if text == "" {
			text = part.Text.Content
		}
		output.WriteString(text)
	}
	return output.String()
}

func escapeMarkdownText(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"`", "\\`",
		"*", "\\*",
		"_", "\\_",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"!", "\\!",
		"|", "\\|",
		">", "\\>",
	)
	return replacer.Replace(value)
}

func escapeMarkdownURL(value string) string {
	return strings.NewReplacer(
		"\\", "%5C",
		"(", "%28",
		")", "%29",
		"\n", "",
		"\r", "",
	).Replace(value)
}

func safeCodeLanguage(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 32 {
		return ""
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' && char != '_' && char != '+' {
			return ""
		}
	}
	return value
}

func isListBlock(blockType string) bool {
	switch blockType {
	case "bulleted_list_item", "numbered_list_item", "to_do":
		return true
	default:
		return false
	}
}

func indentMarkdown(markdown, prefix string) string {
	lines := strings.Split(markdown, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

func markdownTableRow(cells []string) string {
	if len(cells) == 0 {
		return ""
	}
	return "| " + strings.Join(cells, " | ") + " |"
}
