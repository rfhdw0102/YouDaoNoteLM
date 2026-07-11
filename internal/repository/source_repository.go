package repository

import (
	"YoudaoNoteLm/internal/model/entity"
	"errors"

	"gorm.io/gorm"
)

type sourceRepository struct {
	db *gorm.DB
}

func NewSourceRepository(db *gorm.DB) SourceRepository {
	return &sourceRepository{db: db}
}

func (r *sourceRepository) FindByID(id uint) (*entity.Source, error) {
	var source entity.Source
	err := r.db.First(&source, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &source, nil
}

// FindByIDs 批量查询资料（一次 SQL，避免 N+1）
func (r *sourceRepository) FindByIDs(ids []uint) ([]*entity.Source, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var sources []*entity.Source
	err := r.db.Where("id IN ?", ids).Find(&sources).Error
	return sources, err
}

func (r *sourceRepository) Create(source *entity.Source) error {
	return r.db.Create(source).Error
}

func (r *sourceRepository) Update(source *entity.Source) error {
	return r.db.Save(source).Error
}

func (r *sourceRepository) Delete(id uint) error {
	return r.db.Delete(&entity.Source{}, id).Error
}

func (r *sourceRepository) BatchDelete(ids []uint) error {
	return r.db.Delete(&entity.Source{}, "id IN ?", ids).Error
}

func (r *sourceRepository) DeleteByNotebookID(notebookID uint) error {
	return r.db.Where("notebook_id = ?", notebookID).Delete(&entity.Source{}).Error
}

func (r *sourceRepository) ListByNotebook(userID, notebookID uint, keyword string, offset, limit int) ([]*entity.Source, int64, error) {
	var sources []*entity.Source
	var total int64

	query := r.db.Where("user_id = ? AND notebook_id = ?", userID, notebookID)
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}

	if err := query.Model(&entity.Source{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&sources).Error
	if err != nil {
		return nil, 0, err
	}

	return sources, total, nil
}

func (r *sourceRepository) UpdateStatus(id uint, status string, errMsg string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if errMsg != "" {
		// 截断过长的错误信息，防止超出数据库列宽（varchar(1024)）
		const maxErrMsgLen = 1000
		if len(errMsg) > maxErrMsgLen {
			errMsg = errMsg[:maxErrMsgLen] + "...(truncated)"
		}
		updates["error_message"] = errMsg
	}
	return r.db.Model(&entity.Source{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateContent 更新源内容和状态（不覆盖其他字段如 notebook_id）
func (r *sourceRepository) UpdateContent(id uint, markdown string, status string) error {
	updates := map[string]interface{}{
		"markdown_content": markdown,
		"status":           status,
	}
	return r.db.Model(&entity.Source{}).Where("id = ?", id).Updates(updates).Error
}

func (r *sourceRepository) SetVectorized(id uint) error {
	return r.db.Model(&entity.Source{}).Where("id = ?", id).Update("vectorized", true).Error
}

func (r *sourceRepository) DeleteFailedByNotebook(userID, notebookID uint) (int64, error) {
	result := r.db.Where("user_id = ? AND notebook_id = ? AND status = ?", userID, notebookID, "failed").Delete(&entity.Source{})
	return result.RowsAffected, result.Error
}

// ResetVectorizedByUserID 重置用户所有资料的向量化状态
// 删除向量模型后调用，将所有已向量化的资料标记为未向量化，状态改为 ready 以便重新导入
func (r *sourceRepository) ResetVectorizedByUserID(userID uint) error {
	return r.db.Model(&entity.Source{}).
		Where("user_id = ? AND vectorized = ?", userID, true).
		Updates(map[string]interface{}{
			"vectorized":    false,
			"status":        "ready",
			"error_message": "",
		}).Error
}

// FindUnvectorizedByUserID 获取用户所有未向量化的资料（状态为 ready 且未向量化）
func (r *sourceRepository) FindUnvectorizedByUserID(userID uint) ([]*entity.Source, error) {
	var sources []*entity.Source
	err := r.db.Where("user_id = ? AND status = ? AND vectorized = ?", userID, "ready", false).
		Find(&sources).Error
	return sources, err
}

// UpdateSummary 更新资料摘要
func (r *sourceRepository) UpdateSummary(id uint, summary string) error {
	return r.db.Model(&entity.Source{}).Where("id = ?", id).Update("summary", summary).Error
}

// FindSummaryByID 获取资料摘要
func (r *sourceRepository) FindSummaryByID(id uint) (string, error) {
	var source entity.Source
	err := r.db.Select("summary").First(&source, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return source.Summary, nil
}
