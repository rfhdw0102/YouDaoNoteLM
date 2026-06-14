package search

import (
	"context"
	"strconv"

	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/pkg/eino"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 联网搜索控制器。
type Controller struct {
	importerService service.ImporterService
	tokenBlacklist  service.TokenBlacklistService
	runner          *eino.WebSearchRunner
	runnerErr       error
}

// NewController 创建联网搜索控制器。
func NewController(
	searchService service.SearchService,
	importerService service.ImporterService,
	tokenBlacklist service.TokenBlacklistService,
) *Controller {
	runner, err := eino.NewWebSearchRunner(context.Background(), searchService)
	return &Controller{
		importerService: importerService,
		tokenBlacklist:  tokenBlacklist,
		runner:          runner,
		runnerErr:       err,
	}
}

// Preview 预览联网搜索结果。
func (ctrl *Controller) Preview(c *gin.Context) {
	if !ctrl.ensureRunner(c) {
		return
	}

	userID := middleware.GetUserID(c)
	nbID64, err := strconv.ParseUint(c.Param("nbId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的笔记本ID")
		return
	}

	var req request.SearchPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := ctrl.runner.Search(c.Request.Context(), &service.SearchRequest{
		UserID:         userID,
		Scene:          service.SearchSceneImport,
		Query:          req.Query,
		Freshness:      req.Freshness,
		Count:          req.Count,
		NeedSummary:    req.NeedSummary,
		NeedContent:    req.NeedContent,
		Language:       req.Language,
		AllowedDomains: req.AllowedDomains,
		BlockedDomains: req.BlockedDomains,
		NotebookID:     uint(nbID64),
		TraceID:        req.TraceID,
		AllowDegrade:   req.AllowDegrade,
	})
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, resp)
}

// Import 导入选中的搜索结果 URL。
func (ctrl *Controller) Import(c *gin.Context) {
	userID := middleware.GetUserID(c)
	nbID64, err := strconv.ParseUint(c.Param("nbId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的笔记本ID")
		return
	}

	var req request.SearchImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if len(req.URLs) == 0 {
		response.BizError(c, bizerrors.New(bizerrors.CodeInvalidParam, "urls 不能为空"))
		return
	}

	taskID, err := ctrl.importerService.ImportSearchResults(userID, uint(nbID64), req.URLs)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, gin.H{"task_id": taskID})
}

func (ctrl *Controller) ensureRunner(c *gin.Context) bool {
	if ctrl.runner != nil && ctrl.runnerErr == nil {
		return true
	}
	response.BizError(c, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "搜索执行器初始化失败", ctrl.runnerErr))
	return false
}
