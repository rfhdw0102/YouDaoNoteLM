// agent_factory.go 定义各类型生成 Agent 的构造工厂。
//
// 每种生成类型（mindmap/note/quiz/ppt）都有一个对应的 new<Type>Agent 工厂函数，
// 返回实现了 generationAgent 接口的具体 Agent 实例。
// Agent 的差异化逻辑通过重写 baseGenerationAgent 的方法实现（见 *_agent_steps.go）。
package generation

import (
	"YoudaoNoteLm/pkg/logger"
	"context"
	"github.com/cloudwego/eino/compose"
	"go.uber.org/zap"
	"time"
)

// newMindmapAgent 构造思维导图生成 Agent。
func newMindmapAgent(model GenerationModel) generationAgent {
	return &mindmapGenerationAgent{
		baseGenerationAgent: baseGenerationAgent{
			name:      "mindmap",
			typ:       GenerationTypeMindmap,
			model:     model,
			validator: validateMindmapContent,
			fallback:  fallbackMindmapContent,
		},
	}
}

// newNoteAgent 构造笔记生成 Agent。
func newNoteAgent(model GenerationModel) generationAgent {
	return &noteGenerationAgent{
		baseGenerationAgent: baseGenerationAgent{
			name:      "note",
			typ:       GenerationTypeNote,
			model:     model,
			validator: validateNoteContent,
			fallback:  fallbackNoteContent,
		},
	}
}

// newQuizAgent 构造测验生成 Agent。
func newQuizAgent(model GenerationModel) generationAgent {
	return &quizGenerationAgent{
		baseGenerationAgent: baseGenerationAgent{
			name:      "quiz",
			typ:       GenerationTypeQuiz,
			model:     model,
			validator: validateQuizContent,
			fallback:  fallbackQuizContent,
		},
	}
}

// newPPTAgent 构造 PPT 生成 Agent。
func newPPTAgent(model GenerationModel) generationAgent {
	return &pptGenerationAgent{
		baseGenerationAgent: baseGenerationAgent{
			name:      "ppt",
			typ:       GenerationTypePPT,
			model:     model,
			validator: validatePPTContent,
			fallback:  fallbackPPTContent,
		},
	}
}

type pptGenerationAgent struct {
	baseGenerationAgent
}

// Generate 执行 PPT 14 步链式生成流程。
func (a *pptGenerationAgent) Generate(ctx context.Context, input generationAgentInput) (generationAgentOutput, error) {
	overallStart := time.Now()
	logger.Info("[PPT] generation started",
		zap.Int("prompt_len", len(input.Request.Prompt)),
		zap.Int("context_len", len(input.Context)),
	)

	var state pptChainState
	var err error
	var stepStart time.Time

	stepStart = time.Now()
	state, err = a.analyzePPTContent(ctx, input)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 1/13: analyzePPTContent done",
		zap.Duration("elapsed", time.Since(stepStart)),
		zap.Int("sections", len(state.analysis.Sections)),
	)

	stepStart = time.Now()
	state, err = a.planPPTChainOutline(ctx, state)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 2/13: planPPTChainOutline done",
		zap.Duration("elapsed", time.Since(stepStart)),
		zap.Int("slides", len(state.outlinePlan.Slides)),
	)

	stepStart = time.Now()
	state, err = a.expandPPTChainContent(ctx, state)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 3/13: expandPPTChainContent done",
		zap.Duration("elapsed", time.Since(stepStart)),
	)

	stepStart = time.Now()
	state, err = a.approvePPTExpandedOutline(ctx, state)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 4/13: approvePPTExpandedOutline done",
		zap.Duration("elapsed", time.Since(stepStart)),
		zap.Bool("outline_approved", state.outlineApproved),
		zap.Int("slides_after_approval", len(state.expanded.Slides)),
	)

	stepStart = time.Now()
	state, err = a.enrichPPTContent(ctx, state)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 5/13: enrichPPTContent done",
		zap.Duration("elapsed", time.Since(stepStart)),
		zap.Int("rich_slides", len(state.richContent.Slides)),
	)

	stepStart = time.Now()
	state, err = a.designPPTStyle(ctx, state)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 6/13: designPPTStyle done",
		zap.Duration("elapsed", time.Since(stepStart)),
		zap.String("theme", state.styleTheme.Name),
	)

	stepStart = time.Now()
	draft, err := a.generatePPTHTML(ctx, state)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 7/13: generatePPTHTML done",
		zap.Duration("elapsed", time.Since(stepStart)),
		zap.Int("html_len", len(draft.content)),
		zap.Bool("fallback_used", draft.fallbackUsed),
	)

	stepStart = time.Now()
	draft, err = a.structureCheck(ctx, draft)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 8/13: structureCheck done",
		zap.Duration("elapsed", time.Since(stepStart)),
	)

	stepStart = time.Now()
	draft, err = a.polishPPTHTML(ctx, draft)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 9/13: polishPPTHTML done",
		zap.Duration("elapsed", time.Since(stepStart)),
		zap.Int("html_len_after", len(draft.content)),
	)

	stepStart = time.Now()
	draft, err = a.repairPPTStructure(ctx, draft)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 10/13: repairPPTStructure done",
		zap.Duration("elapsed", time.Since(stepStart)),
		zap.Bool("fallback_used", draft.fallbackUsed),
	)

	stepStart = time.Now()
	draft, err = a.factEnhance(ctx, draft)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 11/13: factEnhance done",
		zap.Duration("elapsed", time.Since(stepStart)),
	)

	stepStart = time.Now()
	draft, err = a.formatValidate(ctx, draft)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 12/13: formatValidate done",
		zap.Duration("elapsed", time.Since(stepStart)),
		zap.Bool("format_valid", draft.formatValid),
	)

	stepStart = time.Now()
	output, err := a.finalize(ctx, draft)
	if err != nil {
		return generationAgentOutput{}, err
	}
	logger.Info("[PPT] step 13/13: finalize done",
		zap.Duration("elapsed", time.Since(stepStart)),
		zap.Int("final_len", len(output.Content)),
	)

	logger.Info("[PPT] generation completed",
		zap.Duration("total_elapsed", time.Since(overallStart)),
		zap.Bool("fallback_used", output.FallbackUsed),
	)

	return output, nil
}

type mindmapGenerationAgent struct {
	baseGenerationAgent
}

// Generate 执行思维导图 9 步链式生成流程。
func (a *mindmapGenerationAgent) Generate(ctx context.Context, input generationAgentInput) (generationAgentOutput, error) {
	chain := compose.NewChain[generationAgentInput, generationAgentOutput]().
		AppendLambda(compose.InvokableLambda(a.analyzeMindmapContent)).
		AppendLambda(compose.InvokableLambda(a.planMindmapOutline)).
		AppendLambda(compose.InvokableLambda(a.expandMindmapChainContent)).
		AppendLambda(compose.InvokableLambda(a.generateMindmapDraft)).
		AppendLambda(compose.InvokableLambda(a.structureCheck)).
		AppendLambda(compose.InvokableLambda(a.repairMindmapStructure)).
		AppendLambda(compose.InvokableLambda(a.factEnhance)).
		AppendLambda(compose.InvokableLambda(a.formatValidate)).
		AppendLambda(compose.InvokableLambda(a.finalize))

	runner, err := chain.Compile(ctx)
	if err != nil {
		return generationAgentOutput{}, err
	}
	return runner.Invoke(ctx, input)
}

type noteGenerationAgent struct {
	baseGenerationAgent
}

// Generate 执行笔记 9 步链式生成流程。
func (a *noteGenerationAgent) Generate(ctx context.Context, input generationAgentInput) (generationAgentOutput, error) {
	chain := compose.NewChain[generationAgentInput, generationAgentOutput]().
		AppendLambda(compose.InvokableLambda(a.analyzeNoteContent)).
		AppendLambda(compose.InvokableLambda(a.planNoteOutline)).
		AppendLambda(compose.InvokableLambda(a.expandNoteChainContent)).
		AppendLambda(compose.InvokableLambda(a.generateNoteDraft)).
		AppendLambda(compose.InvokableLambda(a.structureCheck)).
		AppendLambda(compose.InvokableLambda(a.repairNoteStructure)).
		AppendLambda(compose.InvokableLambda(a.factEnhance)).
		AppendLambda(compose.InvokableLambda(a.formatValidate)).
		AppendLambda(compose.InvokableLambda(a.finalize))

	runner, err := chain.Compile(ctx)
	if err != nil {
		return generationAgentOutput{}, err
	}
	return runner.Invoke(ctx, input)
}

type quizGenerationAgent struct {
	baseGenerationAgent
}

// Generate 执行测验 9 步链式生成流程。
func (a *quizGenerationAgent) Generate(ctx context.Context, input generationAgentInput) (generationAgentOutput, error) {
	chain := compose.NewChain[generationAgentInput, generationAgentOutput]().
		AppendLambda(compose.InvokableLambda(a.analyzeQuizContent)).
		AppendLambda(compose.InvokableLambda(a.planQuizQuestions)).
		AppendLambda(compose.InvokableLambda(a.expandQuizChainContent)).
		AppendLambda(compose.InvokableLambda(a.generateQuizDraft)).
		AppendLambda(compose.InvokableLambda(a.structureCheck)).
		AppendLambda(compose.InvokableLambda(a.repairQuizStructure)).
		AppendLambda(compose.InvokableLambda(a.factEnhance)).
		AppendLambda(compose.InvokableLambda(a.formatValidate)).
		AppendLambda(compose.InvokableLambda(a.finalize))

	runner, err := chain.Compile(ctx)
	if err != nil {
		return generationAgentOutput{}, err
	}
	return runner.Invoke(ctx, input)
}
