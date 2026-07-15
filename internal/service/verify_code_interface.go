package service

import "context"

// VerifyCodeService 验证码服务接口
type VerifyCodeService interface {
	// Generate 生成验证码并存储到 Redis，返回验证码和是否在冷却中
	Generate(ctx context.Context, email string, codeType string) (string, error)
	// Verify 校验验证码
	Verify(ctx context.Context, email string, codeType string, code string) error
	// GetCooldownRemaining 获取剩余冷却秒数，0 表示不在冷却中
	GetCooldownRemaining(ctx context.Context, email string, codeType string) (int, error)
}
