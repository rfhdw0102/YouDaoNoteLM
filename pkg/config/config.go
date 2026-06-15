package config

import (
	"fmt"
	"time"
)

// Validate 校验所有必填配置项，缺失或无效时返回 error。
func (c *Config) Validate() error {
	// App
	if c.App.Name == "" {
		return fmt.Errorf("app.name 不能为空")
	}
	if c.App.Port == 0 {
		return fmt.Errorf("app.port 不能为空")
	}

	// MySQL
	if c.Database.MySQL.Host == "" {
		return fmt.Errorf("database.mysql.host 不能为空")
	}
	if c.Database.MySQL.Port == 0 {
		return fmt.Errorf("database.mysql.port 不能为空")
	}
	if c.Database.MySQL.Username == "" {
		return fmt.Errorf("database.mysql.username 不能为空")
	}
	if c.Database.MySQL.Database == "" {
		return fmt.Errorf("database.mysql.database 不能为空")
	}

	// Redis
	if c.Database.Redis.Host == "" {
		return fmt.Errorf("database.redis.host 不能为空")
	}
	if c.Database.Redis.Port == 0 {
		return fmt.Errorf("database.redis.port 不能为空")
	}

	// JWT
	if c.JWT.Secret == "" {
		return fmt.Errorf("jwt.secret 不能为空")
	}

	// Log
	if c.Log.Filename == "" {
		return fmt.Errorf("log.filename 不能为空")
	}

	// Email
	if c.Email.Host == "" {
		return fmt.Errorf("email.host 不能为空")
	}
	if c.Email.Port == 0 {
		return fmt.Errorf("email.port 不能为空")
	}
	if c.Email.Username == "" {
		return fmt.Errorf("email.username 不能为空")
	}
	if c.Email.Password == "" {
		return fmt.Errorf("email.password 不能为空")
	}

	// Milvus
	if c.Milvus.Host == "" {
		return fmt.Errorf("milvus.host 不能为空")
	}
	if c.Milvus.Port == 0 {
		return fmt.Errorf("milvus.port 不能为空")
	}

	// External - MarkItDown
	if c.External.MarkItDown.URL == "" {
		return fmt.Errorf("external.markitdown.url 不能为空")
	}

	// External - MinIO
	if c.External.MinIO.Endpoint == "" {
		return fmt.Errorf("external.minio.endpoint 不能为空")
	}
	if c.External.MinIO.AccessKey == "" {
		return fmt.Errorf("external.minio.access_key 不能为空")
	}
	if c.External.MinIO.SecretKey == "" {
		return fmt.Errorf("external.minio.secret_key 不能为空")
	}
	if c.External.MinIO.Bucket == "" {
		return fmt.Errorf("external.minio.bucket 不能为空")
	}

	// External - Youdao
	if c.External.Youdao.CLIPath == "" {
		return fmt.Errorf("external.youdao.cli_path 不能为空")
	}

	return nil
}

// Config 应用配置结构体
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Database DatabaseConfig `mapstructure:"database"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Log      LogConfig      `mapstructure:"log"`
	CORS     CORSConfig     `mapstructure:"cors"`
	Email    EmailConfig    `mapstructure:"email"`
	External ExternalConfig `mapstructure:"external"`
	Milvus   MilvusConfig   `mapstructure:"milvus"`
}

// MilvusConfig Milvus 向量数据库配置
type MilvusConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// GetAddress 返回 host:port 格式的地址
func (c *MilvusConfig) GetAddress() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// ExternalConfig 外部服务配置
type ExternalConfig struct {
	MarkItDown MarkItDownConfig `mapstructure:"markitdown"`
	MinIO      MinIOConfig      `mapstructure:"minio"`
	Youdao     YoudaoConfig     `mapstructure:"youdao"`
}

// YoudaoConfig 有道云笔记 CLI 配置
type YoudaoConfig struct {
	CLIPath             string `mapstructure:"cli_path"`              // CLI 路径，默认 "youdaonote"（在 PATH 中）
	ConverterScriptPath string `mapstructure:"converter_script_path"` // youdaonote-pull 转换脚本路径（可选，用于 .note 格式转换）
	CookiesPath         string `mapstructure:"cookies_path"`          // youdaonote cookies 文件路径（可选，用于 .note 格式转换）
}

// MarkItDownConfig 文档转换服务配置
type MarkItDownConfig struct {
	URL string `mapstructure:"url"`
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
