// internal/agent/search/types.go
package search

import "context"

// contextKey 工具调用上下文（从 context.Context 传递 userID 等信息）
type contextKey string

const userIDKey contextKey = "userID"

// WithUserID 将 userID 注入 context
func WithUserID(ctx context.Context, userID uint) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// GetUserID 从 context 获取 userID
func GetUserID(ctx context.Context) uint {
	if v, ok := ctx.Value(userIDKey).(uint); ok {
		return v
	}
	return 0
}
