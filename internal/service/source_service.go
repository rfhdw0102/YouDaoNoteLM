package service

import (
	"context"

	"YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/rag"
	"YoudaoNoteLm/internal/repository"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

type sourceService struct {
	sourceRepo   repository.SourceRepository
	ingestionSvc rag.IngestionService
}

func NewSourceService(sourceRepo repository.SourceRepository, ingestionSvc rag.IngestionService) SourceService {
	return &sourceService{
		sourceRepo:   sourceRepo,
		ingestionSvc: ingestionSvc,
	}
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
	source, err := s.GetByID(id)
	if err != nil {
		return err
	}

	// 删除 Milvus 向量数据
	if s.ingestionSvc != nil && source.Vectorized {
		ctx := context.Background()
		if err := s.ingestionSvc.DeleteSource(ctx, source.UserID, source.ID); err != nil {
			logger.Warn("删除向量数据失败，继续删除数据库记录",
				zap.Uint("source_id", source.ID),
				zap.Error(err),
			)
		}
	}

	return s.sourceRepo.Delete(id)
}

func (s *sourceService) BatchDelete(ids []uint) error {
	// 先获取所有 source 信息，删除 Milvus 数据
	if s.ingestionSvc != nil {
		ctx := context.Background()
		for _, id := range ids {
			source, err := s.sourceRepo.FindByID(id)
			if err != nil || source == nil {
				continue
			}
			if source.Vectorized {
				if err := s.ingestionSvc.DeleteSource(ctx, source.UserID, source.ID); err != nil {
					logger.Warn("批量删除：删除向量数据失败",
						zap.Uint("source_id", source.ID),
						zap.Error(err),
					)
				}
			}
		}
	}

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
		// 返回文件路径，前端通过该路径从对象存储获取并渲染原格式（PDF/DOCX等）
		return source.FilePath, source.MimeType, nil
	case "url":
		return source.OriginalURL, "url", nil
	case "audio":
		return "", "", bizerrors.New(bizerrors.CodeBadRequest, "音频类型不支持查看原格式")
	case "note", "youdao":
		return source.MarkdownContent, "raw_markdown", nil
	default:
		return "", "", bizerrors.New(bizerrors.CodeBadRequest, "该类型不支持查看原格式")
	}
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
