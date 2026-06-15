package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

// MinioStorage MinIO 存储实现（导出，供外部包类型断言使用）
type MinioStorage struct {
	client *minio.Client
	bucket string
}

// NewMinIOStorage 创建 MinIO 存储
func NewMinIOStorage(endpoint, accessKey, secretKey, bucket string) (FileStorage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("MinIO 初始化失败: %w", err)
	}

	return &MinioStorage{client: client, bucket: bucket}, nil
}

// Upload 上传文件到 MinIO
func (s *MinioStorage) Upload(file *multipart.FileHeader) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("打开上传文件失败: %w", err)
	}
	defer func() {
		if closeErr := src.Close(); closeErr != nil {
			logger.Warn("关闭上传文件句柄失败", zap.String("file", file.Filename), zap.Error(closeErr))
		}
	}()

	objectName := fmt.Sprintf("uploads/%d%s", time.Now().UnixMilli(), filepath.Ext(file.Filename))

	_, err = s.client.PutObject(context.Background(), s.bucket, objectName, src, file.Size,
		minio.PutObjectOptions{ContentType: file.Header.Get("Content-Type")})
	if err != nil {
		return "", fmt.Errorf("MinIO 上传失败: %w", err)
	}

	logger.Info("文件上传成功", zap.String("path", objectName), zap.Int64("size", file.Size))
	return objectName, nil
}

// Download 从 MinIO 下载文件
func (s *MinioStorage) Download(filePath string) ([]byte, error) {
	obj, err := s.client.GetObject(context.Background(), s.bucket, filePath, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("MinIO 获取文件失败: %w", err)
	}
	defer func() {
		if closeErr := obj.Close(); closeErr != nil {
			logger.Warn("关闭 MinIO 对象流失败", zap.String("path", filePath), zap.Error(closeErr))
		}
	}()

	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("MinIO 读取文件失败: %w", err)
	}
	return data, nil
}

// Delete 从 MinIO 删除文件
func (s *MinioStorage) Delete(filePath string) error {
	err := s.client.RemoveObject(context.Background(), s.bucket, filePath, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("MinIO 删除文件失败: %w", err)
	}
	logger.Info("文件删除成功", zap.String("path", filePath))
	return nil
}

// GetPresignedURL 获取临时访问 URL
func (s *MinioStorage) GetPresignedURL(filePath string, expiry time.Duration) (string, error) {
	url, err := s.client.PresignedGetObject(context.Background(), s.bucket, filePath, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("生成预签名URL失败: %w", err)
	}
	return url.String(), nil
}

// UploadBytes 上传字节数据（用于内部调用）
func (s *MinioStorage) UploadBytes(objectName string, data []byte, contentType string) error {
	_, err := s.client.PutObject(context.Background(), s.bucket, objectName,
		bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("MinIO 上传字节数据失败: %w", err)
	}
	return nil
}
