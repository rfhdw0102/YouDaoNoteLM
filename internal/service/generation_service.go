package service

import (
	"context"
	"strings"

	"YoudaoNoteLm/internal/rag"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

type generationService struct {
	retriever rag.RAGRetriever
	search    SearchService
	model     GenerationModel
	memory    GenerationMemoryStore
	agents    map[GenerationType]generationAgent
}

type generationAgent interface {
	Generate(ctx context.Context, input generationAgentInput) (generationAgentOutput, error)
}

type generationAgentInput struct {
	Request       *GenerationRequest
	Context       string
	References    []GenerationReference
	SearchResults []SearchResult
}

type generationAgentOutput struct {
	Content      string
	FormatValid  bool
	FallbackUsed bool
}

// NewGenerationService creates the supervisor generation service.
func NewGenerationService(retriever rag.RAGRetriever, search SearchService, model GenerationModel) GenerationService {
	return NewGenerationServiceWithMemory(retriever, search, model, nil)
}

func NewGenerationServiceWithMemory(retriever rag.RAGRetriever, search SearchService, model GenerationModel, memory GenerationMemoryStore) GenerationService {
	return &generationService{
		retriever: retriever,
		search:    search,
		model:     model,
		memory:    memory,
		agents: map[GenerationType]generationAgent{
			GenerationTypeMindmap: newMindmapAgent(model),
			GenerationTypePPT:     newPPTAgent(model),
			GenerationTypeQuiz:    newQuizAgent(model),
			GenerationTypeNote:    newNoteAgent(model),
		},
	}
}

func (s *generationService) Generate(ctx context.Context, req *GenerationRequest) (*GenerationResponse, error) {
	if err := validateGenerationRequest(req); err != nil {
		return nil, err
	}

	plan := buildGenerationQueryPlan(req)
	selection, err := s.retrieve(ctx, req, plan)
	if err != nil {
		return nil, err
	}
	refs := selection.References
	searchResp, err := s.searchWeb(ctx, req, plan)
	if err != nil {
		return nil, err
	}

	searchResults := []SearchResult{}
	searchSummary := ""
	searchDegraded := false
	if searchResp != nil {
		searchResults = pruneGenerationSearchResults(searchResp.Results, optionInt(req.Options, "search_count", 5))
		searchSummary = searchResp.Summary
		if searchResp.Meta != nil {
			if degraded, ok := searchResp.Meta["degraded"].(bool); ok {
				searchDegraded = degraded
			}
		}
	}

	memoryEntries := []GenerationMemoryEntry{}
	memoryEnabled := s.memory != nil
	if memoryEnabled {
		entries, readErr := s.memory.GetRecent(ctx, generationMemoryScopeFromRequest(req), generationMemoryDefaultLimit)
		if readErr != nil {
			logger.Warn("read generation memory failed",
				zap.Uint("user_id", req.UserID),
				zap.Uint("notebook_id", req.NotebookID),
				zap.String("type", string(req.Type)),
				zap.Error(readErr),
			)
		} else {
			memoryEntries = entries
		}
	}

	contextValue := buildGenerationContext(req, refs, searchSummary, searchResults)
	contextValue = appendGenerationMemoryContext(contextValue, memoryEntries)

	agent := s.agents[req.Type]
	agentOutput, err := agent.Generate(ctx, generationAgentInput{
		Request:       req,
		Context:       contextValue,
		References:    refs,
		SearchResults: searchResults,
	})
	if err != nil {
		return nil, err
	}

	if memoryEnabled {
		entry := buildGenerationMemoryEntry(req, agentOutput.Content)
		if writeErr := s.memory.Add(ctx, generationMemoryScopeFromRequest(req), entry); writeErr != nil {
			logger.Warn("write generation memory failed",
				zap.Uint("user_id", req.UserID),
				zap.Uint("notebook_id", req.NotebookID),
				zap.String("type", string(req.Type)),
				zap.Error(writeErr),
			)
		}
	}

	return &GenerationResponse{
		Type:          req.Type,
		Content:       agentOutput.Content,
		References:    refs,
		SearchResults: searchResults,
		Meta: map[string]any{
			"agent":                    string(req.Type),
			"reference_count":          len(refs),
			"local_reference_count":    selection.StrongLocalCount,
			"filtered_reference_count": selection.IrrelevantLocalCount + selection.WeakLocalCount,
			"local_filter_reason":      selection.WebSupplementReason,
			"search_count":             len(searchResults),
			"search_degraded":          searchDegraded,
			"local_query":              plan.LocalQuery,
			"web_query":                plan.WebQuery,
			"format_valid":             agentOutput.FormatValid,
			"fallback_used":            agentOutput.FallbackUsed,
			"memory_enabled":           memoryEnabled,
			"memory_count":             len(memoryEntries),
			"orchestration_steps":      generationOrchestrationSteps(),
		},
	}, nil
}

func validateGenerationRequest(req *GenerationRequest) error {
	if req == nil {
		return bizerrors.New(bizerrors.CodeInvalidParam, "generation request cannot be empty")
	}
	if strings.TrimSpace(req.Markdown) == "" {
		return bizerrors.New(bizerrors.CodeInvalidParam, "markdown cannot be empty")
	}
	switch req.Type {
	case GenerationTypeMindmap, GenerationTypePPT, GenerationTypeQuiz, GenerationTypeNote:
		return nil
	default:
		return bizerrors.New(bizerrors.CodeInvalidParam, "unsupported generation type")
	}
}

func (s *generationService) retrieve(ctx context.Context, req *GenerationRequest, plan generationQueryPlan) (generationReferenceSelection, error) {
	inlineLimit := optionInt(req.Options, "inline_top_k", 6)
	inlineRefs, err := buildInlineMarkdownReferences(ctx, req.Markdown, plan, inlineLimit)
	if err != nil {
		return generationReferenceSelection{}, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "preprocess input markdown failed", err)
	}
	selectionLimit := inlineLimit + optionInt(req.Options, "top_k", 5)
	if s.retriever == nil {
		return generationReferenceSelection{
			References: inlineRefs,
		}, nil
	}
	results, err := s.retriever.Retrieve(ctx, &rag.RetrieveRequest{
		Query:     plan.LocalQuery,
		UserID:    req.UserID,
		SourceIDs: append([]uint(nil), req.SourceIDs...),
		TopK:      optionInt(req.Options, "top_k", 5),
	})
	if err != nil {
		logger.Warn("retrieve generation context failed, fallback to inline markdown",
			zap.Uint("user_id", req.UserID),
			zap.Uint("notebook_id", req.NotebookID),
			zap.String("type", string(req.Type)),
			zap.String("query", plan.LocalQuery),
			zap.Uints("source_ids", req.SourceIDs),
			zap.Error(err),
		)
		return generationReferenceSelection{
			References:          inlineRefs,
			NeedsWebSupplement:  req.UseWeb,
			WebSupplementReason: "local_retriever_unavailable",
		}, nil
	}

	ragRefs := make([]GenerationReference, 0, len(results))
	for _, item := range results {
		if item == nil {
			continue
		}
		content := strings.TrimSpace(firstNonEmpty(item.ParentContent, item.Content))
		if content == "" {
			continue
		}
		ragRefs = append(ragRefs, GenerationReference{
			SourceID:    item.SourceID,
			SourceName:  item.SourceName,
			Content:     content,
			Score:       item.Score,
			Heading:     item.Heading,
			ChapterPath: item.ChapterPath,
		})
	}
	return selectGenerationReferences(inlineRefs, ragRefs, plan, selectionLimit), nil
}

func (s *generationService) searchWeb(ctx context.Context, req *GenerationRequest, plan generationQueryPlan) (*SearchResponse, error) {
	if !req.UseWeb || s.search == nil {
		return nil, nil
	}
	resp, err := s.search.SearchAndSummarize(ctx, &SearchRequest{
		UserID:         req.UserID,
		Scene:          SearchSceneGeneration,
		Query:          plan.WebQuery,
		Count:          optionInt(req.Options, "search_count", 5),
		NeedSummary:    true,
		NeedContent:    true,
		NotebookID:     req.NotebookID,
		AllowDegrade:   req.AllowDegrade,
		SkipUserConfig: true, // 生成 Agent 直接使用 YAML 配置，不查数据库用户配置
	})
	if err != nil {
		if req.AllowDegrade {
			return &SearchResponse{
				Query:   plan.WebQuery,
				Results: []SearchResult{},
				Meta:    map[string]any{"degraded": true, "reason": err.Error()},
			}, nil
		}
		return nil, err
	}
	return resp, nil
}

func buildGenerationQuery(req *GenerationRequest) string {
	if req == nil {
		return ""
	}
	if prompt := strings.TrimSpace(req.Prompt); prompt != "" {
		return prompt
	}
	lines := strings.Split(req.Markdown, "\n")
	for _, line := range lines {
		line = strings.Trim(strings.TrimSpace(line), "# ")
		if line != "" {
			return line
		}
	}
	return strings.TrimSpace(req.Markdown)
}

func optionInt(options map[string]any, key string, fallback int) int {
	if options == nil {
		return fallback
	}
	switch value := options[key].(type) {
	case int:
		if value > 0 {
			return value
		}
	case int64:
		if value > 0 {
			return int(value)
		}
	case float64:
		if value > 0 {
			return int(value)
		}
	}
	return fallback
}

func generationOrchestrationSteps() []string {
	return []string{
		"context_prepare",
		"content_analyze",
		"outline_plan",
		"outline_review",
		"content_expand",
		"style_design",
		"css_generate",
		"html_generate",
		"structure_check",
		"html_polish",
		"structure_repair",
		"fact_enhance",
		"format_validate",
		"finalize",
	}
}
