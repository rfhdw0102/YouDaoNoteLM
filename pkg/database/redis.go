package database

import (
	"YoudaoNoteLm/pkg/config"
	"YoudaoNoteLm/pkg/logger"
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var redisClient *redis.Client

// InitRedis 初始化 Redis 连接
func InitRedis(cfg *config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		logger.Error("Redis 连接失败", zap.Error(err))
		return nil, fmt.Errorf("Redis 连接失败: %w", err)
	}

	redisClient = client
	logger.Info("Redis 连接成功",
		zap.String("addr", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)),
		zap.Int("db", cfg.DB),
	)

	return client, nil
}

// GetRedis 获取 Redis 实例
func GetRedis() *redis.Client {
	if redisClient == nil {
		panic("Redis 未初始化")
	}
	return redisClient
}

// CloseRedis 关闭 Redis 连接
func CloseRedis() error {
	if redisClient != nil {
		return redisClient.Close()
	}
	return nil
}
