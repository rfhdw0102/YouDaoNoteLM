package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache Redis缓存操作封装
type Cache struct {
	client *redis.Client
}

// New 创建缓存实例
func New(client *redis.Client) *Cache {
	return &Cache{client: client}
}

// Set 设置缓存
func (c *Cache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
	}
	return c.client.Set(ctx, key, data, expiration).Err()
}

// Get 获取缓存
func (c *Cache) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// Delete 删除缓存
func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

// Exists 检查key是否存在
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	result, err := c.client.Exists(ctx, key).Result()
	return result > 0, err
}

// SetHash 设置Hash缓存
func (c *Cache) SetHash(ctx context.Context, key string, field string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
	}
	return c.client.HSet(ctx, key, field, data).Err()
}

// GetHash 获取Hash字段
func (c *Cache) GetHash(ctx context.Context, key string, field string, dest interface{}) error {
	data, err := c.client.HGet(ctx, key, field).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// GetAllHash 获取Hash所有字段
func (c *Cache) GetAllHash(ctx context.Context, key string) (map[string]string, error) {
	return c.client.HGetAll(ctx, key).Result()
}

// DeleteHashField 删除Hash字段
func (c *Cache) DeleteHashField(ctx context.Context, key string, fields ...string) error {
	return c.client.HDel(ctx, key, fields...).Err()
}

// SetWithExpire 设置带过期时间的缓存（以key为维度）
func (c *Cache) SetWithExpire(ctx context.Context, key string, value interface{}, expireAt time.Time) error {
	ttl := time.Until(expireAt)
	if ttl <= 0 {
		return nil // 已过期，不存储
	}
	return c.Set(ctx, key, value, ttl)
}
