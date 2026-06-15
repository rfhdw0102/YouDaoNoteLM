package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"

	// 触发 provider 注册（测试环境也需要）
	_ "YoudaoNoteLm/internal/service/external/asr"
	_ "YoudaoNoteLm/internal/service/external/embedding"
	_ "YoudaoNoteLm/internal/service/external/llm"
	_ "YoudaoNoteLm/internal/service/external/search"
)

// ============================================================
// Mock 实现
// ============================================================

// --- MockCache: 内存缓存 ---

type mockCache struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func newMockCache() *mockCache {
	return &mockCache{data: make(map[string][]byte)}
}

func (c *mockCache) Get(ctx context.Context, key string, dest interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	raw, ok := c.data[key]
	if !ok {
		return errors.New("cache miss")
	}
	return json.Unmarshal(raw, dest)
}

func (c *mockCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.data[key] = raw
	return nil
}

func (c *mockCache) Delete(ctx context.Context, keys ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, key := range keys {
		delete(c.data, key)
	}
	return nil
}

func (c *mockCache) has(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.data[key]
	return ok
}

// --- MockUserConfigRepository ---

type mockUserConfigRepo struct {
	mu      sync.RWMutex
	configs map[uint]*entity.UserConfig // id -> config
	nextID  uint
}

func newMockUserConfigRepo() *mockUserConfigRepo {
	return &mockUserConfigRepo{
		configs: make(map[uint]*entity.UserConfig),
		nextID:  1,
	}
}

func (r *mockUserConfigRepo) FindByUserAndType(userID uint, configType string) (*entity.UserConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, cfg := range r.configs {
		if cfg.UserID == userID && cfg.ConfigType == configType {
			return cfg, nil
		}
	}
	return nil, nil
}

func (r *mockUserConfigRepo) FindByUserAndTypeIncludingDeleted(userID uint, configType string) (*entity.UserConfig, error) {
	// Mock 实现：简单委托给 FindByUserAndType
	return r.FindByUserAndType(userID, configType)
}

func (r *mockUserConfigRepo) FindByID(id uint) (*entity.UserConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cfg, ok := r.configs[id]
	if !ok {
		return nil, nil
	}
	return cfg, nil
}

func (r *mockUserConfigRepo) Create(config *entity.UserConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	config.ID = r.nextID
	r.nextID++
	r.configs[config.ID] = config
	return nil
}

func (r *mockUserConfigRepo) Update(config *entity.UserConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs[config.ID] = config
	return nil
}

func (r *mockUserConfigRepo) Delete(id uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.configs, id)
	return nil
}

// --- MockSysConfigRepository ---

type mockSysConfigRepo struct {
	mu      sync.RWMutex
	configs map[string][]*entity.SysConfig // group -> configs
}

func newMockSysConfigRepo() *mockSysConfigRepo {
	return &mockSysConfigRepo{
		configs: make(map[string][]*entity.SysConfig),
	}
}

func (r *mockSysConfigRepo) FindByGroup(group string) ([]*entity.SysConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.configs[group], nil
}

func (r *mockSysConfigRepo) FindByGroupAndKey(group, key string) (*entity.SysConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, cfg := range r.configs[group] {
		if cfg.ConfigKey == key {
			return cfg, nil
		}
	}
	return nil, nil
}

func (r *mockSysConfigRepo) Create(config *entity.SysConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	group := config.ConfigGroup
	r.configs[group] = append(r.configs[group], config)
	return nil
}

func (r *mockSysConfigRepo) Update(config *entity.SysConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	group := config.ConfigGroup
	for i, cfg := range r.configs[group] {
		if cfg.ID == config.ID {
			r.configs[group][i] = config
			return nil
		}
	}
	return errors.New("not found")
}

func (r *mockSysConfigRepo) Delete(id uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for group, configs := range r.configs {
		for i, cfg := range configs {
			if cfg.ID == id {
				r.configs[group] = append(configs[:i], configs[i+1:]...)
				return nil
			}
		}
	}
	return errors.New("not found")
}

func (r *mockSysConfigRepo) GetConfigStatusSummary() ([]map[string]interface{}, error) {
	return nil, nil
}

// ============================================================
// 辅助函数
// ============================================================

func newTestConfigService(
	userRepo repository.UserConfigRepository,
	sysRepo repository.SysConfigRepository,
	cache CacheStore,
) ConfigService {
	return NewConfigService(sysRepo, userRepo, nil, cache, nil)
}

// ============================================================
// GetSearchEngine 测试
// ============================================================

func TestGetSearchEngine_UserConfig_CacheHit(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	// 预填充缓存
	cfg := entity.UserConfig{
		UserID:      1,
		ConfigType:  "search",
		Provider:    "custom",
		Name:        "MySearch",
		APIURL:      "https://api.example.com/search",
		APIKey:      "key123",
		ExtraConfig: `{"name":"MySearch"}`,
		Enabled:     true,
	}
	cache.Set(context.Background(), "config:user:1:search", cfg, time.Minute)

	engine, err := svc.GetSearchEngine(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.Name() != "MySearch" {
		t.Errorf("expected engine name 'MySearch', got '%s'", engine.Name())
	}
}

func TestGetSearchEngine_UserConfig_DBHit(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	// 写入 DB
	userRepo.Create(&entity.UserConfig{
		UserID:      1,
		ConfigType:  "search",
		Provider:    "custom",
		Name:        "DBSearch",
		APIURL:      "https://api.db.com/search",
		APIKey:      "dbkey",
		ExtraConfig: `{"name":"DBSearch"}`,
		Enabled:     true,
	})

	engine, err := svc.GetSearchEngine(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.Name() != "DBSearch" {
		t.Errorf("expected 'DBSearch', got '%s'", engine.Name())
	}

	// 验证缓存已回填
	if !cache.has("config:user:1:search") {
		t.Error("expected cache to be populated after DB hit")
	}
}

func TestGetSearchEngine_UserConfig_Disabled(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	// 用户配置已禁用
	userRepo.Create(&entity.UserConfig{
		UserID:     1,
		ConfigType: "search",
		Name:       "DisabledSearch",
		Enabled:    false,
	})

	// 无系统配置且无兜底 provider → 应返回错误
	_, err := svc.GetSearchEngine(1)
	if err == nil {
		t.Fatal("expected error when no config and no default provider")
	}
}

func TestGetSearchEngine_NoConfig_ReturnsError(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	// 无任何配置且无兜底 provider → 返回错误
	_, err := svc.GetSearchEngine(999)
	if err == nil {
		t.Fatal("expected error when no config and no default provider")
	}
}

// ============================================================
// GetASRService 测试
// ============================================================

func TestGetASRService_UserConfig_CacheHit(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	cfg := entity.UserConfig{
		UserID:      1,
		ConfigType:  "asr",
		Provider:    "aliyun_nls",
		APIKey:      "access_key_id",
		ExtraConfig: `{"access_key_id":"key","access_key_secret":"secret","app_key":"app"}`,
		Enabled:     true,
	}
	cache.Set(context.Background(), "config:user:1:asr", cfg, time.Minute)

	asrSvc, err := svc.GetASRService(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if asrSvc == nil {
		t.Fatal("expected non-nil ASRService")
	}
}

func TestGetASRService_UserConfig_DBHit(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	userRepo.Create(&entity.UserConfig{
		UserID:      1,
		ConfigType:  "asr",
		Provider:    "aliyun_nls",
		APIKey:      "key",
		ExtraConfig: `{"access_key_id":"key","access_key_secret":"secret","app_key":"app"}`,
		Enabled:     true,
	})

	asrSvc, err := svc.GetASRService(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if asrSvc == nil {
		t.Fatal("expected non-nil ASRService")
	}

	// 验证缓存已回填
	if !cache.has("config:user:1:asr") {
		t.Error("expected cache to be populated after DB hit")
	}
}

func TestGetASRService_NoConfig_ReturnsError(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	_, err := svc.GetASRService(999)
	if err == nil {
		t.Fatal("expected error when no ASR config")
	}
	t.Logf("正确返回错误: %v", err)
}

func TestGetASRService_UnsupportedProvider_ReturnsError(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	userRepo.Create(&entity.UserConfig{
		UserID:     1,
		ConfigType: "asr",
		Provider:   "unknown_provider",
		APIKey:     "key",
		Enabled:    true,
	})

	_, err := svc.GetASRService(1)
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
	t.Logf("正确返回错误: %v", err)
}

// ============================================================
// GetEmbeddingService 测试
// ============================================================

func TestGetEmbeddingService_NoConfig_ReturnsError(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	_, err := svc.GetEmbeddingService(999)
	if err == nil {
		t.Fatal("expected error when no Embedding config")
	}
	t.Logf("正确返回错误: %v", err)
}

// ============================================================
// UpdateUserConfig / DeleteUserConfig 测试
// ============================================================

func TestUpdateUserConfig_ClearsCache(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	// 预填充缓存
	cfg := entity.UserConfig{
		BaseEntity: entity.BaseEntity{ID: 1},
		UserID:     1,
		ConfigType: "search",
		Name:       "Old",
		Enabled:    true,
	}
	cache.Set(context.Background(), "config:user:1:search", cfg, time.Minute)

	if !cache.has("config:user:1:search") {
		t.Fatal("cache should have key before update")
	}

	// 更新
	cfg.Name = "New"
	err := svc.UpdateUserConfig(&cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 缓存应被清除
	if cache.has("config:user:1:search") {
		t.Error("cache should be cleared after update")
	}
}

func TestDeleteUserConfig_ClearsCache(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	// 创建配置
	userRepo.Create(&entity.UserConfig{
		UserID:     1,
		ConfigType: "search",
		Name:       "ToDelete",
		Enabled:    true,
	})

	// 预填充缓存
	cache.Set(context.Background(), "config:user:1:search", "dummy", time.Minute)

	err := svc.DeleteUserConfig(1, "search")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 缓存应被清除
	if cache.has("config:user:1:search") {
		t.Error("cache should be cleared after delete")
	}

	// DB 中也应不存在
	cfg, _ := userRepo.FindByUserAndType(1, "search")
	if cfg != nil {
		t.Error("config should be deleted from DB")
	}
}

func TestDeleteUserConfig_NotExists_NoError(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	// 删除不存在的配置不应报错
	err := svc.DeleteUserConfig(999, "nonexistent")
	if err != nil {
		t.Fatalf("expected no error for deleting non-existent config, got: %v", err)
	}
}

// ============================================================
// ClearUserConfigCache 测试
// ============================================================

func TestClearUserConfigCache(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	cache.Set(context.Background(), "config:user:1:search", "dummy", time.Minute)
	if !cache.has("config:user:1:search") {
		t.Fatal("cache should have key")
	}

	svc.ClearUserConfigCache(1, "search")

	if cache.has("config:user:1:search") {
		t.Error("cache should be cleared")
	}
}

// ============================================================
// 缓存 Key 生成函数测试
// ============================================================

func TestCacheKeyGeneration(t *testing.T) {
	if key := userConfigCacheKey(42, "search"); key != "config:user:42:search" {
		t.Errorf("unexpected user cache key: %s", key)
	}
	if key := sysConfigCacheKey("asr"); key != "config:sys:asr" {
		t.Errorf("unexpected sys cache key: %s", key)
	}
}

// ============================================================
// 缓存策略验证：TTL / 回填 / 写入失效
// ============================================================

func TestCachePolicy_WriteInvalidation(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	// 1. 写入 DB + 缓存
	userRepo.Create(&entity.UserConfig{
		UserID:      1,
		ConfigType:  "search",
		Provider:    "custom",
		Name:        "V1",
		ExtraConfig: `{"name":"V1"}`,
		Enabled:     true,
	})
	svc.GetSearchEngine(1) // 触发缓存回填
	if !cache.has("config:user:1:search") {
		t.Fatal("cache should be populated after GetSearchEngine")
	}

	// 2. 更新 → 缓存失效
	cfg, _ := userRepo.FindByUserAndType(1, "search")
	cfg.Name = "V2"
	cfg.ExtraConfig = `{"name":"V2"}`
	svc.UpdateUserConfig(cfg)
	if cache.has("config:user:1:search") {
		t.Error("cache should be invalidated after update")
	}

	// 3. 再次查询 → 从 DB 读到新值
	engine, _ := svc.GetSearchEngine(1)
	if engine.Name() != "V2" {
		t.Errorf("expected 'V2' after update, got '%s'", engine.Name())
	}
}

func TestCachePolicy_CacheThenDB(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	// 写入 DB
	userRepo.Create(&entity.UserConfig{
		UserID:      1,
		ConfigType:  "search",
		Provider:    "custom",
		Name:        "CachedEngine",
		ExtraConfig: `{"name":"CachedEngine"}`,
		Enabled:     true,
	})

	// 第一次查询 → DB → 回填缓存
	engine1, _ := svc.GetSearchEngine(1)
	if engine1.Name() != "CachedEngine" {
		t.Errorf("first call: expected 'CachedEngine', got '%s'", engine1.Name())
	}

	// 修改 DB（不经过 service，不触发缓存清除）
	userRepo.Update(&entity.UserConfig{
		BaseEntity:  entity.BaseEntity{ID: 1},
		UserID:      1,
		ConfigType:  "search",
		Provider:    "custom",
		Name:        "StaleInCache",
		ExtraConfig: `{"name":"StaleInCache"}`,
		Enabled:     true,
	})

	// 第二次查询 → 缓存命中（旧值）
	engine2, _ := svc.GetSearchEngine(1)
	if engine2.Name() != "CachedEngine" {
		t.Errorf("second call should hit cache with 'CachedEngine', got '%s'", engine2.Name())
	}
}

// ============================================================
// sys_config 降级测试（从系统配置创建服务）
// ============================================================

func TestGetSearchEngine_SysConfig_Fallback(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	// 无用户配置，有系统配置
	sysRepo.Create(&entity.SysConfig{
		ConfigGroup: "search",
		ConfigKey:   "custom",
		ConfigValue: `{"provider":"custom","name":"serper","api_url":"https://google.serper.dev/search","api_key":"key123"}`,
		Enabled:     true,
		Description: "Serper 搜索 API",
	})

	engine, err := svc.GetSearchEngine(999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.Name() != "serper" {
		t.Errorf("expected 'serper' from sys_config, got '%s'", engine.Name())
	}
}

func TestGetSearchEngine_SysConfig_CacheHit(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	// 预填充系统配置缓存
	sysConfigs := []*entity.SysConfig{
		{
			ConfigGroup: "search",
			ConfigKey:   "custom",
			ConfigValue: `{"provider":"custom","name":"cached","api_url":"http://cached.com","api_key":"k"}`,
			Enabled:     true,
		},
	}
	cache.Set(context.Background(), "config:sys:search", sysConfigs, time.Minute)

	engine, err := svc.GetSearchEngine(999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.Name() != "cached" {
		t.Errorf("expected 'cached' from cache, got '%s'", engine.Name())
	}
}

func TestGetASRService_SysConfig_Fallback(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	// 无用户配置，有系统 ASR 配置
	sysRepo.Create(&entity.SysConfig{
		ConfigGroup: "asr",
		ConfigKey:   "aliyun_nls",
		ConfigValue: `{"provider":"aliyun_nls","access_key_id":"kid","access_key_secret":"ksecret","app_key":"app"}`,
		Enabled:     true,
		Description: "阿里云 ASR",
	})

	asrSvc, err := svc.GetASRService(999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if asrSvc == nil {
		t.Fatal("expected non-nil ASRService from sys_config")
	}
}

func TestGetSearchEngine_SysConfig_Disabled(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	// 系统配置已禁用
	sysRepo.Create(&entity.SysConfig{
		ConfigGroup: "search",
		ConfigKey:   "disabled_engine",
		ConfigValue: `{"name":"disabled","api_url":"http://disabled.com","api_key":"k"}`,
		Enabled:     false,
	})

	// 无可用配置且无兜底 provider → 返回错误
	_, err := svc.GetSearchEngine(999)
	if err == nil {
		t.Fatal("expected error when sys_config disabled and no default provider")
	}
}

func TestGetSearchEngine_SysConfig_InvalidJSON(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	// 无效 JSON → 跳过该条，无可用配置 → 返回错误
	sysRepo.Create(&entity.SysConfig{
		ConfigGroup: "search",
		ConfigKey:   "bad_json",
		ConfigValue: `not-json`,
		Enabled:     true,
	})

	_, err := svc.GetSearchEngine(999)
	if err == nil {
		t.Fatal("expected error for invalid JSON and no default provider")
	}
}

func TestGetSearchEngine_SysConfig_MissingAPIURL(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	// 缺少 api_url → 跳过，无可用配置 → 返回错误
	sysRepo.Create(&entity.SysConfig{
		ConfigGroup: "search",
		ConfigKey:   "no_url",
		ConfigValue: `{"name":"no_url","api_key":"k"}`,
		Enabled:     true,
	})

	_, err := svc.GetSearchEngine(999)
	if err == nil {
		t.Fatal("expected error for missing api_url and no default provider")
	}
}

// ============================================================
// 完整降级链测试：用户配置 → 系统配置 → 兜底
// ============================================================

func TestGetSearchEngine_FullFallbackChain(t *testing.T) {
	cache := newMockCache()
	userRepo := newMockUserConfigRepo()
	sysRepo := newMockSysConfigRepo()
	svc := newTestConfigService(userRepo, sysRepo, cache)

	// 第一层：无任何配置 → 返回错误
	_, err := svc.GetSearchEngine(1)
	if err == nil {
		t.Fatal("step 1: expected error when no config and no default provider")
	}

	// 第二层：添加系统配置 → sys_config 引擎
	sysRepo.Create(&entity.SysConfig{
		ConfigGroup: "search",
		ConfigKey:   "custom",
		ConfigValue: `{"provider":"custom","name":"sys_engine","api_url":"http://sys.com","api_key":"k"}`,
		Enabled:     true,
	})
	// 清缓存，强制重新查询
	cache.Delete(context.Background(), "config:sys:search")

	engine, err := svc.GetSearchEngine(1)
	if err != nil {
		t.Fatalf("step 2: unexpected error: %v", err)
	}
	if engine.Name() != "sys_engine" {
		t.Fatalf("step 2: expected 'sys_engine', got '%s'", engine.Name())
	}

	// 第三层：添加用户配置 → 用户引擎优先
	userRepo.Create(&entity.UserConfig{
		UserID:      1,
		ConfigType:  "search",
		Provider:    "custom",
		Name:        "user_engine",
		APIURL:      "http://user.com",
		APIKey:      "uk",
		ExtraConfig: `{"name":"user_engine"}`,
		Enabled:     true,
	})

	engine, _ = svc.GetSearchEngine(1)
	if engine.Name() != "user_engine" {
		t.Fatalf("step 3: expected 'user_engine', got '%s'", engine.Name())
	}
}
