package file

import (
	"path/filepath"
	"strings"

	"YoudaoNoteLm/internal/service/external/storage"
	"YoudaoNoteLm/pkg/logger"
	"YoudaoNoteLm/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Controller 文件代理控制器（presign 失败时的降级访问入口）
type Controller struct {
	storage storage.FileStorage
}

// NewController 创建文件代理控制器
func NewController(storage storage.FileStorage) *Controller {
	return &Controller{storage: storage}
}

// GetAvatar 通过后端代理访问 MinIO 中的头像文件
// presign 失败时返回的降级 URL 指向此 handler，避免浏览器直接请求 raw objectName 导致 404
// GET /api/v1/files/avatar/*objectName （通配符捕获含 "/" 的 objectName，如 avatars/1.png）
func (ctrl *Controller) GetAvatar(c *gin.Context) {
	// *objectName 通配符捕获的值带前导 "/"，需要去掉
	objectName := strings.TrimPrefix(c.Param("objectName"), "/")
	if objectName == "" {
		response.NotFound(c, "文件不存在")
		return
	}

	data, err := ctrl.storage.Download(objectName)
	if err != nil {
		logger.Warn("代理下载头像失败",
			zap.String("object", objectName),
			zap.Error(err),
		)
		response.NotFound(c, "文件不存在")
		return
	}

	// 根据扩展名推断 Content-Type
	contentType := contentTypeByExt(filepath.Ext(objectName))
	c.Data(200, contentType, data)
}

// contentTypeByExt 根据扩展名返回 Content-Type
func contentTypeByExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}
