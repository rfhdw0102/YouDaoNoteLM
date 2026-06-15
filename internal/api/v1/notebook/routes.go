package notebook

import (
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/service"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册笔记本路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup, tokenBlacklist service.TokenBlacklistService) {
	notebookGroup := r.Group("/notebooks")
	notebookGroup.Use(middleware.Auth(tokenBlacklist))
	{
		notebookGroup.POST("", ctrl.Create)
		notebookGroup.GET("", ctrl.List)
		notebookGroup.PUT("/:nbId", ctrl.Rename)
		notebookGroup.DELETE("/:nbId", ctrl.Delete)
	}
}
