// internal/agent/search/tools.go
package search

import (
	"context"
	"fmt"

	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/pkg/logger"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"go.uber.org/zap"
)

// ========== web_search 工具 ==========

// WebSearchInput web_search 工具输入
type WebSearchInput struct {
	Query string `json:"query" jsonschema_description:"搜索关键词"`
	Limit int    `json:"limit" jsonschema_description:"返回结果数量，默认10"`
}

// WebSearchOutput web_search 工具输出
type WebSearchOutput struct {
	Results any `json:"results"`
}

// NewWebSearchTool 创建 web_search 工具
func NewWebSearchTool(configService service.ConfigService) (tool.InvokableTool, error) {
	return utils.InferTool("web_search", "搜索网络内容。输入搜索关键词，返回搜索结果列表（标题、URL、摘要）",
		func(ctx context.Context, input *WebSearchInput) (*WebSearchOutput, error) {
			limit := input.Limit
			if limit <= 0 {
				limit = 10
			}

			userID := GetUserID(ctx)
			engine, err := configService.GetSearchEngine(userID)
			if err != nil {
				return nil, err
			}

			results, err := engine.Search(input.Query, limit)
			if err != nil {
				return nil, fmt.Errorf("搜索失败: %w", err)
			}

			logger.Info("web_search 执行成功",
				zap.String("query", input.Query),
				zap.Int("results", len(results)),
			)

			return &WebSearchOutput{Results: results}, nil
		},
	)
}
