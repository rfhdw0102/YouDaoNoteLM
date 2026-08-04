package feedback

import (
	"time"

	"YoudaoNoteLm/internal/model/entity"
)

// AnswerFeedback is deliberately not based on entity.BaseEntity.撤销评价必须
// 真实删除而非留下软删除反馈；用户与消息被硬删除时由外键级联清理。
type AnswerFeedback struct {
	ID         uint           `gorm:"primaryKey;autoIncrement"`
	UserID     uint           `gorm:"not null;uniqueIndex:uk_feedback_user_message;index"`
	User       entity.User    `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	MessageID  uint           `gorm:"not null;uniqueIndex:uk_feedback_user_message;index"`
	Message    entity.Message `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Rating     string         `gorm:"type:varchar(8);not null;index"`
	ReasonCode string         `gorm:"type:varchar(48);not null;index"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TableName 指定表名
func (AnswerFeedback) TableName() string {
	return "answer_feedbacks"
}
