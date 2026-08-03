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
	// generationTaskQueueKey 队列基于 Redis Set 实现，SADD 入队、SPOP 出队。
	// Set 天然去重，SPOP 原子弹出；不保证严格 FIFO，但生成任务串行处理，顺序无关紧要。
	generationTaskQueueKey    = "generation:task:queue"
	generationTaskDefaultTTL  = 24 * time.Hour
	generationTaskDefaultSize = 100
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

// Delete 按 taskID 删除任务：先读取任务拿到 userID，再用 pipeline 同时删 task 数据和
// user sorted set 中的 member。任务不存在或已删除均返回 nil（幂等）。
func (c *GenerationTaskCache) Delete(ctx context.Context, taskID string) error {
	if taskID == "" {
		return nil
	}
	key := fmt.Sprintf("%s%s", generationTaskPrefix, taskID)

	// 先读取任务以拿到 user_id，便于从 user sorted set 中移除。
	// 读不到也继续删除 task key 本身，保证幂等。
	var payload struct {
		UserID uint `json:"user_id"`
	}
	_ = c.cache.Get(ctx, key, &payload)

	pipe := c.cache.client.TxPipeline()
	pipe.Del(ctx, key)
	pipe.Del(ctx, fmt.Sprintf("%s%s", generationTaskRequestPrefix, taskID))
	if payload.UserID != 0 {
		pipe.ZRem(ctx, generationTaskUserKey(payload.UserID), taskID)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// Enqueue 将任务 ID 投递到 Redis Set 队列，并缓存请求体。
// 使用 SADD 入队，Set 结构天然去重，重复投递同一 taskID 不会产生重复消费。
func (c *GenerationTaskCache) Enqueue(ctx context.Context, taskID string, req interface{}) error {
	reqKey := fmt.Sprintf("%s%s", generationTaskRequestPrefix, taskID)
	data, err := marshalCacheValue(req)
	if err != nil {
		return err
	}
	pipe := c.cache.client.TxPipeline()
	pipe.Set(ctx, reqKey, data, generationTaskDefaultTTL)
	pipe.SAdd(ctx, generationTaskQueueKey, taskID)
	_, err = pipe.Exec(ctx)
	return err
}

// Dequeue 从 Redis Set 队列原子弹出一个 taskID 并读取其请求体。
// 队列为空时返回 redis.Nil 错误，调用方应轮询重试。
func (c *GenerationTaskCache) Dequeue(ctx context.Context, dest interface{}) (string, error) {
	taskID, err := c.cache.client.SPop(ctx, generationTaskQueueKey).Result()
	if err != nil {
		return "", err
	}
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
