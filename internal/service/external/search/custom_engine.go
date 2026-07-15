package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type customEngine struct {
	name   string
	apiURL string
	apiKey string
	client *http.Client
}

func NewCustomEngine(name, apiURL, apiKey string) SearchEngine {
	return &customEngine{
		name:   name,
		apiURL: apiURL,
		apiKey: apiKey,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (e *customEngine) Name() string {
	return e.name
}

func (e *customEngine) Search(query string, limit int) ([]SearchResultItem, error) {
	reqBody, err := json.Marshal(map[string]interface{}{
		"query": query,
		"limit": limit,
	})
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequest("POST", e.apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("自定义搜索API请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("搜索API返回错误 %d, 且读取响应体失败: %w", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("搜索API返回错误 %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Results []SearchResultItem `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("解析搜索结果失败: %w", err)
	}

	return apiResp.Results, nil
}
