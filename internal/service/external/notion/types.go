package notion

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	defaultAPIBaseURL      = "https://api.notion.com"
	defaultNotionVersion   = "2022-06-28"
	defaultRequestTimeout  = 15 * time.Second
	defaultPageSize        = 50
	maximumPageSize        = 100
	defaultRenderMaxDepth  = 8
	defaultRenderMaxBlocks = 10_000
	defaultRenderMaxBytes  = 5 * 1024 * 1024
)

var (
	// ErrAuthExpired means the saved OAuth token is no longer accepted by Notion.
	ErrAuthExpired = errors.New("notion authentication expired")
	// ErrAccessDenied means the token is valid but cannot read the requested resource.
	ErrAccessDenied = errors.New("notion access denied")
	// ErrNotFound means the requested Notion page or block cannot be found.
	ErrNotFound = errors.New("notion resource not found")
	// ErrRateLimited means Notion still returned 429 after the one allowed retry.
	ErrRateLimited = errors.New("notion rate limited")
	// ErrUnavailable means Notion or the network could not complete the request.
	ErrUnavailable = errors.New("notion service unavailable")
	// ErrRequestTimeout means the bounded Notion request exceeded its deadline.
	ErrRequestTimeout = errors.New("notion request timed out")
	// ErrInvalidResponse means Notion returned an empty or structurally invalid response.
	ErrInvalidResponse = errors.New("invalid notion response")
	// ErrInvalidRequest means the adapter was called without the required input.
	ErrInvalidRequest = errors.New("invalid notion request")

	// ErrEmptyMarkdown means the page has no indexable rendered content.
	ErrEmptyMarkdown = errors.New("notion page has no indexable content")
	// ErrRenderDepthLimit means recursive block nesting exceeded the configured limit.
	ErrRenderDepthLimit = errors.New("notion block depth limit exceeded")
	// ErrRenderBlockLimit means a page exceeded the configured block count limit.
	ErrRenderBlockLimit = errors.New("notion block count limit exceeded")
	// ErrRenderByteLimit means the rendered Markdown exceeded its byte limit.
	ErrRenderByteLimit = errors.New("notion markdown byte limit exceeded")
)

// ClientConfig contains only application configuration and HTTP dependencies;
// an OAuth access token is never retained here.
type ClientConfig struct {
	BaseURL      string
	APIVersion   string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Timeout      time.Duration
	HTTPClient   *http.Client
}

// OAuthToken is the subset of an OAuth token exchange response required by the
// binding service. AccessToken is excluded from accidental JSON serialization.
type OAuthToken struct {
	AccessToken   string `json:"-"`
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	WorkspaceIcon string `json:"workspace_icon"`
	BotID         string `json:"bot_id"`
}

// Page is the safe subset of page metadata needed by the service and UI.
type Page struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	URL            string `json:"url"`
	LastEditedTime string `json:"last_edited_time"`
	HasChildren    bool   `json:"has_children"`
}

// PageSearchResult is the provider-neutral result of one Notion search page.
type PageSearchResult struct {
	Pages      []Page `json:"pages"`
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
}

// Block retains only the common metadata and the raw payload for its declared
// type. Keeping the payload raw avoids exposing an unstable Notion block DTO.
type Block struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	HasChildren bool            `json:"has_children"`
	Data        json.RawMessage `json:"-"`
}

// UnmarshalJSON keeps the block field named by Type as its raw type-specific payload.
func (b *Block) UnmarshalJSON(data []byte) error {
	var common struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		HasChildren bool   `json:"has_children"`
	}
	if err := json.Unmarshal(data, &common); err != nil {
		return err
	}
	if common.Type == "" {
		return fmt.Errorf("notion block type is empty")
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}

	b.ID = common.ID
	b.Type = common.Type
	b.HasChildren = common.HasChildren
	b.Data = append(b.Data[:0], fields[common.Type]...)
	return nil
}

// BlockChildrenResult is one cursor page of child blocks.
type BlockChildrenResult struct {
	Blocks     []Block `json:"results"`
	NextCursor string  `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

// RenderLimits bounds recursive rendering so one remote page cannot exhaust a worker.
type RenderLimits struct {
	MaxDepth  int
	MaxBlocks int
	MaxBytes  int
}

// DefaultRenderLimits returns the hard bounds from the Notion import design.
func DefaultRenderLimits() RenderLimits {
	return RenderLimits{
		MaxDepth:  defaultRenderMaxDepth,
		MaxBlocks: defaultRenderMaxBlocks,
		MaxBytes:  defaultRenderMaxBytes,
	}
}

func (l RenderLimits) normalized() RenderLimits {
	defaults := DefaultRenderLimits()
	if l.MaxDepth <= 0 {
		l.MaxDepth = defaults.MaxDepth
	}
	if l.MaxBlocks <= 0 {
		l.MaxBlocks = defaults.MaxBlocks
	}
	if l.MaxBytes <= 0 {
		l.MaxBytes = defaults.MaxBytes
	}
	return l
}

// APIError keeps the status and a safe sentinel without retaining Notion's raw
// response body, which may contain sensitive provider details.
type APIError struct {
	StatusCode int
	Kind       error
}

func (e *APIError) Error() string {
	if e == nil || e.Kind == nil {
		return "notion request failed"
	}
	if e.StatusCode > 0 {
		return fmt.Sprintf("notion request failed (HTTP %d): %s", e.StatusCode, e.Kind)
	}
	return fmt.Sprintf("notion request failed: %s", e.Kind)
}

// Unwrap allows callers to classify provider failures with errors.Is.
func (e *APIError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Kind
}
