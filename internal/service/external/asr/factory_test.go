package asr

import (
	"YoudaoNoteLm/internal/service/external/storage"
	"encoding/json"
	"testing"
)

func TestNewASRServiceFromDB_AliyunNLS(t *testing.T) {
	extraConfig := map[string]interface{}{
		"access_key_secret": "secret123",
		"app_key":           "app456",
	}
	extraJSON, _ := json.Marshal(extraConfig)

	svc := NewASRServiceFromDB("aliyun_nls", "", "access_key_id_789", string(extraJSON))
	if svc == nil {
		t.Fatal("expected non-nil ASRService for aliyun_nls")
	}

	// 验证实现了 ASRService 接口
	var _ ASRService = svc

	// 验证实现了 SetStorage 接口
	setter, ok := svc.(interface{ SetStorage(storage.FileStorage) })
	if !ok {
		t.Fatal("aliyun NLS service should implement SetStorage")
	}
	setter.SetStorage(nil) // 不应 panic
}

func TestNewASRServiceFromDB_AliyunNLS_EmptyExtraConfig(t *testing.T) {
	svc := NewASRServiceFromDB("aliyun_nls", "", "key", "")
	if svc != nil {
		t.Fatal("expected nil with empty extra config (missing required fields)")
	}
}

func TestNewASRServiceFromDB_AliyunNLS_InvalidJSON(t *testing.T) {
	svc := NewASRServiceFromDB("aliyun_nls", "", "key", "not-json")
	if svc != nil {
		t.Fatal("expected nil with invalid JSON")
	}
}

func TestNewASRServiceFromDB_UnsupportedProvider(t *testing.T) {
	svc := NewASRServiceFromDB("unknown_provider", "", "", "")
	if svc != nil {
		t.Fatal("expected nil for unsupported provider")
	}
}

func TestNewASRServiceFromDB_EmptyProvider(t *testing.T) {
	svc := NewASRServiceFromDB("", "", "", "")
	if svc != nil {
		t.Fatal("expected nil for empty provider")
	}
}
