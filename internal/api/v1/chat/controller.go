package chat

import (
	"io"
	"strconv"

	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 对话控制器
type Controller struct {
	chatService service.ChatAgentService
}

// NewController 创建对话控制器
func NewController(chatService service.ChatAgentService) *Controller {
	return &Controller{
		chatService: chatService,
	}
}

// Create 创建对话
func (ctrl *Controller) Create(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	title := req.Title
	if title == "" {
		title = "新对话"
	}

	convID, err := ctrl.chatService.CreateConversation(c.Request.Context(), userID, req.NotebookID, title)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, gin.H{"id": convID})
}

// List 获取对话列表
func (ctrl *Controller) List(c *gin.Context) {
	notebookID, err := strconv.ParseUint(c.Param("nbId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的笔记本 ID")
		return
	}

	convs, err := ctrl.chatService.ListConversations(c.Request.Context(), uint(notebookID))
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, convs)
}

// Get 获取对话详情
func (ctrl *Controller) Get(c *gin.Context) {
	convID, err := strconv.ParseUint(c.Param("convId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的对话 ID")
		return
	}

	conv, err := ctrl.chatService.GetConversation(c.Request.Context(), uint(convID))
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, conv)
}

// Update 更新对话
func (ctrl *Controller) Update(c *gin.Context) {
	convID, err := strconv.ParseUint(c.Param("convId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的对话 ID")
		return
	}

	var req request.UpdateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.chatService.UpdateConversation(c.Request.Context(), uint(convID), req.Title); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// Delete 删除对话
func (ctrl *Controller) Delete(c *gin.Context) {
	convID, err := strconv.ParseUint(c.Param("convId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的对话 ID")
		return
	}

	if err := ctrl.chatService.DeleteConversation(c.Request.Context(), uint(convID)); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// GetMessages 获取消息历史
func (ctrl *Controller) GetMessages(c *gin.Context) {
	convID, err := strconv.ParseUint(c.Param("convId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的对话 ID")
		return
	}

	msgs, err := ctrl.chatService.GetMessages(c.Request.Context(), uint(convID))
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, msgs)
}

// SendMessage 发送消息（Agent 模式，SSE 流式响应）
func (ctrl *Controller) SendMessage(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	convID, err := strconv.ParseUint(c.Param("convId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的对话 ID")
		return
	}

	var req request.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 设置 SSE 头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	eventCh, err := ctrl.chatService.ProcessMessageWithAgent(c.Request.Context(), &request.ProcessMessageRequest{
		ConversationID: uint(convID),
		Content:        req.Content,
		SourceIDs:      req.SourceIDs,
		UserID:         userID,
	})
	if err != nil {
		c.SSEvent("error", gin.H{"content": err.Error()})
		return
	}

	// 流式输出
	c.Stream(func(w io.Writer) bool {
		event, ok := <-eventCh
		if !ok {
			return false
		}
		c.SSEvent(event.Type, event)
		return true
	})
}

// StopGeneration 终止回答
func (ctrl *Controller) StopGeneration(c *gin.Context) {
	convID, err := strconv.ParseUint(c.Param("convId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的对话 ID")
		return
	}

	if err := ctrl.chatService.StopGeneration(c.Request.Context(), uint(convID)); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}
