package youdao

import (
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册有道云笔记路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup, tokenBlacklist service.TokenBlacklistService) {
	youdao := r.Group("/youdao")
	youdao.Use(middleware.Auth(tokenBlacklist))
	{
		// 绑定管理
		youdao.POST("/bind", ctrl.Bind)
		youdao.DELETE("/bind", ctrl.Unbind)
		youdao.GET("/bind", ctrl.GetBinding)

		// 浏览笔记
		youdao.GET("/notes", ctrl.ListNotes)

		// 导入
		youdao.POST("/import", ctrl.ImportNote)
		youdao.POST("/import/batch", ctrl.ImportBatch)
	}
}
