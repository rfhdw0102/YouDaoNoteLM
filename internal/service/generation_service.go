package service

import (
	"context"
	"strings"

	"YoudaoNoteLm/internal/rag"
	bizerrors "YoudaoNoteLm/pkg/errors"
)

type generationService struct {
	retriever rag.RAGRetriever
	search    SearchService
	model     GenerationModel
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
	return &generationService{
		retriever: retriever,
		search:    search,
		model:     model,
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
	refs, err := s.retrieve(ctx, req, plan)
	if err != nil {
		return nil, err
	}
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

	agent := s.agents[req.Type]
	agentOutput, err := agent.Generate(ctx, generationAgentInput{
		Request:       req,
		Context:       buildGenerationContext(req, refs, searchSummary, searchResults),
		References:    refs,
		SearchResults: searchResults,
	})
	if err != nil {
		return nil, err
	}

	return &GenerationResponse{
		Type:          req.Type,
		Content:       agentOutput.Content,
		References:    refs,
		SearchResults: searchResults,
		Meta: map[string]any{
			"agent":               string(req.Type),
			"reference_count":     len(refs),
			"search_count":        len(searchResults),
			"search_degraded":     searchDegraded,
			"local_query":         plan.LocalQuery,
			"web_query":           plan.WebQuery,
			"format_valid":        agentOutput.FormatValid,
			"fallback_used":       agentOutput.FallbackUsed,
			"orchestration_steps": generationOrchestrationSteps(),
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

func (s *generationService) retrieve(ctx context.Context, req *GenerationRequest, plan generationQueryPlan) ([]GenerationReference, error) {
	inlineLimit := optionInt(req.Options, "inline_top_k", 6)
	inlineRefs, err := buildInlineMarkdownReferences(ctx, req.Markdown, plan, inlineLimit)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "preprocess input markdown failed", err)
	}
	if s.retriever == nil {
		return inlineRefs, nil
	}
	results, err := s.retriever.Retrieve(ctx, &rag.RetrieveRequest{
		Query:     plan.LocalQuery,
		UserID:    req.UserID,
		SourceIDs: append([]uint(nil), req.SourceIDs...),
		TopK:      optionInt(req.Options, "top_k", 5),
	})
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "retrieve generation context failed", err)
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
	return mergeGenerationReferences(inlineRefs, ragRefs, inlineLimit+optionInt(req.Options, "top_k", 5)), nil
}

func (s *generationService) searchWeb(ctx context.Context, req *GenerationRequest, plan generationQueryPlan) (*SearchResponse, error) {
	if !req.UseWeb || s.search == nil {
		return nil, nil
	}
	resp, err := s.search.SearchAndSummarize(ctx, &SearchRequest{
		UserID:       req.UserID,
		Scene:        SearchSceneGeneration,
		Query:        plan.WebQuery,
		Count:        optionInt(req.Options, "search_count", 5),
		NeedSummary:  true,
		NeedContent:  true,
		NotebookID:   req.NotebookID,
		AllowDegrade: req.AllowDegrade,
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
		"content_expand",
		"draft_generate",
		"structure_check",
		"structure_repair",
		"fact_enhance",
		"format_validate",
		"finalize",
	}
}
