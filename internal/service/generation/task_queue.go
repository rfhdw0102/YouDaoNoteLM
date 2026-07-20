// task_queue.go 实现生成任务队列。
//
// 提供两种队列实现：
//   - inMemoryGenerationTaskQueue：基于带缓冲 channel 的内存队列（单机部署）
//   - redisGenerationTaskQueue：基于 Redis List 的分布式队列（多实例部署）
//
// worker 通过 Dequeue 阻塞等待新任务，Submit 通过 Enqueue 投递任务。
package generation

import (
	"YoudaoNoteLm/pkg/cache"
	"context"
	"errors"
)

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

// NewGenerationTaskRedisQueue 创建并返回基于 Redis List 的分布式队列实例。
func NewGenerationTaskRedisQueue(taskCache *cache.GenerationTaskCache) GenerationTaskQueue {
	if taskCache == nil {
		return nil
	}
	return &redisGenerationTaskQueue{cache: taskCache}
}

// Enqueue 将任务投递到 Redis 队列。
func (q *redisGenerationTaskQueue) Enqueue(ctx context.Context, item queuedGenerationTask) error {
	return q.cache.Enqueue(ctx, item.taskID, item.req)
}

// Dequeue 阻塞等待从 Redis 队列取出任务。
func (q *redisGenerationTaskQueue) Dequeue(ctx context.Context) (queuedGenerationTask, error) {
	var req GenerationRequest
	taskID, err := q.cache.BlockingDequeue(ctx, &req)
	if err != nil {
		if taskID != "" {
			return queuedGenerationTask{taskID: taskID}, err
		}
		return queuedGenerationTask{}, err
	}
	return queuedGenerationTask{taskID: taskID, req: &req}, nil
}
