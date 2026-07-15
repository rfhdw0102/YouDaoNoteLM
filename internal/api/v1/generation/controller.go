package generation

import (
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/pkg/logger"
	"YoudaoNoteLm/pkg/response"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type Controller struct {
	generationService     service.GenerationService
	generationTaskService service.GenerationTaskService
}

var generationTaskUpgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool {
		return true
	},
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

// 通过长连接推送任务快照和状态变更。
func (ctrl *Controller) WatchTasks(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "user is not authenticated")
		return
	}

	notebookID, ok := parseNotebookIDQuery(c)
	if !ok {
		return
	}

	conn, err := generationTaskUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Warn("upgrade generation task websocket failed", zap.Error(err))
		return
	}
	defer conn.Close()

	events, unsubscribe, err := ctrl.generationTaskService.SubscribeTasks(c.Request.Context(), userID, notebookID)
	if err != nil {
		_ = conn.WriteJSON(gin.H{"event": "error", "message": err.Error()})
		return
	}
	defer unsubscribe()

	tasks, err := ctrl.generationTaskService.ListTasks(c.Request.Context(), userID, notebookID, 100)
	if err != nil {
		_ = conn.WriteJSON(gin.H{"event": "error", "message": err.Error()})
		return
	}
	if err := conn.WriteJSON(gin.H{"event": "snapshot", "tasks": tasks}); err != nil {
		return
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()
	// 定期补发 snapshot：作为事件丢失的兜底。
	// 即使 eventHub channel 丢弃了事件、或后端重启导致订阅中断重连，
	// 前端也能在 15 秒内通过 snapshot 修正状态。
	snapshotTick := time.NewTicker(15 * time.Second)
	defer snapshotTick.Stop()

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			if err := conn.WriteJSON(event); err != nil {
				return
			}
		case <-snapshotTick.C:
			// 从 store 重新读取任务列表，推送完整 snapshot。
			// store 是任务状态的唯一真相源，snapshot 能修正任何丢失或错乱的事件。
			snapshotTasks, err := ctrl.generationTaskService.ListTasks(c.Request.Context(), userID, notebookID, 100)
			if err != nil {
				logger.Warn("push periodic generation task snapshot failed",
					zap.Uint("user_id", userID), zap.Uint("notebook_id", notebookID), zap.Error(err))
				continue
			}
			if err := conn.WriteJSON(gin.H{"event": "snapshot", "tasks": snapshotTasks}); err != nil {
				return
			}
		case <-ping.C:
			if err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second)); err != nil {
				return
			}
		case <-done:
			return
		case <-c.Request.Context().Done():
			return
		}
	}
}

// 取消等待中或运行中的生成任务。
func (ctrl *Controller) DeleteTask(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "user is not authenticated")
		return
	}

	taskID := c.Param("taskId")
	if err := ctrl.generationTaskService.CancelTask(c.Request.Context(), userID, taskID); err != nil {
		response.BizError(c, err)
		return
	}

	response.SuccessWithMessage(c, "任务已停止", nil)
}

func parseNotebookIDQuery(c *gin.Context) (uint, bool) {
	var notebookID uint
	if raw := strings.TrimSpace(c.Query("notebook_id")); raw != "" {
		value, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			response.BadRequest(c, "invalid notebook_id")
			return 0, false
		}
		notebookID = uint(value)
	}
	return notebookID, true
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
