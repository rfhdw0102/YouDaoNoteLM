package storage

import (
	"errors"
	"strings"
	"testing"
)

func TestUnavailableStorageReturnsHelpfulError(t *testing.T) {
	store := NewUnavailableStorage("MinIO", errors.New("dial timeout"))

	if store == nil {
		t.Fatal("expected unavailable storage")
	}

	err := store.UploadBytes("avatars/1.png", []byte("image"), "image/png")
	if err == nil {
		t.Fatal("expected upload to fail")
	}
	if !strings.Contains(err.Error(), "MinIO 存储服务不可用") {
		t.Fatalf("expected service unavailable message, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "dial timeout") {
		t.Fatalf("expected original cause in error, got %q", err.Error())
	}
}
