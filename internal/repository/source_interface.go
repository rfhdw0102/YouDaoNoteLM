package repository

import "YoudaoNoteLm/internal/model/entity"

// SourceRepository 资料来源仓储接口
type SourceRepository interface {
	FindByID(id uint) (*entity.Source, error)
	Create(source *entity.Source) error
	Update(source *entity.Source) error
	Delete(id uint) error
	BatchDelete(ids []uint) error
	ListByNotebook(userID, notebookID uint, keyword string, offset, limit int) ([]*entity.Source, int64, error)
	UpdateStatus(id uint, status string, errMsg string) error
	SetVectorized(id uint) error
}
