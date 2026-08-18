package service

import (
	"YoudaoNoteLm/pkg/cache"
	"context"
)

// NotionService is the application-facing Notion OAuth and import use case.
type NotionService interface {
	StartOAuth(ctx context.Context, userID uint) (string, error)
	HandleOAuthCallback(ctx context.Context, code, state string) (OAuthCallbackResult, error)
	GetBinding(ctx context.Context, userID uint) (*NotionBindingView, error)
	Unbind(ctx context.Context, userID uint) error
	ListPages(ctx context.Context, userID uint, query, cursor string, pageSize int) (PageList, error)
	ImportPagesBatch(ctx context.Context, userID, notebookID uint, pageIDs []string) (taskID string, sourceIDs []uint, err error)
	GetImportTask(ctx context.Context, userID uint, taskID string) (*cache.ImportTask, error)
	CancelImportTask(ctx context.Context, userID uint, taskID string) error
}

type OAuthCallbackReason string

const (
	OAuthCallbackReasonSuccess      OAuthCallbackReason = "success"
	OAuthCallbackReasonInvalidState OAuthCallbackReason = "invalid_state"
	OAuthCallbackReasonUserDisabled OAuthCallbackReason = "user_disabled"
	OAuthCallbackReasonExchangeFail OAuthCallbackReason = "exchange_failed"
	OAuthCallbackReasonSaveFail     OAuthCallbackReason = "save_failed"
)

// OAuthCallbackResult intentionally contains no provider details or credentials.
type OAuthCallbackResult struct {
	Success bool                `json:"success"`
	Reason  OAuthCallbackReason `json:"reason"`
}

// NotionBindingView is safe to return from an API. It never contains a token.
type NotionBindingView struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	WorkspaceIcon string `json:"workspace_icon"`
	BotID         string `json:"bot_id"`
	Status        string `json:"status"`
}

type PageList struct {
	Items      []PageItem `json:"items"`
	NextCursor string     `json:"next_cursor"`
	HasMore    bool       `json:"has_more"`
}

type PageItem struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	URL            string `json:"url"`
	LastEditedTime string `json:"last_edited_time"`
	HasChildren    bool   `json:"has_children"`
}
