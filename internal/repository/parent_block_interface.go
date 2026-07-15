package repository

import "YoudaoNoteLm/internal/model/entity"

// ParentBlockRepository 父块数据访问接口
type ParentBlockRepository interface {
	Create(block *entity.ParentBlock) error
	BatchCreate(blocks []entity.ParentBlock) error
	FindByID(id uint) (*entity.ParentBlock, error)
	FindByIDs(ids []uint) ([]*entity.ParentBlock, error)
	FindBySourceID(sourceID uint) ([]*entity.ParentBlock, error)
	DeleteBySourceID(sourceID uint) error
}
