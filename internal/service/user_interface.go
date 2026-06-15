package service

import (
	"YoudaoNoteLm/internal/model/dto/request"
	dto "YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/pkg/response"
	"context"
	"mime/multipart"
)

// UserService 用户服务接口
type UserService interface {
	// Register 用户注册（邮箱+验证码）
	Register(ctx context.Context, req *request.RegisterRequest) error
	// GetUserByID 根据 ID 获取用户
	GetUserByID(id uint) (*entity.User, error)
	// UpdateUser 更新用户信息
	UpdateUser(id uint, req *request.UpdateUserRequest) error
	// UpdateUsername 修改用户名
	UpdateUsername(id uint, req *request.UpdateUsernameRequest) error
	// UploadAvatar 上传头像
	UploadAvatar(id uint, file *multipart.FileHeader) (string, error)
	// ChangePassword 修改密码
	ChangePassword(id uint, req *request.ChangePasswordRequest) error
	// DeleteAccount 注销用户（硬删除）
	DeleteAccount(id uint, req *request.DeleteAccountRequest) error
	// GetUserResponse 获取用户响应
	GetUserResponse(user *entity.User) *dto.UserResponse
	// ListUsers 分页获取用户列表
	ListUsers(req *request.UserListRequest) (*response.PageResponse, error)
}
