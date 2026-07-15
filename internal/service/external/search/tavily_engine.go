// internal/service/external/search/tavily_engine.go
package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const tavilyAPIURL = "https://api.tavily.com/search"

// TavilyEngine Tavily 搜索引擎
type TavilyEngine struct {
	apiKey string
	client *http.Client
}

// NewTavilyEngine 创建 Tavily 引擎
func NewTavilyEngine(apiKey string) *TavilyEngine {
	return &TavilyEngine{
		apiKey: apiKey,
		client: &http.Client{},
	}
}

// tavilyRequest Tavily API 请求
type tavilyRequest struct {
	APIKey        string `json:"api_key"`
	Query         string `json:"query"`
	MaxResults    int    `json:"max_results"`
	IncludeAnswer bool   `json:"include_answer,omitempty"`
}

// tavilyResponse Tavily API 响应
type tavilyResponse struct {
	Results []struct {
		Title   string  `json:"title"`
		URL     string  `json:"url"`
		Content string  `json:"content"`
		Score   float64 `json:"score"`
	} `json:"results"`
}

func (e *TavilyEngine) Name() string {
	return "tavily"
}

func (e *TavilyEngine) Search(query string, limit int) ([]SearchResultItem, error) {
	if e.apiKey == "" {
		return nil, fmt.Errorf("Tavily API key 未配置")
	}

	reqBody := tavilyRequest{
		APIKey:     e.apiKey,
		Query:      query,
		MaxResults: limit,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequest("POST", tavilyAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Tavily API 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Tavily API 返回错误 %d: %s", resp.StatusCode, string(body))
	}

	var result tavilyResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	items := make([]SearchResultItem, 0, len(result.Results))
	for _, r := range result.Results {
		items = append(items, SearchResultItem{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
		})
	}

	return items, nil
}
