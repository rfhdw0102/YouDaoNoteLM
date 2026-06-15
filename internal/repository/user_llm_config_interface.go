package repository

import "YoudaoNoteLm/internal/model/entity"

// UserLLMConfigRepository 用户 LLM 配置仓储接口
type UserLLMConfigRepository interface {
	// FindByID 根据 ID 查找配置
	FindByID(id uint) (*entity.UserLLMConfig, error)
	// FindByUserID 查找用户的所有 LLM 配置
	FindByUserID(userID uint) ([]*entity.UserLLMConfig, error)
	// FindEnabledByUserID 查找用户的启用的 LLM 配置
	FindEnabledByUserID(userID uint) ([]*entity.UserLLMConfig, error)
	// FindDefaultByUserID 查找用户的默认 LLM 配置（第一个启用的）
	FindDefaultByUserID(userID uint) (*entity.UserLLMConfig, error)
	// Create 创建配置
	Create(config *entity.UserLLMConfig) error
	// Update 更新配置
	Update(config *entity.UserLLMConfig) error
	// Delete 删除配置
	Delete(id uint) error
}
