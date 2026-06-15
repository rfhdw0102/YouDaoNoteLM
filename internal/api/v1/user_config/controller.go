package user_config

import (
	"encoding/json"
	"strconv"

	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/service"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/response"

	"github.com/gin-gonic/gin"
)

type Controller struct {
	configService  service.UserConfigService
	tokenBlacklist service.TokenBlacklistService
}

func NewController(configService service.UserConfigService, tokenBlacklist service.TokenBlacklistService) *Controller {
	return &Controller{configService: configService, tokenBlacklist: tokenBlacklist}
}

// ===== Config Health Check =====

// TestConfig 测试配置连通性（不保存，仅验证）
func (ctrl *Controller) TestConfig(c *gin.Context) {
	configType := c.Param("type")
	validTypes := map[string]bool{
		"llm": true, "search": true, "asr": true, "embedding": true,
	}
	if !validTypes[configType] {
		response.BadRequest(c, "无效的配置类型")
		return
	}

	var req request.UserConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	config := &entity.UserConfig{
		Provider:    req.Provider,
		APIKey:      req.APIKey,
		APIURL:      req.APIURL,
		Model:       req.Model,
		ExtraConfig: string(req.ExtraConfig),
	}

	result := ctrl.configService.TestConfig(configType, config)
	if result.Healthy {
		response.SuccessWithMessage(c, result.Message, result)
	} else {
		response.ErrorWithData(c, bizerrors.CodeConfigTestFailed, result.Message, result)
	}
}

// validateBeforeSave 保存前验证配置连通性
// 如果验证失败，返回 false 并已写入响应；调用方应直接 return
func (ctrl *Controller) validateBeforeSave(c *gin.Context, configType string, config *entity.UserConfig) bool {
	result := ctrl.configService.TestConfig(configType, config)
	if !result.Healthy {
		response.ErrorWithData(c, bizerrors.CodeConfigTestFailed,
			"配置验证失败: "+result.Message, result)
		return false
	}
	return true
}

// ===== LLM Config =====

func (ctrl *Controller) ListLLMConfigs(c *gin.Context) {
	userID := middleware.GetUserID(c)
	configs, err := ctrl.configService.ListLLMConfigs(userID)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, configs)
}

func (ctrl *Controller) CreateLLMConfig(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req request.UserConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	config := &entity.UserLLMConfig{
		Name: req.Name, Provider: req.Provider, APIKey: req.APIKey,
		APIURL: req.APIURL, Model: req.Model, Enabled: true,
	}

	// 保存前验证连通性
	if !ctrl.validateBeforeSave(c, "llm", &entity.UserConfig{
		Provider: req.Provider, APIKey: req.APIKey, APIURL: req.APIURL, Model: req.Model,
	}) {
		return
	}

	if err := ctrl.configService.CreateLLMConfig(userID, config); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, config)
}

func (ctrl *Controller) UpdateLLMConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的配置ID")
		return
	}
	var req request.UserConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	config := &entity.UserLLMConfig{
		Name: req.Name, Provider: req.Provider, APIKey: req.APIKey,
		APIURL: req.APIURL, Model: req.Model,
	}

	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	} else {
		config.Enabled = true
	}

	// 保存前验证连通性
	if !ctrl.validateBeforeSave(c, "llm", &entity.UserConfig{
		Provider: req.Provider, APIKey: req.APIKey, APIURL: req.APIURL, Model: req.Model,
	}) {
		return
	}

	if err := ctrl.configService.UpdateLLMConfig(uint(id), config); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, config)
}

func (ctrl *Controller) DeleteLLMConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的配置ID")
		return
	}
	if err := ctrl.configService.DeleteLLMConfig(uint(id)); err != nil {
		response.BizError(c, err)
		return
	}
	response.SuccessWithMessage(c, "删除成功", nil)
}

// ===== Search Config =====

func (ctrl *Controller) ListSearchConfigs(c *gin.Context) {
	userID := middleware.GetUserID(c)
	configs, err := ctrl.configService.ListSearchConfigs(userID)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, configs)
}

func (ctrl *Controller) CreateSearchConfig(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req request.UserConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	config := &entity.UserConfig{
		Name: req.Name, Provider: req.Provider, APIKey: req.APIKey,
		APIURL: req.APIURL, DailyQuota: req.DailyQuota,
		ExtraConfig: string(req.ExtraConfig), Enabled: true,
	}

	// 保存前验证连通性
	if !ctrl.validateBeforeSave(c, "search", config) {
		return
	}

	if err := ctrl.configService.CreateSearchConfig(userID, config); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, config)
}

func (ctrl *Controller) UpdateSearchConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的配置ID")
		return
	}
	var req request.UserConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	config := &entity.UserConfig{
		Name: req.Name, Provider: req.Provider, APIKey: req.APIKey,
		APIURL: req.APIURL, DailyQuota: req.DailyQuota,
		ExtraConfig: string(req.ExtraConfig),
	}

	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	} else {
		config.Enabled = true
	}

	// 保存前验证连通性
	if !ctrl.validateBeforeSave(c, "search", config) {
		return
	}

	if err := ctrl.configService.UpdateSearchConfig(uint(id), config); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, config)
}

func (ctrl *Controller) DeleteSearchConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的配置ID")
		return
	}
	if err := ctrl.configService.DeleteSearchConfig(uint(id)); err != nil {
		response.BizError(c, err)
		return
	}
	response.SuccessWithMessage(c, "删除成功", nil)
}

// ===== ASR Config =====

func (ctrl *Controller) ListASRConfigs(c *gin.Context) {
	userID := middleware.GetUserID(c)
	configs, err := ctrl.configService.ListASRConfigs(userID)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, configs)
}

func (ctrl *Controller) CreateASRConfig(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req request.UserConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	config := &entity.UserConfig{
		Name: req.Name, Provider: req.Provider, APIKey: req.APIKey,
		APIURL: req.APIURL, ExtraConfig: string(req.ExtraConfig), Enabled: true,
	}

	// 保存前验证连通性
	if !ctrl.validateBeforeSave(c, "asr", config) {
		return
	}

	if err := ctrl.configService.CreateASRConfig(userID, config); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, config)
}

func (ctrl *Controller) UpdateASRConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的配置ID")
		return
	}
	var req request.UserConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	config := &entity.UserConfig{
		Name: req.Name, Provider: req.Provider, APIKey: req.APIKey,
		APIURL: req.APIURL, ExtraConfig: string(req.ExtraConfig),
	}

	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	} else {
		config.Enabled = true
	}

	// 保存前验证连通性
	if !ctrl.validateBeforeSave(c, "asr", config) {
		return
	}

	if err := ctrl.configService.UpdateASRConfig(uint(id), config); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, config)
}

func (ctrl *Controller) DeleteASRConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的配置ID")
		return
	}
	if err := ctrl.configService.DeleteASRConfig(uint(id)); err != nil {
		response.BizError(c, err)
		return
	}
	response.SuccessWithMessage(c, "删除成功", nil)
}

// ===== Embedding Config =====

func (ctrl *Controller) ListEmbeddingConfigs(c *gin.Context) {
	userID := middleware.GetUserID(c)
	configs, err := ctrl.configService.ListEmbeddingConfigs(userID)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, configs)
}

// ===== Active Config =====

// GetActiveConfig 获取当前生效的配置
func (ctrl *Controller) GetActiveConfig(c *gin.Context) {
	userID := middleware.GetUserID(c)
	configType := c.Param("type")

	// 验证配置类型
	validTypes := map[string]bool{
		"llm":       true,
		"search":    true,
		"asr":       true,
		"embedding": true,
	}
	if !validTypes[configType] {
		response.BadRequest(c, "无效的配置类型")
		return
	}

	config, err := ctrl.configService.GetActiveConfig(userID, configType)
	if err != nil {
		response.BizError(c, err)
		return
	}

	// 如果没有配置，返回空对象
	if config == nil {
		response.Success(c, gin.H{
			"config_type": configType,
			"active":      false,
			"message":     "未配置",
		})
		return
	}

	// 检查来源
	source := "user"
	if config.ExtraConfig != "" {
		var extra map[string]interface{}
		if json.Unmarshal([]byte(config.ExtraConfig), &extra) == nil {
			if s, ok := extra["source"].(string); ok {
				source = s
			}
		}
	}

	response.Success(c, gin.H{
		"config_type": config.ConfigType,
		"name":        config.Name,
		"provider":    config.Provider,
		"api_url":     config.APIURL,
		"model":       config.Model,
		"enabled":     config.Enabled,
		"source":      source, // user 或 system
		"active":      true,
	})
}

func (ctrl *Controller) CreateEmbeddingConfig(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req request.UserConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	config := &entity.UserConfig{
		Name: req.Name, Provider: req.Provider, APIKey: req.APIKey,
		APIURL: req.APIURL, Model: req.Model, Dimensions: req.Dimensions,
		ExtraConfig: string(req.ExtraConfig), Enabled: true,
	}

	// 保存前验证连通性
	if !ctrl.validateBeforeSave(c, "embedding", config) {
		return
	}

	if err := ctrl.configService.CreateEmbeddingConfig(userID, config); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, config)
}

func (ctrl *Controller) UpdateEmbeddingConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的配置ID")
		return
	}
	var req request.UserConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	config := &entity.UserConfig{
		Name: req.Name, Provider: req.Provider, APIKey: req.APIKey,
		APIURL: req.APIURL, Model: req.Model, Dimensions: req.Dimensions,
		ExtraConfig: string(req.ExtraConfig),
	}

	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	} else {
		config.Enabled = true
	}

	// 保存前验证连通性
	if !ctrl.validateBeforeSave(c, "embedding", config) {
		return
	}

	if err := ctrl.configService.UpdateEmbeddingConfig(uint(id), config); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, config)
}

func (ctrl *Controller) DeleteEmbeddingConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的配置ID")
		return
	}
	if err := ctrl.configService.DeleteEmbeddingConfig(uint(id)); err != nil {
		response.BizError(c, err)
		return
	}
	response.SuccessWithMessage(c, "删除成功", nil)
}
