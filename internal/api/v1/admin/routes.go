package admin

import (
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/service"

	"github.com/gin-gonic/gin"
)

func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup, blacklist service.TokenBlacklistService) {
	admin := r.Group("/admin", middleware.Auth(blacklist))
	{
		admin.GET("/users", ctrl.ListUsers)
		admin.PUT("/users/:id/status", ctrl.UpdateUserStatus)
		admin.GET("/config/status", ctrl.GetConfigStatus)
		admin.GET("/config/:group", ctrl.GetConfigs)
		admin.POST("/config/:group", ctrl.AddConfig)
		admin.PUT("/config/:group/:key", ctrl.UpdateConfig)
		admin.DELETE("/config/:group/:key", ctrl.DeleteConfig)
	}
}
