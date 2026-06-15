package search

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCustomEngine_Name(t *testing.T) {
	engine := NewCustomEngine("my-search", "http://localhost", "key")
	if engine.Name() != "my-search" {
		t.Errorf("expected 'my-search', got '%s'", engine.Name())
	}
}

func TestCustomEngine_Search_Success(t *testing.T) {
	// 启动 mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求方法和头
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Bearer test-key, got %s", r.Header.Get("Authorization"))
		}

		// 验证请求体
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		if reqBody["query"] != "golang" {
			t.Errorf("expected query 'golang', got '%v'", reqBody["query"])
		}

		// 返回响应
		resp := map[string]interface{}{
			"results": []SearchResultItem{
				{Title: "Go 官网", URL: "https://go.dev", Snippet: "Go 语言官方网站"},
				{Title: "Go Doc", URL: "https://pkg.go.dev", Snippet: "Go 包文档"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	engine := NewCustomEngine("test", server.URL, "test-key")
	results, err := engine.Search("golang", 10)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Title != "Go 官网" {
		t.Errorf("expected 'Go 官网', got '%s'", results[0].Title)
	}
	if results[0].URL != "https://go.dev" {
		t.Errorf("expected 'https://go.dev', got '%s'", results[0].URL)
	}
	if results[1].Snippet != "Go 包文档" {
		t.Errorf("expected 'Go 包文档', got '%s'", results[1].Snippet)
	}
}

func TestCustomEngine_Search_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	engine := NewCustomEngine("test", server.URL, "")
	_, err := engine.Search("test", 10)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	t.Logf("正确捕获错误: %v", err)
}

func TestCustomEngine_Search_NoAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 无 apiKey 时不应发送 Authorization 头
		if r.Header.Get("Authorization") != "" {
			t.Error("无 apiKey 时不应发送 Authorization 头")
		}
		resp := map[string]interface{}{"results": []SearchResultItem{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	engine := NewCustomEngine("test", server.URL, "") // 空 apiKey
	results, err := engine.Search("test", 10)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	if results == nil {
		t.Error("results 不应为 nil，应为空切片")
	}
}

func TestCustomEngine_Search_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	engine := NewCustomEngine("test", server.URL, "")
	_, err := engine.Search("test", 10)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	t.Logf("正确捕获 JSON 解析错误: %v", err)
}
