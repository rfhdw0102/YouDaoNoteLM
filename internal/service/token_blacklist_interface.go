package service

import "context"

// TokenBlacklistService Token 黑名单服务接口
type TokenBlacklistService interface {
	// RevokeToken 将 token 加入黑名单，TTL = token 剩余过期时间
	RevokeToken(ctx context.Context, tokenString string) error
	// IsRevoked 检查 token 是否已被撤销
	IsRevoked(ctx context.Context, jti string) (bool, error)
}
