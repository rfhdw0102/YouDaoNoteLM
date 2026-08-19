package memory

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	core "YoudaoNoteLm/internal/memory"
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/pkg/response"

	"github.com/gin-gonic/gin"
)

type fakeService struct {
	listUserID   uint
	upsertUserID uint
	upsertType   core.Type
	upsertBody   string
	deleteUserID uint
	deleteType   core.Type
	preferences  []core.Preference
}

func (s *fakeService) List(_ context.Context, userID uint) ([]core.Preference, error) {
	s.listUserID = userID
	return s.preferences, nil
}

func (s *fakeService) Upsert(_ context.Context, userID uint, typ core.Type, content string) (core.Preference, error) {
	s.upsertUserID = userID
	s.upsertType = typ
	s.upsertBody = content
	return core.Preference{Type: typ, Content: content}, nil
}

func (s *fakeService) Delete(_ context.Context, userID uint, typ core.Type) error {
	s.deleteUserID = userID
	s.deleteType = typ
	return nil
}

func (s *fakeService) LoadSnapshot(context.Context, uint) (core.Snapshot, error) {
	return core.Snapshot{}, nil
}

func newTestContext(method, target, body string, userID uint) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set(middleware.ContextUserID, userID)
	return ctx, recorder
}

func TestUpsertUsesAuthenticatedUserAndPathType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeService{}
	controller := NewController(svc)
	ctx, recorder := newTestContext("PUT", "/user/memories/output_format", `{"content":"使用表格"}`, 7)
	ctx.Params = gin.Params{{Key: "type", Value: "output_format"}}

	controller.Upsert(ctx)
	if svc.upsertUserID != 7 || svc.upsertType != core.TypeOutputFormat || svc.upsertBody != "使用表格" {
		t.Fatalf("unexpected service call: %+v", svc)
	}
	assertSuccess(t, recorder)
}

func TestListAndDeleteUseAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeService{preferences: []core.Preference{{Type: core.TypeLanguage, Content: "中文"}}}
	controller := NewController(svc)

	listCtx, listRecorder := newTestContext("GET", "/user/memories", "", 5)
	controller.List(listCtx)
	if svc.listUserID != 5 {
		t.Fatalf("list used user %d", svc.listUserID)
	}
	assertSuccess(t, listRecorder)

	deleteCtx, deleteRecorder := newTestContext("DELETE", "/user/memories/language", "", 5)
	deleteCtx.Params = gin.Params{{Key: "type", Value: "language"}}
	controller.Delete(deleteCtx)
	if svc.deleteUserID != 5 || svc.deleteType != core.TypeLanguage {
		t.Fatalf("delete used wrong identity: %+v", svc)
	}
	assertSuccess(t, deleteRecorder)
}

func assertSuccess(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	var body response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != 0 {
		t.Fatalf("expected success, got %+v", body)
	}
}
