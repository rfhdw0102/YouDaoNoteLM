package repository

import (
	"YoudaoNoteLm/internal/model/entity"
	"errors"
	"time"

	"gorm.io/gorm"
)

// userRepository 用户仓储实现
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建用户仓储
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// FindByID 根据 ID 查找用户
func (r *userRepository) FindByID(id uint) (*entity.User, error) {
	var user entity.User
	err := r.db.First(&user, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// FindByUsername 根据用户名查找用户
func (r *userRepository) FindByUsername(username string) (*entity.User, error) {
	var user entity.User
	err := r.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// FindByEmail 根据邮箱查找用户
func (r *userRepository) FindByEmail(email string) (*entity.User, error) {
	var user entity.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// Create 创建用户
func (r *userRepository) Create(user *entity.User) error {
	return r.db.Create(user).Error
}

// Update 更新用户
func (r *userRepository) Update(user *entity.User) error {
	return r.db.Save(user).Error
}

// Delete 删除用户（软删除）
func (r *userRepository) Delete(id uint) error {
	return r.db.Delete(&entity.User{}, id).Error
}

// HardDelete 硬删除用户（级联删除所有关联数据）
func (r *userRepository) HardDelete(id uint) error {
	// 使用 Unscoped() 绕过软删除，执行真正的删除
	// 由于外键 CASCADE 级联删除，关联数据会自动删除
	return r.db.Unscoped().Delete(&entity.User{}, id).Error
}

// List 分页获取用户列表
func (r *userRepository) List(offset, limit int) ([]*entity.User, int64, error) {
	var users []*entity.User
	var total int64

	// 统计总数
	if err := r.db.Model(&entity.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	err := r.db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&users).Error
	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// ExistsByUsername 检查用户名是否存在
func (r *userRepository) ExistsByUsername(username string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.User{}).Where("username = ?", username).Count(&count).Error
	return count > 0, err
}

// ExistsByEmail 检查邮箱是否存在
func (r *userRepository) ExistsByEmail(email string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.User{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}

// UpdateLoginAttempts 更新登录失败次数
func (r *userRepository) UpdateLoginAttempts(id uint, attempts int) error {
	return r.db.Model(&entity.User{}).Where("id = ?", id).Update("failed_attempts", attempts).Error
}

// LockUser 锁定用户到指定时间
func (r *userRepository) LockUser(id uint, until time.Time) error {
	return r.db.Model(&entity.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"failed_attempts": 0,
		"locked_until":    until,
	}).Error
}

// ResetLoginAttempts 重置登录失败次数
func (r *userRepository) ResetLoginAttempts(id uint) error {
	return r.db.Model(&entity.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"failed_attempts": 0,
		"locked_until":    nil,
	}).Error
}
