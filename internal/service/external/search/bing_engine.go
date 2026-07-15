// internal/service/external/search/bing_engine.go
package search

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const bingSearchAPIURL = "https://api.bing.microsoft.com/v7.0/search"

// BingEngine Bing 搜索引擎
type BingEngine struct {
	apiKey string
	client *http.Client
}

// NewBingEngine 创建 Bing 引擎
func NewBingEngine(apiKey string) *BingEngine {
	return &BingEngine{
		apiKey: apiKey,
		client: &http.Client{},
	}
}

// bingResponse Bing API 响应
type bingResponse struct {
	WebPages struct {
		Value []struct {
			Name    string `json:"name"`
			URL     string `json:"url"`
			Snippet string `json:"snippet"`
		} `json:"value"`
	} `json:"webPages"`
}

func (e *BingEngine) Name() string {
	return "bing"
}

func (e *BingEngine) Search(query string, limit int) ([]SearchResultItem, error) {
	if e.apiKey == "" {
		return nil, fmt.Errorf("Bing API key 未配置")
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("count", fmt.Sprintf("%d", limit))
	params.Set("mkt", "zh-CN")

	reqURL := fmt.Sprintf("%s?%s", bingSearchAPIURL, params.Encode())

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Bing API 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bing API 返回错误 %d: %s", resp.StatusCode, string(body))
	}

	var result bingResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	items := make([]SearchResultItem, 0, len(result.WebPages.Value))
	for _, r := range result.WebPages.Value {
		items = append(items, SearchResultItem{
			Title:   r.Name,
			URL:     r.URL,
			Snippet: r.Snippet,
		})
	}

	return items, nil
}
