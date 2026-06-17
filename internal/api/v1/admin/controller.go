package admin

import (
	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/pkg/response"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type Controller struct {
	adminService service.AdminService
}

func NewController(adminService service.AdminService) *Controller {
	return &Controller{adminService: adminService}
}

func (ctrl *Controller) ListUsers(c *gin.Context) {
	keyword := c.Query("keyword")
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	size, err := strconv.Atoi(c.DefaultQuery("size", "10"))
	if err != nil || size < 1 {
		size = 10
	}

	users, total, err := ctrl.adminService.ListUsers(page, size, keyword)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, response.NewPageResponse(users, total, page, size))
}

func (ctrl *Controller) UpdateUserStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	var req request.UserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, response.ParseValidationErrors(err))
		return
	}

	if err := ctrl.adminService.UpdateUserStatus(uint(id), req.Enabled); err != nil {
		response.BizError(c, err)
		return
	}

	response.SuccessWithMessage(c, "更新成功", nil)
}

func (ctrl *Controller) GetConfigs(c *gin.Context) {
	group := c.Param("group")
	configs, err := ctrl.adminService.GetConfigs(group)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, configs)
}

func (ctrl *Controller) UpdateConfig(c *gin.Context) {
	group := c.Param("group")
	key := c.Param("key")

	var req request.ConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, response.ParseValidationErrors(err))
		return
	}

	// 校验 config_value 不为空
	value := strings.TrimSpace(string(req.ConfigValue))
	if value == "" || value == "null" {
		response.BadRequest(c, "配置值不能为空")
		return
	}

	if err := ctrl.adminService.UpdateConfig(group, key, req.ConfigValue, req.Enabled); err != nil {
		response.BizError(c, err)
		return
	}
	response.SuccessWithMessage(c, "更新成功", nil)
}

func (ctrl *Controller) AddConfig(c *gin.Context) {
	group := c.Param("group")

	var req request.ConfigAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, response.ParseValidationErrors(err))
		return
	}

	// 去除空白后校验
	req.ConfigKey = strings.TrimSpace(req.ConfigKey)
	if req.ConfigKey == "" {
		response.BadRequest(c, "配置键不能为空")
		return
	}

	// 校验 config_value 不为空 JSON 对象
	value := strings.TrimSpace(string(req.ConfigValue))
	if value == "" || value == "{}" || value == "null" {
		response.BadRequest(c, "配置值不能为空")
		return
	}

	if err := ctrl.adminService.AddConfig(group, req.ConfigKey, req.ConfigValue, req.Description); err != nil {
		response.BizError(c, err)
		return
	}
	response.SuccessWithMessage(c, "新增成功", nil)
}

func (ctrl *Controller) GetConfigStatus(c *gin.Context) {
	status, err := ctrl.adminService.GetConfigStatus()
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, gin.H{"groups": status})
}

func (ctrl *Controller) DeleteConfig(c *gin.Context) {
	group := c.Param("group")
	key := c.Param("key")

	if err := ctrl.adminService.DeleteConfig(group, key); err != nil {
		response.BizError(c, err)
		return
	}
	response.SuccessWithMessage(c, "删除成功", nil)
}
