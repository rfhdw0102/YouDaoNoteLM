package notion

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchPagesSendsRequiredRequestAndMapsPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/search" {
			t.Fatalf("path = %s, want /v1/search", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := r.Header.Get("Notion-Version"); got != "2022-06-28" {
			t.Fatalf("Notion-Version = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q", got)
		}

		var body struct {
			Query       string `json:"query"`
			PageSize    int    `json:"page_size"`
			StartCursor string `json:"start_cursor"`
			Filter      struct {
				Object string `json:"object"`
			} `json:"filter"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Query != "项目计划" || body.PageSize != 25 || body.StartCursor != "cursor-1" || body.Filter.Object != "page" {
			t.Fatalf("unexpected search request: %#v", body)
		}

		writeJSON(t, w, http.StatusOK, map[string]any{
			"results": []any{notionPageJSON("page-1", "测试页面")},
			"next_cursor": "cursor-2",
			"has_more":    true,
		})
	}))
	defer server.Close()

	result, err := newTestClient(server.URL).SearchPages(context.Background(), "access-token", "项目计划", "cursor-1", 25)
	if err != nil {
		t.Fatal(err)
	}
	if result.NextCursor != "cursor-2" || !result.HasMore {
		t.Fatalf("pagination = %#v", result)
	}
	if len(result.Pages) != 1 {
		t.Fatalf("pages = %#v", result.Pages)
	}
	page := result.Pages[0]
	if page.ID != "page-1" || page.Title != "测试页面" || page.URL != "https://www.notion.so/page-1" || page.LastEditedTime != "2026-08-18T00:00:00.000Z" || !page.HasChildren {
		t.Fatalf("page = %#v", page)
	}
}

func TestSearchPagesRetriesRateLimitExactlyOnce(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeJSON(t, w, http.StatusOK, map[string]any{"results": []any{}, "has_more": false})
	}))
	defer server.Close()

	if _, err := newTestClient(server.URL).SearchPages(context.Background(), "access-token", "", "", 50); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want exactly 2", requests)
	}
}

func TestSearchPagesReturnsAuthExpired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	_, err := newTestClient(server.URL).SearchPages(context.Background(), "access-token", "", "", 50)
	if !errors.Is(err, ErrAuthExpired) {
		t.Fatalf("error = %v, want ErrAuthExpired", err)
	}
}

func TestExchangeCodeUsesBasicAuthAndMapsWorkspace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/oauth/token" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		user, password, ok := r.BasicAuth()
		if !ok || user != "client-id" || password != "client-secret" {
			t.Fatalf("unexpected basic auth: user=%q present=%v", user, ok)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["grant_type"] != "authorization_code" || body["code"] != "code-placeholder" || body["redirect_uri"] != "https://app.example/callback" {
			t.Fatalf("unexpected oauth body: %#v", body)
		}
		writeJSON(t, w, http.StatusOK, map[string]any{
			"access_token":  "token-placeholder",
			"workspace_id":  "workspace-1",
			"workspace_name": "工作区",
			"workspace_icon": "https://cdn.example/icon.png",
			"bot_id":        "bot-1",
		})
	}))
	defer server.Close()

	token, err := newTestClient(server.URL).ExchangeCode(context.Background(), "code-placeholder")
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "token-placeholder" || token.WorkspaceID != "workspace-1" || token.WorkspaceName != "工作区" || token.WorkspaceIcon != "https://cdn.example/icon.png" || token.BotID != "bot-1" {
		t.Fatalf("token = %#v", token)
	}
}

func TestGetPageAndListBlockChildrenMapResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Fatalf("Authorization = %q", got)
		}
		switch r.URL.Path {
		case "/v1/pages/page-1":
			writeJSON(t, w, http.StatusOK, notionPageJSON("page-1", "单页"))
		case "/v1/blocks/block-1/children":
			if got := r.URL.Query().Get("page_size"); got != "42" {
				t.Fatalf("page_size = %q", got)
			}
			if got := r.URL.Query().Get("start_cursor"); got != "block-cursor" {
				t.Fatalf("start_cursor = %q", got)
			}
			writeJSON(t, w, http.StatusOK, map[string]any{
				"results": []any{map[string]any{
					"id":           "block-2",
					"type":         "paragraph",
					"has_children": true,
					"paragraph": map[string]any{
						"rich_text": []any{},
					},
				}},
				"next_cursor": "block-cursor-2",
				"has_more":    true,
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	page, err := client.GetPage(context.Background(), "access-token", "page-1")
	if err != nil {
		t.Fatal(err)
	}
	if page.Title != "单页" {
		t.Fatalf("page = %#v", page)
	}

	blocks, err := client.ListBlockChildren(context.Background(), "access-token", "block-1", "block-cursor", 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks.Blocks) != 1 || blocks.Blocks[0].ID != "block-2" || blocks.Blocks[0].Type != "paragraph" || !blocks.Blocks[0].HasChildren || len(blocks.Blocks[0].Data) == 0 || blocks.NextCursor != "block-cursor-2" || !blocks.HasMore {
		t.Fatalf("blocks = %#v", blocks)
	}
}

func TestRevokeTokenUsesBasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/oauth/revoke" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		user, password, ok := r.BasicAuth()
		if !ok || user != "client-id" || password != "client-secret" {
			t.Fatalf("unexpected basic auth: user=%q present=%v", user, ok)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["token"] != "token-placeholder" {
			t.Fatalf("token = %q", body["token"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	if err := newTestClient(server.URL).RevokeToken(context.Background(), "token-placeholder"); err != nil {
		t.Fatal(err)
	}
}

func TestNotionClientMapsSafeTypedErrors(t *testing.T) {
	tests := []struct {
		name string
		code int
		want error
	}{
		{name: "forbidden", code: http.StatusForbidden, want: ErrAccessDenied},
		{name: "not found", code: http.StatusNotFound, want: ErrNotFound},
		{name: "rate limited", code: http.StatusTooManyRequests, want: ErrRateLimited},
		{name: "server unavailable", code: http.StatusBadGateway, want: ErrUnavailable},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.code == http.StatusTooManyRequests {
					w.Header().Set("Retry-After", "0")
				}
				w.WriteHeader(tc.code)
			}))
			defer server.Close()

			_, err := newTestClient(server.URL).SearchPages(context.Background(), "access-token", "", "", 50)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNotionClientReturnsInvalidResponseForMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not-json"))
	}))
	defer server.Close()

	_, err := newTestClient(server.URL).SearchPages(context.Background(), "access-token", "", "", 50)
	if !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("error = %v, want ErrInvalidResponse", err)
	}
}

func TestSearchPagesCapsPageSizeAtNotionMaximum(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			PageSize int `json:"page_size"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.PageSize != 100 {
			t.Fatalf("page_size = %d, want 100", body.PageSize)
		}
		writeJSON(t, w, http.StatusOK, map[string]any{"results": []any{}, "has_more": false})
	}))
	defer server.Close()

	if _, err := newTestClient(server.URL).SearchPages(context.Background(), "access-token", "", "", 101); err != nil {
		t.Fatal(err)
	}
}

func newTestClient(baseURL string) Client {
	return NewClient(ClientConfig{
		BaseURL:      baseURL,
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURI:  "https://app.example/callback",
	})
}

func notionPageJSON(id, title string) map[string]any {
	return map[string]any{
		"id":               id,
		"url":              "https://www.notion.so/" + id,
		"last_edited_time": "2026-08-18T00:00:00.000Z",
		"has_children":     true,
		"properties": map[string]any{
			"Name": map[string]any{
				"type": "title",
				"title": []any{map[string]any{
					"type":       "text",
					"plain_text": title,
					"text": map[string]any{
						"content": title,
					},
				}},
			},
		},
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}
