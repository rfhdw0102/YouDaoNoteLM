// internal/service/search_agent_service.go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

type searchAgentService struct {
	configService ConfigService
	importer      ImporterService
	searchAgent   SearchAgentInterface
}

// NewSearchAgentService 创建搜索 Agent 服务
func NewSearchAgentService(
	configService ConfigService,
	importer ImporterService,
	searchAgent SearchAgentInterface,
) SearchAgentService {
	return &searchAgentService{
		configService: configService,
		importer:      importer,
		searchAgent:   searchAgent,
	}
}

// Search 智能搜索
func (s *searchAgentService) Search(userID, notebookID uint, query string) (*response.SearchResponse, error) {
	// 执行 Agent
	ctx := context.Background()
	result, err := s.searchAgent.Execute(ctx, userID, notebookID, query)
	if err != nil {
		return nil, err
	}

	// 解析 Agent 结果为 SearchResponse
	return parseAgentResult(result.Content, result.SearchRounds)
}

// SearchStream 智能搜索（流式）：返回事件 channel，用于 SSE 推送
func (s *searchAgentService) SearchStream(userID, notebookID uint, query string) <-chan *SearchAgentEvent {
	ctx := context.Background()
	return s.searchAgent.ExecuteStream(ctx, userID, notebookID, query)
}

// ImportFromURL URL 直接导入（返回任务 ID 和 Source ID）
func (s *searchAgentService) ImportFromURL(userID, notebookID uint, url string) (string, uint, error) {
	taskID, sourceIDs, err := s.importer.ImportSearchResults(userID, notebookID, []SearchResultItem{
		{URL: url},
	})
	if err != nil {
		return "", 0, err
	}
	if len(sourceIDs) == 0 {
		return "", 0, fmt.Errorf("导入失败：未创建Source记录")
	}

	logger.Info("URL导入任务已创建",
		zap.Uint("user_id", userID),
		zap.String("url", url),
		zap.String("task_id", taskID),
		zap.Uint("source_id", sourceIDs[0]),
	)

	return taskID, sourceIDs[0], nil
}

// ImportSearchResults 批量导入
func (s *searchAgentService) ImportSearchResults(userID, notebookID uint, items []SearchResultItem) (string, []uint, error) {
	return s.importer.ImportSearchResults(userID, notebookID, items)
}

// SearchAndImport 搜索并自动导入（主Agent调用模式）
func (s *searchAgentService) SearchAndImport(userID, notebookID uint, query string) (*response.SearchResponse, error) {
	// 执行 Agent（自动导入模式）
	ctx := context.Background()
	result, err := s.searchAgent.ExecuteWithImport(ctx, userID, notebookID, query)
	if err != nil {
		return nil, err
	}

	// 解析 Agent 结果为 SearchResponse
	return parseAgentResult(result.Content, result.SearchRounds)
}

// parseAgentResult 解析 Agent 返回的内容为 SearchResponse
func parseAgentResult(content string, searchRounds int) (*response.SearchResponse, error) {
	// 尝试从 Agent 回复中提取 JSON 代码块
	if jsonBlock := extractJSONBlock(content); jsonBlock != "" {
		var result response.SearchResponse
		if err := json.Unmarshal([]byte(jsonBlock), &result); err == nil {
			result.SearchRounds = searchRounds
			// summary 如果为空，用前面的文本
			if result.Summary == "" {
				result.Summary = extractTextBeforeJSON(content)
			}
			return &result, nil
		}
	}

	// 尝试直接解析整个内容为 JSON
	var result response.SearchResponse
	if err := json.Unmarshal([]byte(content), &result); err == nil {
		return &result, nil
	}

	// 如果 Agent 没有返回结构化 JSON，则将整个内容作为 summary
	result = response.SearchResponse{
		Results:      []response.SearchResultItem{},
		Summary:      content,
		SearchRounds: searchRounds,
	}

	// 尝试从文本中提取 URL 作为结果
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			result.Results = append(result.Results, response.SearchResultItem{
				URL: line,
			})
		}
	}

	// 如果没有提取到 URL，尝试从 Markdown 链接中提取
	if len(result.Results) == 0 {
		for _, line := range lines {
			if idx := strings.Index(line, "]("); idx != -1 {
				start := idx + 2
				end := strings.Index(line[start:], ")")
				if end != -1 {
					url := line[start : start+end]
					if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
						result.Results = append(result.Results, response.SearchResultItem{
							URL: url,
						})
					}
				}
			}
		}
	}

	return &result, nil
}

// extractJSONBlock 从文本中提取 ```json ... ``` 代码块
func extractJSONBlock(content string) string {
	startMarker := "```json"
	endMarker := "```"

	startIdx := strings.Index(content, startMarker)
	if startIdx == -1 {
		// 尝试 ```  开头
		startMarker = "```"
		startIdx = strings.Index(content, startMarker)
		if startIdx == -1 {
			return ""
		}
		// 跳过 ```\n
		startIdx += len(startMarker)
		if startIdx < len(content) && content[startIdx] == '\n' {
			startIdx++
		}
	} else {
		startIdx += len(startMarker)
		if startIdx < len(content) && content[startIdx] == '\n' {
			startIdx++
		}
	}

	endIdx := strings.Index(content[startIdx:], endMarker)
	if endIdx == -1 {
		return ""
	}

	return strings.TrimSpace(content[startIdx : startIdx+endIdx])
}

// extractTextBeforeJSON 提取 JSON 代码块之前的文本
func extractTextBeforeJSON(content string) string {
	idx := strings.Index(content, "```json")
	if idx == -1 {
		idx = strings.Index(content, "```")
	}
	if idx == -1 {
		return content
	}
	return strings.TrimSpace(content[:idx])
}
