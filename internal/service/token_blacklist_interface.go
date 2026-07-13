package service

import (
	"context"
	"time"
)

// TokenBlacklistService Token 黑名单服务接口
type TokenBlacklistService interface {
	// RevokeToken 将单个 token 加入黑名单，TTL = token 剩余过期时间
	RevokeToken(ctx context.Context, tokenString string) error
	// IsRevoked 检查 token 是否已被撤销
	IsRevoked(ctx context.Context, jti string) (bool, error)

	// AddUserToken 将 jti 加入用户 token 集合，并续期集合 TTL
	// 用于登录/refresh 时登记新会话，配合 RevokeUserTokens 实现用户级批量吊销
	AddUserToken(ctx context.Context, userID uint, jti string, ttl time.Duration) error
	// RemoveUserToken 从用户 token 集合移除 jti
	// 用于 refresh 换发新 access token 后清理旧 jti，避免集合膨胀（可选调用）
	RemoveUserToken(ctx context.Context, userID uint, jti string) error
	// RevokeUserTokens 拉黑该用户集合中所有 token，并删除集合，返回拉黑数量
	// tokenTTL 应取 refresh token 的有效期（24h），保证 refresh token 也被拉黑到过期；
	// access token 15m 自然过期，多拉黑无害
	RevokeUserTokens(ctx context.Context, userID uint, tokenTTL time.Duration) (int, error)
}
