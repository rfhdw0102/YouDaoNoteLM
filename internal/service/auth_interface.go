package service

import (
	"YoudaoNoteLm/internal/model/dto/request"
	dto "YoudaoNoteLm/internal/model/dto/response"
	"context"
)

// AuthService 认证服务接口
type AuthService interface {
	// Login 用户登录（邮箱+密码+滑块验证），返回双 token
	Login(ctx context.Context, req *request.LoginRequest) (*dto.LoginResponse, error)
	// RefreshToken 用 refresh token 换取新的 token 对
	RefreshToken(ctx context.Context, refreshToken string) (*dto.LoginResponse, error)
	// Logout 用户登出，将 token 加入黑名单
	Logout(ctx context.Context, accessToken string, refreshToken string) error
	// SendCode 发送验证码
	SendCode(ctx context.Context, req *request.SendCodeRequest) (*dto.SendCodeResponse, error)
	// ResetPassword 重置密码（找回密码）
	ResetPassword(req *request.ResetPasswordRequest) error
}
