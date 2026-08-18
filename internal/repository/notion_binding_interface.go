package repository

import "YoudaoNoteLm/internal/model/entity"

// NotionBindingRepository 提供 Notion OAuth 绑定的数据访问。
type NotionBindingRepository interface {
	FindByUserID(userID uint) (*entity.NotionBinding, error)
	Upsert(binding *entity.NotionBinding) error
	Delete(userID uint) error
}
