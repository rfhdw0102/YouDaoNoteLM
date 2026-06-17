package entity

// SysConfig 系统配置实体
type SysConfig struct {
	BaseEntity
	ConfigGroup string `gorm:"type:varchar(50);not null;uniqueIndex:uk_group_key" json:"config_group"` // 配置分组: search/asr/embedding
	ConfigKey   string `gorm:"type:varchar(100);not null;uniqueIndex:uk_group_key" json:"config_key"` // 配置键
	ConfigValue string `gorm:"type:json;not null" json:"config_value"`                                // 配置值(JSON)
	Enabled     bool   `gorm:"default:true" json:"enabled"`                                          // 是否启用
	Description string `gorm:"type:varchar(255)" json:"description"`                                 // 描述
}

func (SysConfig) TableName() string {
	return "sys_config"
}
