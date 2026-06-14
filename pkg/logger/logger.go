package logger

import (
	"YoudaoNoteLm/pkg/config"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	log   = zap.NewNop()
	sugar = log.Sugar()
)

// Init 初始化日志系统
func Init(cfg *config.LogConfig) error {
	// 日志编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 日志级别
	level := zapcore.InfoLevel
	switch cfg.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	}

	// 文件输出
	fileWriter := &lumberjack.Logger{
		Filename:   cfg.Filename,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}

	// 创建多个输出（文件 + 控制台）
	var writers []zapcore.WriteSyncer
	writers = append(writers, zapcore.AddSync(fileWriter))
	writers = append(writers, zapcore.AddSync(os.Stdout))

	// 核心
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.NewMultiWriteSyncer(writers...),
		level,
	)

	// 创建 logger
	log = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1), zap.AddStacktrace(zapcore.ErrorLevel))
	sugar = log.Sugar()

	return nil
}

// Debug 调试日志
func Debug(msg string, fields ...zap.Field) {
	log.Debug(msg, fields...)
}

// Info 信息日志
func Info(msg string, fields ...zap.Field) {
	log.Info(msg, fields...)
}

// Warn 警告日志
func Warn(msg string, fields ...zap.Field) {
	log.Warn(msg, fields...)
}

// Error 错误日志
func Error(msg string, fields ...zap.Field) {
	log.Error(msg, fields...)
}

// Fatal 致命错误日志
func Fatal(msg string, fields ...zap.Field) {
	log.Fatal(msg, fields...)
}

// Debugf 格式化调试日志
func Debugf(format string, args ...interface{}) {
	sugar.Debugf(format, args...)
}

// Infof 格式化信息日志
func Infof(format string, args ...interface{}) {
	sugar.Infof(format, args...)
}

// Warnf 格式化警告日志
func Warnf(format string, args ...interface{}) {
	sugar.Warnf(format, args...)
}

// Errorf 格式化错误日志
func Errorf(format string, args ...interface{}) {
	sugar.Errorf(format, args...)
}

// Fatalf 格式化致命错误日志
func Fatalf(format string, args ...interface{}) {
	sugar.Fatalf(format, args...)
}

// With 创建带字段的 logger
func With(fields ...zap.Field) *zap.Logger {
	return log.With(fields...)
}

// Sync 同步日志缓冲区
func Sync() error {
	return log.Sync()
}

// GetLogger 获取原始 logger
func GetLogger() *zap.Logger {
	return log
}

// GetSugaredLogger 获取 sugared logger
func GetSugaredLogger() *zap.SugaredLogger {
	return sugar
}
