package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	requestThrottleInterval = 400 * time.Millisecond
	maximumRetryAfter       = 5 * time.Second
	maximumResponseBytes    = 8 * 1024 * 1024
)

var sharedRequestThrottle = newRequestThrottle(requestThrottleInterval)

type notionClient struct {
	baseURL        string
	apiVersion     string
	clientID       string
	clientSecret   string
	redirectURI    string
	httpClient     *http.Client
	requestTimeout time.Duration
	throttle       *requestThrottle
}

type requestThrottle struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func newRequestThrottle(interval time.Duration) *requestThrottle {
	return &requestThrottle{interval: interval}
}

func (t *requestThrottle) wait(ctx context.Context) error {
	t.mu.Lock()
	now := time.Now()
	start := now
	if t.next.After(now) {
		start = t.next
	}
	t.next = start.Add(t.interval)
	t.mu.Unlock()

	delay := time.Until(start)
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// NewClient creates a standard-library Notion client. Missing optional values
// use the documented API URL, version, and 15-second request deadline.
func NewClient(cfg ClientConfig) Client {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultAPIBaseURL
	}
	apiVersion := strings.TrimSpace(cfg.APIVersion)
	if apiVersion == "" {
		apiVersion = defaultNotionVersion
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultRequestTimeout
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}

	return &notionClient{
		baseURL:        baseURL,
		apiVersion:     apiVersion,
		clientID:       cfg.ClientID,
		clientSecret:   cfg.ClientSecret,
		redirectURI:    cfg.RedirectURI,
		httpClient:     httpClient,
		requestTimeout: timeout,
		throttle:       sharedRequestThrottle,
	}
}

// ExchangeCode exchanges a short-lived OAuth authorization code using HTTP Basic Auth.
func (c *notionClient) ExchangeCode(ctx context.Context, code string) (OAuthToken, error) {
	if strings.TrimSpace(code) == "" {
		return OAuthToken{}, &APIError{Kind: ErrInvalidRequest}
	}

	payload, err := json.Marshal(struct {
		GrantType   string `json:"grant_type"`
		Code        string `json:"code"`
		RedirectURI string `json:"redirect_uri,omitempty"`
	}{
		GrantType:   "authorization_code",
		Code:        code,
		RedirectURI: c.redirectURI,
	})
	if err != nil {
		return OAuthToken{}, &APIError{Kind: ErrInvalidRequest}
	}

	body, err := c.doJSON(ctx, http.MethodPost, "/v1/oauth/token", payload, requestAuthentication{basic: true})
	if err != nil {
		return OAuthToken{}, err
	}
	var response struct {
		AccessToken   string `json:"access_token"`
		WorkspaceID   string `json:"workspace_id"`
		WorkspaceName string `json:"workspace_name"`
		WorkspaceIcon string `json:"workspace_icon"`
		BotID         string `json:"bot_id"`
	}
	if err := decodeResponse(body, &response); err != nil {
		return OAuthToken{}, err
	}
	if response.AccessToken == "" {
		return OAuthToken{}, &APIError{Kind: ErrInvalidResponse}
	}
	return OAuthToken{
		AccessToken:   response.AccessToken,
		WorkspaceID:   response.WorkspaceID,
		WorkspaceName: response.WorkspaceName,
		WorkspaceIcon: response.WorkspaceIcon,
		BotID:         response.BotID,
	}, nil
}

// SearchPages searches only Notion page objects and maps Notion's cursor shape.
func (c *notionClient) SearchPages(ctx context.Context, accessToken, query, cursor string, pageSize int) (PageSearchResult, error) {
	if strings.TrimSpace(accessToken) == "" {
		return PageSearchResult{}, &APIError{Kind: ErrInvalidRequest}
	}
	request := struct {
		Query  string `json:"query"`
		Filter struct {
			Object string `json:"object"`
		} `json:"filter"`
		PageSize    int    `json:"page_size"`
		StartCursor string `json:"start_cursor,omitempty"`
	}{}
	request.Query = query
	request.PageSize = normalizedPageSize(pageSize)
	request.StartCursor = cursor
	request.Filter.Object = "page"
	payload, err := json.Marshal(request)
	if err != nil {
		return PageSearchResult{}, &APIError{Kind: ErrInvalidRequest}
	}

	body, err := c.doJSON(ctx, http.MethodPost, "/v1/search", payload, requestAuthentication{bearerToken: accessToken})
	if err != nil {
		return PageSearchResult{}, err
	}
	var response struct {
		Results    []json.RawMessage `json:"results"`
		NextCursor string            `json:"next_cursor"`
		HasMore    bool              `json:"has_more"`
	}
	if err := decodeResponse(body, &response); err != nil {
		return PageSearchResult{}, err
	}

	pages := make([]Page, 0, len(response.Results))
	for _, rawPage := range response.Results {
		var object struct {
			Object string `json:"object"`
		}
		if err := json.Unmarshal(rawPage, &object); err != nil {
			return PageSearchResult{}, &APIError{Kind: ErrInvalidResponse}
		}
		if object.Object != "" && object.Object != "page" {
			continue
		}
		page, err := decodePage(rawPage)
		if err != nil {
			return PageSearchResult{}, err
		}
		pages = append(pages, page)
	}

	return PageSearchResult{Pages: pages, NextCursor: response.NextCursor, HasMore: response.HasMore}, nil
}

// GetPage retrieves the safe page metadata needed before importing its blocks.
func (c *notionClient) GetPage(ctx context.Context, accessToken, pageID string) (Page, error) {
	if strings.TrimSpace(accessToken) == "" || strings.TrimSpace(pageID) == "" {
		return Page{}, &APIError{Kind: ErrInvalidRequest}
	}
	body, err := c.doJSON(ctx, http.MethodGet, "/v1/pages/"+url.PathEscape(pageID), nil, requestAuthentication{bearerToken: accessToken})
	if err != nil {
		return Page{}, err
	}
	return decodePage(body)
}

// ListBlockChildren fetches one Notion cursor page of child blocks.
func (c *notionClient) ListBlockChildren(ctx context.Context, accessToken, blockID, cursor string, pageSize int) (BlockChildrenResult, error) {
	if strings.TrimSpace(accessToken) == "" || strings.TrimSpace(blockID) == "" {
		return BlockChildrenResult{}, &APIError{Kind: ErrInvalidRequest}
	}
	query := url.Values{}
	query.Set("page_size", strconv.Itoa(normalizedPageSize(pageSize)))
	if cursor != "" {
		query.Set("start_cursor", cursor)
	}
	path := "/v1/blocks/" + url.PathEscape(blockID) + "/children?" + query.Encode()
	body, err := c.doJSON(ctx, http.MethodGet, path, nil, requestAuthentication{bearerToken: accessToken})
	if err != nil {
		return BlockChildrenResult{}, err
	}
	var response BlockChildrenResult
	if err := decodeResponse(body, &response); err != nil {
		return BlockChildrenResult{}, err
	}
	return response, nil
}

// RevokeToken asks Notion to revoke the token. Callers may intentionally treat
// an error as non-fatal after local credentials have already been removed.
func (c *notionClient) RevokeToken(ctx context.Context, accessToken string) error {
	if strings.TrimSpace(accessToken) == "" {
		return &APIError{Kind: ErrInvalidRequest}
	}
	payload, err := json.Marshal(struct {
		Token string `json:"token"`
	}{Token: accessToken})
	if err != nil {
		return &APIError{Kind: ErrInvalidRequest}
	}
	_, err = c.doJSON(ctx, http.MethodPost, "/v1/oauth/revoke", payload, requestAuthentication{basic: true})
	return err
}

type requestAuthentication struct {
	bearerToken string
	basic       bool
}

func (c *notionClient) doJSON(ctx context.Context, method, path string, payload []byte, auth requestAuthentication) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	requestCtx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()

	endpoint := c.baseURL + path
	for attempt := 0; attempt < 2; attempt++ {
		if err := c.throttle.wait(requestCtx); err != nil {
			return nil, contextRequestError(err)
		}
		req, err := http.NewRequestWithContext(requestCtx, method, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, &APIError{Kind: ErrInvalidRequest}
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Notion-Version", c.apiVersion)
		if auth.basic {
			req.SetBasicAuth(c.clientID, c.clientSecret)
		} else if auth.bearerToken != "" {
			req.Header.Set("Authorization", "Bearer "+auth.bearerToken)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, contextRequestError(requestCtx.Err())
		}
		responseBody, readErr := readResponseBody(resp)
		if readErr != nil {
			return nil, &APIError{StatusCode: resp.StatusCode, Kind: ErrInvalidResponse}
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt == 0 {
			if err := waitForRetry(requestCtx, retryAfter(resp.Header)); err != nil {
				return nil, contextRequestError(err)
			}
			continue
		}
		if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
			return responseBody, nil
		}
		return nil, statusError(resp.StatusCode)
	}

	return nil, &APIError{Kind: ErrRateLimited}
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maximumResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maximumResponseBytes {
		return nil, fmt.Errorf("notion response exceeds limit")
	}
	return body, nil
}

func decodeResponse(body []byte, target any) error {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return &APIError{Kind: ErrInvalidResponse}
	}
	if err := json.Unmarshal(body, target); err != nil {
		return &APIError{Kind: ErrInvalidResponse}
	}
	return nil
}

func decodePage(raw []byte) (Page, error) {
	var response struct {
		ID             string                     `json:"id"`
		URL            string                     `json:"url"`
		LastEditedTime string                     `json:"last_edited_time"`
		HasChildren    bool                       `json:"has_children"`
		Properties     map[string]json.RawMessage `json:"properties"`
	}
	if err := decodeResponse(raw, &response); err != nil {
		return Page{}, err
	}
	if response.ID == "" {
		return Page{}, &APIError{Kind: ErrInvalidResponse}
	}
	return Page{
		ID:             response.ID,
		Title:          extractPageTitle(response.Properties),
		URL:            response.URL,
		LastEditedTime: response.LastEditedTime,
		HasChildren:    response.HasChildren,
	}, nil
}

func extractPageTitle(properties map[string]json.RawMessage) string {
	for _, rawProperty := range properties {
		var property struct {
			Type  string `json:"type"`
			Title []struct {
				PlainText string `json:"plain_text"`
				Text      struct {
					Content string `json:"content"`
				} `json:"text"`
			} `json:"title"`
		}
		if err := json.Unmarshal(rawProperty, &property); err != nil {
			continue
		}
		if property.Type != "" && property.Type != "title" {
			continue
		}
		var title strings.Builder
		for _, part := range property.Title {
			if part.PlainText != "" {
				title.WriteString(part.PlainText)
			} else {
				title.WriteString(part.Text.Content)
			}
		}
		if value := strings.TrimSpace(title.String()); value != "" {
			return value
		}
	}
	return "未命名 Notion 页面"
}

func normalizedPageSize(pageSize int) int {
	if pageSize <= 0 {
		return defaultPageSize
	}
	if pageSize > maximumPageSize {
		return maximumPageSize
	}
	return pageSize
}

func statusError(status int) error {
	switch {
	case status == http.StatusUnauthorized:
		return &APIError{StatusCode: status, Kind: ErrAuthExpired}
	case status == http.StatusForbidden:
		return &APIError{StatusCode: status, Kind: ErrAccessDenied}
	case status == http.StatusNotFound:
		return &APIError{StatusCode: status, Kind: ErrNotFound}
	case status == http.StatusTooManyRequests:
		return &APIError{StatusCode: status, Kind: ErrRateLimited}
	case status >= http.StatusInternalServerError:
		return &APIError{StatusCode: status, Kind: ErrUnavailable}
	default:
		return &APIError{StatusCode: status, Kind: ErrInvalidRequest}
	}
}

func contextRequestError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return &APIError{Kind: ErrRequestTimeout}
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	return &APIError{Kind: ErrUnavailable}
}

func retryAfter(header http.Header) time.Duration {
	value := strings.TrimSpace(header.Get("Retry-After"))
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return boundedRetryDelay(time.Duration(seconds) * time.Second)
	}
	if at, err := http.ParseTime(value); err == nil {
		return boundedRetryDelay(time.Until(at))
	}
	return 0
}

func boundedRetryDelay(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}
	if delay > maximumRetryAfter {
		return maximumRetryAfter
	}
	return delay
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
