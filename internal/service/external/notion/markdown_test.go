package notion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderPageMarkdownRendersRichTextAndNestedBlocks(t *testing.T) {
	got, err := RenderPageMarkdown(context.Background(), fixtureFetcher(t, "page_blocks.json"), "token-placeholder", "page-1", DefaultRenderLimits())
	if err != nil {
		t.Fatal(err)
	}
	want := "# 标题\n\n正文 **重点**\n\n- 第一项\n- 第二项\n\n[子页面](https://www.notion.so/child)"
	if !strings.Contains(got, want) {
		t.Fatalf("markdown missing expected content: %s", got)
	}
	if !strings.Contains(got, "嵌套父块\n\n嵌套子块") {
		t.Fatalf("nested child was not rendered: %s", got)
	}
}

func TestRenderPageMarkdownMarksUnsupportedBlocks(t *testing.T) {
	got, err := RenderPageMarkdown(context.Background(), fixtureFetcher(t, "unsupported_block.json"), "token-placeholder", "page-1", DefaultRenderLimits())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Notion 区块暂不支持") {
		t.Fatal("unsupported block was silently dropped")
	}
}

func TestRenderPageMarkdownRendersNestedListChildren(t *testing.T) {
	fetcher := staticBlockFetcher{results: map[string]BlockChildrenResult{
		"page-1\x00": {Blocks: []Block{paragraphLikeBlock(t, "parent", "bulleted_list_item", "父项", true)}},
		"parent\x00": {Blocks: []Block{paragraphLikeBlock(t, "child", "bulleted_list_item", "子项", false)}},
	}}

	got, err := RenderPageMarkdown(context.Background(), fetcher, "token-placeholder", "page-1", DefaultRenderLimits())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "- 父项\n  - 子项") {
		t.Fatalf("nested list was not indented: %q", got)
	}
}

func TestRenderPageMarkdownRendersSupportedBlockForms(t *testing.T) {
	fetcher := staticBlockFetcher{results: map[string]BlockChildrenResult{
		"page-1\x00": {
			Blocks: []Block{
				paragraphLikeBlock(t, "heading", "heading_2", "二级标题", false),
				paragraphLikeBlock(t, "numbered", "numbered_list_item", "编号项", false),
				blockWithData(t, "todo", "to_do", false, map[string]any{"rich_text": richText("已完成"), "checked": true}),
				paragraphLikeBlock(t, "quote", "quote", "引用内容", false),
				paragraphLikeBlock(t, "callout", "callout", "提示内容", false),
				blockWithData(t, "code", "code", false, map[string]any{"rich_text": richText("fmt.Println(1)"), "language": "go"}),
				blockWithData(t, "divider", "divider", false, map[string]any{}),
				blockWithData(t, "bookmark", "bookmark", false, map[string]any{"url": "https://example.com/docs"}),
				blockWithData(t, "image", "image", false, map[string]any{"type": "external", "external": map[string]any{"url": "https://cdn.example/image.png"}}),
				blockWithData(t, "table", "table", true, map[string]any{"table_width": 2}),
			},
		},
		"table\x00": {
			Blocks: []Block{
				blockWithData(t, "row-1", "table_row", false, map[string]any{"cells": []any{richText("列 A"), richText("列 B")}}),
				blockWithData(t, "row-2", "table_row", false, map[string]any{"cells": []any{richText("值 A"), richText("值 B")}}),
			},
		},
	}}

	got, err := RenderPageMarkdown(context.Background(), fetcher, "token-placeholder", "page-1", DefaultRenderLimits())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"## 二级标题",
		"1. 编号项",
		"- [x] 已完成",
		"> 引用内容",
		"> 提示内容",
		"```go\nfmt.Println(1)\n```",
		"---",
		"[链接](https://example.com/docs)",
		"[图片未下载](https://cdn.example/image.png)",
		"| 列 A | 列 B |",
		"| --- | --- |",
		"| 值 A | 值 B |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown missing %q: %s", want, got)
		}
	}
}

func TestRenderPageMarkdownEscapesRichTextMarkdownSyntax(t *testing.T) {
	fetcher := staticBlockFetcher{results: map[string]BlockChildrenResult{
		"page-1\x00": {
			Blocks: []Block{blockWithData(t, "paragraph", "paragraph", false, map[string]any{
				"rich_text": []any{
					map[string]any{
						"type":       "text",
						"plain_text": "*文字*",
						"text": map[string]any{
							"content": "*文字*",
						},
						"annotations": map[string]any{"bold": false, "italic": false, "strikethrough": false, "code": false},
					},
					map[string]any{
						"type":       "text",
						"plain_text": "链接",
						"text": map[string]any{
							"content": "链接",
							"link":    map[string]any{"url": "https://example.com/path?q=1"},
						},
						"annotations": map[string]any{"bold": true, "italic": false, "strikethrough": false, "code": false},
					},
				},
			})},
		},
	}}

	got, err := RenderPageMarkdown(context.Background(), fetcher, "token-placeholder", "page-1", DefaultRenderLimits())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "\\*文字\\*") || !strings.Contains(got, "[**链接**](https://example.com/path?q=1)") {
		t.Fatalf("rich text was not safely rendered: %q", got)
	}
}

func TestRenderPageMarkdownFollowsBlockPagination(t *testing.T) {
	fetcher := staticBlockFetcher{results: map[string]BlockChildrenResult{
		"page-1\x00": {Blocks: []Block{paragraphLikeBlock(t, "first", "paragraph", "第一页", false)}, NextCursor: "cursor-2", HasMore: true},
		"page-1\x00cursor-2": {Blocks: []Block{paragraphLikeBlock(t, "second", "paragraph", "第二页", false)}},
	}}

	got, err := RenderPageMarkdown(context.Background(), fetcher, "token-placeholder", "page-1", DefaultRenderLimits())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "第一页\n\n第二页") {
		t.Fatalf("paginated blocks were not rendered: %q", got)
	}
}

func TestRenderPageMarkdownReturnsEmptyContentError(t *testing.T) {
	_, err := RenderPageMarkdown(context.Background(), staticBlockFetcher{results: map[string]BlockChildrenResult{
		"page-1\x00": {},
	}}, "token-placeholder", "page-1", DefaultRenderLimits())
	if !errors.Is(err, ErrEmptyMarkdown) {
		t.Fatalf("error = %v, want ErrEmptyMarkdown", err)
	}
}

func TestRenderPageMarkdownEnforcesSafetyLimits(t *testing.T) {
	t.Run("depth", func(t *testing.T) {
		_, err := RenderPageMarkdown(context.Background(), depthBlockFetcher{t: t, levels: 9}, "token-placeholder", "page-1", RenderLimits{MaxDepth: 8, MaxBlocks: 100, MaxBytes: 1024})
		if !errors.Is(err, ErrRenderDepthLimit) {
			t.Fatalf("error = %v, want ErrRenderDepthLimit", err)
		}
	})

	t.Run("block count", func(t *testing.T) {
		fetcher := staticBlockFetcher{results: map[string]BlockChildrenResult{
			"page-1\x00": {Blocks: []Block{
				paragraphLikeBlock(t, "first", "paragraph", "first", false),
				paragraphLikeBlock(t, "second", "paragraph", "second", false),
			}},
		}}
		_, err := RenderPageMarkdown(context.Background(), fetcher, "token-placeholder", "page-1", RenderLimits{MaxDepth: 8, MaxBlocks: 1, MaxBytes: 1024})
		if !errors.Is(err, ErrRenderBlockLimit) {
			t.Fatalf("error = %v, want ErrRenderBlockLimit", err)
		}
	})

	t.Run("markdown bytes", func(t *testing.T) {
		fetcher := staticBlockFetcher{results: map[string]BlockChildrenResult{
			"page-1\x00": {Blocks: []Block{paragraphLikeBlock(t, "block", "paragraph", "内容超过限制", false)}},
		}}
		_, err := RenderPageMarkdown(context.Background(), fetcher, "token-placeholder", "page-1", RenderLimits{MaxDepth: 8, MaxBlocks: 100, MaxBytes: 3})
		if !errors.Is(err, ErrRenderByteLimit) {
			t.Fatalf("error = %v, want ErrRenderByteLimit", err)
		}
	})
}

type fixtureBlockFetcher struct {
	results map[string]BlockChildrenResult
}

func fixtureFetcher(t *testing.T, name string) BlockFetcher {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	results := make(map[string]BlockChildrenResult)
	if err := json.Unmarshal(contents, &results); err != nil {
		t.Fatal(err)
	}
	return fixtureBlockFetcher{results: results}
}

func (f fixtureBlockFetcher) ListBlockChildren(_ context.Context, _ string, blockID, cursor string, _ int) (BlockChildrenResult, error) {
	result, ok := f.results[blockID]
	if !ok || cursor != "" {
		return BlockChildrenResult{}, fmt.Errorf("missing fixture for block %q cursor %q", blockID, cursor)
	}
	return result, nil
}

type staticBlockFetcher struct {
	results map[string]BlockChildrenResult
}

func (f staticBlockFetcher) ListBlockChildren(_ context.Context, _ string, blockID, cursor string, _ int) (BlockChildrenResult, error) {
	result, ok := f.results[blockID+"\x00"+cursor]
	if !ok {
		return BlockChildrenResult{}, fmt.Errorf("missing response for block %q cursor %q", blockID, cursor)
	}
	return result, nil
}

type depthBlockFetcher struct {
	t      *testing.T
	levels int
}

func (f depthBlockFetcher) ListBlockChildren(_ context.Context, _ string, blockID, _ string, _ int) (BlockChildrenResult, error) {
	level := 0
	if blockID != "page-1" {
		if _, err := fmt.Sscanf(blockID, "depth-%d", &level); err != nil {
			return BlockChildrenResult{}, err
		}
	}
	if level >= f.levels {
		return BlockChildrenResult{}, nil
	}
	return BlockChildrenResult{Blocks: []Block{paragraphLikeBlock(f.t, fmt.Sprintf("depth-%d", level+1), "paragraph", "x", true)}}, nil
}

func paragraphLikeBlock(t *testing.T, id, typ, text string, hasChildren bool) Block {
	t.Helper()
	return blockWithData(t, id, typ, hasChildren, map[string]any{"rich_text": richText(text)})
}

func blockWithData(t *testing.T, id, typ string, hasChildren bool, value map[string]any) Block {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return Block{ID: id, Type: typ, HasChildren: hasChildren, Data: data}
}

func richText(content string) []any {
	return []any{map[string]any{
		"type":       "text",
		"plain_text": content,
		"text": map[string]any{
			"content": content,
		},
		"annotations": map[string]any{
			"bold":          false,
			"italic":        false,
			"strikethrough": false,
			"code":          false,
		},
	}}
}
