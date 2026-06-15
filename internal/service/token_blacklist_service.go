package service

import (
	"YoudaoNoteLm/pkg/jwt"
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// tokenBlacklistKeyPrefix 单个 token 黑名单 key 前缀
	tokenBlacklistKeyPrefix = "token:blacklist:"
	// userTokenKeyPrefix 用户 token 集合 key 前缀（用于批量吊销）
	userTokenKeyPrefix = "token:user:"
)

// tokenBlacklistService Token 黑名单服务实现
type tokenBlacklistService struct {
	redis *redis.Client
}

// NewTokenBlacklistService 创建 Token 黑名单服务
func NewTokenBlacklistService(redisClient *redis.Client) TokenBlacklistService {
	return &tokenBlacklistService{
		redis: redisClient,
	}
}

// blacklistKey 黑名单 key: token:blacklist:{jti}
func (s *tokenBlacklistService) blacklistKey(jti string) string {
	return fmt.Sprintf("%s%s", tokenBlacklistKeyPrefix, jti)
}

// RevokeToken 将 token 加入黑名单
func (s *tokenBlacklistService) RevokeToken(ctx context.Context, tokenString string) error {
	// 解析 token 获取 JTI 和过期时间（不做有效性校验，过期的 token 也需要记录）
	parser := jwt.GetParser()
	claims := &jwt.CustomClaims{}
	_, _, err := parser.ParseUnverified(tokenString, claims)
	if err != nil {
		return fmt.Errorf("解析 token 失败: %w", err)
	}

	if claims.ID == "" {
		return fmt.Errorf("token 缺少 JTI")
	}

	// 计算 token 剩余过期时间作为黑名单 TTL
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		// token 已过期，无需加入黑名单
		return nil
	}

	// 存入 Redis，过期后自动清除
	key := s.blacklistKey(claims.ID)
	return s.redis.Set(ctx, key, "1", ttl).Err()
}

// IsRevoked 检查 token 是否已被撤销
func (s *tokenBlacklistService) IsRevoked(ctx context.Context, jti string) (bool, error) {
	key := s.blacklistKey(jti)
	exists, err := s.redis.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("查询黑名单失败: %w", err)
	}
	return exists > 0, nil
}
