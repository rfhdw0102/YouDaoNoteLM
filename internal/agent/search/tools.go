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

// ========== import_urls 工具 ==========

// ImportURLItem 导入项（包含标题和URL）
type ImportURLItem struct {
	Title string `json:"title" jsonschema_description:"网页标题"`
	URL   string `json:"url" jsonschema_description:"网页URL"`
}

// ImportURLsInput import_urls 工具输入
type ImportURLsInput struct {
	Items []ImportURLItem `json:"items" jsonschema_description:"要导入的网页列表，每项包含标题和URL"`
}

// ImportURLsOutput import_urls 工具输出
type ImportURLsOutput struct {
	TaskID   string `json:"task_id"`
	URLCount int    `json:"url_count"`
}

// NewImportURLsTool 创建 import_urls 工具
func NewImportURLsTool(importer service.ImporterService) (tool.InvokableTool, error) {
	return utils.InferTool("import_urls", "批量导入网页内容到资料库。输入网页列表（包含标题和URL），返回导入任务ID",
		func(ctx context.Context, input *ImportURLsInput) (*ImportURLsOutput, error) {
			if len(input.Items) == 0 {
				return nil, fmt.Errorf("导入列表为空")
			}

			userID := GetUserID(ctx)
			notebookID := GetNotebookID(ctx)

			// 转换为 SearchResultItem
			items := make([]service.SearchResultItem, len(input.Items))
			for i, item := range input.Items {
				items[i] = service.SearchResultItem{
					Title: item.Title,
					URL:   item.URL,
				}
			}

			taskID, sourceIDs, err := importer.ImportSearchResults(userID, notebookID, items)
			if err != nil {
				return nil, fmt.Errorf("导入失败: %w", err)
			}

			logger.Info("import_urls 执行成功",
				zap.Uint("user_id", userID),
				zap.Int("url_count", len(input.Items)),
				zap.String("task_id", taskID),
				zap.Any("source_ids", sourceIDs),
			)

			return &ImportURLsOutput{
				TaskID:   taskID,
				URLCount: len(input.Items),
			}, nil
		},
	)
}
