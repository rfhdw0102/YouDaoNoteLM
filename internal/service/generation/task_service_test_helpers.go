// task_service_test_helpers.go 提供任务服务的测试辅助工具。
//
// 包含内存版 store/queue 的测试实现、阻塞型 generation service、
// 等待任务达到指定状态的工具函数等，供 task_service 单元测试使用。
package generation

import (
	"context"
	"errors"
	"sync"
)

type generationTaskMemoryStore struct {
	mu    sync.Mutex
	tasks map[string]*GenerationTask
}

// newGenerationTaskMemoryStore 创建并返回测试用内存 store 实例。
func newGenerationTaskMemoryStore() *generationTaskMemoryStore {
	return &generationTaskMemoryStore{tasks: map[string]*GenerationTask{}}
}

// Save 将任务副本存入内存 map。
func (s *generationTaskMemoryStore) Save(_ context.Context, task *GenerationTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *task
	s.tasks[task.TaskID] = &cp
	return nil
}

// Get 按任务 ID 从内存 map 读取任务副本。
func (s *generationTaskMemoryStore) Get(_ context.Context, taskID string) (*GenerationTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[taskID]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := *task
	return &cp, nil
}

// List 按过滤条件从内存 map 查询任务列表并排序。
func (s *generationTaskMemoryStore) List(_ context.Context, filter GenerationTaskListFilter) ([]*GenerationTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	if filter.Limit > 0 && len(tasks) > filter.Limit {
		tasks = tasks[:filter.Limit]
	}
	return tasks, nil
}

type fakeGenerationService struct {
	resp *GenerationResponse
	err  error
}

// Generate 返回预设的响应或错误。
func (s *fakeGenerationService) Generate(_ context.Context, req *GenerationRequest) (*GenerationResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.resp, nil
}

// Export 返回空结果，用于实现接口。
func (s *fakeGenerationService) Export(_ context.Context, _ *GenerationExportRequest) (*GenerationExportResult, error) {
	return nil, nil
}

type blockingGenerationService struct {
	started   chan GenerationType
	release   chan struct{}
	completed chan GenerationType
}

// Generate 阻塞直到收到 release 信号才返回结果。
func (s *blockingGenerationService) Generate(_ context.Context, req *GenerationRequest) (*GenerationResponse, error) {
	s.started <- req.Type
	<-s.release
	s.completed <- req.Type
	return &GenerationResponse{
		Type:    req.Type,
		Content: "# " + string(req.Type),
	}, nil
}

// Export 返回空结果，用于实现接口。
func (s *blockingGenerationService) Export(_ context.Context, _ *GenerationExportRequest) (*GenerationExportResult, error) {
	return nil, nil
}

type cancellableGenerationService struct {
	started   chan struct{}
	cancelled chan struct{}
}

// Generate 阻塞直到 ctx 被取消，用于测试取消逻辑。
func (s *cancellableGenerationService) Generate(ctx context.Context, req *GenerationRequest) (*GenerationResponse, error) {
	close(s.started)
	<-ctx.Done()
	close(s.cancelled)
	return nil, ctx.Err()
}

// Export 返回空结果，用于实现接口。
func (s *cancellableGenerationService) Export(_ context.Context, _ *GenerationExportRequest) (*GenerationExportResult, error) {
	return nil, nil
}

type submitOnlyGenerationTaskQueue struct {
	enqueued chan queuedGenerationTask
}

// newSubmitOnlyGenerationTaskQueue 创建并返回仅支持入队的测试队列实例。
func newSubmitOnlyGenerationTaskQueue() *submitOnlyGenerationTaskQueue {
	return &submitOnlyGenerationTaskQueue{enqueued: make(chan queuedGenerationTask, 1)}
}

// Enqueue 将任务投递到 channel。
func (q *submitOnlyGenerationTaskQueue) Enqueue(_ context.Context, item queuedGenerationTask) error {
	q.enqueued <- item
	return nil
}

// Dequeue 阻塞直到 ctx 被取消，模拟无任务可出队。
func (q *submitOnlyGenerationTaskQueue) Dequeue(ctx context.Context) (queuedGenerationTask, error) {
	<-ctx.Done()
	return queuedGenerationTask{}, ctx.Err()
}
