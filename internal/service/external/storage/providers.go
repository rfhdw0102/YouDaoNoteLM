// internal/service/external/storage/providers.go
package storage

import (
	"YoudaoNoteLm/internal/service/external"
)

func init() {
	r := external.GetGlobalRegistry()

	// MinIO 对象存储
	r.Register("storage", "minio", "MinIO 对象存储",
		[]string{"endpoint", "access_key", "secret_key", "bucket"}, nil,
		func(cfg *external.ServiceConfig) (interface{}, error) {
			endpoint := cfg.APIURL
			if endpoint == "" {
				endpoint = cfg.GetExtraString("endpoint")
			}
			accessKey := cfg.APIKey
			if accessKey == "" {
				accessKey = cfg.GetExtraString("access_key")
			}
			secretKey := cfg.GetExtraString("secret_key")
			bucket := cfg.GetExtraString("bucket")
			publicEndpoint := cfg.GetExtraString("public_endpoint")
			return NewMinIOStorage(endpoint, accessKey, secretKey, bucket, publicEndpoint)
		}, map[string]string{
			"endpoint":   "MinIO 地址",
			"access_key": "Access Key",
			"secret_key": "Secret Key",
			"bucket":     "Bucket 名称",
		})
}
