// task_store.go 实现生成任务的持久化存储。
//
// generationTaskCacheStore 是 GenerationTaskStore 接口的缓存实现，
// 底层使用 pkg/cache 提供的 Redis 客户端，以 JSON 序列化方式存储任务对象。
// 任务 ID 作为 Redis key，支持 Save/Get/List 操作。
//
// 任务状态是前端轮询查询的唯一真相源，前端通过 GET /generations/tasks 获取最新状态。
package generation

import (
	"YoudaoNoteLm/pkg/cache"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"context"
	"sort"
	"sync"
)

type generationTaskCacheStore struct {
	cache *cache.GenerationTaskCache
}

// NewGenerationTaskCacheStore 创建并返回基于 Redis 缓存的任务存储实例。
func NewGenerationTaskCacheStore(taskCache *cache.GenerationTaskCache) GenerationTaskStore {
	if taskCache == nil {
		return nil
	}
	return &generationTaskCacheStore{cache: taskCache}
}

// Save 将任务序列化后存入 Redis。
func (s *generationTaskCacheStore) Save(ctx context.Context, task *GenerationTask) error {
	return s.cache.Save(ctx, task.TaskID, task.UserID, task.Sequence, task)
}

// Get 按任务 ID 从 Redis 读取并反序列化任务。
func (s *generationTaskCacheStore) Get(ctx context.Context, taskID string) (*GenerationTask, error) {
	var task GenerationTask
	if err := s.cache.Get(ctx, taskID, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// Delete 委托 cache 删除任务数据，幂等。
func (s *generationTaskCacheStore) Delete(ctx context.Context, taskID string) error {
	return s.cache.Delete(ctx, taskID)
}

// List 按过滤条件查询用户任务列表并排序。
func (s *generationTaskCacheStore) List(ctx context.Context, filter GenerationTaskListFilter) ([]*GenerationTask, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 100
	}
	taskIDs, err := s.cache.ListUserTaskIDs(ctx, filter.UserID, filter.Limit*4)
	if err != nil {
		return nil, err
	}
	tasks := make([]*GenerationTask, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		task, err := s.Get(ctx, taskID)
		if err != nil {
			continue
		}
		if filter.NotebookID != 0 && task.NotebookID != filter.NotebookID {
			continue
		}
		tasks = append(tasks, task)
		if len(tasks) >= filter.Limit {
			break
		}
	}
	sortGenerationTasks(tasks)
	return tasks, nil
}

type inMemoryGenerationTaskStore struct {
	mu    sync.Mutex
	tasks map[string]*GenerationTask
}

// NewInMemoryGenerationTaskStore 创建并返回内存版任务存储实例。
func NewInMemoryGenerationTaskStore() GenerationTaskStore {
	return &inMemoryGenerationTaskStore{tasks: map[string]*GenerationTask{}}
}

// Save 将任务副本存入内存 map。
func (s *inMemoryGenerationTaskStore) Save(_ context.Context, task *GenerationTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *task
	s.tasks[task.TaskID] = &cp
	return nil
}

// Get 按任务 ID 从内存 map 读取任务副本。
func (s *inMemoryGenerationTaskStore) Get(_ context.Context, taskID string) (*GenerationTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[taskID]
	if !ok {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "generation task not found")
	}
	cp := *task
	return &cp, nil
}

// Delete 从内存 map 删除任务，幂等：任务不存在也返回 nil。
func (s *inMemoryGenerationTaskStore) Delete(_ context.Context, taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tasks, taskID)
	return nil
}

// List 按过滤条件从内存 map 查询任务列表并排序。
func (s *inMemoryGenerationTaskStore) List(_ context.Context, filter GenerationTaskListFilter) ([]*GenerationTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 100
	}
	tasks := make([]*GenerationTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		if filter.UserID != 0 && task.UserID != filter.UserID {
			continue
		}
		if filter.NotebookID != 0 && task.NotebookID != filter.NotebookID {
			continue
		}
		cp := *task
		tasks = append(tasks, &cp)
	}
	sortGenerationTasks(tasks)
	if len(tasks) > filter.Limit {
		tasks = tasks[:filter.Limit]
	}
	return tasks, nil
}

// sortGenerationTasks 按 sequence、createdAt、taskID 稳定排序任务。
func sortGenerationTasks(tasks []*GenerationTask) {
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].Sequence != tasks[j].Sequence {
			return tasks[i].Sequence < tasks[j].Sequence
		}
		if tasks[i].CreatedAt != tasks[j].CreatedAt {
			return tasks[i].CreatedAt < tasks[j].CreatedAt
		}
		return tasks[i].TaskID < tasks[j].TaskID
	})
}
