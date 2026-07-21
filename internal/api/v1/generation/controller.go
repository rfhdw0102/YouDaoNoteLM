package generation

import (
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/pkg/logger"
	"YoudaoNoteLm/pkg/response"
	"mime"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Controller struct {
	generationService     service.GenerationService
	generationTaskService service.GenerationTaskService
}

// 创建生成模块控制器。
func NewController(generationService service.GenerationService, generationTaskService service.GenerationTaskService) *Controller {
	return &Controller{generationService: generationService, generationTaskService: generationTaskService}
}

// 提交内容生成任务。
func (ctrl *Controller) Generate(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "user is not authenticated")
		return
	}

	var req request.GenerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	task, err := ctrl.generationTaskService.Submit(c.Request.Context(), &service.GenerationRequest{
		UserID:       userID,
		NotebookID:   req.NotebookID,
		Markdown:     req.Markdown,
		Type:         service.GenerationType(req.Type),
		Prompt:       req.Prompt,
		Options:      req.Options,
		SourceIDs:    req.SourceIDs,
		UseWeb:       req.UseWeb,
		AllowDegrade: req.AllowDegrade,
	})
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, task)
}

// 查询指定生成任务。
func (ctrl *Controller) GetTask(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "user is not authenticated")
		return
	}

	taskID := c.Param("taskId")
	task, err := ctrl.generationTaskService.GetTask(c.Request.Context(), userID, taskID)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, task)
}

// 查询当前用户的生成任务列表。
// 前端通过此接口轮询任务状态，替代原有 WebSocket 实时推送。
func (ctrl *Controller) ListTasks(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "user is not authenticated")
		return
	}

	var notebookID uint
	if raw := strings.TrimSpace(c.Query("notebook_id")); raw != "" {
		value, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			response.BadRequest(c, "invalid notebook_id")
			return
		}
		notebookID = uint(value)
	}

	limit := 100
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			response.BadRequest(c, "invalid limit")
			return
		}
		limit = value
	}

	tasks, err := ctrl.generationTaskService.ListTasks(c.Request.Context(), userID, notebookID, limit)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, tasks)
}

// DeleteTask 删除生成任务：pending/running 状态先取消 worker，再删除持久化数据。
// 已终态任务直接删除。删除幂等：任务不存在视为成功。
func (ctrl *Controller) DeleteTask(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "user is not authenticated")
		return
	}

	taskID := c.Param("taskId")
	if err := ctrl.generationTaskService.DeleteTask(c.Request.Context(), userID, taskID); err != nil {
		response.BizError(c, err)
		return
	}

	response.SuccessWithMessage(c, "任务已删除", nil)
}

// 将生成内容导出为附件。
func (ctrl *Controller) Export(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "user is not authenticated")
		return
	}

	var req request.GenerationExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := ctrl.generationService.Export(c.Request.Context(), &service.GenerationExportRequest{
		Type:     service.GenerationType(req.Type),
		Content:  req.Content,
		Title:    req.Title,
		Template: req.Template,
	})
	if err != nil {
		logger.Warn("generation export failed",
			zap.Uint("user_id", userID),
			zap.String("type", req.Type),
			zap.Int("content_len", len(req.Content)),
			zap.Bool("contains_section", strings.Contains(strings.ToLower(req.Content), "<section")),
			zap.String("template", req.Template),
			zap.Error(err),
		)
		response.BizError(c, err)
		return
	}
	logger.Info("generation export completed",
		zap.Uint("user_id", userID),
		zap.String("type", req.Type),
		zap.Int("content_len", len(req.Content)),
		zap.Int("output_len", len(resp.Data)),
		zap.String("content_type", resp.ContentType),
		zap.String("filename", resp.Filename),
	)

	c.Header("Content-Type", resp.ContentType)
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": resp.Filename}))
	c.Data(200, resp.ContentType, resp.Data)
}
