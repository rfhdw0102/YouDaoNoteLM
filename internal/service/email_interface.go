package service

// EmailService 邮件服务接口
type EmailService interface {
	// SendVerifyCode 发送验证码邮件
	SendVerifyCode(to string, code string) error
}
