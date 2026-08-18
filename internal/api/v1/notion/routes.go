package notion

import (
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/service"
	"github.com/gin-gonic/gin"
)

func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup, tokenBlacklist service.TokenBlacklistService, statusCheck gin.HandlerFunc) {
	r.GET("/notion/oauth/callback", ctrl.OAuthCallback)
	notionGroup := r.Group("/notion")
	notionGroup.Use(middleware.Auth(tokenBlacklist), statusCheck)
	{
		notionGroup.GET("/oauth/start", ctrl.StartOAuth)
		notionGroup.GET("/bind", ctrl.GetBinding)
		notionGroup.DELETE("/bind", ctrl.Unbind)
		notionGroup.GET("/pages", ctrl.ListPages)
		notionGroup.POST("/import/batch", ctrl.ImportBatch)
		notionGroup.GET("/import/tasks/:taskId", ctrl.GetImportTask)
		notionGroup.DELETE("/import/tasks/:taskId", ctrl.CancelImportTask)
	}
}
