package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"YoudaoNoteLm/internal/feedback"
	"YoudaoNoteLm/pkg/response"

	"github.com/gin-gonic/gin"
)

// FeedbackController handles admin feedback API endpoints.
type FeedbackController struct {
	adminReader feedback.AdminReader
}

// NewFeedbackController creates an admin feedback controller.
func NewFeedbackController(adminReader feedback.AdminReader) *FeedbackController {
	return &FeedbackController{adminReader: adminReader}
}

// parseAdminFilter parses common admin filter parameters from the query string.
func parseAdminFilter(c *gin.Context) (feedback.AdminFilter, error) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		return feedback.AdminFilter{}, fmt.Errorf("from 和 to 参数为必填（RFC3339 UTC）")
	}

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		return feedback.AdminFilter{}, fmt.Errorf("from 格式错误，需 RFC3339 UTC: %w", err)
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		return feedback.AdminFilter{}, fmt.Errorf("to 格式错误，需 RFC3339 UTC: %w", err)
	}

	// Max 31 days
	if to.Sub(from) > 31*24*time.Hour {
		return feedback.AdminFilter{}, fmt.Errorf("时间范围不能超过 31 天")
	}
	if to.Before(from) {
		return feedback.AdminFilter{}, fmt.Errorf("to 必须晚于 from")
	}

	filter := feedback.AdminFilter{From: from, To: to}

	if ratingStr := c.Query("rating"); ratingStr != "" {
		r := feedback.Rating(ratingStr)
		if err := feedback.ValidateRating(r); err != nil {
			return feedback.AdminFilter{}, err
		}
		filter.Rating = &r
	}

	if reasonStr := c.Query("reason"); reasonStr != "" {
		r := feedback.ReasonCode(reasonStr)
		if err := feedback.ValidateReason(r); err != nil {
			return feedback.AdminFilter{}, err
		}
		filter.Reason = &r
	}

	return filter, nil
}

// Overview returns aggregate feedback statistics.
func (ctrl *FeedbackController) Overview(c *gin.Context) {
	filter, err := parseAdminFilter(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	overview, err := ctrl.adminReader.Overview(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, "查询反馈概览失败")
		return
	}

	response.Success(c, overview)
}

// List returns a paginated list of desensitized feedback items.
func (ctrl *FeedbackController) List(c *gin.Context) {
	filter, err := parseAdminFilter(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if size < 1 {
		size = 1
	}
	if size > 100 {
		size = 100
	}

	items, total, err := ctrl.adminReader.ListForAdmin(c.Request.Context(), filter, page, size)
	if err != nil {
		response.InternalError(c, "查询反馈列表失败")
		return
	}

	response.Success(c, response.NewPageResponse(items, total, page, size))
}

// ExportCSV exports filtered feedback as a UTF-8 CSV file.
func (ctrl *FeedbackController) ExportCSV(c *gin.Context) {
	filter, err := parseAdminFilter(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	rows, err := ctrl.adminReader.WriteCSV(c.Request.Context(), filter, c.Writer)
	if err != nil {
		// Check if it's the row limit error
		response.BadRequest(c, err.Error())
		return
	}

	filename := fmt.Sprintf("answer-feedback-%s.csv",
		filter.From.UTC().Format("2006-01-02T150405Z"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Cache-Control", "no-store")
	c.Header("X-Export-Rows", strconv.Itoa(rows))
	c.Status(http.StatusOK)
}
