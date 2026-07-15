// internal/api/v1/search/controller.go
package search

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

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
		response.BadRequest(c, response.ParseValidationErrors(err))
		return
	}

	result, err := ctrl.searchService.Search(c.Request.Context(), userID, uint(nbID), req.Query)
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
		response.BadRequest(c, response.ParseValidationErrors(err))
		return
	}

	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // 禁用 Nginx 缓冲

	c.Stream(func(w io.Writer) bool {
		eventCh := ctrl.searchService.SearchStream(c.Request.Context(), userID, uint(nbID), req.Query)

		// 用 http.ResponseController 管理写超时和 Flush(返回 error)
		// Gin 1.10+ 的 ResponseWriter 支持 Unwrap,能让 RC 访问到底层连接的 SetWriteDeadline
		rc := http.NewResponseController(c.Writer)
		const writeTimeout = 10 * time.Second      // 单次写超时:TCP 写应在 1s 内,10s 发不出去说明客户端断网
		const heartbeatInterval = 15 * time.Second // 心跳间隔:在 LLM 调用期间(可能 10-30s)也能检测断开

		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()

		// writeSSE 发送一行 SSE 数据,带写超时检测。返回 false 表示客户端已断开,应退出。
		writeSSE := func(data string) bool {
			if err := rc.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
				logger.Warn("SSE 设置写超时失败(降级为无超时)", zap.Error(err))
			}
			if _, err := w.Write([]byte(data)); err != nil {
				logger.Info("SSE 写入失败,客户端可能已断开", zap.Error(err))
				return false
			}
			if err := rc.Flush(); err != nil {
				logger.Info("SSE Flush 失败,客户端可能已断开", zap.Error(err))
				return false
			}
			return true
		}

		for {
			select {
			case event, ok := <-eventCh:
				if !ok {
					return false // 搜索结束,eventCh 关闭
				}
				data, err := json.Marshal(event)
				if err != nil {
					logger.Warn("SSE 序列化事件失败", zap.Error(err))
					continue
				}
				if !writeSSE(fmt.Sprintf("data: %s\n\n", data)) {
					return false // 写失败/超时,客户端断开,退出触发 ctx cancel
				}
			case <-ticker.C:
				// SSE comment 心跳(: 开头的行),前端 fetch reader 只解析 data: 行,会忽略
				if !writeSSE(": heartbeat\n\n") {
					return false // 心跳写失败,客户端断开
				}
			}
		}
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
		response.BadRequest(c, response.ParseValidationErrors(err))
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
		response.BadRequest(c, response.ParseValidationErrors(err))
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
