package service

import (
	"encoding/json"

	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type userConfigService struct {
	configRepo    repository.UserConfigRepository
	llmConfigRepo repository.UserLLMConfigRepository
	configSvc     ConfigService        // 配置路由服务，用于获取系统配置
	healthChk     *ConfigHealthChecker // 配置健康检查器
}

func NewUserConfigService(configRepo repository.UserConfigRepository, llmConfigRepo repository.UserLLMConfigRepository, configSvc ConfigService) UserConfigService {
	return &userConfigService{
		configRepo:    configRepo,
		llmConfigRepo: llmConfigRepo,
		configSvc:     configSvc,
		healthChk:     NewConfigHealthChecker(),
	}
}

// ===== LLM Config =====

func (s *userConfigService) ListLLMConfigs(userID uint) ([]*entity.UserLLMConfig, error) {
	return s.llmConfigRepo.FindByUserID(userID)
}

func (s *userConfigService) CreateLLMConfig(userID uint, config *entity.UserLLMConfig) error {
	config.UserID = userID
	return s.llmConfigRepo.Create(config)
}

func (s *userConfigService) UpdateLLMConfig(id uint, config *entity.UserLLMConfig) error {
	existing, err := s.llmConfigRepo.FindByID(id)
	if err != nil {
		logger.Error("查找配置失败", zap.Uint("id", id), zap.Error(err))
		return err
	}
	if existing == nil {
		return bizerrors.ErrNotFound
	}
	config.ID = id
	config.UserID = existing.UserID
	logger.Info("更新LLM配置", zap.Uint("id", id), zap.Any("config", config))
	return s.llmConfigRepo.Update(config)
}

func (s *userConfigService) DeleteLLMConfig(id uint) error {
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
	return []*entity.UserConfig{config}, nil
}

func (s *userConfigService) CreateSearchConfig(userID uint, config *entity.UserConfig) error {
	config.UserID = userID
	config.ConfigType = "search"
	if config.ExtraConfig == "" {
		config.ExtraConfig = "{}"
	}

	// 检查是否已经存在相同类型的配置（包括已删除的记录）
	existing, err := s.configRepo.FindByUserAndTypeIncludingDeleted(userID, "search")
	if err != nil {
		return err
	}

	if existing != nil {
		// 如果存在已删除的记录，则更新它并恢复为未删除状态
		config.ID = existing.ID
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
	if config.ExtraConfig == "" {
		config.ExtraConfig = "{}"
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
	if err := s.configRepo.Delete(id); err != nil {
		return err
	}
	if existing != nil {
		s.configSvc.ClearUserConfigCache(existing.UserID, "search")
	}
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
	return []*entity.UserConfig{config}, nil
}

func (s *userConfigService) CreateASRConfig(userID uint, config *entity.UserConfig) error {
	config.UserID = userID
	config.ConfigType = "asr"
	if config.ExtraConfig == "" {
		config.ExtraConfig = "{}"
	}

	// 检查是否已经存在相同类型的配置（包括已删除的记录）
	existing, err := s.configRepo.FindByUserAndTypeIncludingDeleted(userID, "asr")
	if err != nil {
		return err
	}

	if existing != nil {
		// 如果存在已删除的记录，则更新它并恢复为未删除状态
		config.ID = existing.ID
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
	if config.ExtraConfig == "" {
		config.ExtraConfig = "{}"
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
	if err := s.configRepo.Delete(id); err != nil {
		return err
	}
	if existing != nil {
		s.configSvc.ClearUserConfigCache(existing.UserID, "asr")
	}
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
	return []*entity.UserConfig{config}, nil
}

func (s *userConfigService) CreateEmbeddingConfig(userID uint, config *entity.UserConfig) error {
	config.UserID = userID
	config.ConfigType = "embedding"
	if config.ExtraConfig == "" {
		config.ExtraConfig = "{}"
	}

	// 检查是否已经存在相同类型的配置（包括已删除的记录）
	existing, err := s.configRepo.FindByUserAndTypeIncludingDeleted(userID, "embedding")
	if err != nil {
		return err
	}

	if existing != nil {
		// 如果存在已删除的记录，则更新它并恢复为未删除状态
		config.ID = existing.ID
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
	if config.ExtraConfig == "" {
		config.ExtraConfig = "{}"
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
	if err := s.configRepo.Delete(id); err != nil {
		return err
	}
	if existing != nil {
		s.configSvc.ClearUserConfigCache(existing.UserID, "embedding")
	}
	return nil
}

// GetActiveConfig 获取当前生效的配置（用户配置 > 系统配置）
func (s *userConfigService) GetActiveConfig(userID uint, configType string) (*entity.UserConfig, error) {
	// 1. 优先返回用户配置（必须启用）
	userCfg, err := s.configRepo.FindByUserAndType(userID, configType)
	if err == nil && userCfg != nil && userCfg.Enabled {
		// 标记为用户配置
		userCfg.ExtraConfig = `{"source":"user"}` // 临时用 ExtraConfig 标记来源
		return userCfg, nil
	}

	// 2. 降级到系统配置
	sysCfg, err := s.configSvc.GetSysConfig(configType)
	if err == nil && sysCfg != nil {
		// 解析系统配置的 JSON 值
		var params map[string]interface{}
		if jsonErr := json.Unmarshal([]byte(sysCfg.ConfigValue), &params); jsonErr == nil {
			// 从 JSON 中提取配置
			getStr := func(key string) string {
				if v, ok := params[key].(string); ok {
					return v
				}
				return ""
			}

			return &entity.UserConfig{
				ConfigType:  configType,
				Name:        getStr("name"),
				Provider:    getStr("provider"),
				APIURL:      getStr("api_url"),
				APIKey:      getStr("api_key"),
				Model:       getStr("model"),
				Enabled:     sysCfg.Enabled,
				ExtraConfig: `{"source":"system","description":"` + sysCfg.Description + `"}`,
			}, nil
		}

		// 如果解析失败，尝试作为纯 URL 处理
		return &entity.UserConfig{
			ConfigType:  configType,
			Name:        sysCfg.ConfigKey,
			Provider:    sysCfg.ConfigKey,
			APIURL:      sysCfg.ConfigValue,
			Enabled:     sysCfg.Enabled,
			ExtraConfig: `{"source":"system","description":"` + sysCfg.Description + `"}`,
		}, nil
	}

	// 3. 没有配置
	return nil, nil
}

// TestConfig 测试配置连通性（保存前验证）
func (s *userConfigService) TestConfig(configType string, config *entity.UserConfig) *HealthCheckResult {
	return s.healthChk.TestConfig(configType, config)
}
