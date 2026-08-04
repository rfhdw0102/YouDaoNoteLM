package feedback

import (
	"strconv"

	"YoudaoNoteLm/internal/feedback"
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller handles user feedback API endpoints.
type Controller struct {
	feedbackSvc feedback.Service
}

// NewController creates a feedback controller.
func NewController(feedbackSvc feedback.Service) *Controller {
	return &Controller{feedbackSvc: feedbackSvc}
}

// Upsert creates or replaces the current user's feedback for a message.
func (ctrl *Controller) Upsert(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	messageID, err := strconv.ParseUint(c.Param("messageId"), 10, 64)
	if err != nil || messageID == 0 {
		response.BadRequest(c, "无效的消息 ID")
		return
	}

	var req request.UpsertFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	rating := feedback.Rating(req.Rating)
	reason := feedback.ReasonCode(req.Reason)

	fb, err := ctrl.feedbackSvc.Upsert(c.Request.Context(), userID, uint(messageID), rating, reason)
	if err != nil {
		if feedback.IsValidationError(err) {
			response.BadRequest(c, err.Error())
			return
		}
		if err == feedback.ErrAnswerNotFound {
			response.NotFound(c, "回答不存在")
			return
		}
		response.InternalError(c, "保存反馈失败")
		return
	}

	response.Success(c, gin.H{
		"rating":     fb.Rating,
		"reason":     fb.Reason,
		"updated_at": fb.UpdatedAt,
	})
}

// Delete removes the current user's feedback for a message.
func (ctrl *Controller) Delete(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	messageID, err := strconv.ParseUint(c.Param("messageId"), 10, 64)
	if err != nil || messageID == 0 {
		response.BadRequest(c, "无效的消息 ID")
		return
	}

	if err := ctrl.feedbackSvc.Delete(c.Request.Context(), userID, uint(messageID)); err != nil {
		if feedback.IsValidationError(err) {
			response.BadRequest(c, err.Error())
			return
		}
		if err == feedback.ErrAnswerNotFound {
			response.NotFound(c, "回答不存在")
			return
		}
		response.InternalError(c, "删除反馈失败")
		return
	}

	response.Success(c, nil)
}
