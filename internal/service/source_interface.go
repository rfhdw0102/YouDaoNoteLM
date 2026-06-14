package service

import (
	"YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/model/entity"
)

// SourceService 资料来源服务接口
type SourceService interface {
	List(userID, notebookID uint, keyword string, page, size int) ([]*response.SourceResponse, int64, error)
	GetByID(id uint) (*entity.Source, error)
	Rename(id uint, name string) error
	Delete(id uint) error
	BatchDelete(ids []uint) error
	GetContent(id uint) (string, error)
	GetOriginalContent(id uint) (content string, contentType string, err error)
}
