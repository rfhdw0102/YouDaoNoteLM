package source

import (
	"YoudaoNoteLm/internal/middleware"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册资料来源路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	// 用户级别的路由
	userSources := r.Group("/sources")
	userSources.Use(middleware.Auth(ctrl.tokenBlacklist))
	{
		userSources.POST("/reimport-all", ctrl.ReimportAll)
		userSources.POST("/reimport", ctrl.ReimportSelected)
	}

	// 笔记本级别的路由
	sources := r.Group("/notebooks/:nbId/sources")
	sources.Use(middleware.Auth(ctrl.tokenBlacklist))
	{
		sources.GET("", ctrl.List)
		sources.GET("/:id", ctrl.GetByID)
		sources.PUT("/:id", ctrl.Rename)
		sources.DELETE("/:id", ctrl.Delete)
		sources.POST("/batch-delete", ctrl.BatchDelete)
		sources.POST("/delete-failed", ctrl.DeleteFailed)
		sources.GET("/:id/content", ctrl.GetContent)
		sources.GET("/:id/original", ctrl.GetOriginal)
		sources.GET("/:id/download", ctrl.GetDownloadURL)
		sources.POST("/from-note", ctrl.CreateFromNote)
		sources.POST("/delete-by-note", ctrl.DeleteByNote)
	}
}
