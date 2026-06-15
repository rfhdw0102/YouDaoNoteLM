package request

import (
	"YoudaoNoteLm/pkg/response"
	"mime/multipart"
)

// UpdateUserRequest 更新用户信息请求
type UpdateUserRequest struct {
	Nickname string `json:"nickname" binding:"omitempty,max=50"`
	Avatar   string `json:"avatar" binding:"omitempty,url,max=255"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=20"`
}

// UserListRequest 用户列表请求
type UserListRequest struct {
	response.PageRequest
}

// UpdateUsernameRequest 修改用户名请求
type UpdateUsernameRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
}

// UploadAvatarRequest 上传头像请求（multipart/form-data）
type UploadAvatarRequest struct {
	Avatar *multipart.FileHeader `form:"avatar" binding:"required"`
}

// DeleteAccountRequest 注销用户请求
type DeleteAccountRequest struct {
	Password string `json:"password" binding:"required"`
	Code     string `json:"code" binding:"required,len=6"`
}
