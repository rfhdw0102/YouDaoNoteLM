package config

import "time"

// Config 应用配置结构体
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Database DatabaseConfig `mapstructure:"database"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Log      LogConfig      `mapstructure:"log"`
	CORS     CORSConfig     `mapstructure:"cors"`
	Email    EmailConfig    `mapstructure:"email"`
	External ExternalConfig `mapstructure:"external"`
}

// ExternalConfig 外部服务配置
type ExternalConfig struct {
	MarkItDown MarkItDownConfig `mapstructure:"markitdown"`
	ASR        ASRConfig        `mapstructure:"asr"`
	MinIO      MinIOConfig      `mapstructure:"minio"`
	Milvus     MilvusConfig     `mapstructure:"milvus"`
	Bocha      BochaConfig      `mapstructure:"bocha"`
}

// BochaConfig 博查联网搜索配置
type BochaConfig struct {
	BaseURL         string `mapstructure:"base_url"`
	Endpoint        string `mapstructure:"endpoint"`
	APIKey          string `mapstructure:"api_key"`
	TimeoutSeconds  int    `mapstructure:"timeout_seconds"`
	DefaultCount    int    `mapstructure:"default_count"`
	Summary         bool   `mapstructure:"summary"`
	CacheTTLSeconds int    `mapstructure:"cache_ttl_seconds"`
	MaxCount        int    `mapstructure:"max_count"`
}

// MilvusConfig Milvus 向量数据库配
// MilvusConfig Milvus 向量数据库配置
type MilvusConfig struct {
	Address  string `mapstructure:"address"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// MarkItDownConfig 文档转换服务配置
type MarkItDownConfig struct {
	URL string `mapstructure:"url"`
}

// ASRConfig ASR 语音转文本配置
// provider: aliyun_nls / whisper / ...
// params: 各服务商特有参数，通过 provider 分发
type ASRConfig struct {
	Provider string                 `mapstructure:"provider"`
	Params   map[string]interface{} `mapstructure:"params"`
}

// GetString 获取参数中的字符串值
func (c *ASRConfig) GetString(key string) string {
	if v, ok := c.Params[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetInt 获取参数中的整数值
func (c *ASRConfig) GetInt(key string) int {
	if v, ok := c.Params[key]; ok {
		if n, ok := v.(int); ok {
			return n
		}
	}
	return 0
}

// MinIOConfig MinIO 对象存储配置
type MinIOConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Bucket    string `mapstructure:"bucket"`
}

// AppConfig 应用配置
type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Mode    string `mapstructure:"mode"` // debug, release, test
	Port    int    `mapstructure:"port"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	MySQL MySQLConfig `mapstructure:"mysql"`
	Redis RedisConfig `mapstructure:"redis"`
}

// MySQLConfig MySQL 配置
type MySQLConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	Database     string `mapstructure:"database"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret          string        `mapstructure:"secret"`
	ExpireHours     time.Duration `mapstructure:"expire_hours"`
	AccessTokenExp  string        `mapstructure:"access_token_exp"`
	RefreshTokenExp string        `mapstructure:"refresh_token_exp"`
	Issuer          string        `mapstructure:"issuer"`
}

// GetAccessTokenExp 获取 Access Token 过期时间
func (c *JWTConfig) GetAccessTokenExp() time.Duration {
	if c.AccessTokenExp != "" {
		d, err := time.ParseDuration(c.AccessTokenExp)
		if err == nil {
			return d
		}
	}
	// 默认 15 分钟
	return 15 * time.Minute
}

// GetRefreshTokenExp 获取 Refresh Token 过期时间
func (c *JWTConfig) GetRefreshTokenExp() time.Duration {
	if c.RefreshTokenExp != "" {
		d, err := time.ParseDuration(c.RefreshTokenExp)
		if err == nil {
			return d
		}
	}
	// 默认 7 天
	return 7 * 24 * time.Hour
}

// GetIssuer 获取签发者
func (c *JWTConfig) GetIssuer() string {
	if c.Issuer != "" {
		return c.Issuer
	}
	return "youdaonotelm"
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`       // debug, info, warn, error
	Filename   string `mapstructure:"filename"`    // 日志文件路径
	MaxSize    int    `mapstructure:"max_size"`    // 单个日志文件最大大小(MB)
	MaxBackups int    `mapstructure:"max_backups"` // 保留的旧日志文件数量
	MaxAge     int    `mapstructure:"max_age"`     // 保留旧日志文件的最大天数
	Compress   bool   `mapstructure:"compress"`    // 是否压缩
}

// CORSConfig CORS 配置
type CORSConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	AllowOrigins     []string `mapstructure:"allow_origins"`
	AllowMethods     []string `mapstructure:"allow_methods"`
	AllowHeaders     []string `mapstructure:"allow_headers"`
	ExposeHeaders    []string `mapstructure:"expose_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age"`
}

// EmailConfig 邮箱配置
type EmailConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"` // 发件人地址，默认使用 Username
}
