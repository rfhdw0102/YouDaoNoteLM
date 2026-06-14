package entity

// YoudaoBinding 有道云笔记绑定实体
type YoudaoBinding struct {
	BaseEntity
	UserID uint   `gorm:"not null;uniqueIndex:uk_user" json:"user_id"` // 所属用户
	User   User   `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	APIKey string `gorm:"type:varchar(512);not null" json:"api_key"`     // 有道API密钥
	Status string `gorm:"type:varchar(20);default:active" json:"status"` // 状态: active/revoked
}

func (YoudaoBinding) TableName() string {
	return "youdao_binding"
}
