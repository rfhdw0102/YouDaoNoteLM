package source

import (
	"YoudaoNoteLm/internal/middleware"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册资料来源路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	sources := r.Group("/notebooks/:nbId/sources")
	sources.Use(middleware.Auth(ctrl.tokenBlacklist))
	{
		sources.GET("", ctrl.List)
		sources.GET("/:id", ctrl.GetByID)
		sources.PUT("/:id", ctrl.Rename)
		sources.DELETE("/:id", ctrl.Delete)
		sources.POST("/batch-delete", ctrl.BatchDelete)
		sources.GET("/:id/content", ctrl.GetContent)
		sources.GET("/:id/original", ctrl.GetOriginal)
	}
}
