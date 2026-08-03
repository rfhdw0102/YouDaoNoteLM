package repository

import (
	"YoudaoNoteLm/internal/model/entity"
	"time"
)

// UserRepository 用户仓储接口
type UserRepository interface {
	// FindByID 根据 ID 查找用户
	FindByID(id uint) (*entity.User, error)
	// FindByUsername 根据用户名查找用户
	FindByUsername(username string) (*entity.User, error)
	// FindByEmail 根据邮箱查找用户
	FindByEmail(email string) (*entity.User, error)
	// Create 创建用户
	Create(user *entity.User) error
	// Update 更新用户
	Update(user *entity.User) error
	// Delete 删除用户（软删除）
	Delete(id uint) error
	// HardDelete 硬删除用户（级联删除所有关联数据）
	HardDelete(id uint) error
	// List 分页获取用户列表
	List(offset, limit int) ([]*entity.User, int64, error)
	// ExistsByUsername 检查用户名是否存在
	ExistsByUsername(username string) (bool, error)
	// ExistsByEmail 检查邮箱是否存在
	ExistsByEmail(email string) (bool, error)
	// UpdateLoginAttempts 更新登录失败次数
	UpdateLoginAttempts(id uint, attempts int) error
	// LockUser 锁定用户到指定时间
	LockUser(id uint, until time.Time) error
	// ResetLoginAttempts 重置登录失败次数
	ResetLoginAttempts(id uint) error
}
