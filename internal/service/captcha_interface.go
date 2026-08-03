package service

import (
	dto "YoudaoNoteLm/internal/model/dto/response"
	"context"
)

// CaptchaService 验证码服务接口
type CaptchaService interface {
	// Generate 生成滑块验证码
	Generate(ctx context.Context) (*dto.CaptchaData, error)
	// Verify 校验滑块验证码
	Verify(ctx context.Context, captchaID string, userX int) error
}
