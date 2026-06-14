package source

import (
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Controller 资料来源控制器
type Controller struct {
	sourceService  service.SourceService
	tokenBlacklist service.TokenBlacklistService
}

// NewController 创建来源控制器
func NewController(sourceService service.SourceService, tokenBlacklist service.TokenBlacklistService) *Controller {
	return &Controller{sourceService: sourceService, tokenBlacklist: tokenBlacklist}
}

// List 获取来源列表
func (ctrl *Controller) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	nbID64, err := strconv.ParseUint(c.Param("nbId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的笔记本ID")
		return
	}
	nbID := uint(nbID64)

	keyword := c.Query("keyword")
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	size, err := strconv.Atoi(c.DefaultQuery("size", "10"))
	if err != nil || size < 1 {
		size = 10
	}

	sources, total, err := ctrl.sourceService.List(userID, nbID, keyword, page, size)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, response.NewPageResponse(sources, total, page, size))
}

// GetByID 获取来源详情
func (ctrl *Controller) GetByID(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	id := uint(id64)

	source, err := ctrl.sourceService.GetByID(id)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, source)
}

// Rename 重命名来源
func (ctrl *Controller) Rename(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	id := uint(id64)

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.sourceService.Rename(id, req.Name); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// Delete 删除来源
func (ctrl *Controller) Delete(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	id := uint(id64)

	if err := ctrl.sourceService.Delete(id); err != nil {
		response.BizError(c, err)
		return
	}

	response.SuccessWithMessage(c, "删除成功", nil)
}

// BatchDelete 批量删除
func (ctrl *Controller) BatchDelete(c *gin.Context) {
	var req request.BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.sourceService.BatchDelete(req.IDs); err != nil {
		response.BizError(c, err)
		return
	}

	response.SuccessWithMessage(c, "批量删除成功", nil)
}

// GetContent 获取Markdown内容
func (ctrl *Controller) GetContent(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	id := uint(id64)

	content, err := ctrl.sourceService.GetContent(id)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, map[string]string{"content": content})
}

// GetOriginal 获取原格式内容
func (ctrl *Controller) GetOriginal(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	id := uint(id64)

	content, contentType, err := ctrl.sourceService.GetOriginalContent(id)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, map[string]string{
		"content": content,
		"type":    contentType,
	})
}
