package cache

import (
	"context"
	"fmt"
	"time"
)

// ImportTask 导入任务缓存结构
type ImportTask struct {
	TaskID       string `json:"task_id"`       // 任务ID
	UserID       uint   `json:"user_id"`       // 所属用户
	NotebookID   uint   `json:"notebook_id"`   // 所属笔记本
	TaskType     string `json:"task_type"`     // 任务类型: batch_file/batch_url/youdao
	TotalCount   int    `json:"total_count"`   // 总数
	SuccessCount int    `json:"success_count"` // 成功数
	FailCount    int    `json:"fail_count"`    // 失败数
	Status       string `json:"status"`        // 状态: pending/processing/completed
	ErrorDetail  string `json:"error_detail"`  // 错误详情(JSON)
	CreatedAt    int64  `json:"created_at"`    // 创建时间戳
}

// ImportTaskCache 导入任务缓存操作
type ImportTaskCache struct {
	cache *Cache
}

// NewImportTaskCache 创建导入任务缓存
func NewImportTaskCache(cache *Cache) *ImportTaskCache {
	return &ImportTaskCache{cache: cache}
}

// key 前缀
const importTaskPrefix = "import:task:"

// Save 保存导入任务（默认24小时过期）
func (c *ImportTaskCache) Save(ctx context.Context, task *ImportTask) error {
	key := fmt.Sprintf("%s%s", importTaskPrefix, task.TaskID)
	return c.cache.Set(ctx, key, task, 24*time.Hour)
}

// Get 获取导入任务
func (c *ImportTaskCache) Get(ctx context.Context, taskID string) (*ImportTask, error) {
	key := fmt.Sprintf("%s%s", importTaskPrefix, taskID)
	var task ImportTask
	err := c.cache.Get(ctx, key, &task)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// UpdateStatus 更新任务状态
func (c *ImportTaskCache) UpdateStatus(ctx context.Context, taskID string, status string, successCount, failCount int) error {
	task, err := c.Get(ctx, taskID)
	if err != nil {
		return err
	}
	task.Status = status
	task.SuccessCount = successCount
	task.FailCount = failCount
	return c.Save(ctx, task)
}

// Delete 删除导入任务
func (c *ImportTaskCache) Delete(ctx context.Context, taskID string) error {
	key := fmt.Sprintf("%s%s", importTaskPrefix, taskID)
	return c.cache.Delete(ctx, key)
}

// Exists 检查任务是否存在
func (c *ImportTaskCache) Exists(ctx context.Context, taskID string) (bool, error) {
	key := fmt.Sprintf("%s%s", importTaskPrefix, taskID)
	return c.cache.Exists(ctx, key)
}
