package entity

// Source 资料来源实体
type Source struct {
	BaseEntity
	UserID          uint     `gorm:"not null;index:idx_user_notebook" json:"user_id"`     // 所属用户
	NotebookID      uint     `gorm:"not null;index:idx_user_notebook" json:"notebook_id"` // 所属笔记本
	Notebook        Notebook `gorm:"foreignKey:NotebookID;constraint:OnDelete:CASCADE"`
	Name            string   `gorm:"type:varchar(255);not null" json:"name"`                          // 来源名称
	Type            string   `gorm:"type:varchar(20);not null;index:idx_type" json:"type"`            // 类型: file/url/audio/note/youdao
	OriginalURL     string   `gorm:"type:varchar(2048)" json:"original_url"`                          // 原始URL（网址导入时）
	FilePath        string   `gorm:"type:varchar(512)" json:"file_path"`                              // 对象存储文件路径
	FileSize        int64    `json:"file_size"`                                                       // 文件大小(字节)
	MimeType        string   `gorm:"type:varchar(100)" json:"mime_type"`                              // MIME类型
	MarkdownContent string   `gorm:"type:longtext" json:"markdown_content"`                           // 解析后的Markdown内容
	Status          string   `gorm:"type:varchar(20);default:pending;index:idx_status" json:"status"` // 状态: pending/processing/ready/failed
	ErrorMessage    string   `gorm:"type:varchar(512)" json:"error_message"`                          // 失败原因
	Vectorized      bool     `gorm:"default:false" json:"vectorized"`                                 // 是否已向量化

	// 关联
	ParentBlocks []ParentBlock `gorm:"foreignKey:SourceID"`
}

func (Source) TableName() string {
	return "source"
}
