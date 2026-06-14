package user

import (
	"YoudaoNoteLm/internal/middleware"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册用户路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	userGroup := r.Group("/user")
	userGroup.Use(middleware.Auth(ctrl.tokenBlacklist))
	{
		userGroup.GET("/profile", ctrl.GetProfile)
		userGroup.PUT("/profile", ctrl.UpdateProfile)
		userGroup.PUT("/username", ctrl.UpdateUsername)
		userGroup.POST("/avatar", ctrl.UploadAvatar)
		userGroup.POST("/password", ctrl.ChangePassword)
		userGroup.DELETE("/account", ctrl.DeleteAccount)
		userGroup.GET("/list", ctrl.ListUsers)
	}
}
