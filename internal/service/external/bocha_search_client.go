package external

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	bizerrors "YoudaoNoteLm/pkg/errors"
)

const defaultBochaEndpoint = "/v1/web-search"

type bochaSearchClient struct {
	httpClient *http.Client
	endpoint   string
}

type bochaSearchRequest struct {
	Query     string `json:"query"`
	Freshness string `json:"freshness,omitempty"`
	Summary   bool   `json:"summary,omitempty"`
	Count     int    `json:"count,omitempty"`
}

type bochaSearchResponse struct {
	QueryContext struct {
		OriginalQuery string `json:"originalQuery"`
	} `json:"queryContext"`
	WebPages *struct {
		TotalEstimatedMatches int                    `json:"totalEstimatedMatches"`
		Value                 []bochaSearchResultRaw `json:"value"`
	} `json:"webPages"`
}

type bochaSearchResultRaw struct {
	ID                   string         `json:"id"`
	Name                 string         `json:"name"`
	URL                  string         `json:"url"`
	DisplayURL           string         `json:"displayUrl"`
	Snippet              string         `json:"snippet"`
	Summary              string         `json:"summary"`
	SiteName             string         `json:"siteName"`
	SiteIcon             string         `json:"siteIcon"`
	DatePublished        string         `json:"datePublished"`
	DatePublishedDisplay string         `json:"datePublishedDisplayText"`
	Language             string         `json:"language"`
	CachedPageURL        string         `json:"cachedPageUrl"`
	Extra                map[string]any `json:"extra"`
}

// NewBochaSearchClient 创建博查搜索客户端。
func NewBochaSearchClient(httpClient *http.Client, endpoint string) WebSearchClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	if strings.TrimSpace(endpoint) == "" {
		endpoint = defaultBochaEndpoint
	}
	return &bochaSearchClient{
		httpClient: httpClient,
		endpoint:   endpoint,
	}
}

// Search 调用博查 Web Search API。
func (c *bochaSearchClient) Search(ctx context.Context, cfg SearchProviderConfig, req *SearchProviderRequest) (*SearchProviderResponse, error) {
	if req == nil || strings.TrimSpace(req.Query) == "" {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "搜索 query 不能为空")
	}
	if strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, bizerrors.ErrSearchProviderNotConfigured
	}

	payload := bochaSearchRequest{
		Query:     req.Query,
		Freshness: req.Freshness,
		Summary:   req.NeedSummary || req.NeedContent,
		Count:     req.Count,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "序列化博查请求失败", err)
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(
		requestCtx,
		http.MethodPost,
		strings.TrimRight(cfg.BaseURL, "/")+c.endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "创建博查请求失败", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	if req.TraceID != "" {
		httpReq.Header.Set("X-Trace-ID", req.TraceID)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		if isTimeoutErr(err) || requestCtx.Err() == context.DeadlineExceeded {
			return nil, bizerrors.NewWithErr(bizerrors.CodeSearchRequestTimeout, "博查搜索请求超时", err)
		}
		return nil, bizerrors.NewWithErr(bizerrors.CodeSearchProviderUnavailable, "博查搜索请求失败", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeSearchProviderUnavailable, "读取博查响应失败", err)
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, bizerrors.ErrSearchInvalidAPIKey
	case http.StatusTooManyRequests:
		return nil, bizerrors.ErrSearchQuotaExhausted
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return nil, bizerrors.New(bizerrors.CodeSearchProviderUnavailable, "博查搜索服务不可用")
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, bizerrors.NewWithErr(
			bizerrors.CodeSearchProviderUnavailable,
			fmt.Sprintf("博查搜索请求失败: HTTP %d", resp.StatusCode),
			errors.New(strings.TrimSpace(string(respBody))),
		)
	}

	var raw bochaSearchResponse
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeSearchInvalidResponse, "解析博查响应失败", err)
	}
	if raw.WebPages == nil {
		return nil, bizerrors.ErrSearchInvalidResponse
	}

	results := make([]SearchProviderResult, 0, len(raw.WebPages.Value))
	for _, item := range raw.WebPages.Value {
		results = append(results, SearchProviderResult{
			ID:          item.ID,
			Title:       item.Name,
			URL:         item.URL,
			DisplayURL:  item.DisplayURL,
			Snippet:     item.Snippet,
			Summary:     item.Summary,
			SiteName:    item.SiteName,
			SiteIcon:    item.SiteIcon,
			PublishedAt: firstNonEmpty(item.DatePublished, item.DatePublishedDisplay),
			Language:    item.Language,
			Meta: map[string]any{
				"cached_page_url": item.CachedPageURL,
				"site_icon":       item.SiteIcon,
				"extra":           item.Extra,
			},
		})
	}

	return &SearchProviderResponse{
		Query:    firstNonEmpty(raw.QueryContext.OriginalQuery, req.Query),
		Provider: "bocha",
		Total:    raw.WebPages.TotalEstimatedMatches,
		Results:  results,
		Meta: map[string]any{
			"provider": "bocha",
		},
	}, nil
}

func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "timeout")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
