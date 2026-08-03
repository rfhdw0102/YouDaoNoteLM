package user

import (
	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/pkg/logger"
	"YoudaoNoteLm/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Controller 用户控制器
type Controller struct {
	userService    service.UserService
	tokenBlacklist service.TokenBlacklistService
}

// NewController 创建用户控制器
func NewController(userService service.UserService, tokenBlacklist service.TokenBlacklistService) *Controller {
	return &Controller{
		userService:    userService,
		tokenBlacklist: tokenBlacklist,
	}
}

// GetProfile 获取当前用户信息
func (ctrl *Controller) GetProfile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	user, err := ctrl.userService.GetUserByID(userID)
	if err != nil {
		response.BizError(c, err)
		return
	}

	userResp := ctrl.userService.GetUserResponse(user)
	response.Success(c, userResp)
}

// UpdateProfile 更新用户信息
func (ctrl *Controller) UpdateProfile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.userService.UpdateUser(userID, &req); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// ChangePassword 修改密码（修改后需重新登录）
func (ctrl *Controller) ChangePassword(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.userService.ChangePassword(userID, &req); err != nil {
		response.BizError(c, err)
		return
	}

	// 使当前 token 失效，强制重新登录
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		token := authHeader[7:] // 去掉 "Bearer " 前缀
		if err := ctrl.tokenBlacklist.RevokeToken(c.Request.Context(), token); err != nil {
			logger.Warn("update password RevokeToken failed:", zap.Error(err))
		}
	}

	response.Success(c, gin.H{"message": "密码修改成功，请重新登录"})
}

// ListUsers 获取用户列表（分页）
func (ctrl *Controller) ListUsers(c *gin.Context) {
	var req request.UserListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	pageResp, err := ctrl.userService.ListUsers(&req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, pageResp)
}

// UpdateUsername 修改用户名
func (ctrl *Controller) UpdateUsername(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.UpdateUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.userService.UpdateUsername(userID, &req); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// UploadAvatar 上传头像
func (ctrl *Controller) UploadAvatar(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	file, err := c.FormFile("avatar")
	if err != nil {
		response.BadRequest(c, "请上传头像文件")
		return
	}

	avatarURL, err := ctrl.userService.UploadAvatar(userID, file)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, gin.H{"avatar": avatarURL})
}

// DeleteAccount 注销用户
func (ctrl *Controller) DeleteAccount(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.DeleteAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.userService.DeleteAccount(userID, &req); err != nil {
		response.BizError(c, err)
		return
	}

	// 使当前 token 失效
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		token := authHeader[7:]
		if err := ctrl.tokenBlacklist.RevokeToken(c.Request.Context(), token); err != nil {
			logger.Warn("注销时吊销 token 失败", zap.Error(err))
		}
	}

	response.Success(c, gin.H{"message": "账号已注销"})
}
