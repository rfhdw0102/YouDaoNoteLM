package file

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册文件代理路由（公开访问，头像在 <img src> 中加载无法携带鉴权头）
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	fileGroup := r.Group("/files")
	{
		// 通配符 *objectName 捕获含 "/" 的对象路径，如 /api/v1/files/avatar/avatars/1.png
		fileGroup.GET("/avatar/*objectName", ctrl.GetAvatar)
	}
}
