package repository

import "YoudaoNoteLm/internal/model/entity"

// SourceRepository 资料来源仓储接口
type SourceRepository interface {
	FindByID(id uint) (*entity.Source, error)
	FindByIDs(ids []uint) ([]*entity.Source, error)
	Create(source *entity.Source) error
	Update(source *entity.Source) error
	UpdateContent(id uint, markdown string, status string) error
	UpdateSummary(id uint, summary string) error
	Delete(id uint) error
	BatchDelete(ids []uint) error
	DeleteByNotebookID(notebookID uint) error
	ListByNotebook(userID, notebookID uint, keyword string, offset, limit int) ([]*entity.Source, int64, error)
	UpdateStatus(id uint, status string, errMsg string) error
	SetVectorized(id uint) error
	DeleteFailedByNotebook(userID, notebookID uint) (int64, error)
	// ResetVectorizedByUserID 重置用户所有资料的向量化状态（删除向量模型后调用）
	ResetVectorizedByUserID(userID uint) error
	// FindUnvectorizedByUserID 获取用户所有未向量化的资料
	FindUnvectorizedByUserID(userID uint) ([]*entity.Source, error)
	// FindSummaryByID 获取资料摘要
	FindSummaryByID(id uint) (string, error)
	// FindReadyByNotebookID 获取笔记本下所有就绪资料（fallback 用）
	FindReadyByNotebookID(notebookID uint) ([]*entity.Source, error)
}
