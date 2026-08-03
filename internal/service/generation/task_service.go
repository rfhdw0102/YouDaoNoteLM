// task_service.go 实现异步生成任务服务。
//
// generationTaskService 负责任务提交、状态流转、worker 循环：
//   - Submit：创建 pending 任务并入队
//   - worker：单线程串行 dequeue 任务，标记 running，调用 GenerationService.Generate，
//     根据结果标记 completed/failed/cancelled
//   - CancelTask：取消正在执行的任务
//
// 关键设计：
//   - worker 单线程串行执行，避免并发请求压垮 LLM 服务
//   - 每个任务有 generationTaskMaxRunTime 超时（10 分钟），防止 LLM 挂起导致全队阻塞
//   - 前端通过 GET /generations/tasks 轮询任务状态
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
// 后续排队任务无法执行。前端通过轮询 ListTasks/GetTask 感知失败状态。
const generationTaskMaxRunTime = 10 * time.Minute

type generationTaskService struct {
	base      GenerationService
	store     GenerationTaskStore
	queue     GenerationTaskQueue
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
		base:  base,
		store: store,
		queue: queue,
	}
	go svc.worker()
	return svc
}

// Submit 创建 pending 任务并入队。
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
	logger.Info("generation task submitted",
		zap.String("task_id", task.TaskID),
		zap.Uint("user_id", task.UserID),
		zap.Uint("notebook_id", task.NotebookID),
		zap.String("type", string(task.Type)),
		zap.Int64("sequence", task.Sequence),
	)

	//存一份备份 防止数据被污染
	reqCopy := *req
	reqCopy.SourceIDs = append([]uint(nil), req.SourceIDs...)
	if req.Options != nil {
		reqCopy.Options = make(map[string]any, len(req.Options))
		for key, value := range req.Options {
			reqCopy.Options[key] = value
		}
	}

	//入redis队列/内存队列
	if err := s.queue.Enqueue(ctx, queuedGenerationTask{taskID: task.TaskID, req: &reqCopy}); err != nil {
		task.Status = GenerationTaskStatusFailed
		task.Error = "generation task enqueue failed"
		task.UpdatedAt = time.Now().Unix()
		if saveErr := s.store.Save(ctx, task); saveErr != nil {
			logger.Warn("mark generation task enqueue failed", zap.String("task_id", task.TaskID), zap.Error(saveErr))
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

// DeleteTask 删除任务：pending/running 状态先取消 worker，再删除持久化数据。
// 已终态任务直接删除。删除幂等：任务不存在视为成功。
func (s *generationTaskService) DeleteTask(ctx context.Context, userID uint, taskID string) error {
	if taskID == "" {
		return bizerrors.New(bizerrors.CodeInvalidParam, "task id cannot be empty")
	}
	task, err := s.store.Get(ctx, taskID)
	if err != nil {
		// 任务不存在视为已删除，幂等成功；其他错误仍尝试删除以避免残留。
		var bizErr *bizerrors.BizError
		if errors.As(err, &bizErr) && bizErr.Code == bizerrors.CodeResourceNotFound {
			return nil
		}
	}
	if task != nil && task.UserID != userID {
		return bizerrors.New(bizerrors.CodeForbidden, "generation task does not belong to current user")
	}
	// 活跃任务先取消 worker，避免删除后 worker 仍尝试写回结果。
	if task != nil && (task.Status == GenerationTaskStatusPending || task.Status == GenerationTaskStatusRunning) {
		if cancel, ok := s.cancelers.Load(taskID); ok {
			cancel.(context.CancelFunc)()
		}
	}
	if err := s.store.Delete(ctx, taskID); err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "delete generation task failed", err)
	}
	prevStatus := ""
	if task != nil {
		prevStatus = string(task.Status)
	}
	logger.Info("generation task deleted",
		zap.String("task_id", taskID),
		zap.Uint("user_id", userID),
		zap.String("previous_status", prevStatus),
	)
	return nil
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

// failDequeuedTask 将出队失败的任务标记为 failed。
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
	if saveErr := s.store.Save(ctx, task); saveErr != nil {
		logger.Warn("mark generation task running failed, continue anyway",
			zap.String("task_id", taskID), zap.Error(saveErr))
	}
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
	if saveErr := s.store.Save(ctx, task); saveErr != nil {
		logger.Warn("save generation task result failed",
			zap.String("task_id", taskID), zap.Error(saveErr))
	}
	logger.Info("generation task finished",
		zap.String("task_id", task.TaskID),
		zap.Uint("user_id", task.UserID),
		zap.Uint("notebook_id", task.NotebookID),
		zap.String("type", string(task.Type)),
		zap.String("status", string(task.Status)),
		zap.String("error", task.Error),
	)
}
