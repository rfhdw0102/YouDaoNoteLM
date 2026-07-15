package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	generationTaskPrefix        = "generation:task:"
	generationTaskRequestPrefix = "generation:task:request:"
	generationTaskUserPrefix    = "generation:task:user:"
	generationTaskQueueKey      = "generation:task:queue"
	generationTaskDefaultTTL    = 24 * time.Hour
	generationTaskDefaultSize   = 100
)

type GenerationTaskCache struct {
	cache *Cache
}

func NewGenerationTaskCache(cache *Cache) *GenerationTaskCache {
	return &GenerationTaskCache{cache: cache}
}

func (c *GenerationTaskCache) Save(ctx context.Context, taskID string, userID uint, sequence int64, task interface{}) error {
	key := fmt.Sprintf("%s%s", generationTaskPrefix, taskID)
	score := float64(sequence)
	if sequence <= 0 {
		score = float64(time.Now().UnixNano())
	}
	pipe := c.cache.client.TxPipeline()
	data, err := marshalCacheValue(task)
	if err != nil {
		return err
	}
	pipe.Set(ctx, key, data, generationTaskDefaultTTL)
	if userID != 0 {
		userKey := generationTaskUserKey(userID)
		pipe.ZAdd(ctx, userKey, redis.Z{Score: score, Member: taskID})
		pipe.Expire(ctx, userKey, generationTaskDefaultTTL)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (c *GenerationTaskCache) Get(ctx context.Context, taskID string, dest interface{}) error {
	key := fmt.Sprintf("%s%s", generationTaskPrefix, taskID)
	return c.cache.Get(ctx, key, dest)
}

func (c *GenerationTaskCache) ListUserTaskIDs(ctx context.Context, userID uint, limit int) ([]string, error) {
	if limit <= 0 {
		limit = generationTaskDefaultSize
	}
	return c.cache.client.ZRange(ctx, generationTaskUserKey(userID), 0, int64(limit-1)).Result()
}

func (c *GenerationTaskCache) Enqueue(ctx context.Context, taskID string, req interface{}) error {
	reqKey := fmt.Sprintf("%s%s", generationTaskRequestPrefix, taskID)
	data, err := marshalCacheValue(req)
	if err != nil {
		return err
	}
	pipe := c.cache.client.TxPipeline()
	pipe.Set(ctx, reqKey, data, generationTaskDefaultTTL)
	pipe.RPush(ctx, generationTaskQueueKey, taskID)
	_, err = pipe.Exec(ctx)
	return err
}

func (c *GenerationTaskCache) BlockingDequeue(ctx context.Context, dest interface{}) (string, error) {
	values, err := c.cache.client.BLPop(ctx, 0, generationTaskQueueKey).Result()
	if err != nil {
		return "", err
	}
	if len(values) < 2 {
		return "", fmt.Errorf("redis queue returned malformed response")
	}
	taskID := values[1]
	reqKey := fmt.Sprintf("%s%s", generationTaskRequestPrefix, taskID)
	if err := c.cache.Get(ctx, reqKey, dest); err != nil {
		return taskID, err
	}
	_ = c.cache.Delete(ctx, reqKey)
	return taskID, nil
}

func generationTaskUserKey(userID uint) string {
	return fmt.Sprintf("%s%d", generationTaskUserPrefix, userID)
}

func marshalCacheValue(value interface{}) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("序列化失败: %w", err)
	}
	return data, nil
}
