package reranker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	defaultSiliconFlowBaseURL = "https://api.siliconflow.cn"
	defaultSiliconFlowModel   = "BAAI/bge-reranker-v2-m3"
)

// SiliconFlowReranker SiliconFlow Reranker API 实现
// 文档: https://docs.siliconflow.cn/api-reference/rerank
type SiliconFlowReranker struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewSiliconFlowReranker 创建 SiliconFlow Reranker
func NewSiliconFlowReranker(apiKey, baseURL, model string) (*SiliconFlowReranker, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("SiliconFlow API Key 未配置")
	}
	if baseURL == "" {
		baseURL = defaultSiliconFlowBaseURL
	}
	if model == "" {
		model = defaultSiliconFlowModel
	}

	return &SiliconFlowReranker{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{},
	}, nil
}

// siliconFlowRequest SiliconFlow Rerank API 请求
type siliconFlowRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n,omitempty"`
}

// siliconFlowResponse SiliconFlow Rerank API 响应
type siliconFlowResponse struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
	} `json:"results"`
}

func (r *SiliconFlowReranker) Name() string {
	return "siliconflow"
}

func (r *SiliconFlowReranker) Rerank(query string, documents []string, topN int) ([]RerankResult, error) {
	if len(documents) == 0 {
		return nil, nil
	}

	// 构建请求
	reqBody := siliconFlowRequest{
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
		return nil, fmt.Errorf("SiliconFlow API 返回错误 [%d]: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var sfResp siliconFlowResponse
	if err := json.Unmarshal(body, &sfResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 转换结果
	results := make([]RerankResult, len(sfResp.Results))
	for i, item := range sfResp.Results {
		results[i] = RerankResult{
			Index: item.Index,
			Score: item.RelevanceScore,
			Text:  documents[item.Index],
		}
	}

	return results, nil
}
