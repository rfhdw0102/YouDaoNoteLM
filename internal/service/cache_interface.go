package service

import (
	"context"
	"time"
)

// CacheStore 缓存操作接口（解耦 Redis 实现，便于测试）
type CacheStore interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}
