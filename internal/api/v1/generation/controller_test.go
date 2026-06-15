package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/service"

	"github.com/gin-gonic/gin"
)

type fakeGenerationService struct {
	req *service.GenerationRequest
	out *service.GenerationResponse
	err error
}

func (f *fakeGenerationService) Generate(ctx context.Context, req *service.GenerationRequest) (*service.GenerationResponse, error) {
	f.req = req
	return f.out, f.err
}

func TestControllerGenerateBindsRequestAndUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fakeSvc := &fakeGenerationService{out: &service.GenerationResponse{
		Type:    service.GenerationTypeMindmap,
		Content: "# Topic",
	}}
	ctrl := NewController(fakeSvc)

	body := []byte(`{"markdown":"# Topic","type":"mindmap","prompt":"整理","source_ids":[3],"use_web":true,"allow_degrade":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set(middleware.ContextUserID, uint(42))

	ctrl.Generate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if fakeSvc.req == nil {
		t.Fatalf("service was not called")
	}
	if fakeSvc.req.UserID != 42 || fakeSvc.req.Type != service.GenerationTypeMindmap || fakeSvc.req.SourceIDs[0] != 3 || !fakeSvc.req.UseWeb || !fakeSvc.req.AllowDegrade {
		t.Fatalf("request not mapped correctly: %#v", fakeSvc.req)
	}

	var payload struct {
		Code int `json:"code"`
		Data struct {
			Type    service.GenerationType `json:"type"`
			Content string                 `json:"content"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if payload.Code != 0 || payload.Data.Content != "# Topic" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}
