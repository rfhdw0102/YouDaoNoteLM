// internal/api/v1/providers/routes.go
package providers

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册 provider 发现路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup, optionalAuth gin.HandlerFunc) {
	r.GET("/providers", ctrl.ListProviders)
	r.GET("/providers/active", optionalAuth, ctrl.GetActiveConfig)
}
