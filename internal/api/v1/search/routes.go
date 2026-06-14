package search

import (
	"YoudaoNoteLm/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册联网搜索路由。
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	group := r.Group("/notebooks/:nbId/search")
	group.Use(middleware.Auth(ctrl.tokenBlacklist))
	{
		group.POST("/preview", ctrl.Preview)
		group.POST("/import", ctrl.Import)
	}
}
