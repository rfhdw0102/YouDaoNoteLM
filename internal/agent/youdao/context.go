package youdao

import "context"

type contextKey string

const (
	userIDKey     contextKey = "youdao_user_id"
	notebookIDKey contextKey = "youdao_notebook_id"
)

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

// WithNotebookID 将 notebookID 注入 context
func WithNotebookID(ctx context.Context, notebookID uint) context.Context {
	return context.WithValue(ctx, notebookIDKey, notebookID)
}

// GetNotebookID 从 context 获取 notebookID
func GetNotebookID(ctx context.Context) uint {
	if v, ok := ctx.Value(notebookIDKey).(uint); ok {
		return v
	}
	return 0
}
