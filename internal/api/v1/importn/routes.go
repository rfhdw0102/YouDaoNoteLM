package importn

import (
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册导入路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup, tokenBlacklist service.TokenBlacklistService) {
	// 笔记本下的导入操作（需认证）
	notebooks := r.Group("/notebooks/:nbId/import")
	notebooks.Use(middleware.Auth(tokenBlacklist))
	{
		notebooks.POST("/file", ctrl.ImportFile)
		notebooks.POST("/audio/preview", ctrl.PreviewAudio)
	}

	// 全局导入操作（需认证）
	imp := r.Group("/import")
	imp.Use(middleware.Auth(tokenBlacklist))
	{
		imp.POST("/audio/confirm", ctrl.ConfirmAudio)
		imp.GET("/tasks/:taskId", ctrl.GetTask)
	}
}
