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

// userTokenKey 用户 token 集合 key: token:user:{userID}
func (s *tokenBlacklistService) userTokenKey(userID uint) string {
	return fmt.Sprintf("%s%d", userTokenKeyPrefix, userID)
}

// AddUserToken 将 jti 加入用户 token 集合，并续期集合 TTL
// Redis Set 不支持元素级 TTL，每次 SAdd 后整体 Expire 续期
func (s *tokenBlacklistService) AddUserToken(ctx context.Context, userID uint, jti string, ttl time.Duration) error {
	key := s.userTokenKey(userID)
	pipe := s.redis.Pipeline()
	pipe.SAdd(ctx, key, jti)
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("添加用户 token 到集合失败: %w", err)
	}
	return nil
}

// RemoveUserToken 从用户 token 集合移除 jti（refresh 换发新 token 后清理旧 jti）
func (s *tokenBlacklistService) RemoveUserToken(ctx context.Context, userID uint, jti string) error {
	key := s.userTokenKey(userID)
	if err := s.redis.SRem(ctx, key, jti).Err(); err != nil {
		return fmt.Errorf("从用户 token 集合移除失败: %w", err)
	}
	return nil
}

// RevokeUserTokens 拉黑该用户集合中所有 token，并删除集合
// tokenTTL 应取 refresh token 有效期，保证 refresh token 被拉黑到过期
func (s *tokenBlacklistService) RevokeUserTokens(ctx context.Context, userID uint, tokenTTL time.Duration) (int, error) {
	key := s.userTokenKey(userID)
	jtis, err := s.redis.SMembers(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("查询用户 token 集合失败: %w", err)
	}
	if len(jtis) == 0 {
		return 0, nil
	}

	// 批量拉黑 + 删除集合，Pipeline 减少往返
	pipe := s.redis.Pipeline()
	for _, jti := range jtis {
		pipe.Set(ctx, s.blacklistKey(jti), "1", tokenTTL)
	}
	pipe.Del(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("批量拉黑用户 token 失败: %w", err)
	}
	return len(jtis), nil
}
