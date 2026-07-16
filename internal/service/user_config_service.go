package service

import (
	"encoding/json"

	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"
	"YoudaoNoteLm/pkg/utils"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type userConfigService struct {
	configRepo    repository.UserConfigRepository
	llmConfigRepo repository.UserLLMConfigRepository
	configSvc     ConfigService        // 配置路由服务，用于获取系统配置
	healthChk     *ConfigHealthChecker // 配置健康检查器
	encryptionKey []byte               // API Key 加密密钥
}

func NewUserConfigService(configRepo repository.UserConfigRepository, llmConfigRepo repository.UserLLMConfigRepository, configSvc ConfigService, encryptionKey string) UserConfigService {
	return &userConfigService{
		configRepo:    configRepo,
		llmConfigRepo: llmConfigRepo,
		configSvc:     configSvc,
		healthChk:     NewConfigHealthChecker(),
		encryptionKey: []byte(encryptionKey),
	}
}

// ===== LLM Config =====

func (s *userConfigService) ListLLMConfigs(userID uint) ([]*entity.UserLLMConfig, error) {
	configs, err := s.llmConfigRepo.FindByUserID(userID)
	if err != nil {
		return nil, err
	}
	// 解密 API Key
	for _, config := range configs {
		if config.APIKey != "" {
			decrypted, err := utils.Decrypt(config.APIKey, s.encryptionKey)
			if err != nil {
				// 解密失败，可能数据未加密或使用了不同的密钥，保留原值
				logger.Debug("解密 LLM API Key 失败（可能未加密）", zap.Uint("config_id", config.ID), zap.Error(err))
			} else {
				config.APIKey = decrypted
			}
		}
	}
	return configs, nil
}

func (s *userConfigService) CreateLLMConfig(userID uint, config *entity.UserLLMConfig) error {
	config.UserID = userID
	// 加密 API Key
	if config.APIKey != "" {
		encrypted, err := utils.Encrypt(config.APIKey, s.encryptionKey)
		if err != nil {
			logger.Error("加密 LLM API Key 失败", zap.Error(err))
			return err
		}
		config.APIKey = encrypted
	}
	return s.llmConfigRepo.Create(config)
}

func (s *userConfigService) UpdateLLMConfig(userID uint, id uint, config *entity.UserLLMConfig) error {
	existing, err := s.llmConfigRepo.FindByID(id)
	if err != nil {
		logger.Error("查找配置失败", zap.Uint("id", id), zap.Error(err))
		return err
	}
	if existing == nil {
		return bizerrors.ErrNotFound
	}
	if existing.UserID != userID {
		return bizerrors.New(bizerrors.CodeForbidden, "无权操作此配置")
	}
	config.ID = id
	config.UserID = existing.UserID
	config.CreatedAt = existing.CreatedAt
	config.UpdatedAt = existing.UpdatedAt
	// 加密 API Key
	if config.APIKey != "" {
		encrypted, err := utils.Encrypt(config.APIKey, s.encryptionKey)
		if err != nil {
			logger.Error("加密 LLM API Key 失败", zap.Error(err))
			return err
		}
		config.APIKey = encrypted
	}
	logger.Info("更新LLM配置", zap.Uint("id", id), zap.Any("config", config))
	return s.llmConfigRepo.Update(config)
}

func (s *userConfigService) DeleteLLMConfig(userID uint, id uint) error {
	existing, err := s.llmConfigRepo.FindByID(id)
	if err != nil {
		return err
	}
	if existing == nil {
		return bizerrors.ErrNotFound
	}
	if existing.UserID != userID {
		return bizerrors.New(bizerrors.CodeForbidden, "无权操作此配置")
	}
	return s.llmConfigRepo.Delete(id)
}

// ===== Search Config =====

func (s *userConfigService) ListSearchConfigs(userID uint) ([]*entity.UserConfig, error) {
	config, err := s.configRepo.FindByUserAndType(userID, "search")
	if err != nil {
		return nil, err
	}
	if config == nil {
		return []*entity.UserConfig{}, nil
	}
	// 解密 API Key
	if config.APIKey != "" {
		decrypted, err := utils.Decrypt(config.APIKey, s.encryptionKey)
		if err != nil {
			logger.Debug("解密 Search API Key 失败（可能未加密）", zap.Uint("config_id", config.ID), zap.Error(err))
		} else {
			config.APIKey = decrypted
		}
	}
	return []*entity.UserConfig{config}, nil
}

func (s *userConfigService) CreateSearchConfig(userID uint, config *entity.UserConfig) error {
	config.UserID = userID
	config.ConfigType = "search"
	if config.ExtraConfig == "" {
		config.ExtraConfig = "{}"
	}

	// 加密 API Key
	if config.APIKey != "" {
		encrypted, err := utils.Encrypt(config.APIKey, s.encryptionKey)
		if err != nil {
			logger.Error("加密 Search API Key 失败", zap.Error(err))
			return err
		}
		config.APIKey = encrypted
	}

	// 检查是否已经存在相同类型的配置（包括已删除的记录）
	existing, err := s.configRepo.FindByUserAndTypeIncludingDeleted(userID, "search")
	if err != nil {
		return err
	}

	if existing != nil {
		// 如果存在已删除的记录，则更新它并恢复为未删除状态
		config.ID = existing.ID
		config.CreatedAt = existing.CreatedAt
		config.UpdatedAt = existing.UpdatedAt
		config.DeletedAt = gorm.DeletedAt{} // 恢复为未删除状态
		return s.configRepo.Update(config)
	}

	return s.configRepo.Create(config)
}

func (s *userConfigService) UpdateSearchConfig(id uint, config *entity.UserConfig) error {
	existing, err := s.configRepo.FindByID(id)
	if err != nil {
		return err
	}
	if existing == nil {
		return bizerrors.ErrNotFound
	}
	config.ID = id
	config.UserID = existing.UserID
	config.ConfigType = "search"
	config.CreatedAt = existing.CreatedAt
	config.UpdatedAt = existing.UpdatedAt
	if config.ExtraConfig == "" {
		config.ExtraConfig = "{}"
	}
	// 加密 API Key
	if config.APIKey != "" {
		encrypted, err := utils.Encrypt(config.APIKey, s.encryptionKey)
		if err != nil {
			logger.Error("加密 Search API Key 失败", zap.Error(err))
			return err
		}
		config.APIKey = encrypted
	}
	if err := s.configRepo.Update(config); err != nil {
		return err
	}
	s.configSvc.ClearUserConfigCache(existing.UserID, "search")
	return nil
}

func (s *userConfigService) DeleteSearchConfig(id uint) error {
	existing, err := s.configRepo.FindByID(id)
	if err != nil {
		return err
	}
	if existing == nil {
		return bizerrors.ErrNotFound
	}
	if err := s.configRepo.Delete(id); err != nil {
		return err
	}
	s.configSvc.ClearUserConfigCache(existing.UserID, "search")
	return nil
}

// ===== ASR Config =====

func (s *userConfigService) ListASRConfigs(userID uint) ([]*entity.UserConfig, error) {
	config, err := s.configRepo.FindByUserAndType(userID, "asr")
	if err != nil {
		return nil, err
	}
	if config == nil {
		return []*entity.UserConfig{}, nil
	}
	// 解密 API Key
	if config.APIKey != "" {
		decrypted, err := utils.Decrypt(config.APIKey, s.encryptionKey)
		if err != nil {
			logger.Debug("解密 ASR API Key 失败（可能未加密）", zap.Uint("config_id", config.ID), zap.Error(err))
		} else {
			config.APIKey = decrypted
		}
	}
	return []*entity.UserConfig{config}, nil
}

func (s *userConfigService) CreateASRConfig(userID uint, config *entity.UserConfig) error {
	config.UserID = userID
	config.ConfigType = "asr"
	if config.ExtraConfig == "" {
		config.ExtraConfig = "{}"
	}

	// 加密 API Key
	if config.APIKey != "" {
		encrypted, err := utils.Encrypt(config.APIKey, s.encryptionKey)
		if err != nil {
			logger.Error("加密 ASR API Key 失败", zap.Error(err))
			return err
		}
		config.APIKey = encrypted
	}

	// 检查是否已经存在相同类型的配置（包括已删除的记录）
	existing, err := s.configRepo.FindByUserAndTypeIncludingDeleted(userID, "asr")
	if err != nil {
		return err
	}

	if existing != nil {
		// 如果存在已删除的记录，则更新它并恢复为未删除状态
		config.ID = existing.ID
		config.CreatedAt = existing.CreatedAt
		config.UpdatedAt = existing.UpdatedAt
		config.DeletedAt = gorm.DeletedAt{} // 恢复为未删除状态
		return s.configRepo.Update(config)
	}

	return s.configRepo.Create(config)
}

func (s *userConfigService) UpdateASRConfig(id uint, config *entity.UserConfig) error {
	existing, err := s.configRepo.FindByID(id)
	if err != nil {
		return err
	}
	if existing == nil {
		return bizerrors.ErrNotFound
	}
	config.ID = id
	config.UserID = existing.UserID
	config.ConfigType = "asr"
	config.CreatedAt = existing.CreatedAt
	config.UpdatedAt = existing.UpdatedAt
	if config.ExtraConfig == "" {
		config.ExtraConfig = "{}"
	}
	// 加密 API Key
	if config.APIKey != "" {
		encrypted, err := utils.Encrypt(config.APIKey, s.encryptionKey)
		if err != nil {
			logger.Error("加密 ASR API Key 失败", zap.Error(err))
			return err
		}
		config.APIKey = encrypted
	}
	if err := s.configRepo.Update(config); err != nil {
		return err
	}
	s.configSvc.ClearUserConfigCache(existing.UserID, "asr")
	return nil
}

func (s *userConfigService) DeleteASRConfig(id uint) error {
	existing, err := s.configRepo.FindByID(id)
	if err != nil {
		return err
	}
	if existing == nil {
		return bizerrors.ErrNotFound
	}
	if err := s.configRepo.Delete(id); err != nil {
		return err
	}
	s.configSvc.ClearUserConfigCache(existing.UserID, "asr")
	return nil
}

// ===== Embedding Config =====

func (s *userConfigService) ListEmbeddingConfigs(userID uint) ([]*entity.UserConfig, error) {
	config, err := s.configRepo.FindByUserAndType(userID, "embedding")
	if err != nil {
		return nil, err
	}
	if config == nil {
		return []*entity.UserConfig{}, nil
	}
	// 解密 API Key
	if config.APIKey != "" {
		decrypted, err := utils.Decrypt(config.APIKey, s.encryptionKey)
		if err != nil {
			logger.Debug("解密 Embedding API Key 失败（可能未加密）", zap.Uint("config_id", config.ID), zap.Error(err))
		} else {
			config.APIKey = decrypted
		}
	}
	return []*entity.UserConfig{config}, nil
}

func (s *userConfigService) CreateEmbeddingConfig(userID uint, config *entity.UserConfig) error {
	config.UserID = userID
	config.ConfigType = "embedding"
	if config.ExtraConfig == "" {
		config.ExtraConfig = "{}"
	}

	// 加密 API Key
	if config.APIKey != "" {
		encrypted, err := utils.Encrypt(config.APIKey, s.encryptionKey)
		if err != nil {
			logger.Error("加密 Embedding API Key 失败", zap.Error(err))
			return err
		}
		config.APIKey = encrypted
	}

	// 检查是否已经存在相同类型的配置（包括已删除的记录）
	existing, err := s.configRepo.FindByUserAndTypeIncludingDeleted(userID, "embedding")
	if err != nil {
		return err
	}

	if existing != nil {
		// 如果存在已删除的记录，则更新它并恢复为未删除状态
		config.ID = existing.ID
		config.CreatedAt = existing.CreatedAt
		config.UpdatedAt = existing.UpdatedAt
		config.DeletedAt = gorm.DeletedAt{} // 恢复为未删除状态
		return s.configRepo.Update(config)
	}

	return s.configRepo.Create(config)
}

func (s *userConfigService) UpdateEmbeddingConfig(id uint, config *entity.UserConfig) error {
	existing, err := s.configRepo.FindByID(id)
	if err != nil {
		return err
	}
	if existing == nil {
		return bizerrors.ErrNotFound
	}
	config.ID = id
	config.UserID = existing.UserID
	config.ConfigType = "embedding"
	config.CreatedAt = existing.CreatedAt
	config.UpdatedAt = existing.UpdatedAt
	if config.ExtraConfig == "" {
		config.ExtraConfig = "{}"
	}
	// 加密 API Key
	if config.APIKey != "" {
		encrypted, err := utils.Encrypt(config.APIKey, s.encryptionKey)
		if err != nil {
			logger.Error("加密 Embedding API Key 失败", zap.Error(err))
			return err
		}
		config.APIKey = encrypted
	}
	if err := s.configRepo.Update(config); err != nil {
		return err
	}
	s.configSvc.ClearUserConfigCache(existing.UserID, "embedding")
	return nil
}

func (s *userConfigService) DeleteEmbeddingConfig(id uint) error {
	existing, err := s.configRepo.FindByID(id)
	if err != nil {
		return err
	}
	if existing == nil {
		return bizerrors.ErrNotFound
	}
	if err := s.configRepo.Delete(id); err != nil {
		return err
	}
	s.configSvc.ClearUserConfigCache(existing.UserID, "embedding")
	return nil
}

// ===== Reranker Config =====

func (s *userConfigService) ListRerankerConfigs(userID uint) ([]*entity.UserConfig, error) {
	config, err := s.configRepo.FindByUserAndType(userID, "reranker")
	if err != nil {
		return nil, err
	}
	if config == nil {
		return []*entity.UserConfig{}, nil
	}
	// 解密 API Key
	if config.APIKey != "" {
		decrypted, err := utils.Decrypt(config.APIKey, s.encryptionKey)
		if err != nil {
			logger.Debug("解密 Reranker API Key 失败（可能未加密）", zap.Uint("config_id", config.ID), zap.Error(err))
		} else {
			config.APIKey = decrypted
		}
	}
	return []*entity.UserConfig{config}, nil
}

func (s *userConfigService) CreateRerankerConfig(userID uint, config *entity.UserConfig) error {
	config.UserID = userID
	config.ConfigType = "reranker"
	if config.ExtraConfig == "" {
		config.ExtraConfig = "{}"
	}

	// 加密 API Key
	if config.APIKey != "" {
		encrypted, err := utils.Encrypt(config.APIKey, s.encryptionKey)
		if err != nil {
			logger.Error("加密 Reranker API Key 失败", zap.Error(err))
			return err
		}
		config.APIKey = encrypted
	}

	// 检查是否已经存在相同类型的配置（包括已删除的记录）
	existing, err := s.configRepo.FindByUserAndTypeIncludingDeleted(userID, "reranker")
	if err != nil {
		return err
	}

	if existing != nil {
		// 如果存在已删除的记录，则更新它并恢复为未删除状态
		config.ID = existing.ID
		config.CreatedAt = existing.CreatedAt
		config.UpdatedAt = existing.UpdatedAt
		config.DeletedAt = gorm.DeletedAt{} // 恢复为未删除状态
		return s.configRepo.Update(config)
	}

	return s.configRepo.Create(config)
}

func (s *userConfigService) UpdateRerankerConfig(id uint, config *entity.UserConfig) error {
	existing, err := s.configRepo.FindByID(id)
	if err != nil {
		return err
	}
	if existing == nil {
		return bizerrors.ErrNotFound
	}
	config.ID = id
	config.UserID = existing.UserID
	config.ConfigType = "reranker"
	config.CreatedAt = existing.CreatedAt
	config.UpdatedAt = existing.UpdatedAt
	if config.ExtraConfig == "" {
		config.ExtraConfig = "{}"
	}
	// 加密 API Key
	if config.APIKey != "" {
		encrypted, err := utils.Encrypt(config.APIKey, s.encryptionKey)
		if err != nil {
			logger.Error("加密 Reranker API Key 失败", zap.Error(err))
			return err
		}
		config.APIKey = encrypted
	}
	if err := s.configRepo.Update(config); err != nil {
		return err
	}
	s.configSvc.ClearUserConfigCache(existing.UserID, "reranker")
	return nil
}

func (s *userConfigService) DeleteRerankerConfig(id uint) error {
	existing, err := s.configRepo.FindByID(id)
	if err != nil {
		return err
	}
	if existing == nil {
		return bizerrors.ErrNotFound
	}
	if err := s.configRepo.Delete(id); err != nil {
		return err
	}
	s.configSvc.ClearUserConfigCache(existing.UserID, "reranker")
	return nil
}

// GetActiveConfig 获取当前生效的配置（用户配置 > 系统配置）
func (s *userConfigService) GetActiveConfig(userID uint, configType string) (*entity.UserConfig, error) {
	// LLM 配置存储在独立的 user_llm_config 表，需要特殊处理
	if configType == "llm" {
		return s.getActiveLLMConfig(userID)
	}

	// 1. 优先返回用户配置（必须启用）
	userCfg, err := s.configRepo.FindByUserAndType(userID, configType)
	if err == nil && userCfg != nil && userCfg.Enabled {
		userCfg.Source = "user"
		// 解密 API Key
		if userCfg.APIKey != "" {
			decrypted, err := utils.Decrypt(userCfg.APIKey, s.encryptionKey)
			if err != nil {
				logger.Debug("解密用户配置 API Key 失败（可能未加密）", zap.Uint("user_id", userID), zap.Error(err))
			} else {
				userCfg.APIKey = decrypted
			}
		}
		return userCfg, nil
	}

	// 2. 降级到系统配置
	sysCfg, err := s.configSvc.GetSysConfig(configType)
	if err == nil && sysCfg != nil {
		// 解析系统配置的 JSON 值
		var params map[string]interface{}
		if jsonErr := json.Unmarshal([]byte(sysCfg.ConfigValue), &params); jsonErr == nil {
			getStr := func(key string) string {
				if v, ok := params[key].(string); ok {
					return v
				}
				return ""
			}

			return &entity.UserConfig{
				ConfigType: configType,
				Name:       getStr("name"),
				Provider:   getStr("provider"),
				APIURL:     getStr("api_url"),
				APIKey:     getStr("api_key"),
				Model:      getStr("model"),
				Enabled:    sysCfg.Enabled,
				Source:     "system",
			}, nil
		}

		// 如果解析失败，尝试作为纯 URL 处理
		return &entity.UserConfig{
			ConfigType: configType,
			Name:       sysCfg.ConfigKey,
			Provider:   sysCfg.ConfigKey,
			APIURL:     sysCfg.ConfigValue,
			Enabled:    sysCfg.Enabled,
			Source:     "system",
		}, nil
	}

	// 3. 没有配置
	return nil, nil
}

// getActiveLLMConfig 获取当前生效的 LLM 配置（LLM 存储在 user_llm_config 表）
func (s *userConfigService) getActiveLLMConfig(userID uint) (*entity.UserConfig, error) {
	// 1. 优先返回用户 LLM 配置（第一个启用的）
	llmCfg, err := s.llmConfigRepo.FindDefaultByUserID(userID)
	if err == nil && llmCfg != nil && llmCfg.Enabled {
		apiKey := llmCfg.APIKey
		if apiKey != "" {
			decrypted, err := utils.Decrypt(apiKey, s.encryptionKey)
			if err != nil {
				logger.Debug("解密 LLM API Key 失败（可能未加密）", zap.Uint("user_id", userID), zap.Error(err))
			} else {
				apiKey = decrypted
			}
		}
		return &entity.UserConfig{
			ConfigType: "llm",
			Name:       llmCfg.Name,
			Provider:   llmCfg.Provider,
			APIURL:     llmCfg.APIURL,
			APIKey:     apiKey,
			Model:      llmCfg.Model,
			Enabled:    llmCfg.Enabled,
			Source:     "user",
		}, nil
	}

	// 2. 降级到系统配置
	sysCfg, err := s.configSvc.GetSysConfig("llm")
	if err == nil && sysCfg != nil {
		var params map[string]interface{}
		if jsonErr := json.Unmarshal([]byte(sysCfg.ConfigValue), &params); jsonErr == nil {
			getStr := func(key string) string {
				if v, ok := params[key].(string); ok {
					return v
				}
				return ""
			}
			return &entity.UserConfig{
				ConfigType: "llm",
				Name:       getStr("name"),
				Provider:   getStr("provider"),
				APIURL:     getStr("api_url"),
				APIKey:     getStr("api_key"),
				Model:      getStr("model"),
				Enabled:    sysCfg.Enabled,
				Source:     "system",
			}, nil
		}
		return &entity.UserConfig{
			ConfigType: "llm",
			Name:       sysCfg.ConfigKey,
			Provider:   sysCfg.ConfigKey,
			APIURL:     sysCfg.ConfigValue,
			Enabled:    sysCfg.Enabled,
			Source:     "system",
		}, nil
	}

	// 3. 没有配置
	return nil, nil
}

// TestConfig 测试配置连通性（保存前验证）
func (s *userConfigService) TestConfig(configType string, config *entity.UserConfig) *HealthCheckResult {
	return s.healthChk.TestConfig(configType, config)
}
