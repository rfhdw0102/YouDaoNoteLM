// internal/api/v1/search/routes.go
package search

import (
	"YoudaoNoteLm/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册搜索路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	// 笔记本下的搜索操作（需认证）
	notebooks := r.Group("/notebooks/:nbId/search")
	notebooks.Use(middleware.Auth(ctrl.tokenBlacklist))
	{
		notebooks.POST("", ctrl.Search)
		notebooks.POST("/stream", ctrl.SearchStream) // SSE 流式搜索
		notebooks.POST("/url", ctrl.ImportFromURL)
		notebooks.POST("/import", ctrl.ImportSearchResults)
	}
}
