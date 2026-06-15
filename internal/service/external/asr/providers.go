// internal/service/external/asr/providers.go
package asr

import (
	"fmt"

	"YoudaoNoteLm/internal/service/external"
)

const ServiceType = "asr"

func init() {
	r := external.GetGlobalRegistry()

	// 阿里云 NLS（唯一支持的 ASR 服务商）
	r.Register(ServiceType, "aliyun_nls", "阿里云智能语音",
		[]string{"access_key_id", "access_key_secret", "app_key"}, nil,
		func(cfg *external.ServiceConfig) (interface{}, error) {
			accessKeyID := cfg.APIKey
			if v := cfg.GetExtraString("access_key_id"); v != "" {
				accessKeyID = v
			}
			accessKeySecret := cfg.GetExtraString("access_key_secret")
			appKey := cfg.GetExtraString("app_key")

			if accessKeyID == "" || accessKeySecret == "" || appKey == "" {
				return nil, fmt.Errorf("阿里云 ASR 配置不完整: access_key_id, access_key_secret, app_key 均为必填")
			}

			return NewAliyunNLSASRService(accessKeyID, accessKeySecret, appKey), nil
		}, map[string]string{
			"access_key_id":     "Access Key ID",
			"access_key_secret": "Access Key Secret",
			"app_key":           "App Key",
		})
}
