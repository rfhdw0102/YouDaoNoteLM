package chat

import (
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/service"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册对话路由
func (ctrl *Controller) RegisterRoutes(rg *gin.RouterGroup, tokenBlacklist service.TokenBlacklistService) {
	chat := rg.Group("/chat")
	chat.Use(middleware.Auth(tokenBlacklist))
	{
		// 对话管理
		chat.POST("/conversations", ctrl.Create)
		chat.GET("/notebooks/:nbId/conversations", ctrl.List)
		chat.GET("/conversations/:convId", ctrl.Get)
		chat.PUT("/conversations/:convId", ctrl.Update)
		chat.DELETE("/conversations/:convId", ctrl.Delete)

		// 消息
		chat.GET("/conversations/:convId/messages", ctrl.GetMessages)
		chat.POST("/conversations/:convId/messages", ctrl.SendMessage)
		chat.POST("/conversations/:convId/stop", ctrl.StopGeneration)
	}
}
