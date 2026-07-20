package reranker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	defaultCohereBaseURL = "https://api.cohere.com"
	defaultCohereModel   = "rerank-multilingual-v3.0"
)

// CohereReranker Cohere Rerank API 实现
// 文档: https://docs.cohere.com/docs/reranking
type CohereReranker struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewCohereReranker 创建 Cohere Reranker
func NewCohereReranker(apiKey, baseURL, model string) (*CohereReranker, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Cohere API Key 未配置")
	}
	if baseURL == "" {
		baseURL = defaultCohereBaseURL
	}
	if model == "" {
		model = defaultCohereModel
	}

	return &CohereReranker{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{},
	}, nil
}

// cohereRequest Cohere Rerank API 请求
type cohereRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n,omitempty"`
}

// cohereResponse Cohere Rerank API 响应
type cohereResponse struct {
	Results []struct {
		Index int     `json:"index"`
		Score float64 `json:"relevance_score"`
	} `json:"results"`
}

func (r *CohereReranker) Name() string {
	return "cohere"
}

func (r *CohereReranker) Rerank(query string, documents []string, topN int) ([]RerankResult, error) {
	if len(documents) == 0 {
		return nil, nil
	}

	// 构建请求
	reqBody := cohereRequest{
		Model:     r.model,
		Query:     query,
		Documents: documents,
		TopN:      topN,
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
		return nil, fmt.Errorf("Cohere API 返回错误 [%d]: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var cohereResp cohereResponse
	if err := json.Unmarshal(body, &cohereResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 转换结果
	results := make([]RerankResult, len(cohereResp.Results))
	for i, item := range cohereResp.Results {
		results[i] = RerankResult{
			Index: item.Index,
			Score: item.Score,
			Text:  documents[item.Index],
		}
	}

	return results, nil
}
