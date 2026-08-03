package logger

import (
	"YoudaoNoteLm/pkg/config"
)

// InitFromConfig 从全局配置初始化日志
func InitFromConfig() error {
	cfg := config.Get().Log
	return Init(&cfg)
}

// InitDefault 使用默认配置初始化日志
func InitDefault() error {
	cfg := &config.LogConfig{
		Level:      "debug",
		Filename:   "logs/app.log",
		MaxSize:    100,
		MaxBackups: 3,
		MaxAge:     28,
		Compress:   true,
	}
	return Init(cfg)
}
