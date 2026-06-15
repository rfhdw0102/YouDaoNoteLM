package generation

import (
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers generation routes.
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup, tokenBlacklist service.TokenBlacklistService) {
	group := r.Group("/generations")
	group.Use(middleware.Auth(tokenBlacklist))
	{
		group.POST("", ctrl.Generate)
	}
}
