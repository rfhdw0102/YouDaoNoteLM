// agent_base.go 实现 baseGenerationAgent 的 5 步生成链。
//
// 生成链顺序：generateDraft → structureCheck → factEnhance → formatValidate → finalize
//   - generateDraft：调用 LLM 生成初稿
//   - structureCheck：结构校验，必要时触发结构修复
//   - factEnhance：事实增强，补充检索到的背景知识
//   - formatValidate：格式校验，确保符合类型最小要求
//   - finalize：最终输出
//
// 各类型 Agent 通过重写对应步骤方法实现差异化逻辑（见 *_agent_steps.go）。
// 使用 cloudwego/eino 的 compose 框架编排链式调用。
package generation

import (
	"context"
	"github.com/cloudwego/eino/compose"
	"strings"
)

// Generate 编排并执行 5 步链式生成流程。
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

// generateDraft 调用 LLM 生成初稿，模型不可用时使用 fallback。
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

// structureCheck 校验初稿结构，内容为空时回退到 fallback。
func (a *baseGenerationAgent) structureCheck(ctx context.Context, draft generationDraft) (generationDraft, error) {
	if strings.TrimSpace(draft.content) == "" {
		draft.content = a.fallback(draft.input)
		draft.fallbackUsed = true
	}
	return draft, nil
}

// factEnhance 事实增强基类空实现，留给子类重写。
func (a *baseGenerationAgent) factEnhance(ctx context.Context, draft generationDraft) (generationDraft, error) {
	return draft, nil
}

// formatValidate 校验内容格式，不合格时回退到 fallback。
func (a *baseGenerationAgent) formatValidate(ctx context.Context, draft generationDraft) (generationDraft, error) {
	draft.formatValid = a.validator(draft.content)
	if !draft.formatValid {
		draft.content = a.fallback(draft.input)
		draft.fallbackUsed = true
		draft.formatValid = a.validator(draft.content)
	}
	return draft, nil
}

// finalize 整理初稿并输出最终生成结果。
func (a *baseGenerationAgent) finalize(ctx context.Context, draft generationDraft) (generationAgentOutput, error) {
	return generationAgentOutput{
		Content:      strings.TrimSpace(draft.content),
		FormatValid:  draft.formatValid,
		FallbackUsed: draft.fallbackUsed,
	}, nil
}
