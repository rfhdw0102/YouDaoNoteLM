package service

import (
	"context"

	"YoudaoNoteLm/pkg/cache"
)

type generationMemoryCache interface {
	GetRecent(ctx context.Context, userID, notebookID uint, typ string, limit int) ([]cache.GenerationMemoryCacheEntry, error)
	Add(ctx context.Context, userID, notebookID uint, typ string, entry cache.GenerationMemoryCacheEntry) error
}

type generationMemoryCacheStore struct {
	cache generationMemoryCache
}

func NewGenerationMemoryCacheStore(cacheClient generationMemoryCache) GenerationMemoryStore {
	return &generationMemoryCacheStore{cache: cacheClient}
}

func (s *generationMemoryCacheStore) GetRecent(ctx context.Context, scope GenerationMemoryScope, limit int) ([]GenerationMemoryEntry, error) {
	if s == nil || s.cache == nil {
		return []GenerationMemoryEntry{}, nil
	}
	cached, err := s.cache.GetRecent(ctx, scope.UserID, scope.NotebookID, string(scope.Type), limit)
	if err != nil {
		return nil, err
	}
	entries := make([]GenerationMemoryEntry, 0, len(cached))
	for _, item := range cached {
		entries = append(entries, GenerationMemoryEntry{
			Prompt:        item.Prompt,
			InputSummary:  item.InputSummary,
			OutputSummary: item.OutputSummary,
			CreatedAt:     item.CreatedAt,
		})
	}
	return entries, nil
}

func (s *generationMemoryCacheStore) Add(ctx context.Context, scope GenerationMemoryScope, entry GenerationMemoryEntry) error {
	if s == nil || s.cache == nil {
		return nil
	}
	return s.cache.Add(ctx, scope.UserID, scope.NotebookID, string(scope.Type), cache.GenerationMemoryCacheEntry{
		Prompt:        entry.Prompt,
		InputSummary:  entry.InputSummary,
		OutputSummary: entry.OutputSummary,
		CreatedAt:     entry.CreatedAt,
	})
}
