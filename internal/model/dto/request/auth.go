package request

import "errors"

// RegisterRequest 用户注册请求
type RegisterRequest struct {
	Email           string `json:"email" binding:"required,email"`
	Password        string `json:"password" binding:"required,min=8,max=20"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
	Code            string `json:"code" binding:"required,len=6"`
}

// Validate 校验注册请求
func (r *RegisterRequest) Validate() error {
	if r.Password != r.ConfirmPassword {
		return errors.New("两次密码输入不一致")
	}
	return nil
}

// LoginRequest 用户登录请求（含滑块验证码）
type LoginRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required"`
	CaptchaID string `json:"captcha_id" binding:"required"`
	CaptchaX  int    `json:"captcha_x" binding:"required"`
}

// SendCodeRequest 发送验证码请求
type SendCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
	Type  string `json:"type" binding:"required,oneof=register reset delete_account"` // register=注册, reset=重置密码, delete_account=注销账号
}

// ResetPasswordRequest 重置密码请求
type ResetPasswordRequest struct {
	Email           string `json:"email" binding:"required,email"`
	Code            string `json:"code" binding:"required,len=6"`
	NewPassword     string `json:"new_password" binding:"required,min=8,max=20"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

// Validate 校验重置密码请求
func (r *ResetPasswordRequest) Validate() error {
	if r.NewPassword != r.ConfirmPassword {
		return errors.New("两次密码输入不一致")
	}
	return nil
}

// RefreshTokenRequest 刷新 Token 请求
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// LogoutRequest 登出请求
type LogoutRequest struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}
