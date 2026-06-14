package entity

import "time"

// UserConfig 用户配置（搜索/ASR/Embedding，一人一条）
type UserConfig struct {
	BaseEntity
	UserID       uint       `gorm:"not null;uniqueIndex:uk_user_type" json:"user_id"` // 所属用户
	User         User       `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	ConfigType   string     `gorm:"type:varchar(20);not null;uniqueIndex:uk_user_type" json:"config_type"` // 类型: search/asr/embedding
	Name         string     `gorm:"type:varchar(100);not null" json:"name"`                                // 配置名称
	Provider     string     `gorm:"type:varchar(50);not null" json:"provider"`                             // 服务商
	APIKey       string     `gorm:"type:varchar(512)" json:"api_key"`                                      // API密钥
	APIURL       string     `gorm:"type:varchar(512)" json:"api_url"`                                      // API地址(自建/代理)
	Model        string     `gorm:"type:varchar(100)" json:"model"`                                        // 模型名称(embedding用)
	Dimensions   *int       `json:"dimensions"`                                                            // 向量维度(embedding用)
	DailyQuota   *int       `json:"daily_quota"`                                                           // 每日配额(search用)
	QuotaUsed    int        `gorm:"default:0" json:"quota_used"`                                           // 已使用配额(search用)
	QuotaResetAt *time.Time `json:"quota_reset_at"`                                                        // 配额重置时间(search用)
	ExtraConfig  string     `gorm:"type:json" json:"extra_config"`                                         // 服务商特有配置(JSON)
	Enabled      bool       `gorm:"default:true" json:"enabled"`                                           // 是否启用
}

func (UserConfig) TableName() string {
	return "user_config"
}
