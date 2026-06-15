package entity

// Conversation 会话实体
type Conversation struct {
	BaseEntity
	NotebookID      uint     `gorm:"index;not null;comment:所属笔记本ID"`
	Notebook        Notebook `gorm:"foreignKey:NotebookID;constraint:OnDelete:CASCADE"`
	UserID          uint     `gorm:"index;not null;comment:所属用户ID"`
	Title           string   `gorm:"type:varchar(100);not null;default:'新对话';comment:会话标题"`
	Summary         string   `gorm:"type:text;comment:对话摘要"`
	SummaryMsgCount int      `gorm:"default:0;comment:摘要覆盖的消息数"`

	// 关联
	Messages []Message `gorm:"foreignKey:ConversationID"`
}

// TableName 指定表名
func (Conversation) TableName() string {
	return "conversations"
}
