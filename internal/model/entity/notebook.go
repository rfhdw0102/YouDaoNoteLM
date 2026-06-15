package entity

// Notebook 笔记本实体
type Notebook struct {
	BaseEntity
	UserID uint   `gorm:"index;not null;comment:所属用户ID"`
	User   User   `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	Name   string `gorm:"type:varchar(100);not null;comment:笔记本名称"`

	// 关联
	Conversations []Conversation `gorm:"foreignKey:NotebookID"`
}

// TableName 指定表名
func (Notebook) TableName() string {
	return "notebooks"
}
