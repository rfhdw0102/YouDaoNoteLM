package generation

import (
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers generation routes.
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup, tokenBlacklist service.TokenBlacklistService, statusCheck gin.HandlerFunc) {
	group := r.Group("/generations")
	group.Use(middleware.Auth(tokenBlacklist), statusCheck)
	{
		group.POST("", ctrl.Generate)
		group.POST("/export", ctrl.Export)
	}
}
