// internal/service/external/search/serper_engine.go
package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const serperAPIURL = "https://google.serper.dev/search"

// SerperEngine Serper 搜索引擎（Google 搜索代理）
type SerperEngine struct {
	apiKey string
	client *http.Client
}

// NewSerperEngine 创建 Serper 引擎
func NewSerperEngine(apiKey string) *SerperEngine {
	return &SerperEngine{
		apiKey: apiKey,
		client: &http.Client{},
	}
}

// serperRequest Serper API 请求
type serperRequest struct {
	Query      string `json:"q"`
	NumResults int    `json:"num"`
}

// serperResponse Serper API 响应
type serperResponse struct {
	Organic []struct {
		Title   string `json:"title"`
		Link    string `json:"link"`
		Snippet string `json:"snippet"`
	} `json:"organic"`
}

func (e *SerperEngine) Name() string {
	return "serper"
}

func (e *SerperEngine) Search(query string, limit int) ([]SearchResultItem, error) {
	if e.apiKey == "" {
		return nil, fmt.Errorf("Serper API key 未配置")
	}

	reqBody := serperRequest{
		Query:      query,
		NumResults: limit,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequest("POST", serperAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Serper API 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Serper API 返回错误 %d: %s", resp.StatusCode, string(body))
	}

	var result serperResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	items := make([]SearchResultItem, 0, len(result.Organic))
	for _, r := range result.Organic {
		items = append(items, SearchResultItem{
			Title:   r.Title,
			URL:     r.Link,
			Snippet: r.Snippet,
		})
	}

	return items, nil
}
