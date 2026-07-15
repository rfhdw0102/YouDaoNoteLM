// generation_task_service_test.go 测试异步生成任务服务。
//
// 覆盖任务提交、状态流转、事件推送、串行执行、取消、失败等场景。
package generation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestGenerationTaskSubmitUsesConfiguredQueue(t *testing.T) {
	store := newGenerationTaskMemoryStore()
	queue := newSubmitOnlyGenerationTaskQueue()
	svc := NewGenerationTaskServiceWithQueue(&fakeGenerationService{}, store, queue)

	submitted, err := svc.Submit(context.Background(), &GenerationRequest{
		UserID:     42,
		NotebookID: 10,
		Markdown:   "# Source",
		Type:       GenerationTypeQuiz,
		SourceIDs:  []uint{1, 2},
		Options:    map[string]any{"difficulty": "hard"},
	})
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}

	select {
	case item := <-queue.enqueued:
		if item.taskID != submitted.TaskID {
			t.Fatalf("expected queued task id %q, got %q", submitted.TaskID, item.taskID)
		}
		if item.req == nil || item.req.Type != GenerationTypeQuiz || item.req.NotebookID != 10 {
			t.Fatalf("unexpected queued request: %#v", item.req)
		}
		if len(item.req.SourceIDs) != 2 || item.req.SourceIDs[0] != 1 || item.req.SourceIDs[1] != 2 {
			t.Fatalf("expected copied source ids, got %#v", item.req.SourceIDs)
		}
	case <-time.After(time.Second):
		t.Fatal("task was not enqueued")
	}
}

func TestGenerationTaskServicePublishesTaskEvents(t *testing.T) {
	store := newGenerationTaskMemoryStore()
	svc := NewGenerationTaskService(&fakeGenerationService{resp: &GenerationResponse{
		Type:    GenerationTypeNote,
		Content: "# Event result",
	}}, store)

	events, unsubscribe, err := svc.SubscribeTasks(context.Background(), 42, 10)
	if err != nil {
		t.Fatalf("SubscribeTasks returned error: %v", err)
	}
	defer unsubscribe()

	submitted, err := svc.Submit(context.Background(), &GenerationRequest{
		UserID:     42,
		NotebookID: 10,
		Markdown:   "# Source",
		Type:       GenerationTypeNote,
	})
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}

	pending := waitForGenerationTaskEvent(t, events, submitted.TaskID, GenerationTaskStatusPending)
	if pending.Event != GenerationTaskEventTask {
		t.Fatalf("expected task event, got %q", pending.Event)
	}
	waitForGenerationTaskEvent(t, events, submitted.TaskID, GenerationTaskStatusRunning)
	completed := waitForGenerationTaskEvent(t, events, submitted.TaskID, GenerationTaskStatusCompleted)
	if completed.Task.Result == nil || completed.Task.Result.Content != "# Event result" {
		t.Fatalf("expected completed event result, got %#v", completed.Task.Result)
	}
}

func TestGenerationTaskServiceRunsSubmittedTasksSequentially(t *testing.T) {
	store := newGenerationTaskMemoryStore()
	base := &blockingGenerationService{
		started:   make(chan GenerationType, 2),
		release:   make(chan struct{}),
		completed: make(chan GenerationType, 2),
	}
	var releaseOnce sync.Once
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(base.release) })
	})

	svc := NewGenerationTaskService(base, store)

	first, err := svc.Submit(context.Background(), &GenerationRequest{
		UserID:   42,
		Markdown: "# Source",
		Type:     GenerationTypeNote,
	})
	if err != nil {
		t.Fatalf("Submit first returned error: %v", err)
	}
	second, err := svc.Submit(context.Background(), &GenerationRequest{
		UserID:   42,
		Markdown: "# Source",
		Type:     GenerationTypeQuiz,
	})
	if err != nil {
		t.Fatalf("Submit second returned error: %v", err)
	}

	select {
	case startedType := <-base.started:
		if startedType != GenerationTypeNote {
			t.Fatalf("expected first task to start first, got %q", startedType)
		}
	case <-time.After(time.Second):
		t.Fatal("first task did not start")
	}

	firstTask := waitForGenerationTaskStatus(t, svc, 42, first.TaskID, GenerationTaskStatusRunning)
	if firstTask.Status != GenerationTaskStatusRunning {
		t.Fatalf("expected first task to be running, got %q", firstTask.Status)
	}

	secondTask, err := svc.GetTask(context.Background(), 42, second.TaskID)
	if err != nil {
		t.Fatalf("GetTask second returned error: %v", err)
	}
	if secondTask.Status != GenerationTaskStatusPending {
		t.Fatalf("expected second task to remain pending before first completes, got %q", secondTask.Status)
	}

	select {
	case startedType := <-base.started:
		t.Fatalf("second task started before first completed: %q", startedType)
	case <-time.After(50 * time.Millisecond):
	}

	releaseOnce.Do(func() { close(base.release) })

	waitForGenerationTaskStatus(t, svc, 42, first.TaskID, GenerationTaskStatusCompleted)
	waitForGenerationTaskStatus(t, svc, 42, second.TaskID, GenerationTaskStatusCompleted)
}

func TestGenerationTaskSubmitCompletesWithResult(t *testing.T) {
	store := newGenerationTaskMemoryStore()
	base := &fakeGenerationService{resp: &GenerationResponse{
		Type:    GenerationTypeMindmap,
		Content: "# Async result",
	}}
	svc := NewGenerationTaskService(base, store)

	submitted, err := svc.Submit(context.Background(), &GenerationRequest{
		UserID:   42,
		Markdown: "# Source",
		Type:     GenerationTypeMindmap,
	})
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if submitted.TaskID == "" {
		t.Fatal("expected task id")
	}
	if submitted.Status != GenerationTaskStatusPending && submitted.Status != GenerationTaskStatusRunning {
		t.Fatalf("expected initial pending/running status, got %q", submitted.Status)
	}

	task := waitForGenerationTaskStatus(t, svc, 42, submitted.TaskID, GenerationTaskStatusCompleted)
	if task.Result == nil || task.Result.Content != "# Async result" {
		t.Fatalf("expected completed task result, got %#v", task.Result)
	}
	if task.Error != "" {
		t.Fatalf("expected empty error, got %q", task.Error)
	}
}

func TestGenerationTaskSubmitStoresFailure(t *testing.T) {
	store := newGenerationTaskMemoryStore()
	svc := NewGenerationTaskService(&fakeGenerationService{err: errors.New("model unavailable")}, store)

	submitted, err := svc.Submit(context.Background(), &GenerationRequest{
		UserID:   7,
		Markdown: "# Source",
		Type:     GenerationTypeQuiz,
	})
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}

	task := waitForGenerationTaskStatus(t, svc, 7, submitted.TaskID, GenerationTaskStatusFailed)
	if task.Error != "model unavailable" {
		t.Fatalf("expected failure error to be persisted, got %q", task.Error)
	}
	if task.Result != nil {
		t.Fatalf("expected no result on failure, got %#v", task.Result)
	}
}

func TestGenerationTaskStatusRejectsOtherUsers(t *testing.T) {
	store := newGenerationTaskMemoryStore()
	svc := NewGenerationTaskService(&fakeGenerationService{resp: &GenerationResponse{
		Type:    GenerationTypeNote,
		Content: "# Owned",
	}}, store)

	submitted, err := svc.Submit(context.Background(), &GenerationRequest{
		UserID:   100,
		Markdown: "# Source",
		Type:     GenerationTypeNote,
	})
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}

	if _, err := svc.GetTask(context.Background(), 101, submitted.TaskID); err == nil {
		t.Fatal("expected ownership error")
	}
}

func TestGenerationTaskServiceListsTasksForUserAndNotebookInQueueOrder(t *testing.T) {
	store := newGenerationTaskMemoryStore()
	base := &blockingGenerationService{
		started:   make(chan GenerationType, 2),
		release:   make(chan struct{}),
		completed: make(chan GenerationType, 2),
	}
	var releaseOnce sync.Once
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(base.release) })
	})

	svc := NewGenerationTaskService(base, store)

	first, err := svc.Submit(context.Background(), &GenerationRequest{
		UserID:     42,
		NotebookID: 10,
		Markdown:   "# Source",
		Type:       GenerationTypeNote,
	})
	if err != nil {
		t.Fatalf("Submit first returned error: %v", err)
	}
	second, err := svc.Submit(context.Background(), &GenerationRequest{
		UserID:     42,
		NotebookID: 10,
		Markdown:   "# Source",
		Type:       GenerationTypeQuiz,
	})
	if err != nil {
		t.Fatalf("Submit second returned error: %v", err)
	}
	_, err = svc.Submit(context.Background(), &GenerationRequest{
		UserID:     43,
		NotebookID: 10,
		Markdown:   "# Other user",
		Type:       GenerationTypePPT,
	})
	if err != nil {
		t.Fatalf("Submit other user returned error: %v", err)
	}

	tasks, err := svc.ListTasks(context.Background(), 42, 10, 20)
	if err != nil {
		t.Fatalf("ListTasks returned error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks for user/notebook, got %d: %#v", len(tasks), tasks)
	}
	if tasks[0].TaskID != first.TaskID || tasks[1].TaskID != second.TaskID {
		t.Fatalf("expected queue order [%s %s], got [%s %s]", first.TaskID, second.TaskID, tasks[0].TaskID, tasks[1].TaskID)
	}
}

func TestGenerationTaskServiceCancelsPendingTaskBeforeExecution(t *testing.T) {
	store := newGenerationTaskMemoryStore()
	base := &blockingGenerationService{
		started:   make(chan GenerationType, 2),
		release:   make(chan struct{}),
		completed: make(chan GenerationType, 2),
	}
	var releaseOnce sync.Once
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(base.release) })
	})

	svc := NewGenerationTaskService(base, store)

	first, err := svc.Submit(context.Background(), &GenerationRequest{
		UserID:   42,
		Markdown: "# Source",
		Type:     GenerationTypeNote,
	})
	if err != nil {
		t.Fatalf("Submit first returned error: %v", err)
	}
	second, err := svc.Submit(context.Background(), &GenerationRequest{
		UserID:   42,
		Markdown: "# Source",
		Type:     GenerationTypeQuiz,
	})
	if err != nil {
		t.Fatalf("Submit second returned error: %v", err)
	}

	waitForGenerationTaskStatus(t, svc, 42, first.TaskID, GenerationTaskStatusRunning)
	if err := svc.CancelTask(context.Background(), 42, second.TaskID); err != nil {
		t.Fatalf("CancelTask returned error: %v", err)
	}
	cancelled := waitForGenerationTaskStatus(t, svc, 42, second.TaskID, GenerationTaskStatusCancelled)
	if cancelled.Error != "任务已取消" {
		t.Fatalf("expected cancellation message, got %q", cancelled.Error)
	}

	releaseOnce.Do(func() { close(base.release) })
	waitForGenerationTaskStatus(t, svc, 42, first.TaskID, GenerationTaskStatusCompleted)

	select {
	case startedType := <-base.started:
		if startedType == GenerationTypeQuiz {
			t.Fatal("cancelled pending task was executed")
		}
	case <-time.After(100 * time.Millisecond):
	}
}

func TestGenerationTaskServiceStopsRunningTask(t *testing.T) {
	store := newGenerationTaskMemoryStore()
	base := &cancellableGenerationService{
		started:   make(chan struct{}),
		cancelled: make(chan struct{}),
	}
	svc := NewGenerationTaskService(base, store)

	submitted, err := svc.Submit(context.Background(), &GenerationRequest{
		UserID:   42,
		Markdown: "# Source",
		Type:     GenerationTypeMindmap,
	})
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}

	select {
	case <-base.started:
	case <-time.After(time.Second):
		t.Fatal("task did not start")
	}

	if err := svc.CancelTask(context.Background(), 42, submitted.TaskID); err != nil {
		t.Fatalf("CancelTask returned error: %v", err)
	}

	select {
	case <-base.cancelled:
	case <-time.After(time.Second):
		t.Fatal("running task context was not cancelled")
	}

	waitForGenerationTaskStatus(t, svc, 42, submitted.TaskID, GenerationTaskStatusCancelled)
}

func waitForGenerationTaskStatus(t *testing.T, svc GenerationTaskService, userID uint, taskID string, status GenerationTaskStatus) *GenerationTask {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		task, err := svc.GetTask(context.Background(), userID, taskID)
		if err != nil {
			t.Fatalf("GetTask returned error: %v", err)
		}
		if task.Status == status {
			return task
		}
		time.Sleep(10 * time.Millisecond)
	}
	task, _ := svc.GetTask(context.Background(), userID, taskID)
	t.Fatalf("task did not reach %q, last task: %#v", status, task)
	return nil
}

func waitForGenerationTaskEvent(t *testing.T, events <-chan GenerationTaskEvent, taskID string, status GenerationTaskStatus) GenerationTaskEvent {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case event := <-events:
			if event.Task != nil && event.Task.TaskID == taskID && event.Task.Status == status {
				return event
			}
		case <-deadline:
			t.Fatalf("task event %s/%s was not published", taskID, status)
		}
	}
}
