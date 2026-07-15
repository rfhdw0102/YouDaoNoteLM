package utils

import (
	"crypto/rand"
	"encoding/base64"
	"math/big"
	"strings"
)

// Charset 字符集类型
type Charset string

const (
	// Numeric 纯数字字符集
	Numeric Charset = "numeric"
	// AlphaNumeric 字母数字字符集
	AlphaNumeric Charset = "alphanumeric"
)

var charsetMap = map[Charset]string{
	Numeric:      "0123456789",
	AlphaNumeric: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
}

// GenerateRandomString 生成随机字符串
func GenerateRandomString(length int, charset ...Charset) (string, error) {
	if len(charset) == 0 {
		// 默认使用 base64 方式
		bytes := make([]byte, length)
		if _, err := rand.Read(bytes); err != nil {
			return "", err
		}
		return base64.URLEncoding.EncodeToString(bytes)[:length], nil
	}

	// 使用指定字符集
	chars := charsetMap[charset[0]]
	if chars == "" {
		chars = charsetMap[Numeric]
	}

	result := make([]byte, length)
	max := big.NewInt(int64(len(chars)))
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		result[i] = chars[n.Int64()]
	}
	return string(result), nil
}

// Contains 检查字符串是否在切片中
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// TrimSpace 去除首尾空格
func TrimSpace(s string) string {
	return strings.TrimSpace(s)
}

// IsEmpty 检查字符串是否为空
func IsEmpty(s string) bool {
	return TrimSpace(s) == ""
}
