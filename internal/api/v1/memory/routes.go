package memory

import (
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/service"

	"github.com/gin-gonic/gin"
)

func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup, tokenBlacklist service.TokenBlacklistService, statusCheck gin.HandlerFunc) {
	memories := r.Group("/user/memories")
	memories.Use(middleware.Auth(tokenBlacklist), statusCheck)
	{
		memories.GET("", ctrl.List)
		memories.PUT("/:type", ctrl.Upsert)
		memories.DELETE("/:type", ctrl.Delete)
	}
}
