package user_config

import (
	"YoudaoNoteLm/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册用户配置路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	cfg := r.Group("/user/config").Use(middleware.Auth(ctrl.tokenBlacklist))
	{
		// 配置连通性测试（不保存，仅验证）
		cfg.POST("/:type/test", ctrl.TestConfig)

		// 获取当前生效的配置
		cfg.GET("/active/:type", ctrl.GetActiveConfig)

		cfg.GET("/llm", ctrl.ListLLMConfigs)
		cfg.POST("/llm", ctrl.CreateLLMConfig)
		cfg.PUT("/llm/:id", ctrl.UpdateLLMConfig)
		cfg.DELETE("/llm/:id", ctrl.DeleteLLMConfig)

		cfg.GET("/search", ctrl.ListSearchConfigs)
		cfg.POST("/search", ctrl.CreateSearchConfig)
		cfg.PUT("/search/:id", ctrl.UpdateSearchConfig)
		cfg.DELETE("/search/:id", ctrl.DeleteSearchConfig)

		cfg.GET("/asr", ctrl.ListASRConfigs)
		cfg.POST("/asr", ctrl.CreateASRConfig)
		cfg.PUT("/asr/:id", ctrl.UpdateASRConfig)
		cfg.DELETE("/asr/:id", ctrl.DeleteASRConfig)

		cfg.GET("/embedding", ctrl.ListEmbeddingConfigs)
		cfg.POST("/embedding", ctrl.CreateEmbeddingConfig)
		cfg.PUT("/embedding/:id", ctrl.UpdateEmbeddingConfig)
		cfg.DELETE("/embedding/:id", ctrl.DeleteEmbeddingConfig)
	}
}
