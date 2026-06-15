package service

import (
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"
	"YoudaoNoteLm/pkg/utils"
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	verifyCodeTTL      = 5 * time.Minute  // 验证码有效期 5 分钟
	verifyCodeCooldown = 60 * time.Second // 发送冷却 60 秒
	verifyCodeMaxRetry = 5                // 最大验证次数
)

// verifyCodeService 验证码服务实现
type verifyCodeService struct {
	redis    *redis.Client
	emailSvc EmailService
}

// NewVerifyCodeService 创建验证码服务
func NewVerifyCodeService(redisClient *redis.Client, emailSvc EmailService) VerifyCodeService {
	return &verifyCodeService{
		redis:    redisClient,
		emailSvc: emailSvc,
	}
}

// codeKey 验证码存储 key: verify_code:{type}:{email}
func (s *verifyCodeService) codeKey(email, codeType string) string {
	return fmt.Sprintf("verify_code:%s:%s", codeType, email)
}

// cooldownKey 冷却 key: verify_code_cooldown:{type}:{email}
func (s *verifyCodeService) cooldownKey(email, codeType string) string {
	return fmt.Sprintf("verify_code_cooldown:%s:%s", codeType, email)
}

// retryKey 重试次数 key: verify_code_retry:{type}:{email}
func (s *verifyCodeService) retryKey(email, codeType string) string {
	return fmt.Sprintf("verify_code_retry:%s:%s", codeType, email)
}

// Generate 生成验证码并存储到 Redis，同时发送邮件
func (s *verifyCodeService) Generate(ctx context.Context, email string, codeType string) (string, error) {
	// 检查冷却时间
	remaining, err := s.GetCooldownRemaining(ctx, email, codeType)
	if err != nil {
		return "", err
	}
	if remaining > 0 {
		return "", bizerrors.ErrVerifyCodeTooFrequent
	}

	// 生成 6 位数字验证码
	code, err := utils.GenerateRandomString(6, utils.Numeric)
	if err != nil {
		logger.Error("生成验证码失败", zap.Error(err))
		return "", fmt.Errorf("生成验证码失败: %w", err)
	}

	// 存储验证码到 Redis，5 分钟过期
	if err := s.redis.Set(ctx, s.codeKey(email, codeType), code, verifyCodeTTL).Err(); err != nil {
		logger.Error("存储验证码失败", zap.Error(err))
		return "", fmt.Errorf("存储验证码失败: %w", err)
	}

	// 重置重试次数
	if err := s.redis.Del(ctx, s.retryKey(email, codeType)).Err(); err != nil {
		logger.Warn("清除重试次数失败", zap.String("email", email), zap.Error(err))
	}

	// 设置冷却时间
	if err := s.redis.Set(ctx, s.cooldownKey(email, codeType), 1, verifyCodeCooldown).Err(); err != nil {
		logger.Warn("设置冷却时间失败", zap.String("email", email), zap.Error(err))
	}

	// 发送邮件
	if err := s.emailSvc.SendVerifyCode(email, code); err != nil {
		logger.Error("发送验证码邮件失败", zap.String("email", email), zap.Error(err))
		// 邮件发送失败不影响验证码已生成，返回错误让用户重试
		return "", fmt.Errorf("发送验证码邮件失败: %w", err)
	}

	return code, nil
}

// Verify 校验验证码
func (s *verifyCodeService) Verify(ctx context.Context, email string, codeType string, code string) error {
	// 检查重试次数
	retryCount, err := s.redis.Get(ctx, s.retryKey(email, codeType)).Int()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("查询重试次数失败: %w", err)
	}
	if retryCount >= verifyCodeMaxRetry {
		// 清除验证码
		if err := s.redis.Del(ctx, s.codeKey(email, codeType)).Err(); err != nil {
			logger.Warn("清除验证码失败", zap.String("email", email), zap.Error(err))
		}
		return bizerrors.ErrVerifyCodeLocked
	}

	// 获取存储的验证码
	storedCode, err := s.redis.Get(ctx, s.codeKey(email, codeType)).Result()
	if err == redis.Nil {
		return bizerrors.ErrVerifyCodeExpired
	}
	if err != nil {
		return fmt.Errorf("查询验证码失败: %w", err)
	}

	// 校验验证码
	if storedCode != code {
		// 增加重试次数
		if err := s.redis.Incr(ctx, s.retryKey(email, codeType)).Err(); err != nil {
			logger.Warn("增加重试次数失败", zap.String("email", email), zap.Error(err))
		}
		if err := s.redis.Expire(ctx, s.retryKey(email, codeType), verifyCodeTTL).Err(); err != nil {
			logger.Warn("设置重试次数过期时间失败", zap.String("email", email), zap.Error(err))
		}
		return bizerrors.ErrVerifyCodeInvalid
	}

	// 验证成功，清除验证码和重试次数
	if err := s.redis.Del(ctx, s.codeKey(email, codeType)).Err(); err != nil {
		logger.Warn("清除验证码失败", zap.String("email", email), zap.Error(err))
	}
	if err := s.redis.Del(ctx, s.retryKey(email, codeType)).Err(); err != nil {
		logger.Warn("清除重试次数失败", zap.String("email", email), zap.Error(err))
	}

	return nil
}

// GetCooldownRemaining 获取剩余冷却秒数
func (s *verifyCodeService) GetCooldownRemaining(ctx context.Context, email string, codeType string) (int, error) {
	ttl, err := s.redis.TTL(ctx, s.cooldownKey(email, codeType)).Result()
	if err != nil {
		return 0, fmt.Errorf("查询冷却时间失败: %w", err)
	}
	if ttl <= 0 {
		return 0, nil
	}
	return int(ttl.Seconds()), nil
}
