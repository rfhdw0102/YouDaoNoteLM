package importn

import (
	"strconv"

	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 导入控制器
type Controller struct {
	importerService service.ImporterService
}

// NewController 创建导入控制器
func NewController(importerService service.ImporterService) *Controller {
	return &Controller{importerService: importerService}
}

// ImportFile 文件上传导入
// @Summary 文件上传导入
// @Description 上传文件并导入到指定笔记本
// @Tags 导入
// @Accept multipart/form-data
// @Produce json
// @Param nbId path int true "笔记本ID"
// @Param file formData file true "文件"
// @Success 200 {object} response.Response{data=entity.Source}
// @Router /api/v1/notebooks/{nbId}/import/file [post]
func (ctrl *Controller) ImportFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	nbID, err := strconv.ParseUint(c.Param("nbId"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的笔记本ID")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "请选择要上传的文件")
		return
	}

	source, err := ctrl.importerService.ImportFile(userID, uint(nbID), file)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, source)
}

// PreviewAudio 音频上传转写预览
// @Summary 音频上传转写预览
// @Description 上传音频文件，立即返回 previewID，后台异步执行 ASR 转写
// @Tags 导入
// @Accept multipart/form-data
// @Produce json
// @Param nbId path int true "笔记本ID"
// @Param file formData file true "音频文件"
// @Success 200 {object} response.Response{data=response.AudioPreviewResponse}
// @Router /api/v1/notebooks/{nbId}/import/audio/preview [post]
func (ctrl *Controller) PreviewAudio(c *gin.Context) {
	userID := middleware.GetUserID(c)
	nbID, err := strconv.ParseUint(c.Param("nbId"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的笔记本ID")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "请选择要上传的音频文件")
		return
	}

	previewID, fileName, err := ctrl.importerService.PreviewAudio(userID, uint(nbID), file)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, gin.H{
		"preview_id": previewID,
		"file_name":  fileName,
		"status":     "pending",
	})
}

// GetAudioPreviewStatus 查询音频转写状态
// @Summary 查询音频转写状态
// @Description 根据 previewID 查询音频转写进度（前端轮询用）
// @Tags 导入
// @Produce json
// @Param previewId path string true "预览ID"
// @Success 200 {object} response.Response{data=cache.AudioPreview}
// @Router /api/v1/import/audio/preview/{previewId} [get]
func (ctrl *Controller) GetAudioPreviewStatus(c *gin.Context) {
	userID := middleware.GetUserID(c)
	previewID := c.Param("previewId")
	if previewID == "" {
		response.BadRequest(c, "无效的预览ID")
		return
	}

	preview, err := ctrl.importerService.GetAudioPreviewStatus(userID, previewID)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, preview)
}

// ConfirmAudio 确认音频导入
// @Summary 确认音频导入
// @Description 确认音频转写结果并导入
// @Tags 导入
// @Accept json
// @Produce json
// @Param request body request.AudioConfirmRequest true "确认请求"
// @Success 200 {object} response.Response{data=entity.Source}
// @Router /api/v1/import/audio/confirm [post]
func (ctrl *Controller) ConfirmAudio(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req request.AudioConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	source, err := ctrl.importerService.ConfirmAudio(userID, req.PreviewID, req.Content)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, source)
}

// GetTask 查询导入任务进度
// @Summary 查询导入任务进度
// @Description 根据任务ID查询导入进度
// @Tags 导入
// @Produce json
// @Param taskId path string true "任务ID"
// @Success 200 {object} response.Response{data=response.ImportTaskResponse}
// @Router /api/v1/import/tasks/{taskId} [get]
func (ctrl *Controller) GetTask(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		response.BadRequest(c, "无效的任务ID")
		return
	}

	task, err := ctrl.importerService.GetImportTask(taskID)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, task)
}

// DeleteTask 删除导入任务
// @Summary 删除导入任务
// @Description 根据任务ID删除导入任务（运行中的任务会被取消）
// @Tags 导入
// @Produce json
// @Param taskId path string true "任务ID"
// @Success 200 {object} response.Response
// @Router /api/v1/import/tasks/{taskId} [delete]
func (ctrl *Controller) DeleteTask(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		response.BadRequest(c, "无效的任务ID")
		return
	}

	if err := ctrl.importerService.DeleteImportTask(taskID); err != nil {
		response.BizError(c, err)
		return
	}

	response.SuccessWithMessage(c, "任务已删除", nil)
}
