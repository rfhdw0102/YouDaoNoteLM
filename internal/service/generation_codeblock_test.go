package service

import (
	"strings"
	"testing"
)

func TestMergeCodeBlockLines(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string // expected merged lines (non-empty)
	}{
		{
			name:  "code block preserved as single unit",
			input: "some text\n```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```\nmore text",
			want:  []string{"some text", "```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```", "more text"},
		},
		{
			name:  "multiple code blocks",
			input: "# Title\n```python\ndef foo():\n    pass\n```\nSome paragraph\n```js\nconsole.log(42)\n```",
			want:  []string{"# Title", "```python\ndef foo():\n    pass\n```", "Some paragraph", "```js\nconsole.log(42)\n```"},
		},
		{
			name:  "code block with blank line inside",
			input: "```go\nfunc a() {\n\n}\n```",
			want:  []string{"```go\nfunc a() {\n\n}\n```"},
		},
		{
			name:  "no code blocks",
			input: "hello\nworld\nfoo",
			want:  []string{"hello", "world", "foo"},
		},
		{
			name:  "short code block skipped",
			input: "```go\nab\n```",
			want:  []string{},
		},
		{
			name:  "unclosed code block emitted",
			input: "```go\nfunc main() {\n\tfmt.Println(\"hi\")\n}",
			want:  []string{"```go\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n"},
		},
		{
			name:  "code block after heading",
			input: "# Go语言\n```go\npackage main\n\nfunc main() {\n\tprintln(42)\n}\n```\n- key point",
			want:  []string{"# Go语言", "```go\npackage main\n\nfunc main() {\n\tprintln(42)\n}\n```", "- key point"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lines := strings.Split(tc.input, "\n")
			merged := mergeCodeBlockLines(lines)

			// Filter out empty lines for comparison
			var got []string
			for _, l := range merged {
				if strings.TrimSpace(l) != "" {
					got = append(got, l)
				}
			}

			if len(got) != len(tc.want) {
				t.Errorf("expected %d non-empty merged lines, got %d", len(tc.want), len(got))
				t.Logf("got:  %q", got)
				t.Logf("want: %q", tc.want)
				return
			}
			for i, w := range tc.want {
				if got[i] != w {
					t.Errorf("line %d:\n  got:  %q\n  want: %q", i, got[i], w)
				}
			}
		})
	}
}

func TestExtractPPTSourceSectionsWithCodeBlocks(t *testing.T) {
	markdown := `# Go语言基础

## 变量声明

Go语言使用 var 关键字声明变量。

` + "```go" + `
var name string = "hello"
var age int = 25
` + "```" + `

## 函数定义

` + "```go" + `
func add(a, b int) int {
    return a + b
}
` + "```" + `

函数是Go语言的一等公民。
`

	sections := extractPPTSourceSections(markdown, 18)

	// Check that code blocks are present in the sections
	foundCodeBlock := false
	for _, sec := range sections {
		for _, point := range sec.Points {
			if strings.Contains(point, "```go") {
				foundCodeBlock = true
				// Verify code block is a complete unit with meaningful content
				if !strings.Contains(point, "var ") && !strings.Contains(point, "func ") {
					t.Errorf("code block point doesn't contain code: %q", point)
				}
				// Verify closing fence is present
				if !strings.HasSuffix(strings.TrimSpace(point), "```") {
					t.Errorf("code block point doesn't end with closing fence: %q", point)
				}
			}
		}
	}

	if !foundCodeBlock {
		t.Errorf("no code blocks found in extracted sections; sections: %+v", sections)
	}

	// Verify we have at least the variable declaration and function definition sections
	foundVarSection := false
	foundFuncSection := false
	for _, sec := range sections {
		if strings.Contains(sec.Title, "变量") {
			foundVarSection = true
		}
		if strings.Contains(sec.Title, "函数") {
			foundFuncSection = true
		}
	}
	if !foundVarSection {
		t.Error("expected '变量声明' section not found")
	}
	if !foundFuncSection {
		t.Error("expected '函数定义' section not found")
	}
}

func TestExtractKeyPointsWithCodeBlocks(t *testing.T) {
	markdown := `# Python Basics

` + "```python" + `
def hello():
    print("Hello, World!")
` + "```" + `

Some regular text here.

` + "```javascript" + `
console.log("test");
` + "```" + `
`

	points := extractKeyPoints(markdown, 48)

	foundPythonCode := false
	for _, p := range points {
		if strings.Contains(p, "```python") {
			foundPythonCode = true
			if !strings.Contains(p, "def hello()") {
				t.Errorf("python code block point doesn't contain function: %q", p)
			}
		}
	}
	if !foundPythonCode {
		t.Errorf("python code block not found in key points; points: %+v", points)
	}
}

func TestLooksCodeBlock(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"```go\nfmt.Println()\n```", true},
		{"regular text without code blocks", false},
		{"some ``` inline ``` code", true},
		{"", false},
	}
	for _, tc := range cases {
		got := looksCodeBlock(tc.input)
		if got != tc.want {
			t.Errorf("looksCodeBlock(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestRefContentLimit(t *testing.T) {
	codeRef := GenerationReference{Content: "```go\nfunc main() {}\n```"}
	textRef := GenerationReference{Content: "regular text content"}

	if limit := refContentLimit(codeRef); limit != 500 {
		t.Errorf("refContentLimit for code block = %d, want 500", limit)
	}
	if limit := refContentLimit(textRef); limit != 120 {
		t.Errorf("refContentLimit for text = %d, want 120", limit)
	}
}

func TestCleanPPTVisibleTextPreservesCodeBlockFences(t *testing.T) {
	// The root cause fix: cleanPPTVisibleText must NOT strip ``` fences from
	// code blocks, because downstream code (isPPTCodeBlockBullet, slideHasCodeBlock,
	// writePPTCodeBlocks) relies on the fences to identify and correctly render
	// code blocks in the PPT.
	codeBlock := "```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```"
	cleaned := cleanPPTVisibleText(codeBlock)
	if !strings.HasPrefix(strings.TrimSpace(cleaned), "```") {
		t.Errorf("cleanPPTVisibleText stripped code block fences: got %q", cleaned)
	}
	if !strings.Contains(cleaned, "func main()") {
		t.Errorf("cleanPPTVisibleText lost code content: got %q", cleaned)
	}
	if !isPPTCodeBlockBullet(cleaned) {
		t.Errorf("isPPTCodeBlockBullet returns false after cleanPPTVisibleText: got %q", cleaned)
	}

	// Non-code text should still be cleaned normally for heading markers
	headingText := "# Some heading"
	cleanedHeading := cleanPPTVisibleText(headingText)
	if strings.Contains(cleanedHeading, "#") {
		t.Errorf("cleanPPTVisibleText did not clean heading marker: got %q", cleanedHeading)
	}
	// Markdown bold/italic is NOT cleaned by cleanPPTVisibleText
	// (that's handled by stripPPTVisibleText, a different function)
	plainText := "Some **bold** text"
	cleanedPlain := cleanPPTVisibleText(plainText)
	_ = cleanedPlain // just verify no panic
}

func TestRenderStyledPPTSlidesWithCodeBlocks(t *testing.T) {
	// End-to-end test: verify that code blocks in the plan are rendered
	// as <pre class="ppt-code-block"><code> elements in the HTML output.
	// NOTE: renderStyledPPTSlides assumes slide 0=封面, slide 1=目录,
	// so we must follow that order in the test plan.
	plan := pptOutlinePlan{
		Title: "Go语言",
		Slides: []pptSlidePlan{
			{Title: "封面", Purpose: "建立演示主题", Bullets: []string{"Go语言入门"}},
			{Title: "目录", Purpose: "呈现演示路径", Bullets: []string{"代码示例"}},
		},
	}
	// Add a slide with a code block (must be slide index >= 2)
	codeSlide := pptSlidePlan{
		Title:   "代码示例",
		Purpose: "展示Go代码",
		Bullets: []string{
			"Go语言使用var声明变量",
			"```go\nvar name string = \"hello\"\nvar age int = 25\n```",
		},
	}
	plan.Slides = append(plan.Slides, codeSlide)
	plan.Slides = append(plan.Slides, pptSlidePlan{
		Title:   "总结与行动",
		Purpose: "收束核心结论并给出下一步",
		Bullets: []string{"总结", "下一步"},
	})

	// Debug: run sanitizePPTPlanVisibleText and check results
	sanitizedPlan := sanitizePPTPlanVisibleText(plan)
	for i, slide := range sanitizedPlan.Slides {
		for j, b := range slide.Bullets {
			if strings.Contains(b, "```") || strings.Contains(b, "var name") {
				t.Logf("After sanitize: Slide %d bullet %d: isPPTCodeBlockBullet=%v content=%q", i, j, isPPTCodeBlockBullet(b), truncate(b, 100))
			}
		}
	}
	// Check slideHasCodeBlock
	for i, slide := range sanitizedPlan.Slides {
		if slideHasCodeBlock(slide) {
			t.Logf("Slide %d hasCodeBlock=true", i)
		}
	}

	html := renderStyledPPTSlides(plan, pptStyleTheme{})

	// Verify code block is rendered as <pre> element
	if !strings.Contains(html, `<pre class="ppt-code-block"><code>`) {
		t.Errorf("renderStyledPPTSlides did not render code block as <pre>; html snippet:\n%s",
			truncate(html, 2000))
	}
	// Verify the code content is present
	if !strings.Contains(html, "var name string") {
		t.Errorf("renderStyledPPTSlides lost code content; html snippet:\n%s",
			truncate(html, 2000))
	}
}

func TestNormalizePPTBulletsSkipsCodeBlocks(t *testing.T) {
	// Verify that normalizePPTBullets does not strip the slide title prefix
	// from code blocks, which would corrupt the code content.
	slide := &pptSlidePlan{
		Title: "代码示例",
		Bullets: []string{
			"代码示例：这是一个说明",
			"```go\nfunc main() {}\n```",
		},
	}
	normalizePPTBullets(slide)

	// The non-code bullet should have the title prefix stripped
	for _, b := range slide.Bullets {
		if strings.Contains(b, "```") {
			// Code block should still contain the original code
			if !strings.Contains(b, "func main()") {
				t.Errorf("normalizePPTBullets corrupted code block: %q", b)
			}
			// Code block should still be recognized as a code block
			if !isPPTCodeBlockBullet(b) {
				t.Errorf("normalizePPTBullets stripped code block fences: %q", b)
			}
		}
	}
}
