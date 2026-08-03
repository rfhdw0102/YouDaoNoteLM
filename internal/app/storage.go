package app

import (
	"fmt"

	externalStorage "YoudaoNoteLm/internal/service/external/storage"
	"YoudaoNoteLm/pkg/config"
)

type minioStorageFactory func(endpoint, accessKey, secretKey, bucket, publicEndpoint string) (externalStorage.FileStorage, error)

func newOptionalMinIOStorage(cfg config.MinIOConfig, factory minioStorageFactory) (externalStorage.FileStorage, error) {
	store, err := factory(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey, cfg.Bucket, cfg.PublicEndpoint)
	if err == nil {
		return store, nil
	}

	degradedErr := fmt.Errorf("MinIO initialization failed: %w", err)
	return externalStorage.NewUnavailableStorage("MinIO", degradedErr), degradedErr
}
