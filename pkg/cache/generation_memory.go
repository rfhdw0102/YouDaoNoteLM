package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	generationMemoryKeyFormat  = "generation:memory:%d:%d:%s"
	generationMemoryMaxEntries = 10
	generationMemoryTTL        = 7 * 24 * time.Hour
)

type GenerationMemoryCacheEntry struct {
	Prompt        string    `json:"prompt"`
	InputSummary  string    `json:"input_summary"`
	OutputSummary string    `json:"output_summary"`
	CreatedAt     time.Time `json:"created_at"`
}

type GenerationMemoryCache struct {
	rdb *redis.Client
}

func NewGenerationMemoryCache(rdb *redis.Client) *GenerationMemoryCache {
	return &GenerationMemoryCache{rdb: rdb}
}

func (c *GenerationMemoryCache) GetRecent(ctx context.Context, userID, notebookID uint, typ string, limit int) ([]GenerationMemoryCacheEntry, error) {
	if c == nil || c.rdb == nil {
		return []GenerationMemoryCacheEntry{}, nil
	}
	limit = normalizeGenerationMemoryLimit(limit)
	key := generationMemoryKey(userID, notebookID, typ)
	values, err := c.rdb.LRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}
	entries := make([]GenerationMemoryCacheEntry, 0, len(values))
	for _, value := range values {
		var entry GenerationMemoryCacheEntry
		if err := json.Unmarshal([]byte(value), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (c *GenerationMemoryCache) Add(ctx context.Context, userID, notebookID uint, typ string, entry GenerationMemoryCacheEntry) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	key := generationMemoryKey(userID, notebookID, typ)
	pipe := c.rdb.Pipeline()
	pipe.LPush(ctx, key, string(data))
	pipe.LTrim(ctx, key, 0, generationMemoryMaxEntries-1)
	pipe.Expire(ctx, key, generationMemoryTTL)
	_, err = pipe.Exec(ctx)
	return err
}

func generationMemoryKey(userID, notebookID uint, typ string) string {
	return fmt.Sprintf(generationMemoryKeyFormat, userID, notebookID, typ)
}

func normalizeGenerationMemoryLimit(limit int) int {
	if limit <= 0 || limit > generationMemoryMaxEntries {
		return generationMemoryMaxEntries
	}
	return limit
}
