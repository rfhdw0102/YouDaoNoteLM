package generation

import (
	"context"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type fakeGenerationTaskService struct {
	mu     sync.Mutex
	calls  []string
	events chan service.GenerationTaskEvent
}

func (s *fakeGenerationTaskService) Submit(context.Context, *service.GenerationRequest) (*service.GenerationTask, error) {
	return nil, nil
}

func (s *fakeGenerationTaskService) GetTask(context.Context, uint, string) (*service.GenerationTask, error) {
	return nil, nil
}

func (s *fakeGenerationTaskService) ListTasks(context.Context, uint, uint, int) ([]*service.GenerationTask, error) {
	s.record("list")
	return []*service.GenerationTask{}, nil
}

func (s *fakeGenerationTaskService) CancelTask(context.Context, uint, string) error {
	return nil
}

func (s *fakeGenerationTaskService) SubscribeTasks(context.Context, uint, uint) (<-chan service.GenerationTaskEvent, func(), error) {
	s.record("subscribe")
	if s.events == nil {
		s.events = make(chan service.GenerationTaskEvent)
	}
	return s.events, func() {}, nil
}

func (s *fakeGenerationTaskService) record(call string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, call)
}

func (s *fakeGenerationTaskService) firstCall() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) == 0 {
		return ""
	}
	return s.calls[0]
}

func TestWatchTasksSubscribesBeforeSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	taskSvc := &fakeGenerationTaskService{}
	ctrl := NewController(nil, taskSvc)
	router := gin.New()
	router.GET("/ws", func(c *gin.Context) {
		c.Set(middleware.ContextUserID, uint(42))
		ctrl.WatchTasks(c)
	})
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?notebook_id=10"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	defer conn.Close()

	var snapshot map[string]any
	if err := conn.ReadJSON(&snapshot); err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}
	if snapshot["event"] != "snapshot" {
		t.Fatalf("expected snapshot event, got %#v", snapshot)
	}
	if first := taskSvc.firstCall(); first != "subscribe" {
		t.Fatalf("expected SubscribeTasks before ListTasks, first call was %q", first)
	}
}
