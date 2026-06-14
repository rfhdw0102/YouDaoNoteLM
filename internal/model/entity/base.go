package entity

import (
	"time"

	"gorm.io/gorm"
)

// BaseEntity 基础实体
type BaseEntity struct {
	ID        uint           `gorm:"primarykey;autoIncrement" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
