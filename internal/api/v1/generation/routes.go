package generation

import (
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/service"

	"github.com/gin-gonic/gin"
)

// 注册生成模块路由。
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup, tokenBlacklist service.TokenBlacklistService, statusCheck gin.HandlerFunc) {
	group := r.Group("/generations")
	group.Use(middleware.Auth(tokenBlacklist), statusCheck)
	{
		group.POST("", ctrl.Generate)
		group.GET("/ws", ctrl.WatchTasks)
		group.GET("/tasks", ctrl.ListTasks)
		group.GET("/tasks/:taskId", ctrl.GetTask)
		group.DELETE("/tasks/:taskId", ctrl.DeleteTask)
		group.POST("/export", ctrl.Export)
	}
}
