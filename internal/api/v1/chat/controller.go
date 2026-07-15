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
	convService service.ConversationService
}

// NewController 创建对话控制器
func NewController(chatService service.ChatAgentService, convService service.ConversationService) *Controller {
	return &Controller{
		chatService: chatService,
		convService: convService,
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

	convID, err := ctrl.convService.CreateConversation(c.Request.Context(), userID, req.NotebookID, req.Title)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, gin.H{"id": convID})
}

// List 获取对话列表
func (ctrl *Controller) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	notebookID, err := strconv.ParseUint(c.Param("nbId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的笔记本 ID")
		return
	}

	convs, err := ctrl.convService.ListConversations(c.Request.Context(), userID, uint(notebookID))
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, convs)
}

// Get 获取对话详情
func (ctrl *Controller) Get(c *gin.Context) {
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

	conv, err := ctrl.convService.GetConversation(c.Request.Context(), userID, uint(convID))
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, conv)
}

// Update 更新对话
func (ctrl *Controller) Update(c *gin.Context) {
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

	var req request.UpdateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.convService.UpdateConversation(c.Request.Context(), userID, uint(convID), req.Title); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// Delete 删除对话
func (ctrl *Controller) Delete(c *gin.Context) {
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

	if err := ctrl.convService.DeleteConversation(c.Request.Context(), userID, uint(convID)); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// GetMessages 获取消息历史
func (ctrl *Controller) GetMessages(c *gin.Context) {
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

	msgs, err := ctrl.convService.GetMessages(c.Request.Context(), userID, uint(convID))
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

	// 注意：先调用 Service，等校验/锁/创建对话都通过再写 SSE 头，
	// 这样错误情况下能直接走普通 JSON 错误响应。
	eventCh, err := ctrl.chatService.ProcessMessageWithAgent(c.Request.Context(), &request.ProcessMessageRequest{
		ConversationID: uint(convID),
		NotebookID:     req.NotebookID,
		Content:        req.Content,
		SourceIDs:      req.SourceIDs,
		UserID:         userID,
		LLMConfigID:    req.LLMConfigID,
	})
	if err != nil {
		response.BizError(c, err)
		return
	}

	// 设置 SSE 头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

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

	if err := ctrl.chatService.StopGeneration(c.Request.Context(), userID, uint(convID)); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}
