package response

import (
	"YoudaoNoteLm/pkg/errors"
	"math"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(200, Response{
		Code:    errors.CodeSuccess,
		Message: errors.GetMessage(errors.CodeSuccess),
		Data:    data,
	})
}

// SuccessWithMessage 成功响应（自定义消息）
func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(200, Response{
		Code:    errors.CodeSuccess,
		Message: message,
		Data:    data,
	})
}

// Error 错误响应
func Error(c *gin.Context, code int, message string) {
	c.JSON(200, Response{
		Code:    code,
		Message: message,
	})
}

// ErrorWithData 错误响应（带数据）
func ErrorWithData(c *gin.Context, code int, message string, data interface{}) {
	c.JSON(200, Response{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// BizError 业务错误响应
func BizError(c *gin.Context, err error) {
	if bizErr, ok := err.(*errors.BizError); ok {
		c.JSON(200, Response{
			Code:    bizErr.Code,
			Message: bizErr.Message,
		})
		return
	}
	// 其他错误类型
	Error(c, errors.CodeInternalError, errors.GetMessage(errors.CodeInternalError))
}

// BadRequest 400 错误
func BadRequest(c *gin.Context, message string) {
	Error(c, errors.CodeBadRequest, message)
}

// Unauthorized 401 错误
func Unauthorized(c *gin.Context, message string) {
	Error(c, errors.CodeUnauthorized, message)
}

// Forbidden 403 错误
func Forbidden(c *gin.Context, message string) {
	Error(c, errors.CodeForbidden, message)
}

// NotFound 404 错误
func NotFound(c *gin.Context, message string) {
	Error(c, errors.CodeNotFound, message)
}

// InternalError 500 错误
func InternalError(c *gin.Context, message string) {
	Error(c, errors.CodeInternalError, message)
}

// PageRequest 分页请求参数
type PageRequest struct {
	Page int `form:"page" binding:"required,min=1"`         // 页码，从1开始
	Size int `form:"size" binding:"required,min=1,max=100"` // 每页大小，最大100
}

// PageResponse 分页响应结构
type PageResponse struct {
	List      interface{} `json:"list"`       // 数据列表
	Total     int64       `json:"total"`      // 总记录数
	Page      int         `json:"page"`       // 当前页码
	Size      int         `json:"size"`       // 每页大小
	TotalPage int         `json:"total_page"` // 总页数
}

// NewPageResponse 创建分页响应
func NewPageResponse(list interface{}, total int64, page, size int) *PageResponse {
	totalPage := int(math.Ceil(float64(total) / float64(size)))
	return &PageResponse{
		List:      list,
		Total:     total,
		Page:      page,
		Size:      size,
		TotalPage: totalPage,
	}
}
