package notebook

import (
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Controller 笔记本控制器
type Controller struct {
	notebookService service.NotebookService
}

// NewController 创建笔记本控制器
func NewController(notebookService service.NotebookService) *Controller {
	return &Controller{
		notebookService: notebookService,
	}
}

// Create 创建笔记本
func (ctrl *Controller) Create(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.CreateNotebookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	notebook, err := ctrl.notebookService.Create(userID, &req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, notebook)
}

// List 查询用户的所有笔记本
func (ctrl *Controller) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	notebooks, err := ctrl.notebookService.List(userID)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, notebooks)
}

// Rename 重命名笔记本
func (ctrl *Controller) Rename(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	notebookID64, err := strconv.ParseUint(c.Param("nbId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的笔记本 ID")
		return
	}
	notebookID := uint(notebookID64)

	var req request.RenameNotebookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.notebookService.Rename(userID, notebookID, &req); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// Delete 删除笔记本
func (ctrl *Controller) Delete(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	notebookID64, err := strconv.ParseUint(c.Param("nbId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的笔记本 ID")
		return
	}
	notebookID := uint(notebookID64)

	if err := ctrl.notebookService.Delete(userID, notebookID); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}
