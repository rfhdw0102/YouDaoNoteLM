package entity

// NotionBinding 保存用户的 Notion OAuth 工作区绑定。
// AccessTokenEncrypted 仅供服务端解密后请求 Notion API，绝不作为 JSON 输出。
type NotionBinding struct {
	BaseEntity
	UserID               uint   `gorm:"not null;uniqueIndex:uk_user" json:"user_id"`
	User                 User   `gorm:"foreignKey:UserID"`
	WorkspaceID          string `gorm:"type:varchar(255);not null" json:"workspace_id"`
	WorkspaceName        string `gorm:"type:varchar(255)" json:"workspace_name"`
	WorkspaceIcon        string `gorm:"type:varchar(2048)" json:"workspace_icon"`
	BotID                string `gorm:"type:varchar(255)" json:"bot_id"`
	AccessTokenEncrypted string `gorm:"type:varchar(1024);not null" json:"-"`
	Status               string `gorm:"type:varchar(20);default:active" json:"status"`
}

func (NotionBinding) TableName() string {
	return "notion_binding"
}
