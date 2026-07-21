// task_queue.go 实现生成任务队列。
//
// 提供两种队列实现：
//   - inMemoryGenerationTaskQueue：基于带缓冲 channel 的内存队列（单机部署）
//   - redisGenerationTaskQueue：基于 Redis Set 的分布式队列（多实例部署）
//
// Redis 队列使用 SADD 入队、SPOP 出队。Set 结构天然去重，SPOP 原子弹出。
// 由于 SPOP 不阻塞，队列为空时 worker 通过短暂 sleep 轮询，避免空转压垮 Redis。
// 前端通过 REST 接口 GET /generations/tasks 轮询任务状态，不再依赖 WebSocket。
package generation

import (
	"YoudaoNoteLm/pkg/cache"
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisTaskQueuePollInterval 队列为空时的轮询间隔。
// 取 100ms 在响应延迟与 Redis 压力之间取折中：单实例每秒约 10 次空查询。
const redisTaskQueuePollInterval = 100 * time.Millisecond

type inMemoryGenerationTaskQueue struct {
	ch chan queuedGenerationTask
}

// NewInMemoryGenerationTaskQueue 创建并返回基于 channel 的内存队列实例。
func NewInMemoryGenerationTaskQueue(size int) GenerationTaskQueue {
	if size <= 0 {
		size = generationTaskQueueSize
	}
	return &inMemoryGenerationTaskQueue{ch: make(chan queuedGenerationTask, size)}
}

// Enqueue 将任务投递到 channel，满时返回错误。
func (q *inMemoryGenerationTaskQueue) Enqueue(ctx context.Context, item queuedGenerationTask) error {
	select {
	case q.ch <- item:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return errors.New("generation task queue is full")
	}
}

// Dequeue 阻塞等待从 channel 取出任务。
func (q *inMemoryGenerationTaskQueue) Dequeue(ctx context.Context) (queuedGenerationTask, error) {
	select {
	case item := <-q.ch:
		return item, nil
	case <-ctx.Done():
		return queuedGenerationTask{}, ctx.Err()
	}
}

type redisGenerationTaskQueue struct {
	cache *cache.GenerationTaskCache
}

// NewGenerationTaskRedisQueue 创建并返回基于 Redis Set 的分布式队列实例。
func NewGenerationTaskRedisQueue(taskCache *cache.GenerationTaskCache) GenerationTaskQueue {
	if taskCache == nil {
		return nil
	}
	return &redisGenerationTaskQueue{cache: taskCache}
}

// Enqueue 将任务投递到 Redis Set 队列。
func (q *redisGenerationTaskQueue) Enqueue(ctx context.Context, item queuedGenerationTask) error {
	return q.cache.Enqueue(ctx, item.taskID, item.req)
}

// Dequeue 从 Redis Set 队列弹出任务。
// SPOP 不阻塞，队列为空时返回 redis.Nil，此处通过短 sleep 轮询模拟阻塞语义，
// 并响应 ctx 取消。返回的 (taskID, err) 在请求体读取失败时仍携带 taskID，
// 供 worker 将该任务标记为 failed。
func (q *redisGenerationTaskQueue) Dequeue(ctx context.Context) (queuedGenerationTask, error) {
	for {
		var req GenerationRequest
		taskID, err := q.cache.Dequeue(ctx, &req)
		if err == nil {
			return queuedGenerationTask{taskID: taskID, req: &req}, nil
		}
		if taskID != "" {
			// taskID 已弹出但请求体读取失败，返回 taskID 让 worker 标记失败。
			return queuedGenerationTask{taskID: taskID}, err
		}
		// 队列为空（redis.Nil）或其他错误：短暂等待后重试，期间响应 ctx 取消。
		if !errors.Is(err, redis.Nil) {
			return queuedGenerationTask{}, err
		}
		select {
		case <-ctx.Done():
			return queuedGenerationTask{}, ctx.Err()
		case <-time.After(redisTaskQueuePollInterval):
		}
	}
}
