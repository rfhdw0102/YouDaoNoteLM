package repository

import (
	"YoudaoNoteLm/internal/model/entity"

	"gorm.io/gorm"
)

type parentBlockRepository struct {
	db *gorm.DB
}

// NewParentBlockRepository 创建父块仓库
func NewParentBlockRepository(db *gorm.DB) ParentBlockRepository {
	return &parentBlockRepository{db: db}
}

func (r *parentBlockRepository) Create(block *entity.ParentBlock) error {
	return r.db.Create(block).Error
}

func (r *parentBlockRepository) BatchCreate(blocks []entity.ParentBlock) error {
	if len(blocks) == 0 {
		return nil
	}
	return r.db.CreateInBatches(blocks, 100).Error
}

func (r *parentBlockRepository) FindByID(id uint) (*entity.ParentBlock, error) {
	var block entity.ParentBlock
	err := r.db.First(&block, id).Error
	if err != nil {
		return nil, err
	}
	return &block, nil
}

func (r *parentBlockRepository) FindByIDs(ids []uint) ([]*entity.ParentBlock, error) {
	var blocks []*entity.ParentBlock
	err := r.db.Where("id IN ?", ids).Find(&blocks).Error
	return blocks, err
}

func (r *parentBlockRepository) FindBySourceID(sourceID uint) ([]*entity.ParentBlock, error) {
	var blocks []*entity.ParentBlock
	err := r.db.Where("source_id = ?", sourceID).Order("chunk_index").Find(&blocks).Error
	return blocks, err
}

func (r *parentBlockRepository) DeleteBySourceID(sourceID uint) error {
	return r.db.Where("source_id = ?", sourceID).Delete(&entity.ParentBlock{}).Error
}
