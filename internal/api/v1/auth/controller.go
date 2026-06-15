package auth

import (
	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/pkg/response"
	"github.com/gin-gonic/gin"
)

// Controller 认证控制器
type Controller struct {
	authService service.AuthService
	userService service.UserService
	captchaSvc  service.CaptchaService
}

// NewController 创建认证控制器
func NewController(authService service.AuthService, userService service.UserService, captchaSvc service.CaptchaService) *Controller {
	return &Controller{
		authService: authService,
		userService: userService,
		captchaSvc:  captchaSvc,
	}
}

// Register 用户注册
func (ctrl *Controller) Register(c *gin.Context) {
	var req request.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 校验两次密码是否一致
	if err := req.Validate(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.userService.Register(c.Request.Context(), &req); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// GetCaptcha 获取滑块验证码
func (ctrl *Controller) GetCaptcha(c *gin.Context) {
	data, err := ctrl.captchaSvc.Generate(c.Request.Context())
	if err != nil {
		response.InternalError(c, "生成验证码失败")
		return
	}

	response.Success(c, data)
}

// Login 用户登录
func (ctrl *Controller) Login(c *gin.Context) {
	var req request.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := ctrl.authService.Login(c.Request.Context(), &req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// SendCode 发送验证码
func (ctrl *Controller) SendCode(c *gin.Context) {
	var req request.SendCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := ctrl.authService.SendCode(c.Request.Context(), &req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// ResetPassword 重置密码
func (ctrl *Controller) ResetPassword(c *gin.Context) {
	var req request.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 校验两次密码是否一致
	if err := req.Validate(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.authService.ResetPassword(&req); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// RefreshToken 刷新 Token
func (ctrl *Controller) RefreshToken(c *gin.Context) {
	var req request.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := ctrl.authService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// Logout 用户登出
func (ctrl *Controller) Logout(c *gin.Context) {
	var req request.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.authService.Logout(c.Request.Context(), req.AccessToken, req.RefreshToken); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}
