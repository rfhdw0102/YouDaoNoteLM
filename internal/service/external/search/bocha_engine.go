// internal/service/external/search/bocha_engine.go
package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// BochaEngine 博查 AI 搜索引擎
type BochaEngine struct {
	apiURL string
	apiKey string
	client *http.Client
}

// NewBochaEngine 创建 Bocha 引擎
func NewBochaEngine(apiURL, apiKey string) *BochaEngine {
	if apiURL == "" {
		apiURL = "https://api.bocha.cn/v1/web-search"
	}
	return &BochaEngine{
		apiURL: apiURL,
		apiKey: apiKey,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// bochaRequest Bocha API 请求
type bochaRequest struct {
	Query     string `json:"query"`
	Summary   bool   `json:"summary"`
	Freshness string `json:"freshness"`
	Count     int    `json:"count"`
}

// bochaResponse Bocha API 响应
type bochaResponse struct {
	Data struct {
		WebPages struct {
			Value []struct {
				Name    string `json:"name"`
				URL     string `json:"url"`
				Snippet string `json:"snippet"`
			} `json:"value"`
		} `json:"webPages"`
	} `json:"data"`
}

func (e *BochaEngine) Name() string {
	return "bocha"
}

func (e *BochaEngine) Search(query string, limit int) ([]SearchResultItem, error) {
	if e.apiKey == "" {
		return nil, fmt.Errorf("Bocha API key 未配置")
	}
	if limit <= 0 {
		limit = 10
	}

	reqBody := bochaRequest{
		Query:     query,
		Summary:   true,
		Freshness: "noLimit",
		Count:     limit,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequest("POST", e.apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Bocha API 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bocha API 返回错误 %d: %s", resp.StatusCode, string(body))
	}

	var result bochaResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	items := make([]SearchResultItem, 0, len(result.Data.WebPages.Value))
	for _, r := range result.Data.WebPages.Value {
		items = append(items, SearchResultItem{
			Title:   r.Name,
			URL:     r.URL,
			Snippet: r.Snippet,
		})
	}

	return items, nil
}
