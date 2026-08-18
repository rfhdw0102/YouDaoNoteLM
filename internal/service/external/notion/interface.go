// Package notion adapts the small Notion API surface needed by the import flow.
package notion

import "context"

// Client is the Notion API boundary used by the service layer.
type Client interface {
	ExchangeCode(ctx context.Context, code string) (OAuthToken, error)
	SearchPages(ctx context.Context, accessToken, query, cursor string, pageSize int) (PageSearchResult, error)
	GetPage(ctx context.Context, accessToken, pageID string) (Page, error)
	ListBlockChildren(ctx context.Context, accessToken, blockID, cursor string, pageSize int) (BlockChildrenResult, error)
	RevokeToken(ctx context.Context, accessToken string) error
}

// BlockFetcher is the recursive block-reading dependency of the Markdown renderer.
type BlockFetcher interface {
	ListBlockChildren(ctx context.Context, accessToken, blockID, cursor string, pageSize int) (BlockChildrenResult, error)
}
