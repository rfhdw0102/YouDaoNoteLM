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

// requiredResultCount 搜索结果硬性要求的条数
const requiredResultCount = 10

// WebSearchInput web_search 工具输入
type WebSearchInput struct {
	Query string `json:"query" jsonschema_description:"搜索关键词"`
	Limit int    `json:"limit" jsonschema_description:"返回结果数量，固定为 10（内部硬约束，传入值会被忽略）"`
}

// WebSearchOutput web_search 工具输出
type WebSearchOutput struct {
	Results any `json:"results"`
}

// NewWebSearchTool 创建 web_search 工具
func NewWebSearchTool(configService service.ConfigService) (tool.InvokableTool, error) {
	return utils.InferTool("web_search", "搜索网络内容。输入搜索关键词，返回恰好 10 条搜索结果（标题、URL、摘要）",
		func(ctx context.Context, input *WebSearchInput) (*WebSearchOutput, error) {
			// 硬约束（调用前）：强制 limit=10，不信任 LLM 传入的值
			limit := requiredResultCount

			userID := GetUserID(ctx)
			engine, err := configService.GetSearchEngine(userID)
			if err != nil {
				return nil, err
			}

			results, err := engine.Search(input.Query, limit)
			if err != nil {
				return nil, fmt.Errorf("搜索失败: %w", err)
			}

			// 硬约束（调用后）：校验返回数量
			got := len(results)
			if got > requiredResultCount {
				// 超过 10 条，截断到 10
				results = results[:requiredResultCount]
				logger.Warn("web_search 结果超过 10 条，已截断",
					zap.String("query", input.Query),
					zap.Int("original_count", got),
					zap.Int("truncated_to", requiredResultCount),
				)
				got = requiredResultCount
			} else if got < requiredResultCount {
				logger.Warn("web_search 结果不足 10 条",
					zap.String("query", input.Query),
					zap.Int("got", got),
					zap.Int("required", requiredResultCount),
				)
			}

			logger.Info("web_search 执行成功",
				zap.String("query", input.Query),
				zap.Int("results", got),
			)

			return &WebSearchOutput{Results: results}, nil
		},
	)
}
