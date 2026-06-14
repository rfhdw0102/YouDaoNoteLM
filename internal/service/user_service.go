package service

import (
	"YoudaoNoteLm/internal/model/dto/request"
	dto "YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/internal/service/external"
	"YoudaoNoteLm/pkg/logger"
	"YoudaoNoteLm/pkg/response"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"time"

	bizerrors "YoudaoNoteLm/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// userService 用户服务实现
type userService struct {
	userRepo      repository.UserRepository
	verifyCodeSvc VerifyCodeService
	storage       external.FileStorage
}

// NewUserService 创建用户服务
func NewUserService(userRepo repository.UserRepository, verifyCodeSvc VerifyCodeService, storage external.FileStorage) UserService {
	return &userService{
		userRepo:      userRepo,
		verifyCodeSvc: verifyCodeSvc,
		storage:       storage,
	}
}

// Register 用户注册（邮箱+验证码）
func (s *userService) Register(ctx context.Context, req *request.RegisterRequest) error {
	// 校验验证码
	if err := s.verifyCodeSvc.Verify(ctx, req.Email, "register", req.Code); err != nil {
		return err
	}

	// 检查邮箱是否已被注册
	exists, err := s.userRepo.ExistsByEmail(req.Email)
	if err != nil {
		return err
	}
	if exists {
		return bizerrors.New(bizerrors.CodeUserAlreadyExists, "邮箱已被注册")
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 自动生成用户名（邮箱前缀）
	username := generateUsername(req.Email)

	// 创建用户
	user := &entity.User{
		Username: username,
		Password: string(hashedPassword),
		Email:    req.Email,
		Status:   1, // 默认正常
	}

	if err := s.userRepo.Create(user); err != nil {
		return err
	}

	return nil
}

// generateUsername 从邮箱生成用户名
func generateUsername(email string) string {
	for i, c := range email {
		if c == '@' {
			return email[:i]
		}
	}
	return email
}

// GetUserByID 根据 ID 获取用户
func (s *userService) GetUserByID(id uint) (*entity.User, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, bizerrors.ErrUserNotFound
	}
	return user, nil
}

// UpdateUser 更新用户信息
func (s *userService) UpdateUser(id uint, req *request.UpdateUserRequest) error {
	user, err := s.GetUserByID(id)
	if err != nil {
		return err
	}

	// 更新字段
	if req.Nickname != "" {
		user.Nickname = req.Nickname
	}
	if req.Avatar != "" {
		user.Avatar = req.Avatar
	}

	return s.userRepo.Update(user)
}

// UpdateUsername 修改用户名
func (s *userService) UpdateUsername(id uint, req *request.UpdateUsernameRequest) error {
	user, err := s.GetUserByID(id)
	if err != nil {
		return err
	}

	// 检查用户名是否已被使用
	exists, err := s.userRepo.ExistsByUsername(req.Username)
	if err != nil {
		return err
	}
	if exists {
		return bizerrors.New(bizerrors.CodeUserAlreadyExists, "用户名已被使用")
	}

	user.Username = req.Username
	return s.userRepo.Update(user)
}

// UploadAvatar 上传头像
func (s *userService) UploadAvatar(id uint, file *multipart.FileHeader) (string, error) {
	// 验证文件大小（2MB）
	if file.Size > 2*1024*1024 {
		return "", bizerrors.New(bizerrors.CodeBadRequest, "头像文件大小不能超过 2MB")
	}

	// 验证文件类型
	ext := filepath.Ext(file.Filename)
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true}
	if !allowedExts[ext] {
		return "", bizerrors.New(bizerrors.CodeBadRequest, "仅支持 jpg/jpeg/png 格式")
	}

	// 获取用户信息（用于删除旧头像）
	user, err := s.GetUserByID(id)
	if err != nil {
		return "", err
	}

	// 删除旧头像
	if user.Avatar != "" {
		if err := s.storage.Delete(user.Avatar); err != nil {
			logger.Warn("删除旧头像失败", zap.String("path", user.Avatar), zap.Error(err))
		}
	}

	// 上传到 MinIO，使用 avatars/{user_id}.{ext} 作为 objectName
	objectName := fmt.Sprintf("avatars/%d%s", id, ext)
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("打开上传文件失败: %w", err)
	}
	defer src.Close()

	// 通过 UploadBytes 上传，确保 objectName 可控
	fileBytes, err := io.ReadAll(src)
	if err != nil {
		return "", fmt.Errorf("读取上传文件失败: %w", err)
	}
	if err := s.storage.UploadBytes(objectName, fileBytes, file.Header.Get("Content-Type")); err != nil {
		return "", fmt.Errorf("上传头像到 MinIO 失败: %w", err)
	}

	// 更新用户头像路径（存储 objectName，访问时通过预签名 URL）
	user.Avatar = objectName
	if err := s.userRepo.Update(user); err != nil {
		return "", err
	}

	// 生成预签名 URL 返回给前端
	presignedURL, err := s.storage.GetPresignedURL(objectName, 24*time.Hour)
	if err != nil {
		logger.Warn("生成头像预签名 URL 失败，返回 objectName", zap.Error(err))
		return objectName, nil
	}

	logger.Info("头像上传成功", zap.Uint("user_id", id), zap.String("object", objectName))
	return presignedURL, nil
}

// ChangePassword 修改密码
func (s *userService) ChangePassword(id uint, req *request.ChangePasswordRequest) error {
	user, err := s.GetUserByID(id)
	if err != nil {
		return err
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		return bizerrors.ErrInvalidCredentials
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.Password = string(hashedPassword)
	return s.userRepo.Update(user)
}

// DeleteAccount 注销用户（硬删除）
func (s *userService) DeleteAccount(id uint, req *request.DeleteAccountRequest) error {
	user, err := s.GetUserByID(id)
	if err != nil {
		return err
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return bizerrors.ErrInvalidCredentials
	}

	// 验证邮箱验证码
	ctx := context.Background()
	if err := s.verifyCodeSvc.Verify(ctx, user.Email, "delete_account", req.Code); err != nil {
		return err
	}

	// 删除头像文件
	if user.Avatar != "" {
		if err := s.storage.Delete(user.Avatar); err != nil {
			logger.Warn("删除头像失败", zap.String("path", user.Avatar), zap.Error(err))
		}
	}

	// 硬删除用户（级联删除所有关联数据）
	return s.userRepo.HardDelete(id)
}

// GetUserResponse 获取用户响应
func (s *userService) GetUserResponse(user *entity.User) *dto.UserResponse {
	avatarURL := user.Avatar
	// 如果头像是 MinIO 对象路径，生成预签名 URL
	if user.Avatar != "" && s.storage != nil {
		if presignedURL, err := s.storage.GetPresignedURL(user.Avatar, 24*time.Hour); err == nil {
			avatarURL = presignedURL
		} else {
			logger.Warn("生成头像预签名 URL 失败", zap.String("path", user.Avatar), zap.Error(err))
		}
	}

	return &dto.UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Nickname:  user.Nickname,
		Avatar:    avatarURL,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

// ListUsers 分页获取用户列表
func (s *userService) ListUsers(req *request.UserListRequest) (*response.PageResponse, error) {
	// 参数标准化
	page := req.Page
	if page < 1 {
		page = 1
	}
	size := req.Size
	if size < 1 {
		size = 10
	}
	if size > 100 {
		size = 100
	}

	// 计算偏移量
	offset := (page - 1) * size

	// 查询数据
	users, total, err := s.userRepo.List(offset, size)
	if err != nil {
		return nil, err
	}

	// 转换为响应 DTO
	list := make([]*dto.UserResponse, 0, len(users))
	for _, user := range users {
		list = append(list, s.GetUserResponse(user))
	}

	return response.NewPageResponse(list, total, page, size), nil
}
