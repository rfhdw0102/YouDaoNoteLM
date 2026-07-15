package service

import (
	"YoudaoNoteLm/pkg/config"
	"YoudaoNoteLm/pkg/logger"
	"fmt"
	"net/smtp"

	"go.uber.org/zap"
)

// emailService 邮件服务实现
type emailService struct{}

// NewEmailService 创建邮件服务
func NewEmailService() EmailService {
	return &emailService{}
}

// SendVerifyCode 发送验证码邮件
func (s *emailService) SendVerifyCode(to string, code string) error {
	cfg := config.Get().Email

	subject := "YoudaoNoteLM 验证码"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body>
<div style="max-width:480px;margin:0 auto;padding:20px;font-family:Arial,sans-serif;">
	<h2 style="color:#333;">YoudaoNoteLM</h2>
	<p>您好，您正在进行身份验证：</p>
	<div style="background:#f5f5f5;padding:15px;text-align:center;border-radius:8px;margin:20px 0;">
		<span style="font-size:32px;font-weight:bold;color:#1890ff;letter-spacing:8px;">%s</span>
	</div>
	<p>验证码有效期为 <strong>5 分钟</strong>，请勿泄露给他人。</p>
	<p style="color:#999;font-size:12px;">如非本人操作，请忽略此邮件。</p>
</div>
</body>
</html>`, code)

	msg := []byte(fmt.Sprintf("To: %s\r\n"+
		"From: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n%s",
		to, cfg.From, subject, body))

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)

	if err := smtp.SendMail(addr, auth, cfg.From, []string{to}, msg); err != nil {
		logger.Error("发送邮件失败", zap.String("to", to), zap.Error(err))
		return fmt.Errorf("发送邮件失败: %w", err)
	}

	logger.Info("验证码邮件发送成功", zap.String("to", to))
	return nil
}
