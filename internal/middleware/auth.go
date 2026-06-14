package middleware

import (
	"context"
	"strings"

	"YoudaoNoteLm/internal/service"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/jwt"
	"YoudaoNoteLm/pkg/logger"
	"YoudaoNoteLm/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	// ContextUserID 用户 ID 上下文键
	ContextUserID = "user_id"
	// ContextUsername 用户名 上下文键
	ContextUsername = "username"
)

// Auth JWT 认证中间件（仅接受 Access Token，检查黑名单）
func Auth(blacklist service.TokenBlacklistService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Header 获取 Authorization
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, "请提供认证令牌")
			c.Abort()
			return
		}

		// 解析 Bearer Token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Unauthorized(c, "令牌格式错误")
			c.Abort()
			return
		}

		// 解析 Token
		claims, err := jwt.ParseToken(parts[1])
		if err != nil {
			// 根据错误类型返回对应业务错误码
			if err == jwt.ErrTokenExpired {
				response.Error(c, bizerrors.CodeTokenExpired, err.Error())
			} else {
				response.Error(c, bizerrors.CodeInvalidToken, err.Error())
			}
			c.Abort()
			return
		}

		// 必须是 access token
		if claims.TokenType != jwt.AccessToken {
			response.Error(c, bizerrors.CodeInvalidToken, "请使用 access_token 进行认证")
			c.Abort()
			return
		}

		// 检查 token 是否已被吊销
		revoked, err := blacklist.IsRevoked(c.Request.Context(), claims.ID)
		if err != nil {
			response.InternalError(c, "验证令牌状态失败")
			c.Abort()
			return
		}
		if revoked {
			response.Error(c, bizerrors.CodeInvalidToken, "令牌已失效，请重新登录")
			c.Abort()
			return
		}

		// 将用户信息存入上下文
		c.Set(ContextUserID, claims.GetUserID())
		c.Set(ContextUsername, claims.GetUsername())

		c.Next()
	}
}

// GetUserID 从上下文获取用户 ID
func GetUserID(c *gin.Context) uint {
	if userID, exists := c.Get(ContextUserID); exists {
		return userID.(uint)
	}
	return 0
}

// GetUsername 从上下文获取用户名
func GetUsername(c *gin.Context) string {
	if username, exists := c.Get(ContextUsername); exists {
		return username.(string)
	}
	return ""
}

// contextKey 上下文键类型
type contextKey string

const (
	// ctxUserID 用户 ID 上下文键（用于 context.Context）
	ctxUserID contextKey = "user_id"
)

// WithUserID 将用户 ID 存入 context.Context
func WithUserID(ctx context.Context, userID uint) context.Context {
	return context.WithValue(ctx, ctxUserID, userID)
}

// GetUserIDFromCtx 从 context.Context 获取用户 ID
func GetUserIDFromCtx(ctx context.Context) uint {
	if userID, ok := ctx.Value(ctxUserID).(uint); ok {
		return userID
	}
	return 0
}

// OptionalAuth 可选的 JWT 认证中间件（仅接受 Access Token，检查黑名单）
func OptionalAuth(blacklist service.TokenBlacklistService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		claims, err := jwt.ParseToken(parts[1])
		if err == nil && claims.TokenType == jwt.AccessToken {
			// 检查黑名单
			revoked, err := blacklist.IsRevoked(c.Request.Context(), claims.ID)
			if err != nil {
				logger.Error("检查 token 黑名单失败", zap.Error(err))
				// 查询失败时默认视为已吊销（安全优先）
				revoked = true
			}
			if !revoked {
				c.Set(ContextUserID, claims.GetUserID())
				c.Set(ContextUsername, claims.GetUsername())
			}
		}

		c.Next()
	}
}
