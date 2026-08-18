package service

import (
	"YoudaoNoteLm/pkg/cache"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

const notionOAuthStatePrefix = "notion:oauth:state:"

type notionOAuthStateStore struct {
	cache *cache.Cache
}

func NewNotionOAuthStateStore(redisCache *cache.Cache) OAuthStateStore {
	return &notionOAuthStateStore{cache: redisCache}
}

func (s *notionOAuthStateStore) Save(ctx context.Context, state string, userID uint, ttl time.Duration) error {
	if s == nil || s.cache == nil {
		return fmt.Errorf("notion oauth state store unavailable")
	}
	return s.cache.Set(ctx, notionOAuthStatePrefix+state, struct {
		UserID uint `json:"user_id"`
	}{UserID: userID}, ttl)
}

// Consume uses Redis GETDEL through Cache.GetDelete, so a state cannot be replayed.
func (s *notionOAuthStateStore) Consume(ctx context.Context, state string) (uint, error) {
	if s == nil || s.cache == nil {
		return 0, fmt.Errorf("notion oauth state store unavailable")
	}
	data, err := s.cache.GetDelete(ctx, notionOAuthStatePrefix+state)
	if err != nil {
		return 0, err
	}
	var value struct {
		UserID uint `json:"user_id"`
	}
	if err := json.Unmarshal(data, &value); err != nil || value.UserID == 0 {
		return 0, fmt.Errorf("invalid notion oauth state")
	}
	return value.UserID, nil
}

// notionImportTaskStore keeps local cancellation hooks separate from Redis state.
// Redis remains the task-status source, while the map only covers this process.
type notionImportTaskStore struct {
	cache       *cache.ImportTaskCache
	cancelFuncs sync.Map // taskID -> context.CancelFunc
}

func NewNotionImportTaskStore(taskCache *cache.ImportTaskCache) ImportTaskStore {
	return &notionImportTaskStore{cache: taskCache}
}

func (s *notionImportTaskStore) Save(ctx context.Context, task *cache.ImportTask) error {
	if s == nil || s.cache == nil {
		return fmt.Errorf("notion import task store unavailable")
	}
	return s.cache.Save(ctx, task)
}

func (s *notionImportTaskStore) GetForUser(ctx context.Context, userID uint, taskID string) (*cache.ImportTask, error) {
	if s == nil || s.cache == nil {
		return nil, fmt.Errorf("notion import task store unavailable")
	}
	task, err := s.cache.Get(ctx, taskID)
	if err != nil || task == nil || task.UserID != userID {
		return nil, ErrNotFound
	}
	return task, nil
}

func (s *notionImportTaskStore) CancelForUser(ctx context.Context, userID uint, taskID string) (*cache.ImportTask, error) {
	task, err := s.GetForUser(ctx, userID, taskID)
	if err != nil {
		return nil, err
	}
	if task.Status != "completed" && task.Status != "failed" && task.Status != "partial_failed" && task.Status != "cancelled" {
		task.Status = "cancelled"
		if err := s.Save(ctx, task); err != nil {
			return nil, err
		}
	}
	if value, ok := s.cancelFuncs.Load(taskID); ok {
		value.(context.CancelFunc)()
	}
	return task, nil
}

func (s *notionImportTaskStore) RegisterCancel(taskID string, cancel context.CancelFunc) {
	s.cancelFuncs.Store(taskID, cancel)
}
func (s *notionImportTaskStore) ClearCancel(taskID string) { s.cancelFuncs.Delete(taskID) }
func (s *notionImportTaskStore) Delete(ctx context.Context, taskID string) error {
	return s.cache.Delete(ctx, taskID)
}
