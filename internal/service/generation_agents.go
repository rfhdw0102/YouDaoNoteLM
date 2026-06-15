package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"
)

type baseGenerationAgent struct {
	name      string
	typ       GenerationType
	model     GenerationModel
	fallback  func(generationAgentInput) string
	validator func(string) bool
}

type generationDraft struct {
	input        generationAgentInput
	content      string
	formatValid  bool
	fallbackUsed bool
}

func (a *baseGenerationAgent) Generate(ctx context.Context, input generationAgentInput) (generationAgentOutput, error) {
	chain := compose.NewChain[generationAgentInput, generationAgentOutput]().
		AppendLambda(compose.InvokableLambda(a.generateDraft)).
		AppendLambda(compose.InvokableLambda(a.structureCheck)).
		AppendLambda(compose.InvokableLambda(a.factEnhance)).
		AppendLambda(compose.InvokableLambda(a.formatValidate)).
		AppendLambda(compose.InvokableLambda(a.finalize))

	runner, err := chain.Compile(ctx)
	if err != nil {
		return generationAgentOutput{}, err
	}
	return runner.Invoke(ctx, input)
}

func (a *baseGenerationAgent) generateDraft(ctx context.Context, input generationAgentInput) (generationDraft, error) {
	content := ""
	fallbackUsed := false
	strategy := promptStrategyFor(a.typ)
	if a.model != nil {
		generated, err := a.model.Generate(ctx, GenerationPrompt{
			AgentName:    a.name,
			System:       strategy.System,
			User:         strings.TrimSpace(input.Request.Prompt),
			Context:      input.Context,
			OutputFormat: strategy.OutputFormat,
		})
		if err != nil {
			return generationDraft{}, err
		}
		content = strings.TrimSpace(generated)
	}
	if content == "" {
		content = a.fallback(input)
		fallbackUsed = true
	}
	return generationDraft{input: input, content: content, fallbackUsed: fallbackUsed}, nil
}

func (a *baseGenerationAgent) structureCheck(ctx context.Context, draft generationDraft) (generationDraft, error) {
	if strings.TrimSpace(draft.content) == "" {
		draft.content = a.fallback(draft.input)
		draft.fallbackUsed = true
	}
	return draft, nil
}

func (a *baseGenerationAgent) factEnhance(ctx context.Context, draft generationDraft) (generationDraft, error) {
	if len(draft.input.References) == 0 {
		return draft, nil
	}
	switch a.typ {
	case GenerationTypeQuiz:
		return draft, nil
	case GenerationTypePPT:
		if strings.Contains(strings.ToLower(draft.content), "references") || strings.Contains(draft.content, "参考") {
			return draft, nil
		}
		var b strings.Builder
		b.WriteString(strings.TrimSpace(draft.content))
		b.WriteString("\n<section><h2>参考资料</h2><ul>")
		for i, ref := range draft.input.References {
			if i >= 5 {
				break
			}
			label := generationReferenceLabel(ref)
			b.WriteString("<li>")
			b.WriteString(htmlEscape(label + ": " + summarizeLine(ref.Content, 120)))
			b.WriteString("</li>")
		}
		b.WriteString("</ul></section>")
		draft.content = b.String()
	default:
		if strings.Contains(strings.ToLower(draft.content), "references") || strings.Contains(draft.content, "参考") {
			return draft, nil
		}
		var b strings.Builder
		b.WriteString(strings.TrimSpace(draft.content))
		appendReferenceSection(&b, draft.input.References)
		draft.content = b.String()
	}
	return draft, nil
}

func (a *baseGenerationAgent) formatValidate(ctx context.Context, draft generationDraft) (generationDraft, error) {
	draft.formatValid = a.validator(draft.content)
	if !draft.formatValid {
		draft.content = a.fallback(draft.input)
		draft.fallbackUsed = true
		draft.formatValid = a.validator(draft.content)
	}
	return draft, nil
}

func (a *baseGenerationAgent) finalize(ctx context.Context, draft generationDraft) (generationAgentOutput, error) {
	return generationAgentOutput{
		Content:      strings.TrimSpace(draft.content),
		FormatValid:  draft.formatValid,
		FallbackUsed: draft.fallbackUsed,
	}, nil
}

func newMindmapAgent(model GenerationModel) generationAgent {
	return &baseGenerationAgent{
		name:      "mindmap",
		typ:       GenerationTypeMindmap,
		model:     model,
		validator: validateMindmapContent,
		fallback: func(input generationAgentInput) string {
			title := extractTitle(input.Request.Markdown, "思维导图")
			points := extractKeyPoints(input.Request.Markdown, 6)
			var b strings.Builder
			b.WriteString("# ")
			b.WriteString(title)
			b.WriteString("\n")
			for _, point := range points {
				b.WriteString("## ")
				b.WriteString(point)
				b.WriteString("\n")
			}
			appendReferenceSection(&b, input.References)
			return strings.TrimSpace(b.String())
		},
	}
}

func newPPTAgent(model GenerationModel) generationAgent {
	return &baseGenerationAgent{
		name:      "ppt",
		typ:       GenerationTypePPT,
		model:     model,
		validator: validatePPTContent,
		fallback: func(input generationAgentInput) string {
			title := extractTitle(input.Request.Markdown, "演示文稿")
			points := extractKeyPoints(input.Request.Markdown, 8)
			var b strings.Builder
			b.WriteString(fmt.Sprintf("<section><h1>%s</h1></section>\n", htmlEscape(title)))
			for i := 0; i < len(points); i += 3 {
				b.WriteString("<section><h2>关键点</h2><ul>")
				end := i + 3
				if end > len(points) {
					end = len(points)
				}
				for _, point := range points[i:end] {
					b.WriteString("<li>")
					b.WriteString(htmlEscape(point))
					b.WriteString("</li>")
				}
				b.WriteString("</ul></section>\n")
			}
			return strings.TrimSpace(b.String())
		},
	}
}

func newQuizAgent(model GenerationModel) generationAgent {
	return &baseGenerationAgent{
		name:      "quiz",
		typ:       GenerationTypeQuiz,
		model:     model,
		validator: validateQuizContent,
		fallback: func(input generationAgentInput) string {
			points := extractKeyPoints(input.Request.Markdown, 3)
			if len(points) == 0 {
				points = []string{"来源笔记"}
			}
			var items []string
			for _, point := range points {
				items = append(items, fmt.Sprintf(`{"type":"short_answer","question":"%s 的核心观点是什么？","options":[],"answer":"%s","explanation":"该答案来自提供的笔记上下文。"}`, jsonEscape(point), jsonEscape(point)))
			}
			return `{"questions":[` + strings.Join(items, ",") + `]}`
		},
	}
}

func newNoteAgent(model GenerationModel) generationAgent {
	return &baseGenerationAgent{
		name:      "note",
		typ:       GenerationTypeNote,
		model:     model,
		validator: validateNoteContent,
		fallback: func(input generationAgentInput) string {
			title := extractTitle(input.Request.Markdown, "生成笔记")
			points := extractKeyPoints(input.Request.Markdown, 8)
			var b strings.Builder
			b.WriteString("# ")
			b.WriteString(title)
			b.WriteString("\n\n## 摘要\n")
			if len(points) > 0 {
				b.WriteString(points[0])
			} else {
				b.WriteString("未能从笔记中提取出简明摘要。")
			}
			b.WriteString("\n\n## 关键点\n")
			for _, point := range points {
				b.WriteString("- ")
				b.WriteString(point)
				b.WriteString("\n")
			}
			appendReferenceSection(&b, input.References)
			return strings.TrimSpace(b.String())
		},
	}
}

func extractTitle(markdown, fallback string) string {
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			title := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if title != "" {
				return title
			}
		}
	}
	return fallback
}

func extractKeyPoints(markdown string, limit int) []string {
	var points []string
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "-*#0123456789. "))
		if len([]rune(line)) < 4 {
			continue
		}
		points = append(points, line)
		if len(points) >= limit {
			return points
		}
	}
	return points
}

func appendReferenceSection(b *strings.Builder, refs []GenerationReference) {
	if len(refs) == 0 {
		return
	}
	b.WriteString("\n\n## 参考资料\n")
	for i, ref := range refs {
		label := generationReferenceLabel(ref)
		b.WriteString(fmt.Sprintf("- [%d] %s: %s\n", i+1, label, summarizeLine(ref.Content, 120)))
	}
}

func summarizeLine(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}

func htmlEscape(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return replacer.Replace(value)
}

func jsonEscape(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", "")
	return replacer.Replace(value)
}
