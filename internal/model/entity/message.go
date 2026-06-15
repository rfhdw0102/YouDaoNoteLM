package entity

// Message 消息实体
type Message struct {
	BaseEntity
	ConversationID uint         `gorm:"index;not null;comment:所属会话ID"`
	Conversation   Conversation `gorm:"foreignKey:ConversationID;constraint:OnDelete:CASCADE"`
	Role           string       `gorm:"type:varchar(20);not null;comment:角色:user/assistant/system"`
	Content        string       `gorm:"type:text;not null;comment:消息内容"`
	Metadata       string       `gorm:"type:json;comment:元数据JSON"`
}

// TableName 指定表名
func (Message) TableName() string {
	return "messages"
}
