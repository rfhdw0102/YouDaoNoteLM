package auth

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册认证路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	authGroup := r.Group("/auth")
	{
		authGroup.GET("/captcha", ctrl.GetCaptcha)
		authGroup.POST("/register", ctrl.Register)
		authGroup.POST("/login", ctrl.Login)
		authGroup.POST("/logout", ctrl.Logout)
		authGroup.POST("/send-code", ctrl.SendCode)
		authGroup.POST("/reset-password", ctrl.ResetPassword)
		authGroup.POST("/refresh", ctrl.RefreshToken)
	}
}
