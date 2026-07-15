package service

import (
	"context"

	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/rag"
	gen "YoudaoNoteLm/internal/service/generation"
	"YoudaoNoteLm/pkg/cache"

	"github.com/cloudwego/eino/components/model"
)

type GenerationType = gen.GenerationType

const (
	GenerationTypeMindmap GenerationType = gen.GenerationTypeMindmap
	GenerationTypePPT     GenerationType = gen.GenerationTypePPT
	GenerationTypeQuiz    GenerationType = gen.GenerationTypeQuiz
	GenerationTypeNote    GenerationType = gen.GenerationTypeNote
)

type GenerationRequest = gen.GenerationRequest
type GenerationReference = gen.GenerationReference
type GenerationResponse = gen.GenerationResponse
type GenerationExportRequest = gen.GenerationExportRequest
type GenerationExportResult = gen.GenerationExportResult
type GenerationTaskStatus = gen.GenerationTaskStatus

const (
	GenerationTaskStatusPending   GenerationTaskStatus = gen.GenerationTaskStatusPending
	GenerationTaskStatusRunning   GenerationTaskStatus = gen.GenerationTaskStatusRunning
	GenerationTaskStatusCompleted GenerationTaskStatus = gen.GenerationTaskStatusCompleted
	GenerationTaskStatusFailed    GenerationTaskStatus = gen.GenerationTaskStatusFailed
	GenerationTaskStatusCancelled GenerationTaskStatus = gen.GenerationTaskStatusCancelled
	GenerationTaskEventTask                            = gen.GenerationTaskEventTask
)

type GenerationTask = gen.GenerationTask
type GenerationTaskEvent = gen.GenerationTaskEvent
type GenerationTaskListFilter = gen.GenerationTaskListFilter
type GenerationTaskStore = gen.GenerationTaskStore
type GenerationTaskService = gen.GenerationTaskService
type GenerationPrompt = gen.GenerationPrompt
type GenerationModel = gen.GenerationModel
type GenerationService = gen.GenerationService
type GenerationMemoryScope = gen.GenerationMemoryScope
type GenerationMemoryEntry = gen.GenerationMemoryEntry
type GenerationMemoryStore = gen.GenerationMemoryStore
type GenerationTaskQueue = gen.GenerationTaskQueue

type generationSearchServiceAdapter struct {
	base SearchService
}

func (a generationSearchServiceAdapter) SearchAndSummarize(ctx context.Context, req *gen.SearchRequest) (*gen.SearchResponse, error) {
	resp, err := a.base.SearchAndSummarize(ctx, toServiceSearchRequest(req))
	if err != nil {
		return nil, err
	}
	return toGenerationSearchResponse(resp), nil
}

func adaptGenerationSearchService(search SearchService) gen.SearchService {
	if search == nil {
		return nil
	}
	return generationSearchServiceAdapter{base: search}
}

func toServiceSearchRequest(req *gen.SearchRequest) *SearchRequest {
	if req == nil {
		return nil
	}
	return &SearchRequest{
		UserID:         req.UserID,
		Scene:          SearchScene(req.Scene),
		Query:          req.Query,
		Freshness:      req.Freshness,
		Count:          req.Count,
		NeedSummary:    req.NeedSummary,
		NeedContent:    req.NeedContent,
		Language:       req.Language,
		AllowedDomains: append([]string(nil), req.AllowedDomains...),
		BlockedDomains: append([]string(nil), req.BlockedDomains...),
		NotebookID:     req.NotebookID,
		SourceID:       req.SourceID,
		TraceID:        req.TraceID,
		AllowDegrade:   req.AllowDegrade,
		SkipUserConfig: req.SkipUserConfig,
	}
}

func toGenerationSearchResponse(resp *SearchResponse) *gen.SearchResponse {
	if resp == nil {
		return nil
	}
	results := make([]gen.SearchResult, 0, len(resp.Results))
	for _, item := range resp.Results {
		results = append(results, gen.SearchResult{
			Title:         item.Title,
			Snippet:       item.Snippet,
			URL:           item.URL,
			DisplayURL:    item.DisplayURL,
			PublishedAt:   item.PublishedAt,
			SiteName:      item.SiteName,
			Score:         item.Score,
			Content:       item.Content,
			ProviderRawID: item.ProviderRawID,
			Meta:          item.Meta,
		})
	}
	return &gen.SearchResponse{
		Query:    resp.Query,
		Provider: resp.Provider,
		Results:  results,
		Summary:  resp.Summary,
		Total:    resp.Total,
		Cached:   resp.Cached,
		Meta:     resp.Meta,
	}
}

func NewGenerationService(retriever rag.RAGRetriever, search SearchService, model GenerationModel) GenerationService {
	return gen.NewGenerationService(retriever, adaptGenerationSearchService(search), model)
}

func NewGenerationServiceWithMemory(retriever rag.RAGRetriever, search SearchService, model GenerationModel, memory GenerationMemoryStore) GenerationService {
	return gen.NewGenerationServiceWithMemory(retriever, adaptGenerationSearchService(search), model, memory)
}

func NewGenerationServiceWithUserLLMConfig(retriever rag.RAGRetriever, search SearchService, repo interface {
	FindDefaultByUserID(userID uint) (*entity.UserLLMConfig, error)
}, encryptionKey string) GenerationService {
	return NewGenerationServiceWithUserLLMConfigAndMemory(retriever, search, repo, nil, encryptionKey)
}

func NewGenerationServiceWithUserLLMConfigAndMemory(retriever rag.RAGRetriever, search SearchService, repo interface {
	FindDefaultByUserID(userID uint) (*entity.UserLLMConfig, error)
}, memory GenerationMemoryStore, encryptionKey string) GenerationService {
	return gen.NewGenerationServiceWithUserLLMConfigAndMemory(
		retriever,
		adaptGenerationSearchService(search),
		repo,
		memory,
		encryptionKey,
	)
}

func NewEinoGenerationModel(chat model.BaseChatModel) GenerationModel {
	return gen.NewEinoGenerationModel(chat)
}

func NewGenerationMemoryCacheStore(cacheClient interface {
	GetRecent(ctx context.Context, userID, notebookID uint, typ string, limit int) ([]cache.GenerationMemoryCacheEntry, error)
	Add(ctx context.Context, userID, notebookID uint, typ string, entry cache.GenerationMemoryCacheEntry) error
}) GenerationMemoryStore {
	return gen.NewGenerationMemoryCacheStore(cacheClient)
}

func NewGenerationTaskService(base GenerationService, store GenerationTaskStore) GenerationTaskService {
	return gen.NewGenerationTaskService(base, store)
}

func NewGenerationTaskServiceWithQueue(base GenerationService, store GenerationTaskStore, queue GenerationTaskQueue) GenerationTaskService {
	return gen.NewGenerationTaskServiceWithQueue(base, store, queue)
}

func NewInMemoryGenerationTaskQueue(size int) GenerationTaskQueue {
	return gen.NewInMemoryGenerationTaskQueue(size)
}

func NewGenerationTaskRedisQueue(taskCache *cache.GenerationTaskCache) GenerationTaskQueue {
	return gen.NewGenerationTaskRedisQueue(taskCache)
}

func NewGenerationTaskCacheStore(taskCache *cache.GenerationTaskCache) GenerationTaskStore {
	return gen.NewGenerationTaskCacheStore(taskCache)
}

func NewInMemoryGenerationTaskStore() GenerationTaskStore {
	return gen.NewInMemoryGenerationTaskStore()
}
