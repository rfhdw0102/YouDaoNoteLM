package notion

import (
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/service"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/response"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type Controller struct {
	notionService       service.NotionService
	frontendRedirectURL string
}

func NewController(notionService service.NotionService, frontendRedirectURL string) *Controller {
	return &Controller{notionService: notionService, frontendRedirectURL: frontendRedirectURL}
}

func (ctrl *Controller) StartOAuth(c *gin.Context) {
	if ctrl.notionService == nil {
		response.BizError(c, bizerrors.ErrNotionNotConfigured)
		return
	}
	authorizeURL, err := ctrl.notionService.StartOAuth(c.Request.Context(), middleware.GetUserID(c))
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, gin.H{"authorize_url": authorizeURL})
}

func (ctrl *Controller) OAuthCallback(c *gin.Context) {
	result := service.OAuthCallbackResult{Reason: service.OAuthCallbackReasonInvalidState}
	var err error
	if ctrl.notionService == nil {
		err = bizerrors.ErrNotionNotConfigured
	} else {
		result, err = ctrl.notionService.HandleOAuthCallback(c.Request.Context(), c.Query("code"), c.Query("state"))
	}
	c.Redirect(http.StatusFound, ctrl.oauthRedirect(result, err))
}

func (ctrl *Controller) GetBinding(c *gin.Context) {
	binding, err := ctrl.notionService.GetBinding(c.Request.Context(), middleware.GetUserID(c))
	if err != nil {
		response.BizError(c, err)
		return
	}
	if binding == nil {
		response.Success(c, gin.H{"bound": false})
		return
	}
	response.Success(c, gin.H{
		"bound": binding.Status == "active", "status": binding.Status,
		"workspace_id": binding.WorkspaceID, "workspace_name": binding.WorkspaceName,
		"workspace_icon": binding.WorkspaceIcon,
	})
}

func (ctrl *Controller) Unbind(c *gin.Context) {
	if err := ctrl.notionService.Unbind(c.Request.Context(), middleware.GetUserID(c)); err != nil {
		response.BizError(c, err)
		return
	}
	response.SuccessWithMessage(c, "Notion 已解绑", nil)
}

func (ctrl *Controller) ListPages(c *gin.Context) {
	pageSize := 0
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			response.BadRequest(c, "page_size 参数无效")
			return
		}
		pageSize = parsed
	}
	pages, err := ctrl.notionService.ListPages(c.Request.Context(), middleware.GetUserID(c), c.Query("query"), c.Query("cursor"), pageSize)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, gin.H{"list": pages.Items, "next_cursor": pages.NextCursor, "has_more": pages.HasMore})
}

func (ctrl *Controller) ImportBatch(c *gin.Context) {
	var req BatchImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请提供有效的 Notion 导入参数")
		return
	}
	taskID, sourceIDs, err := ctrl.notionService.ImportPagesBatch(c.Request.Context(), middleware.GetUserID(c), req.NotebookID, req.PageIDs)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, gin.H{"task_id": taskID, "source_ids": sourceIDs})
}

func (ctrl *Controller) GetImportTask(c *gin.Context) {
	task, err := ctrl.notionService.GetImportTask(c.Request.Context(), middleware.GetUserID(c), c.Param("taskId"))
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, task)
}

func (ctrl *Controller) CancelImportTask(c *gin.Context) {
	if err := ctrl.notionService.CancelImportTask(c.Request.Context(), middleware.GetUserID(c), c.Param("taskId")); err != nil {
		response.BizError(c, err)
		return
	}
	response.SuccessWithMessage(c, "Notion 导入任务已取消", nil)
}

func (ctrl *Controller) oauthRedirect(result service.OAuthCallbackResult, err error) string {
	parsed, parseErr := url.Parse(ctrl.frontendRedirectURL)
	if parseErr != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ctrl.frontendRedirectURL
	}
	query := parsed.Query()
	query.Set("tab", "notion")
	if err == nil && result.Success {
		query.Set("notion_oauth", "success")
		query.Del("reason")
	} else {
		query.Set("notion_oauth", "error")
		reason := string(result.Reason)
		if reason == "" {
			reason = string(service.OAuthCallbackReasonExchangeFail)
		}
		query.Set("reason", reason)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
