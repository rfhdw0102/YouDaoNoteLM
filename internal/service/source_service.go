package service

import (
	"YoudaoNoteLm/internal/service/external/storage"
	"time"

	"YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	bizerrors "YoudaoNoteLm/pkg/errors"
)

type sourceService struct {
	sourceRepo repository.SourceRepository
	storage    storage.FileStorage
}

func NewSourceService(sourceRepo repository.SourceRepository, storage storage.FileStorage) SourceService {
	return &sourceService{sourceRepo: sourceRepo, storage: storage}
}

func (s *sourceService) List(userID, notebookID uint, keyword string, page, size int) ([]*response.SourceResponse, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	if size > 100 {
		size = 100
	}

	offset := (page - 1) * size
	sources, total, err := s.sourceRepo.ListByNotebook(userID, notebookID, keyword, offset, size)
	if err != nil {
		return nil, 0, err
	}

	list := make([]*response.SourceResponse, 0, len(sources))
	for _, src := range sources {
		list = append(list, toSourceResponse(src))
	}

	return list, total, nil
}

func (s *sourceService) GetByID(id uint) (*entity.Source, error) {
	source, err := s.sourceRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, bizerrors.ErrNotFound
	}
	return source, nil
}

func (s *sourceService) Rename(id uint, name string) error {
	source, err := s.GetByID(id)
	if err != nil {
		return err
	}
	source.Name = name
	return s.sourceRepo.Update(source)
}

func (s *sourceService) Delete(id uint) error {
	_, err := s.GetByID(id)
	if err != nil {
		return err
	}
	return s.sourceRepo.Delete(id)
}

func (s *sourceService) BatchDelete(ids []uint) error {
	return s.sourceRepo.BatchDelete(ids)
}

func (s *sourceService) GetContent(id uint) (string, error) {
	source, err := s.GetByID(id)
	if err != nil {
		return "", err
	}
	return source.MarkdownContent, nil
}

func (s *sourceService) GetOriginalContent(id uint) (string, string, error) {
	source, err := s.GetByID(id)
	if err != nil {
		return "", "", err
	}

	switch source.Type {
	case "file":
		// 对于文件类型，返回 Markdown 内容作为原内容展示
		// 原始文件通过 GetDownloadURL 提供下载
		return source.MarkdownContent, source.MimeType, nil
	case "url":
		return source.OriginalURL, "url", nil
	case "audio":
		return source.MarkdownContent, "audio_transcript", nil
	case "note", "youdao":
		return source.MarkdownContent, "raw_markdown", nil
	default:
		return "", "", bizerrors.New(bizerrors.CodeBadRequest, "该类型不支持查看原格式")
	}
}

func (s *sourceService) GetDownloadURL(id uint) (string, error) {
	source, err := s.GetByID(id)
	if err != nil {
		return "", err
	}
	if source.FilePath == "" {
		return "", bizerrors.New(bizerrors.CodeBadRequest, "该来源没有可下载的文件")
	}

	// 如果 storage 是 MinIO，生成预签名 URL
	if minioStorage, ok := s.storage.(interface {
		GetPresignedURL(filePath string, expiry time.Duration) (string, error)
	}); ok {
		url, err := minioStorage.GetPresignedURL(source.FilePath, 10*time.Minute)
		if err != nil {
			return "", bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "生成下载链接失败", err)
		}
		return url, nil
	}

	return "", bizerrors.New(bizerrors.CodeInternalServiceError, "存储服务不支持生成下载链接")
}

func toSourceResponse(src *entity.Source) *response.SourceResponse {
	return &response.SourceResponse{
		ID:           src.ID,
		NotebookID:   src.NotebookID,
		Name:         src.Name,
		Type:         src.Type,
		OriginalURL:  src.OriginalURL,
		FilePath:     src.FilePath,
		FileSize:     src.FileSize,
		MimeType:     src.MimeType,
		Status:       src.Status,
		ErrorMessage: src.ErrorMessage,
		Vectorized:   src.Vectorized,
		CreatedAt:    src.CreatedAt,
		UpdatedAt:    src.UpdatedAt,
	}
}
