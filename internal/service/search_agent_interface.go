// internal/service/search_agent_interface.go
package service

import (
	"context"

	"YoudaoNoteLm/internal/model/dto/response"
)

// SearchAgentInterface 搜索 Agent 执行接口（由 agent/search.SearchAgent 实现）
type SearchAgentInterface interface {
	// Execute 执行搜索任务（用户交互模式：只搜索不自动导入）
	Execute(ctx context.Context, userID, notebookID uint, task string) (*SearchAgentResult, error)
	// ExecuteStream 流式执行搜索任务，通过 channel 逐个推送事件，完成后关闭 channel
	ExecuteStream(ctx context.Context, userID, notebookID uint, task string) <-chan *SearchAgentEvent
	// ExecuteWithImport 执行搜索并自动导入任务（主Agent调用模式）
	ExecuteWithImport(ctx context.Context, userID, notebookID uint, task string) (*SearchAgentResult, error)
}

// SearchAgentResult Agent 执行结果（与 agent/search.AgentResult 对应）
type SearchAgentResult struct {
	Content      string `json:"content"`
	SearchRounds int    `json:"search_rounds"`
}

// SearchAgentEvent Agent 流式执行事件
type SearchAgentEvent struct {
	Type         string `json:"type"` // content, tool_call, search_round, error, done
	Content      string `json:"content,omitempty"`
	Role         string `json:"role,omitempty"`
	ToolName     string `json:"tool_name,omitempty"`
	ToolArgs     string `json:"tool_args,omitempty"`
	SearchRounds int    `json:"search_rounds,omitempty"`
	Error        string `json:"error,omitempty"`
	ErrorCode    int    `json:"error_code,omitempty"` // 错误码，用于前端精确判断错误类型
}

// SearchAgentService 搜索 Agent 服务接口
type SearchAgentService interface {
	// Search 智能搜索：Agent 自主执行多轮搜索+分析（用户交互模式，不自动导入）
	Search(userID, notebookID uint, query string) (*response.SearchResponse, error)
	// SearchStream 智能搜索（流式）：返回事件 channel，用于 SSE 推送
	SearchStream(userID, notebookID uint, query string) <-chan *SearchAgentEvent
	// ImportFromURL URL 直接导入（返回任务 ID 和 Source ID）
	ImportFromURL(userID, notebookID uint, url string) (taskID string, sourceID uint, err error)
	// ImportSearchResults 批量导入搜索结果（带标题），返回任务 ID 和创建的 Source ID 列表
	ImportSearchResults(userID, notebookID uint, items []SearchResultItem) (taskID string, sourceIDs []uint, err error)
	// SearchAndImport 搜索并自动导入：Agent 自主执行多轮搜索并自动导入结果（主Agent调用模式）
	SearchAndImport(userID, notebookID uint, query string) (*response.SearchResponse, error)
}
