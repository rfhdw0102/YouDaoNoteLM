package storage

import (
	"fmt"
	"mime/multipart"
	"time"
)

type unavailableStorage struct {
	name  string
	cause error
}

func NewUnavailableStorage(name string, cause error) FileStorage {
	if name == "" {
		name = "file"
	}
	return &unavailableStorage{name: name, cause: cause}
}

func (s *unavailableStorage) Upload(_ *multipart.FileHeader) (string, error) {
	return "", s.err()
}

func (s *unavailableStorage) UploadBytes(_ string, _ []byte, _ string) error {
	return s.err()
}

func (s *unavailableStorage) Download(_ string) ([]byte, error) {
	return nil, s.err()
}

func (s *unavailableStorage) Delete(_ string) error {
	return s.err()
}

func (s *unavailableStorage) GetPresignedURL(_ string, _ time.Duration) (string, error) {
	return "", s.err()
}

func (s *unavailableStorage) err() error {
	if s.cause != nil {
		return fmt.Errorf("%s 存储服务不可用: %w", s.name, s.cause)
	}
	return fmt.Errorf("%s 存储服务不可用", s.name)
}
