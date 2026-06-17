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
	DeleteFailed(userID, notebookID uint) (int64, error)
	GetContent(id uint) (string, error)
	GetOriginalContent(id uint) (content string, contentType string, err error)
	GetDownloadURL(id uint) (string, error)
	// ReimportAll 重新导入用户所有未向量化的资料
	ReimportAll(userID uint) (int, error)
	// ReimportSelected 重新导入指定的未向量化资料
	ReimportSelected(sourceIDs []uint) (int, error)
}
