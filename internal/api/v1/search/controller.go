// internal/api/v1/search/controller.go
package search

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/pkg/logger"
	"YoudaoNoteLm/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Controller 搜索控制器
type Controller struct {
	searchService  service.SearchAgentService
	tokenBlacklist service.TokenBlacklistService
}

// NewController 创建搜索控制器
func NewController(searchService service.SearchAgentService, tokenBlacklist service.TokenBlacklistService) *Controller {
	return &Controller{searchService: searchService, tokenBlacklist: tokenBlacklist}
}

// Search 智能搜索
func (ctrl *Controller) Search(c *gin.Context) {
	userID := middleware.GetUserID(c)
	nbID, err := strconv.ParseUint(c.Param("nbId"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的笔记本ID")
		return
	}

	var req request.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := ctrl.searchService.Search(userID, uint(nbID), req.Query)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, result)
}

// SearchStream 智能搜索（SSE 流式返回）
func (ctrl *Controller) SearchStream(c *gin.Context) {
	userID := middleware.GetUserID(c)
	nbID, err := strconv.ParseUint(c.Param("nbId"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的笔记本ID")
		return
	}

	var req request.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // 禁用 Nginx 缓冲

	c.Stream(func(w io.Writer) bool {
		eventCh := ctrl.searchService.SearchStream(userID, uint(nbID), req.Query)
		for event := range eventCh {
			data, err := json.Marshal(event)
			if err != nil {
				logger.Warn("SSE 序列化事件失败", zap.Error(err))
				continue
			}
			_, err = fmt.Fprintf(w, "data: %s\n\n", data)
			if err != nil {
				return false
			}
			c.Writer.Flush()
		}
		return false
	})
}

// ImportFromURL URL 直接导入
func (ctrl *Controller) ImportFromURL(c *gin.Context) {
	userID := middleware.GetUserID(c)
	nbID, err := strconv.ParseUint(c.Param("nbId"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的笔记本ID")
		return
	}

	var req request.URLImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	taskID, sourceID, err := ctrl.searchService.ImportFromURL(userID, uint(nbID), req.URL)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, gin.H{"task_id": taskID, "source_id": sourceID})
}

// ImportSearchResults 批量导入搜索结果
func (ctrl *Controller) ImportSearchResults(c *gin.Context) {
	userID := middleware.GetUserID(c)
	nbID, err := strconv.ParseUint(c.Param("nbId"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的笔记本ID")
		return
	}

	var req request.SearchImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 自定义验证：URLs 和 Items 至少有一个
	if !req.Validate() {
		response.BadRequest(c, "导入列表为空，请提供 urls 或 items")
		return
	}

	// 转换为 service.SearchResultItem
	var items []service.SearchResultItem

	// 优先使用 Items（带标题）
	if len(req.Items) > 0 {
		items = make([]service.SearchResultItem, len(req.Items))
		for i, item := range req.Items {
			items[i] = service.SearchResultItem{
				Title: item.Title,
				URL:   item.URL,
			}
		}
	} else if len(req.URLs) > 0 {
		// 兼容旧接口：纯URL列表
		items = make([]service.SearchResultItem, len(req.URLs))
		for i, url := range req.URLs {
			items[i] = service.SearchResultItem{URL: url}
		}
	}

	taskID, sourceIDs, err := ctrl.searchService.ImportSearchResults(userID, uint(nbID), items)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, gin.H{"task_id": taskID, "source_ids": sourceIDs})
}
