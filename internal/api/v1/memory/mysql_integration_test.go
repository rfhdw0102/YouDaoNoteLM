//go:build integration

package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	core "YoudaoNoteLm/internal/memory"
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/pkg/config"
	"YoudaoNoteLm/pkg/database"
	jwtpkg "YoudaoNoteLm/pkg/jwt"

	"github.com/gin-gonic/gin"
)

type allowlistBlacklist struct{}

func (allowlistBlacklist) RevokeToken(context.Context, string) error { return nil }

func (allowlistBlacklist) IsRevoked(context.Context, string) (bool, error) { return false, nil }

func (allowlistBlacklist) AddUserToken(context.Context, uint, string, time.Duration) error {
	return nil
}

func (allowlistBlacklist) RemoveUserToken(context.Context, uint, string) error { return nil }

func (allowlistBlacklist) RevokeUserTokens(context.Context, uint, time.Duration) (int, error) {
	return 0, nil
}

var _ service.TokenBlacklistService = allowlistBlacklist{}

type integrationResponse struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
}

func TestLongTermMemoryHTTPMySQLIntegration(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "..", "..", "..", "configs", "config.yaml"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.App.Mode = "release"
	db, err := database.InitMySQL(&cfg.Database.MySQL)
	if err != nil {
		t.Fatalf("connect mysql: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get mysql connection: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.AutoMigrate(&entity.User{}, &core.UserMemory{}); err != nil {
		t.Fatalf("migrate tables: %v", err)
	}
	if !db.Migrator().HasConstraint(&core.UserMemory{}, "User") {
		if err := db.Migrator().CreateConstraint(&core.UserMemory{}, "User"); err != nil {
			t.Fatalf("create user memory foreign key: %v", err)
		}
	}

	stamp := time.Now().UnixNano()
	user := entity.User{
		Username: fmt.Sprintf("memory-integration-%d", stamp),
		Password: "integration-only",
		Email:    fmt.Sprintf("memory-integration-%d@example.invalid", stamp),
		Nickname: "memory integration",
		Status:   1,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create integration user: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Unscoped().Delete(&entity.User{}, user.ID).Error; err != nil {
			t.Errorf("remove integration user: %v", err)
		}
	})

	token, err := jwtpkg.GenerateAccessToken(user.ID, user.Username)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	controller := NewController(core.NewService(core.NewMySQLStore(db)))
	api := engine.Group("/api/v1")
	controller.RegisterRoutes(api, allowlistBlacklist{}, middleware.StatusCheck(repository.NewUserRepository(db)))
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)

	contents := []string{
		"中文",
		"English",
		"中文",
		"English",
		"中文",
		"English",
		"中文",
		"English",
		"中文",
		"English",
	}
	for _, content := range contents {
		putMemory(t, server.URL, token, "language", content)
	}

	var count int64
	if err := db.Model(&core.UserMemory{}).Where("user_id = ? AND memory_type = ?", user.ID, core.TypeLanguage).Count(&count).Error; err != nil {
		t.Fatalf("count stored preferences: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one upserted preference, got %d", count)
	}

	memories := getMemories(t, server.URL, token)
	if len(memories) != 1 || memories[0].Content != contents[len(contents)-1] {
		t.Fatalf("unexpected listed preferences: %+v", memories)
	}

	request, err := http.NewRequest(http.MethodDelete, server.URL+"/api/v1/user/memories/language", nil)
	if err != nil {
		t.Fatalf("create delete request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("delete preference: %v", err)
	}
	defer response.Body.Close()
	assertIntegrationSuccess(t, response)

	memories = getMemories(t, server.URL, token)
	if len(memories) != 0 {
		t.Fatalf("expected delete to remove the preference, got %+v", memories)
	}
}

func putMemory(t *testing.T, baseURL, token, typ, content string) {
	t.Helper()
	body, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		t.Fatalf("marshal preference: %v", err)
	}
	request, err := http.NewRequest(http.MethodPut, baseURL+"/api/v1/user/memories/"+typ, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create put request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("put preference: %v", err)
	}
	defer response.Body.Close()
	assertIntegrationSuccess(t, response)
}

func getMemories(t *testing.T, baseURL, token string) []core.Preference {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/user/memories", nil)
	if err != nil {
		t.Fatalf("create list request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("list preferences: %v", err)
	}
	defer response.Body.Close()

	var payload integrationResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if response.StatusCode != http.StatusOK || payload.Code != 0 {
		t.Fatalf("list response failed: status=%d code=%d", response.StatusCode, payload.Code)
	}
	var preferences []core.Preference
	if err := json.Unmarshal(payload.Data, &preferences); err != nil {
		t.Fatalf("decode preferences: %v", err)
	}
	return preferences
}

func assertIntegrationSuccess(t *testing.T, response *http.Response) {
	t.Helper()
	var payload integrationResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.StatusCode != http.StatusOK || payload.Code != 0 {
		t.Fatalf("request failed: status=%d code=%d", response.StatusCode, payload.Code)
	}
}
