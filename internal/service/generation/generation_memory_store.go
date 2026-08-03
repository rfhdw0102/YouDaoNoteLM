// generation_memory_store.go 实现基于缓存（Redis）的会话记忆存储。
//
// GenerationMemoryCacheStore 是 GenerationMemoryStore 接口的实现，
// 底层使用 pkg/cache 提供的 Redis 客户端，按 scope（用户+笔记本+类型）分组存储记忆条目。
package generation

import (
	"context"

	"YoudaoNoteLm/pkg/cache"
)

type GenerationMemoryCache interface {
	GetRecent(ctx context.Context, userID, notebookID uint, typ string, limit int) ([]cache.GenerationMemoryCacheEntry, error)
	Add(ctx context.Context, userID, notebookID uint, typ string, entry cache.GenerationMemoryCacheEntry) error
}

type generationMemoryCacheStore struct {
	cache GenerationMemoryCache
}

// NewGenerationMemoryCacheStore 创建基于缓存的会话记忆存储实例。
func NewGenerationMemoryCacheStore(cacheClient GenerationMemoryCache) GenerationMemoryStore {
	return &generationMemoryCacheStore{cache: cacheClient}
}

// GetRecent 从缓存中读取指定作用域的最近记忆条目。
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

// Add 将一条记忆条目写入缓存。
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
