package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	sourceSummaryKey = "source:%d:summary"
	sourceSummaryTTL = 30 * 24 * time.Hour // 30天
)

// SourceSummaryCache 资料摘要缓存
type SourceSummaryCache struct {
	rdb *redis.Client
}

// NewSourceSummaryCache 创建资料摘要缓存
func NewSourceSummaryCache(rdb *redis.Client) *SourceSummaryCache {
	return &SourceSummaryCache{rdb: rdb}
}

// Get 获取资料摘要
func (c *SourceSummaryCache) Get(ctx context.Context, sourceID uint) (string, bool, error) {
	key := formatSourceSummaryKey(sourceID)
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

// Set 设置资料摘要
func (c *SourceSummaryCache) Set(ctx context.Context, sourceID uint, summary string) error {
	key := formatSourceSummaryKey(sourceID)
	return c.rdb.Set(ctx, key, summary, sourceSummaryTTL).Err()
}

// Delete 删除资料摘要
func (c *SourceSummaryCache) Delete(ctx context.Context, sourceID uint) error {
	key := formatSourceSummaryKey(sourceID)
	return c.rdb.Del(ctx, key).Err()
}

// BatchDelete 批量删除资料摘要
func (c *SourceSummaryCache) BatchDelete(ctx context.Context, sourceIDs []uint) error {
	if len(sourceIDs) == 0 {
		return nil
	}
	keys := make([]string, len(sourceIDs))
	for i, id := range sourceIDs {
		keys[i] = formatSourceSummaryKey(id)
	}
	return c.rdb.Del(ctx, keys...).Err()
}

func formatSourceSummaryKey(sourceID uint) string {
	return fmt.Sprintf(sourceSummaryKey, sourceID)
}
