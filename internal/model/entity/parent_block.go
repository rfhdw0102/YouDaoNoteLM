package entity

// ParentBlock 父块实体
type ParentBlock struct {
	BaseEntity
	SourceID    uint   `gorm:"index;not null;comment:所属资料来源ID"`
	Source      Source `gorm:"foreignKey:SourceID;constraint:OnDelete:CASCADE"`
	Heading     string `gorm:"type:varchar(255);comment:父块标题/小标题"`
	Level       int    `gorm:"not null;default:0;comment:标题层级(H1=1/H2=2/H3=3...)"`
	ChapterPath string `gorm:"type:varchar(500);comment:完整章节路径"`
	Content     string `gorm:"type:text;not null;comment:父块原文内容"`
	ChunkIndex  int    `gorm:"not null;comment:父块在来源中的序号(从0开始)"`
	Metadata    string `gorm:"type:json;comment:元数据JSON(页码/章节等)"`
}

// TableName 指定表名
func (ParentBlock) TableName() string {
	return "parent_blocks"
}
