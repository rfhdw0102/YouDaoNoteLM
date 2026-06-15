package entity

// UserLLMConfig 主模型配置（一人多条）
type UserLLMConfig struct {
	BaseEntity
	UserID   uint   `gorm:"not null;index" json:"user_id"` // 所属用户
	User     User   `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	Name     string `gorm:"type:varchar(100);not null" json:"name"`    // 配置名称
	Provider string `gorm:"type:varchar(50);not null" json:"provider"` // 服务商: openai/anthropic/deepseek
	APIKey   string `gorm:"type:varchar(512)" json:"api_key"`          // API密钥
	APIURL   string `gorm:"type:varchar(512)" json:"api_url"`          // API地址(自建/代理)
	Model    string `gorm:"type:varchar(100);not null" json:"model"`   // 模型名称
	Enabled  bool   `gorm:"default:true" json:"enabled"`               // 是否启用
}

func (UserLLMConfig) TableName() string {
	return "user_llm_config"
}
