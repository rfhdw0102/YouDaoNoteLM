package memory

import (
	"errors"

	"YoudaoNoteLm/internal/memory"
	"YoudaoNoteLm/internal/middleware"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/response"

	"github.com/gin-gonic/gin"
)

type Controller struct {
	service memory.Service
}

type upsertRequest struct {
	Content string `json:"content"`
}

func NewController(service memory.Service) *Controller {
	return &Controller{service: service}
}

func (ctrl *Controller) List(c *gin.Context) {
	preferences, err := ctrl.service.List(c.Request.Context(), middleware.GetUserID(c))
	if err != nil {
		ctrl.writeError(c, err)
		return
	}
	response.Success(c, preferences)
}

func (ctrl *Controller) Upsert(c *gin.Context) {
	var req upsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, bizerrors.CodeInvalidParam, "请求体格式错误")
		return
	}

	preference, err := ctrl.service.Upsert(
		c.Request.Context(),
		middleware.GetUserID(c),
		memory.Type(c.Param("type")),
		req.Content,
	)
	if err != nil {
		ctrl.writeError(c, err)
		return
	}
	response.Success(c, preference)
}

func (ctrl *Controller) Delete(c *gin.Context) {
	err := ctrl.service.Delete(
		c.Request.Context(),
		middleware.GetUserID(c),
		memory.Type(c.Param("type")),
	)
	if err != nil {
		ctrl.writeError(c, err)
		return
	}
	response.Success(c, nil)
}

func (ctrl *Controller) writeError(c *gin.Context, err error) {
	if errors.Is(err, memory.ErrInvalidUser) {
		response.Unauthorized(c, "用户未登录")
		return
	}
	if memory.IsValidationError(err) {
		response.Error(c, bizerrors.CodeInvalidParam, err.Error())
		return
	}
	response.BizError(c, err)
}
