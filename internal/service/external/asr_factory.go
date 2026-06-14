package external

import (
	"fmt"

	"YoudaoNoteLm/pkg/config"
)

// NewASRService 根据配置创建 ASR 服务
// 配置示例：
//
//	asr:
//	  provider: aliyun_nls
//	  params:
//	    access_key_id: "xxx"
//	    access_key_secret: "xxx"
//	    app_key: "xxx"
func NewASRService(cfg config.ASRConfig) ASRService {
	switch cfg.Provider {
	case "aliyun_nls":
		return NewAliyunNLSASRService(
			cfg.GetString("access_key_id"),
			cfg.GetString("access_key_secret"),
			cfg.GetString("app_key"),
		)
	default:
		panic(fmt.Sprintf("不支持的 ASR provider: %s", cfg.Provider))
	}
}
