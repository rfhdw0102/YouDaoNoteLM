package utils

import "time"

// GetCurrentTimestamp 获取当前时间戳（秒）
func GetCurrentTimestamp() int64 {
	return time.Now().Unix()
}

// GetCurrentMilliTimestamp 获取当前时间戳（毫秒）
func GetCurrentMilliTimestamp() int64 {
	return time.Now().UnixMilli()
}

// FormatTime 格式化时间
func FormatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// ParseTime 解析时间字符串
func ParseTime(s string) (time.Time, error) {
	return time.Parse("2006-01-02 15:04:05", s)
}
