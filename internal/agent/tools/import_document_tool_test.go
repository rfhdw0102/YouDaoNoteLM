package tools

import (
	"context"
	"testing"
)

func TestGetUserID_默认返回零(t *testing.T) {
	ctx := context.Background()
	if got := GetUserID(ctx); got != 0 {
		t.Errorf("GetUserID(empty ctx) = %d, want 0", got)
	}
}

func TestWithAndGetUserID(t *testing.T) {
	ctx := WithUserID(context.Background(), 42)
	if got := GetUserID(ctx); got != 42 {
		t.Errorf("GetUserID = %d, want 42", got)
	}
}

func TestWithAndGetNotebookID(t *testing.T) {
	ctx := WithNotebookID(context.Background(), 99)
	if got := GetNotebookID(ctx); got != 99 {
		t.Errorf("GetNotebookID = %d, want 99", got)
	}
}

func TestGetNotebookID_默认返回零(t *testing.T) {
	ctx := context.Background()
	if got := GetNotebookID(ctx); got != 0 {
		t.Errorf("GetNotebookID(empty ctx) = %d, want 0", got)
	}
}
