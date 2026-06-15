package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// 测试加载配置文件
	configPath := "../../configs/config.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skip("配置文件不存在，跳过测试（CI 环境中无配置文件）")
	}

	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("加载配置文件失败: %v", err)
	}

	// 验证 MinIO 配置
	if config.External.MinIO.Endpoint == "" {
		t.Error("MinIO Endpoint 不能为空")
	}
	if config.External.MinIO.AccessKey == "" {
		t.Error("MinIO AccessKey 不能为空")
	}
	if config.External.MinIO.SecretKey == "" {
		t.Error("MinIO SecretKey 不能为空")
	}
	if config.External.MinIO.Bucket == "" {
		t.Error("MinIO Bucket 不能为空")
	}

	// 验证 MarkItDown 配置
	if config.External.MarkItDown.URL == "" {
		t.Error("MarkItDown URL 不能为空")
	}

	t.Logf("MinIO 配置加载成功: %s", config.External.MinIO.Endpoint)
	t.Logf("MarkItDown 配置加载成功: %s", config.External.MarkItDown.URL)
}

func TestLoadConfigWithEnv(t *testing.T) {
	configPath := "../../configs/config.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skip("配置文件不存在，跳过测试（CI 环境中无配置文件）")
	}

	// 设置环境变量
	os.Setenv("MINIO_ENDPOINT", "localhost:9000")
	os.Setenv("MINIO_ACCESS_KEY", "testkey")
	os.Setenv("MINIO_SECRET_KEY", "testsecret")
	os.Setenv("MINIO_BUCKET", "testbucket")
	defer func() {
		os.Unsetenv("MINIO_ENDPOINT")
		os.Unsetenv("MINIO_ACCESS_KEY")
		os.Unsetenv("MINIO_SECRET_KEY")
		os.Unsetenv("MINIO_BUCKET")
	}()

	// 测试环境变量覆盖
	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("加载配置文件失败: %v", err)
	}

	// 注意：当前代码没有从环境变量加载 MinIO 配置
	// 这个测试只是验证配置加载不会出错
	t.Logf("MinIO 配置: %s", config.External.MinIO.Endpoint)
}
