package repository

import "YoudaoNoteLm/internal/model/entity"

type YoudaoBindingRepository interface {
	FindByUserID(userID uint) (*entity.YoudaoBinding, error)
	Create(binding *entity.YoudaoBinding) error
	Update(binding *entity.YoudaoBinding) error
	Delete(userID uint) error
}
