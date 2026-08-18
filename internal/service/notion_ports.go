package service

import (
	"YoudaoNoteLm/pkg/cache"
	"context"
	"time"
)

// OAuthStateStore 保存并一次性消费用户绑定的 OAuth state。
type OAuthStateStore interface {
	Save(ctx context.Context, state string, userID uint, ttl time.Duration) error
	Consume(ctx context.Context, state string) (uint, error)
}

// ImportTaskStore 提供 Notion 导入任务的持久化端口。
type ImportTaskStore interface {
	Save(ctx context.Context, task *cache.ImportTask) error
	GetForUser(ctx context.Context, userID uint, taskID string) (*cache.ImportTask, error)
	CancelForUser(ctx context.Context, userID uint, taskID string) (*cache.ImportTask, error)
	RegisterCancel(taskID string, cancel context.CancelFunc)
	ClearCancel(taskID string)
	Delete(ctx context.Context, taskID string) error
}
