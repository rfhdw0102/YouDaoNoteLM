package youdao

import (
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/service"
	externalYoudao "YoudaoNoteLm/internal/service/external/youdao"
	"YoudaoNoteLm/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 有道云笔记控制器
type Controller struct {
	youdaoService service.YoudaoService
}

// NewController 创建有道云笔记控制器
func NewController(youdaoService service.YoudaoService) *Controller {
	return &Controller{youdaoService: youdaoService}
}

// Bind 绑定有道 API Key
func (ctrl *Controller) Bind(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req BindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请提供有效的 API Key")
		return
	}

	if err := ctrl.youdaoService.Bind(userID, req.APIKey); err != nil {
		response.BizError(c, err)
		return
	}

	response.SuccessWithMessage(c, "绑定成功", nil)
}

// Unbind 解绑有道账号
func (ctrl *Controller) Unbind(c *gin.Context) {
	userID := middleware.GetUserID(c)

	if err := ctrl.youdaoService.Unbind(userID); err != nil {
		response.BizError(c, err)
		return
	}

	response.SuccessWithMessage(c, "解绑成功", nil)
}

// GetBinding 查询绑定状态
func (ctrl *Controller) GetBinding(c *gin.Context) {
	userID := middleware.GetUserID(c)

	binding, err := ctrl.youdaoService.GetBinding(userID)
	if err != nil {
		response.BizError(c, err)
		return
	}

	if binding == nil {
		response.Success(c, gin.H{
			"bound":   false,
			"user_id": userID,
			"debug":   "binding is nil",
		})
		return
	}

	response.Success(c, gin.H{
		"bound":   binding.Status == "active",
		"status":  binding.Status,
		"user_id": binding.UserID,
		"has_key": binding.APIKey != "",
	})
}

// ListNotes 浏览有道云笔记目录
func (ctrl *Controller) ListNotes(c *gin.Context) {
	userID := middleware.GetUserID(c)
	folderID := c.Query("folderId")

	items, err := ctrl.youdaoService.ListNotes(userID, folderID)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, items)
}

// ImportNote 单篇导入有道云笔记
func (ctrl *Controller) ImportNote(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req ImportNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请提供有效的导入参数")
		return
	}

	source, err := ctrl.youdaoService.ImportNote(userID, req.NotebookID, req.FileID)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, source)
}

// ImportBatch 批量导入有道云笔记
func (ctrl *Controller) ImportBatch(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req ImportBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请提供有效的导入参数")
		return
	}

	taskID, sourceIDs, err := ctrl.youdaoService.ImportNotesBatch(userID, req.NotebookID, req.FileIDs, req.FileNames)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, gin.H{
		"task_id":    taskID,
		"source_ids": sourceIDs,
	})
}

// 确保 youdao 包被引用（用于 Swagger 文档中的类型引用）
var _ *externalYoudao.NoteItem
