// task_service.go 实现异步生成任务服务。
//
// generationTaskService 负责任务提交、状态流转、worker 循环：
//   - Submit：创建 pending 任务，入队，推送 pending 事件
//   - worker：单线程串行 dequeue 任务，标记 running，调用 GenerationService.Generate，
//     根据结果标记 completed/failed/cancelled，推送终态事件
//   - SubscribeTasks：订阅任务状态变更（供 WebSocket 使用）
//   - CancelTask：取消正在执行的任务
//
// 关键设计：
//   - worker 单线程串行执行，避免并发请求压垮 LLM 服务
//   - 每个任务有 generationTaskMaxRunTime 超时（10 分钟），防止 LLM 挂起导致全队阻塞
//   - Save 失败时仍推送事件，让前端能感知真实状态（避免永久卡 pending）
package generation

import (
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"
	"context"
	"errors"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"sync"
	"sync/atomic"
	"time"
)

// generationTaskMaxRunTime 单个生成任务的最大执行时长。
// 超时后任务会被标记为 failed，避免 LLM 调用挂起导致 worker 永久阻塞、
// 后续排队任务无法执行。前端通过 WebSocket 事件感知到失败状态。
const generationTaskMaxRunTime = 10 * time.Minute

type generationTaskService struct {
	base      GenerationService
	store     GenerationTaskStore
	queue     GenerationTaskQueue
	events    *generationTaskEventHub
	sequence  atomic.Int64
	cancelers sync.Map
}

type queuedGenerationTask struct {
	taskID string
	req    *GenerationRequest
}

type GenerationTaskQueue interface {
	Enqueue(ctx context.Context, item queuedGenerationTask) error
	Dequeue(ctx context.Context) (queuedGenerationTask, error)
}

const generationTaskQueueSize = 1024

// NewGenerationTaskService 创建并返回使用默认内存队列的任务服务实例。
func NewGenerationTaskService(base GenerationService, store GenerationTaskStore) GenerationTaskService {
	return NewGenerationTaskServiceWithQueue(base, store, nil)
}

// NewGenerationTaskServiceWithQueue 创建并返回使用指定队列的任务服务实例，并启动 worker。
func NewGenerationTaskServiceWithQueue(base GenerationService, store GenerationTaskStore, queue GenerationTaskQueue) GenerationTaskService {
	if store == nil {
		store = NewInMemoryGenerationTaskStore()
	}
	if queue == nil {
		queue = NewInMemoryGenerationTaskQueue(generationTaskQueueSize)
	}
	svc := &generationTaskService{
		base:   base,
		store:  store,
		queue:  queue,
		events: newGenerationTaskEventHub(),
	}
	go svc.worker()
	return svc
}

// Submit 创建 pending 任务并入队，推送 pending 事件。
func (s *generationTaskService) Submit(ctx context.Context, req *GenerationRequest) (*GenerationTask, error) {
	if s.base == nil {
		return nil, bizerrors.New(bizerrors.CodeInternalServiceError, "generation service is not configured")
	}
	if err := validateGenerationRequest(req); err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	task := &GenerationTask{
		TaskID:     uuid.New().String(),
		UserID:     req.UserID,
		NotebookID: req.NotebookID,
		Type:       req.Type,
		Status:     GenerationTaskStatusPending,
		CreatedAt:  now,
		UpdatedAt:  now,
		Sequence:   s.sequence.Add(1),
	}
	if err := s.store.Save(ctx, task); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "save generation task failed", err)
	}
	s.publishTask(task)
	logger.Info("generation task submitted",
		zap.String("task_id", task.TaskID),
		zap.Uint("user_id", task.UserID),
		zap.Uint("notebook_id", task.NotebookID),
		zap.String("type", string(task.Type)),
		zap.Int64("sequence", task.Sequence),
	)

	reqCopy := *req
	reqCopy.SourceIDs = append([]uint(nil), req.SourceIDs...)
	if req.Options != nil {
		reqCopy.Options = make(map[string]any, len(req.Options))
		for key, value := range req.Options {
			reqCopy.Options[key] = value
		}
	}

	if err := s.queue.Enqueue(ctx, queuedGenerationTask{taskID: task.TaskID, req: &reqCopy}); err != nil {
		task.Status = GenerationTaskStatusFailed
		task.Error = "generation task enqueue failed"
		task.UpdatedAt = time.Now().Unix()
		if saveErr := s.store.Save(ctx, task); saveErr != nil {
			logger.Warn("mark generation task enqueue failed", zap.String("task_id", task.TaskID), zap.Error(saveErr))
		} else {
			s.publishTask(task)
		}
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "generation task enqueue failed", err)
	}

	return task, nil
}

// GetTask 按任务 ID 查询任务，校验所属用户。
func (s *generationTaskService) GetTask(ctx context.Context, userID uint, taskID string) (*GenerationTask, error) {
	if taskID == "" {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "task id cannot be empty")
	}
	task, err := s.store.Get(ctx, taskID)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeResourceNotFound, "generation task not found", err)
	}
	if task.UserID != userID {
		return nil, bizerrors.New(bizerrors.CodeForbidden, "generation task does not belong to current user")
	}
	return task, nil
}

// ListTasks 按用户和笔记本查询任务列表。
func (s *generationTaskService) ListTasks(ctx context.Context, userID, notebookID uint, limit int) ([]*GenerationTask, error) {
	if userID == 0 {
		return nil, bizerrors.New(bizerrors.CodeUnauthorized, "user is not authenticated")
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	tasks, err := s.store.List(ctx, GenerationTaskListFilter{
		UserID:     userID,
		NotebookID: notebookID,
		Limit:      limit,
	})
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "list generation tasks failed", err)
	}
	return tasks, nil
}

// CancelTask 取消 pending 或 running 状态的任务。
func (s *generationTaskService) CancelTask(ctx context.Context, userID uint, taskID string) error {
	if taskID == "" {
		return bizerrors.New(bizerrors.CodeInvalidParam, "task id cannot be empty")
	}
	task, err := s.store.Get(ctx, taskID)
	if err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeResourceNotFound, "generation task not found", err)
	}
	if task.UserID != userID {
		return bizerrors.New(bizerrors.CodeForbidden, "generation task does not belong to current user")
	}
	switch task.Status {
	case GenerationTaskStatusPending, GenerationTaskStatusRunning:
		task.Status = GenerationTaskStatusCancelled
		task.Error = "任务已取消"
		task.UpdatedAt = time.Now().Unix()
		if err := s.store.Save(ctx, task); err != nil {
			return bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "cancel generation task failed", err)
		}
		s.publishTask(task)
		if cancel, ok := s.cancelers.Load(taskID); ok {
			cancel.(context.CancelFunc)()
		}
		return nil
	case GenerationTaskStatusCancelled:
		return nil
	default:
		return bizerrors.New(bizerrors.CodeConflict, "generation task is already finished")
	}
}

// worker 串行消费队列中的任务并执行。
func (s *generationTaskService) worker() {
	for {
		item, err := s.queue.Dequeue(context.Background())
		if err != nil {
			if item.taskID != "" {
				s.failDequeuedTask(item.taskID, err)
				continue
			}
			logger.Warn("dequeue generation task failed", zap.Error(err))
			time.Sleep(time.Second)
			continue
		}
		s.run(item.taskID, item.req)
	}
}

// failDequeuedTask 将出队失败的任务标记为 failed 并推送事件。
func (s *generationTaskService) failDequeuedTask(taskID string, cause error) {
	ctx := context.Background()
	task, err := s.store.Get(ctx, taskID)
	if err != nil {
		logger.Warn("read dequeued generation task failed", zap.String("task_id", taskID), zap.Error(err))
		return
	}
	if task.Status == GenerationTaskStatusCancelled {
		return
	}
	task.Status = GenerationTaskStatusFailed
	task.Error = cause.Error()
	task.UpdatedAt = time.Now().Unix()
	if err := s.store.Save(ctx, task); err != nil {
		logger.Warn("mark dequeued generation task failed", zap.String("task_id", taskID), zap.Error(err))
		return
	}
	s.publishTask(task)
}

// run 执行单个任务：标记 running，调用底层生成服务，按结果更新终态。
func (s *generationTaskService) run(taskID string, req *GenerationRequest) {
	ctx := context.Background()
	task, err := s.store.Get(ctx, taskID)
	if err != nil {
		logger.Warn("read generation task before run failed", zap.String("task_id", taskID), zap.Error(err))
		return
	}
	if task.Status == GenerationTaskStatusCancelled {
		return
	}

	task.Status = GenerationTaskStatusRunning
	task.UpdatedAt = time.Now().Unix()
	// Save 失败时仍推送事件：前端能感知到 running 状态，
	// 避免任务永远停留在 pending（虽然 store 状态可能不一致，但 worker 会继续执行）。
	if saveErr := s.store.Save(ctx, task); saveErr != nil {
		logger.Warn("mark generation task running failed, continue anyway",
			zap.String("task_id", taskID), zap.Error(saveErr))
	}
	s.publishTask(task)
	logger.Info("generation task started",
		zap.String("task_id", task.TaskID),
		zap.Uint("user_id", task.UserID),
		zap.Uint("notebook_id", task.NotebookID),
		zap.String("type", string(task.Type)),
	)

	// 加超时控制：避免单个 LLM 调用挂起导致 worker 永久阻塞、后续任务无法执行。
	// 用户主动取消时 cancel() 会立即触发，不受超时影响。
	taskCtx, cancel := context.WithTimeout(context.Background(), generationTaskMaxRunTime)
	s.cancelers.Store(taskID, cancel)
	task, err = s.store.Get(ctx, taskID)
	if err != nil {
		cancel()
		s.cancelers.Delete(taskID)
		logger.Warn("read generation task after registering canceler failed", zap.String("task_id", taskID), zap.Error(err))
		return
	}
	if task.Status == GenerationTaskStatusCancelled {
		cancel()
		s.cancelers.Delete(taskID)
		return
	}
	resp, genErr := s.base.Generate(taskCtx, req)
	wasCancelled := taskCtx.Err() != nil
	cancel()
	s.cancelers.Delete(taskID)

	task, err = s.store.Get(ctx, taskID)
	if err != nil {
		logger.Warn("read generation task after run failed", zap.String("task_id", taskID), zap.Error(err))
		return
	}
	if task.Status == GenerationTaskStatusCancelled {
		return
	}

	task.UpdatedAt = time.Now().Unix()
	switch {
	case errors.Is(genErr, context.DeadlineExceeded):
		task.Status = GenerationTaskStatusFailed
		task.Error = "生成超时，请重试"
	case errors.Is(genErr, context.Canceled) || wasCancelled:
		task.Status = GenerationTaskStatusCancelled
		task.Error = "任务已取消"
	case genErr != nil:
		task.Status = GenerationTaskStatusFailed
		task.Error = genErr.Error()
	default:
		task.Status = GenerationTaskStatusCompleted
		task.Result = resp
	}
	// Save 失败时仍推送事件：前端能感知到最终状态（completed/failed/cancelled），
	// 避免任务永远停留在 running。即使 store 状态不一致，前端也有足够信息更新 UI。
	if saveErr := s.store.Save(ctx, task); saveErr != nil {
		logger.Warn("save generation task result failed, publish anyway",
			zap.String("task_id", taskID), zap.Error(saveErr))
	}
	s.publishTask(task)
	logger.Info("generation task finished",
		zap.String("task_id", task.TaskID),
		zap.Uint("user_id", task.UserID),
		zap.Uint("notebook_id", task.NotebookID),
		zap.String("type", string(task.Type)),
		zap.String("status", string(task.Status)),
		zap.String("error", task.Error),
	)
}

// SubscribeTasks 订阅指定用户和笔记本的任务状态变更事件。
func (s *generationTaskService) SubscribeTasks(ctx context.Context, userID, notebookID uint) (<-chan GenerationTaskEvent, func(), error) {
	if userID == 0 {
		return nil, nil, bizerrors.New(bizerrors.CodeUnauthorized, "user is not authenticated")
	}
	ch, unsubscribe := s.events.subscribe(userID, notebookID)
	if ctx != nil {
		go func() {
			<-ctx.Done()
			unsubscribe()
		}()
	}
	return ch, unsubscribe, nil
}

// publishTask 克隆任务并向事件中心推送任务事件。
func (s *generationTaskService) publishTask(task *GenerationTask) {
	if task == nil || s.events == nil {
		return
	}
	s.events.publish(GenerationTaskEvent{
		Event: GenerationTaskEventTask,
		Task:  cloneGenerationTask(task),
	})
}
