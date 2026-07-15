// internal/api/v1/providers/controller.go
package providers

import (
	"encoding/json"

	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/internal/service/external"
	"YoudaoNoteLm/pkg/response"

	"github.com/gin-gonic/gin"
)

type Controller struct {
	registry      *external.Registry
	configService service.ConfigService
}

func NewController(configService service.ConfigService) *Controller {
	return &Controller{
		registry:      external.GetGlobalRegistry(),
		configService: configService,
	}
}

// ListProviders 列出所有已注册的 provider
func (ctrl *Controller) ListProviders(c *gin.Context) {
	serviceType := c.Query("type")
	showAll := c.Query("show_all") == "true" // 管理员可查看全部

	providers := ctrl.registry.ListProviders(serviceType)
	if providers == nil {
		providers = []external.ProviderInfo{}
	}

	// 默认只返回已实现的 provider
	if !showAll {
		implemented := make([]external.ProviderInfo, 0, len(providers))
		for _, p := range providers {
			if p.Implemented {
				implemented = append(implemented, p)
			}
		}
		providers = implemented
	}

	response.Success(c, gin.H{
		"providers": providers,
	})
}

// GetActiveConfig 获取当前生效的配置（用户配置优先，否则系统配置）
func (ctrl *Controller) GetActiveConfig(c *gin.Context) {
	serviceType := c.Query("type")
	if serviceType == "" {
		response.BadRequest(c, "缺少 type 参数")
		return
	}

	// 尝试获取用户 ID（可能未登录）
	userID := middleware.GetUserID(c)

	// 如果有用户 ID，先查用户配置
	if userID > 0 {
		userConfig, err := ctrl.configService.GetUserConfig(userID, serviceType)
		if err == nil && userConfig != nil && userConfig.Enabled {
			providerInfo := ctrl.registry.GetProviderInfo(serviceType, userConfig.Provider)
			displayName := userConfig.Provider
			if providerInfo != nil {
				displayName = providerInfo.DisplayName
			}
			response.Success(c, gin.H{
				"source":       "user",
				"provider":     userConfig.Provider,
				"display_name": displayName,
			})
			return
		}
	}

	// 查询系统配置
	sysConfig, err := ctrl.configService.GetSysConfig(serviceType)
	if err == nil && sysConfig != nil {
		// 从 config_value 中解析实际的 provider
		provider := ""
		var params map[string]interface{}
		if jsonErr := json.Unmarshal([]byte(sysConfig.ConfigValue), &params); jsonErr == nil {
			if p, ok := params["provider"].(string); ok {
				provider = p
			}
		}
		// 如果解析失败，使用 config_key 作为 fallback
		if provider == "" {
			provider = sysConfig.ConfigKey
		}

		providerInfo := ctrl.registry.GetProviderInfo(serviceType, provider)
		displayName := provider
		if providerInfo != nil {
			displayName = providerInfo.DisplayName
		}
		response.Success(c, gin.H{
			"source":       "system",
			"provider":     provider,
			"display_name": displayName,
		})
		return
	}

	// 无配置
	response.Success(c, gin.H{
		"source":       "none",
		"provider":     "",
		"display_name": "未配置",
	})
}
