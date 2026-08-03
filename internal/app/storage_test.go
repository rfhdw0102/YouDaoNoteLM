package app

import (
	"errors"
	"testing"

	"YoudaoNoteLm/internal/service/external/storage"
	"YoudaoNoteLm/pkg/config"
)

func TestNewOptionalMinIOStorageDegradesWhenFactoryFails(t *testing.T) {
	cause := errors.New("bucket check failed")

	store, degradedErr := newOptionalMinIOStorage(config.MinIOConfig{
		Endpoint:       "127.0.0.1:9000",
		AccessKey:      "minio",
		SecretKey:      "password",
		Bucket:         "youdaonotelm",
		PublicEndpoint: "http://127.0.0.1:9000",
	}, func(endpoint, accessKey, secretKey, bucket, publicEndpoint string) (storage.FileStorage, error) {
		return nil, cause
	})

	if !errors.Is(degradedErr, cause) {
		t.Fatalf("expected degraded error to wrap factory failure, got %v", degradedErr)
	}
	if store == nil {
		t.Fatal("expected fallback storage")
	}
	if err := store.Delete("uploads/source.pdf"); err == nil {
		t.Fatal("expected fallback storage operations to fail")
	}
}
