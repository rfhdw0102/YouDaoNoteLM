package feedback

import (
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers the feedback routes under the given router group.
func (ctrl *Controller) RegisterRoutes(rg *gin.RouterGroup, tokenBlacklist service.TokenBlacklistService, statusCheck gin.HandlerFunc) {
	chat := rg.Group("/chat")
	chat.Use(middleware.Auth(tokenBlacklist), statusCheck)
	{
		chat.PUT("/messages/:messageId/feedback", ctrl.Upsert)
		chat.DELETE("/messages/:messageId/feedback", ctrl.Delete)
	}
}
