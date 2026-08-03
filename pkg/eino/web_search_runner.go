package eino

import (
	"context"
	"encoding/json"
	"fmt"

	"YoudaoNoteLm/internal/service"
	bizerrors "YoudaoNoteLm/pkg/errors"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// WebSearchRunner 通过 Eino ToolsNode 调用统一搜索工具。
type WebSearchRunner struct {
	toolNode *compose.ToolsNode
}

// NewWebSearchRunner 创建基于 Eino 的搜索执行器。
func NewWebSearchRunner(ctx context.Context, searchService service.SearchService) (*WebSearchRunner, error) {
	searchTool, err := NewWebSearchTool(searchService)
	if err != nil {
		return nil, fmt.Errorf("create web search tool failed: %w", err)
	}

	toolNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: []tool.BaseTool{searchTool},
	})
	if err != nil {
		return nil, fmt.Errorf("create tool node failed: %w", err)
	}

	return &WebSearchRunner{toolNode: toolNode}, nil
}

// Search 通过 Eino ToolsNode 执行联网搜索。
func (r *WebSearchRunner) Search(ctx context.Context, req *service.SearchRequest) (*service.SearchResponse, error) {
	if r == nil || r.toolNode == nil {
		return nil, bizerrors.New(bizerrors.CodeInternalServiceError, "搜索执行器未初始化")
	}
	if req == nil {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "搜索请求不能为空")
	}

	arguments, err := json.Marshal(req)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "序列化搜索参数失败", err)
	}

	messages, err := r.toolNode.Invoke(ctx, schema.AssistantMessage("", []schema.ToolCall{
		{
			ID:   "web-search-call-1",
			Type: "function",
			Function: schema.FunctionCall{
				Name:      webSearchToolName,
				Arguments: string(arguments),
			},
		},
	}))
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "Eino 搜索编排执行失败", err)
	}
	if len(messages) == 0 {
		return nil, bizerrors.New(bizerrors.CodeInternalServiceError, "Eino 搜索编排未返回结果")
	}

	var resp service.SearchResponse
	if err := sonic.UnmarshalString(messages[0].Content, &resp); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "解析 Eino 搜索结果失败", err)
	}
	return &resp, nil
}
