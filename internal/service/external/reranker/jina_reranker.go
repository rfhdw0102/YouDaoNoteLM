package reranker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	defaultJinaBaseURL = "https://api.jina.ai"
	defaultJinaModel   = "jina-reranker-v2-base-multilingual"
)

// JinaReranker Jina Reranker API 实现
// 文档: https://jina.ai/reranker/
type JinaReranker struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewJinaReranker 创建 Jina Reranker
func NewJinaReranker(apiKey, baseURL, model string) (*JinaReranker, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Jina API Key 未配置")
	}
	if baseURL == "" {
		baseURL = defaultJinaBaseURL
	}
	if model == "" {
		model = defaultJinaModel
	}

	return &JinaReranker{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{},
	}, nil
}

// jinaRequest Jina Rerank API 请求
type jinaRequest struct {
	Model     string `json:"model"`
	Query     string `json:"query"`
	TopN      int    `json:"top_n,omitempty"`
	Documents []struct {
		Text string `json:"text"`
	} `json:"documents"`
}

// jinaResponse Jina Rerank API 响应
type jinaResponse struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
		Document       struct {
			Text string `json:"text"`
		} `json:"document"`
	} `json:"results"`
}

func (r *JinaReranker) Name() string {
	return "jina"
}

func (r *JinaReranker) Rerank(query string, documents []string, topN int) ([]RerankResult, error) {
	if len(documents) == 0 {
		return nil, nil
	}

	// 构建请求
	docs := make([]struct {
		Text string `json:"text"`
	}, len(documents))
	for i, doc := range documents {
		docs[i] = struct {
			Text string `json:"text"`
		}{Text: doc}
	}

	reqBody := jinaRequest{
		Model:     r.model,
		Query:     query,
		TopN:      topN,
		Documents: docs,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 发送请求
	url := fmt.Sprintf("%s/v1/rerank", r.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.apiKey))

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Jina API 返回错误 [%d]: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var jinaResp jinaResponse
	if err := json.Unmarshal(body, &jinaResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 转换结果
	results := make([]RerankResult, len(jinaResp.Results))
	for i, item := range jinaResp.Results {
		results[i] = RerankResult{
			Index: item.Index,
			Score: item.RelevanceScore,
			Text:  documents[item.Index],
		}
	}

	return results, nil
}
