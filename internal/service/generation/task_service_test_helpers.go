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

func newGenerationTaskMemoryStore() *generationTaskMemoryStore {
	return &generationTaskMemoryStore{tasks: map[string]*GenerationTask{}}
}

func (s *generationTaskMemoryStore) Save(_ context.Context, task *GenerationTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *task
	s.tasks[task.TaskID] = &cp
	return nil
}

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

func (s *fakeGenerationService) Generate(_ context.Context, req *GenerationRequest) (*GenerationResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.resp, nil
}

func (s *fakeGenerationService) Export(_ context.Context, _ *GenerationExportRequest) (*GenerationExportResult, error) {
	return nil, nil
}

type blockingGenerationService struct {
	started   chan GenerationType
	release   chan struct{}
	completed chan GenerationType
}

func (s *blockingGenerationService) Generate(_ context.Context, req *GenerationRequest) (*GenerationResponse, error) {
	s.started <- req.Type
	<-s.release
	s.completed <- req.Type
	return &GenerationResponse{
		Type:    req.Type,
		Content: "# " + string(req.Type),
	}, nil
}

func (s *blockingGenerationService) Export(_ context.Context, _ *GenerationExportRequest) (*GenerationExportResult, error) {
	return nil, nil
}

type cancellableGenerationService struct {
	started   chan struct{}
	cancelled chan struct{}
}

func (s *cancellableGenerationService) Generate(ctx context.Context, req *GenerationRequest) (*GenerationResponse, error) {
	close(s.started)
	<-ctx.Done()
	close(s.cancelled)
	return nil, ctx.Err()
}

func (s *cancellableGenerationService) Export(_ context.Context, _ *GenerationExportRequest) (*GenerationExportResult, error) {
	return nil, nil
}

type submitOnlyGenerationTaskQueue struct {
	enqueued chan queuedGenerationTask
}

func newSubmitOnlyGenerationTaskQueue() *submitOnlyGenerationTaskQueue {
	return &submitOnlyGenerationTaskQueue{enqueued: make(chan queuedGenerationTask, 1)}
}

func (q *submitOnlyGenerationTaskQueue) Enqueue(_ context.Context, item queuedGenerationTask) error {
	q.enqueued <- item
	return nil
}

func (q *submitOnlyGenerationTaskQueue) Dequeue(ctx context.Context) (queuedGenerationTask, error) {
	<-ctx.Done()
	return queuedGenerationTask{}, ctx.Err()
}
