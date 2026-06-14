package entity

import "time"

// User 用户实体
type User struct {
	BaseEntity
	Username       string     `gorm:"type:varchar(50);uniqueIndex;not null;comment:用户名" json:"username"`
	Password       string     `gorm:"type:varchar(255);not null;comment:密码" json:"-"`
	Email          string     `gorm:"type:varchar(100);uniqueIndex;not null;comment:邮箱" json:"email"`
	Nickname       string     `gorm:"type:varchar(50);comment:昵称" json:"nickname"`
	Avatar         string     `gorm:"type:varchar(255);comment:头像" json:"avatar"`
	Status         int        `gorm:"type:tinyint;default:1;comment:状态:1正常,2禁用" json:"status"`
	FailedAttempts int        `gorm:"type:tinyint;default:0;comment:连续登录失败次数" json:"-"`
	LockedUntil    *time.Time `gorm:"comment:锁定截止时间" json:"-"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// IsLocked 判断用户是否被锁定
func (u *User) IsLocked() bool {
	if u.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*u.LockedUntil)
}
