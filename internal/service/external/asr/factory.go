package asr

import (
	"encoding/json"

	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

// NewASRServiceFromDB 根据数据库配置创建 ASR 服务
// provider: 服务商名称 (如 "aliyun_nls")
// apiURL: API 地址（可选）
// apiKey: API 密钥（可选）
// extraConfig: JSON 格式的额外配置
// Deprecated: 使用 Registry.Create 替代
func NewASRServiceFromDB(provider, apiURL, apiKey, extraConfig string) ASRService {
	switch provider {
	case "aliyun_nls":
		// 解析额外配置
		var config map[string]string
		if extraConfig != "" {
			if err := json.Unmarshal([]byte(extraConfig), &config); err != nil {
				logger.Error("解析 ASR 额外配置失败", zap.Error(err))
				return nil
			}
		}

		// 从配置中获取参数
		accessKeyID := apiKey
		accessKeySecret := config["access_key_secret"]
		appKey := config["app_key"]

		// 如果 apiKey 包含 access_key_id，则从 extraConfig 获取其他参数
		if val, ok := config["access_key_id"]; ok {
			accessKeyID = val
		}

		if accessKeyID == "" || accessKeySecret == "" || appKey == "" {
			logger.Error("阿里云 ASR 配置不完整",
				zap.String("access_key_id", accessKeyID),
				zap.String("app_key", appKey))
			return nil
		}

		return NewAliyunNLSASRService(accessKeyID, accessKeySecret, appKey)
	default:
		logger.Error("不支持的 ASR provider", zap.String("provider", provider))
		return nil
	}
}
