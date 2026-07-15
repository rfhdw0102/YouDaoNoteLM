// internal/service/external/search/searxng_engine.go
package search

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"YoudaoNoteLm/pkg/logger"
	"go.uber.org/zap"
)

type searxngEngine struct {
	baseURL string
	client  *http.Client
}

// NewSearXNGEngine 创建 SearXNG 搜索引擎（自部署）
func NewSearXNGEngine(baseURL string) SearchEngine {
	return &searxngEngine{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (e *searxngEngine) Name() string {
	return "searxng"
}

func (e *searxngEngine) Search(query string, limit int) ([]SearchResultItem, error) {
	if limit <= 0 {
		limit = 10
	}

	// 确保 baseURL 不以 / 结尾
	baseURL := strings.TrimRight(e.baseURL, "/")
	searchURL := fmt.Sprintf("%s/search?q=%s&format=json", baseURL, url.QueryEscape(query))

	logger.Info("SearXNG 请求 URL", zap.String("url", searchURL))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	// 设置标准浏览器 User-Agent 避免 403
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SearXNG请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			logger.Error("SearXNG 返回错误且读取响应体失败", zap.Int("status", resp.StatusCode), zap.Error(readErr))
			return nil, fmt.Errorf("SearXNG返回错误 %d, 且读取响应体失败: %w", resp.StatusCode, readErr)
		}
		logger.Error("SearXNG 返回错误", zap.Int("status", resp.StatusCode), zap.String("body", string(body)))
		return nil, fmt.Errorf("SearXNG返回错误 %d: %s", resp.StatusCode, string(body))
	}

	var apiResp searxngResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("解析SearXNG结果失败: %w", err)
	}

	results := make([]SearchResultItem, 0, len(apiResp.Results))
	for _, r := range apiResp.Results {
		if len(results) >= limit {
			break
		}
		results = append(results, SearchResultItem{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
		})
	}

	return results, nil
}

// searxngResponse SearXNG API 响应结构
type searxngResponse struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
	} `json:"results"`
}
