package service

import (
	"YoudaoNoteLm/internal/service/external/asr"
	"YoudaoNoteLm/internal/service/external/embedding"
	"YoudaoNoteLm/internal/service/external/search"
	"YoudaoNoteLm/internal/service/external/storage"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/internal/service/external"
	"YoudaoNoteLm/internal/service/external/llm"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

const (
	userConfigTTL = 60 * time.Second // 用户配置缓存 60s
	sysConfigTTL  = 5 * time.Minute  // 系统配置缓存 5min
)

// ChatModelConfig Eino ChatModel 配置
type ChatModelConfig struct {
	Provider string // 服务商
	BaseURL  string // API 地址
	APIKey   string // API 密钥
	Model    string // 模型名称
}

// ConfigService 配置路由服务接口
type ConfigService interface {
	GetSearchEngine(userID uint) (search.SearchEngine, error)
	GetASRService(userID uint) (asr.ASRService, error)
	GetEmbeddingService(userID uint) (embedding.EmbeddingService, error)
	GetLLMClient(userID uint) (llm.LLMClient, error)
	GetChatModelConfig(userID uint) (*ChatModelConfig, error)

	// 获取配置（用于 API）
	GetUserConfig(userID uint, configType string) (*entity.UserConfig, error)
	GetSysConfig(configType string) (*entity.SysConfig, error)

	// 配置管理（带缓存失效）
	UpdateUserConfig(config *entity.UserConfig) error
	DeleteUserConfig(userID uint, configType string) error
	ClearUserConfigCache(userID uint, configType string)
	ClearSysConfigCache(group string)
}

type configService struct {
	sysConfigRepo  repository.SysConfigRepository
	userConfigRepo repository.UserConfigRepository
	llmConfigRepo  repository.UserLLMConfigRepository
	cache          CacheStore
	storage        storage.FileStorage // ASR 需要注入存储服务
	registry       *external.Registry  // Provider 注册表
}

func NewConfigService(
	sysConfigRepo repository.SysConfigRepository,
	userConfigRepo repository.UserConfigRepository,
	llmConfigRepo repository.UserLLMConfigRepository,
	cache CacheStore,
	storage storage.FileStorage,
) ConfigService {
	return &configService{
		sysConfigRepo:  sysConfigRepo,
		userConfigRepo: userConfigRepo,
		llmConfigRepo:  llmConfigRepo,
		cache:          cache,
		storage:        storage,
		registry:       external.GetGlobalRegistry(), // 使用全局 Registry
	}
}

// --- 缓存 Key 生成 ---

func userConfigCacheKey(userID uint, configType string) string {
	return fmt.Sprintf("config:user:%d:%s", userID, configType)
}

func sysConfigCacheKey(group string) string {
	return fmt.Sprintf("config:sys:%s", group)
}

// --- 查询（带缓存） ---

// getService 统一获取服务（用户配置 → sys_config → 兜底）
// serviceType: "search" / "asr" / "llm" / "embedding"
func (s *configService) getService(userID uint, serviceType string) (interface{}, error) {
	ctx := context.Background()

	// 1. 查用户配置（先查缓存）
	cacheKey := userConfigCacheKey(userID, serviceType)
	var userCfg entity.UserConfig
	if err := s.cache.Get(ctx, cacheKey, &userCfg); err == nil && userCfg.Enabled {
		logger.Debug("用户配置缓存命中",
			zap.Uint("user_id", userID),
			zap.String("service_type", serviceType))
		sc := external.NewServiceConfigFromEntity(
			userCfg.Provider, userCfg.APIURL, userCfg.APIKey,
			userCfg.Model, userCfg.ExtraConfig)
		return s.registry.Create(serviceType, userCfg.Provider, sc)
	}

	// 缓存未命中，查 DB
	userCfgPtr, err := s.userConfigRepo.FindByUserAndType(userID, serviceType)
	if err == nil && userCfgPtr != nil && userCfgPtr.Enabled {
		if cacheErr := s.cache.Set(ctx, cacheKey, userCfgPtr, userConfigTTL); cacheErr != nil {
			logger.Warn("缓存用户配置失败", zap.String("key", cacheKey), zap.Error(cacheErr))
		}
		sc := external.NewServiceConfigFromEntity(
			userCfgPtr.Provider, userCfgPtr.APIURL, userCfgPtr.APIKey,
			userCfgPtr.Model, userCfgPtr.ExtraConfig)
		return s.registry.Create(serviceType, userCfgPtr.Provider, sc)
	}

	// 2. 降级到系统内置配置
	if svc := s.getSysService(ctx, serviceType); svc != nil {
		return svc, nil
	}

	return nil, fmt.Errorf("未配置 %s 服务，请在用户配置或系统配置中添加", serviceType)
}

// getSysService 从 sys_config 查找并创建服务
func (s *configService) getSysService(ctx context.Context, serviceType string) interface{} {
	sysCacheKey := sysConfigCacheKey(serviceType)
	builtins, err := s.getSysConfigs(ctx, sysCacheKey, serviceType)
	if err != nil {
		return nil
	}

	for _, builtin := range builtins {
		if !builtin.Enabled {
			continue
		}
		params, err := parseSysConfigValue(builtin.ConfigValue)
		if err != nil {
			logger.Error("解析系统配置失败",
				zap.String("service_type", serviceType),
				zap.String("key", builtin.ConfigKey),
				zap.Error(err))
			continue
		}

		provider := params.Provider
		if provider == "" {
			provider = builtin.ConfigKey // 兼容旧数据
		}

		sc := external.NewServiceConfigFromEntity(
			provider, params.APIURL, params.APIKey, "", "")

		// 也解析完整 JSON 到 ExtraConfig
		var fullParams map[string]interface{}
		if jsonErr := json.Unmarshal([]byte(builtin.ConfigValue), &fullParams); jsonErr == nil {
			sc.ExtraConfig = fullParams
			// 从 ExtraConfig 补充 model
			if sc.Model == "" {
				if v, ok := fullParams["model"].(string); ok {
					sc.Model = v
				}
			}
		}

		svc, err := s.registry.Create(serviceType, provider, sc)
		if err != nil {
			logger.Error("创建系统服务失败",
				zap.String("service_type", serviceType),
				zap.String("key", builtin.ConfigKey),
				zap.Error(err))
			continue
		}

		logger.Info("使用系统内置配置",
			zap.String("service_type", serviceType),
			zap.String("key", builtin.ConfigKey))
		return svc
	}
	return nil
}

func (s *configService) GetSearchEngine(userID uint) (search.SearchEngine, error) {
	svc, err := s.getService(userID, "search")
	if err != nil {
		return nil, err
	}
	engine, ok := svc.(search.SearchEngine)
	if !ok {
		return nil, fmt.Errorf("search provider 返回的类型不正确")
	}
	return engine, nil
}

func (s *configService) GetASRService(userID uint) (asr.ASRService, error) {
	svc, err := s.getService(userID, "asr")
	if err != nil {
		return nil, err
	}
	asrSvc, ok := svc.(asr.ASRService)
	if !ok {
		return nil, fmt.Errorf("asr provider 返回的类型不正确")
	}
	// 注入存储服务
	s.injectStorage(asrSvc)
	return asrSvc, nil
}

func (s *configService) GetEmbeddingService(userID uint) (embedding.EmbeddingService, error) {
	ctx := context.Background()

	// 只查用户配置（先查缓存）
	cacheKey := userConfigCacheKey(userID, "embedding")
	var userCfg entity.UserConfig
	if err := s.cache.Get(ctx, cacheKey, &userCfg); err == nil && userCfg.Enabled {
		sc := external.NewServiceConfigFromEntity(
			userCfg.Provider, userCfg.APIURL, userCfg.APIKey,
			userCfg.Model, userCfg.ExtraConfig)
		svc, err := s.registry.Create("embedding", userCfg.Provider, sc)
		if err != nil {
			return nil, err
		}
		embedSvc, ok := svc.(embedding.EmbeddingService)
		if !ok {
			return nil, fmt.Errorf("embedding provider 返回的类型不正确")
		}
		return embedSvc, nil
	}

	// 缓存未命中，查 DB
	userCfgPtr, err := s.userConfigRepo.FindByUserAndType(userID, "embedding")
	if err != nil {
		return nil, fmt.Errorf("未配置 Embedding 服务，请在设置中添加 Embedding 配置")
	}
	if userCfgPtr == nil || !userCfgPtr.Enabled {
		return nil, fmt.Errorf("未配置 Embedding 服务，请在设置中添加 Embedding 配置")
	}

	if cacheErr := s.cache.Set(ctx, cacheKey, userCfgPtr, userConfigTTL); cacheErr != nil {
		logger.Warn("缓存用户配置失败", zap.String("key", cacheKey), zap.Error(cacheErr))
	}

	sc := external.NewServiceConfigFromEntity(
		userCfgPtr.Provider, userCfgPtr.APIURL, userCfgPtr.APIKey,
		userCfgPtr.Model, userCfgPtr.ExtraConfig)
	svc, err := s.registry.Create("embedding", userCfgPtr.Provider, sc)
	if err != nil {
		return nil, err
	}
	embedSvc, ok := svc.(embedding.EmbeddingService)
	if !ok {
		return nil, fmt.Errorf("embedding provider 返回的类型不正确")
	}
	return embedSvc, nil
}

// --- 配置管理（写入时失效） ---

// UpdateUserConfig 更新用户配置并清除缓存
func (s *configService) UpdateUserConfig(config *entity.UserConfig) error {
	if err := s.userConfigRepo.Update(config); err != nil {
		return err
	}
	s.ClearUserConfigCache(config.UserID, config.ConfigType)
	return nil
}

// DeleteUserConfig 删除用户配置并清除缓存
func (s *configService) DeleteUserConfig(userID uint, configType string) error {
	// 先查到配置 ID
	cfg, err := s.userConfigRepo.FindByUserAndType(userID, configType)
	if err != nil {
		return err
	}
	if cfg == nil {
		return nil // 不存在，无需删除
	}
	if err := s.userConfigRepo.Delete(cfg.ID); err != nil {
		return err
	}
	s.ClearUserConfigCache(userID, configType)
	return nil
}

// ClearUserConfigCache 清除用户配置缓存
func (s *configService) ClearUserConfigCache(userID uint, configType string) {
	key := userConfigCacheKey(userID, configType)
	if err := s.cache.Delete(context.Background(), key); err != nil {
		logger.Warn("清除用户配置缓存失败",
			zap.Uint("user_id", userID),
			zap.String("config_type", configType),
			zap.Error(err),
		)
	}
}

// ClearSysConfigCache 清除系统配置缓存
func (s *configService) ClearSysConfigCache(group string) {
	key := sysConfigCacheKey(group)
	if err := s.cache.Delete(context.Background(), key); err != nil {
		logger.Warn("清除系统配置缓存失败",
			zap.String("group", group),
			zap.Error(err),
		)
	}
}

// GetUserConfig 获取用户配置
func (s *configService) GetUserConfig(userID uint, configType string) (*entity.UserConfig, error) {
	ctx := context.Background()

	// 先查缓存
	cacheKey := userConfigCacheKey(userID, configType)
	var userCfg entity.UserConfig
	if err := s.cache.Get(ctx, cacheKey, &userCfg); err == nil {
		return &userCfg, nil
	}

	// 查 DB
	userCfgPtr, err := s.userConfigRepo.FindByUserAndType(userID, configType)
	if err != nil {
		return nil, err
	}
	if userCfgPtr != nil {
		if cacheErr := s.cache.Set(ctx, cacheKey, userCfgPtr, userConfigTTL); cacheErr != nil {
			logger.Warn("缓存用户配置失败", zap.String("key", cacheKey), zap.Error(cacheErr))
		}
	}
	return userCfgPtr, nil
}

// GetSysConfig 获取系统配置（返回第一个启用的配置）
func (s *configService) GetSysConfig(configType string) (*entity.SysConfig, error) {
	ctx := context.Background()
	sysCacheKey := sysConfigCacheKey(configType)

	// 先尝试从缓存读取
	var builtins []*entity.SysConfig
	if err := s.cache.Get(ctx, sysCacheKey, &builtins); err == nil {
		for _, b := range builtins {
			if b.Enabled {
				return b, nil
			}
		}
		// 缓存中没有启用的配置，清除缓存并重新读取
		if delErr := s.cache.Delete(ctx, sysCacheKey); delErr != nil {
			logger.Warn("清除系统配置缓存失败", zap.String("key", sysCacheKey), zap.Error(delErr))
		}
	}

	// 从数据库读取
	builtins, err := s.sysConfigRepo.FindByGroup(configType)
	if err != nil {
		return nil, err
	}
	if len(builtins) == 0 {
		return nil, fmt.Errorf("no sys_config for %s", configType)
	}

	// 更新缓存
	if cacheErr := s.cache.Set(ctx, sysCacheKey, builtins, sysConfigTTL); cacheErr != nil {
		logger.Warn("缓存系统配置失败", zap.String("key", sysCacheKey), zap.Error(cacheErr))
	}

	for _, b := range builtins {
		if b.Enabled {
			return b, nil
		}
	}
	return nil, fmt.Errorf("no enabled sys_config for %s", configType)
}

// injectStorage 注入文件存储到 ASR 服务（如果支持）
func (s *configService) injectStorage(svc asr.ASRService) {
	if svc == nil || s.storage == nil {
		return
	}
	if setter, ok := svc.(interface{ SetStorage(storage.FileStorage) }); ok {
		setter.SetStorage(s.storage)
	}
}

// --- 系统配置解析 ---

// sysConfigParams sys_config.config_value 解析后的参数
type sysConfigParams struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	APIURL   string `json:"api_url"`
	URL      string `json:"url"` // 兼容旧格式（MarkItDown 用 url 字段）
	APIKey   string `json:"api_key"`
}

// parseSysConfigValue 解析 sys_config.config_value，兼容 JSON 对象和纯字符串（URL）两种格式
func parseSysConfigValue(value string) (sysConfigParams, error) {
	var params sysConfigParams
	// 先尝试 JSON 解析
	if err := json.Unmarshal([]byte(value), &params); err == nil {
		return params, nil
	}
	// JSON 解析失败，尝试作为纯 URL 字符串处理
	// 去除可能的引号
	cleaned := value
	if len(cleaned) >= 2 && cleaned[0] == '"' && cleaned[len(cleaned)-1] == '"' {
		cleaned = cleaned[1 : len(cleaned)-1]
	}
	trimmed := strings.TrimSpace(cleaned)
	if len(trimmed) >= 4 && (trimmed[:4] == "http" || trimmed[:3] == "ws:") {
		params.APIURL = trimmed
		return params, nil
	}
	return params, fmt.Errorf("无法解析配置值: %s", value)
}

// GetLLMClient 获取用户的 LLM 客户端
func (s *configService) GetLLMClient(userID uint) (llm.LLMClient, error) {
	svc, err := s.getService(userID, "llm")
	if err != nil {
		return nil, err
	}
	client, ok := svc.(llm.LLMClient)
	if !ok {
		return nil, fmt.Errorf("llm provider 返回的类型不正确")
	}
	return client, nil
}

// GetChatModelConfig 获取用户的 ChatModel 配置（供 Eino 使用）
func (s *configService) GetChatModelConfig(userID uint) (*ChatModelConfig, error) {
	ctx := context.Background()

	// 1. 查用户 LLM 配置（先查缓存）
	cacheKey := userConfigCacheKey(userID, "llm")
	var userCfg entity.UserLLMConfig
	if err := s.cache.Get(ctx, cacheKey, &userCfg); err == nil && userCfg.Enabled {
		return &ChatModelConfig{
			Provider: userCfg.Provider,
			BaseURL:  userCfg.APIURL,
			APIKey:   userCfg.APIKey,
			Model:    userCfg.Model,
		}, nil
	}

	// 缓存未命中，查 DB（user_llm_config 表）
	userCfgPtr, err := s.llmConfigRepo.FindDefaultByUserID(userID)
	if err == nil && userCfgPtr != nil && userCfgPtr.Enabled {
		if cacheErr := s.cache.Set(ctx, cacheKey, userCfgPtr, userConfigTTL); cacheErr != nil {
			logger.Warn("缓存用户LLM配置失败", zap.String("key", cacheKey), zap.Error(cacheErr))
		}
		return &ChatModelConfig{
			Provider: userCfgPtr.Provider,
			BaseURL:  userCfgPtr.APIURL,
			APIKey:   userCfgPtr.APIKey,
			Model:    userCfgPtr.Model,
		}, nil
	}

	// 2. 降级到系统内置配置
	sysCacheKey := sysConfigCacheKey("llm")
	builtins, err := s.getSysConfigs(ctx, sysCacheKey, "llm")
	if err == nil {
		for _, builtin := range builtins {
			if !builtin.Enabled {
				continue
			}
			var params map[string]interface{}
			if err := json.Unmarshal([]byte(builtin.ConfigValue), &params); err != nil {
				// 尝试作为纯字符串处理
				params = map[string]interface{}{
					"api_url": builtin.ConfigValue,
				}
			}
			getStr := func(key string) string {
				if v, ok := params[key].(string); ok {
					return v
				}
				return ""
			}
			cfg, err := s.buildChatModelConfig(getStr("provider"), getStr("api_url"), getStr("api_key"), getStr("model"), "")
			if err == nil {
				return cfg, nil
			}
		}
	}

	return nil, bizerrors.New(bizerrors.CodeLLMNotConfigured, "请先在设置中配置 LLM 服务")
}

// buildChatModelConfig 构建 ChatModelConfig
func (s *configService) buildChatModelConfig(provider, apiURL, apiKey, model, extraConfig string) (*ChatModelConfig, error) {
	// 如果显式传入的 model 为空，尝试从 ExtraConfig 中获取
	if model == "" && extraConfig != "" {
		var config map[string]interface{}
		if err := json.Unmarshal([]byte(extraConfig), &config); err == nil {
			if v, ok := config["model"].(string); ok {
				model = v
			}
		}
	}

	if model == "" {
		return nil, bizerrors.New(bizerrors.CodeLLMNotConfigured, "请先在设置中配置 LLM 服务")
	}

	return &ChatModelConfig{
		Provider: provider,
		BaseURL:  apiURL,
		APIKey:   apiKey,
		Model:    model,
	}, nil
}

// getSysConfigs 获取系统配置（带缓存）
func (s *configService) getSysConfigs(ctx context.Context, cacheKey, group string) ([]*entity.SysConfig, error) {
	var builtins []*entity.SysConfig
	if err := s.cache.Get(ctx, cacheKey, &builtins); err == nil {
		return builtins, nil
	}

	builtins, err := s.sysConfigRepo.FindByGroup(group)
	if err != nil {
		return nil, err
	}
	if len(builtins) == 0 {
		return nil, fmt.Errorf("no sys_config for group %s", group)
	}

	if cacheErr := s.cache.Set(ctx, cacheKey, builtins, sysConfigTTL); cacheErr != nil {
		logger.Warn("缓存系统配置列表失败", zap.String("key", cacheKey), zap.Error(cacheErr))
	}
	return builtins, nil
}
