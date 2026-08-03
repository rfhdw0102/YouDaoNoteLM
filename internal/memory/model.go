package memory

import (
	"time"

	"YoudaoNoteLm/internal/model/entity"
)

// UserMemory is deliberately not based on entity.BaseEntity. Forgetting a
// preference must remove its content instead of leaving a soft-deleted copy.
type UserMemory struct {
	ID         uint        `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     uint        `gorm:"not null;uniqueIndex:uk_user_memory_type;index" json:"user_id"`
	User       entity.User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	MemoryType string      `gorm:"type:varchar(32);not null;uniqueIndex:uk_user_memory_type" json:"type"`
	Content    string      `gorm:"type:varchar(512);not null" json:"content"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

func (UserMemory) TableName() string {
	return "user_memories"
}
