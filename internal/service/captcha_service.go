package service

import (
	bizerrors "YoudaoNoteLm/pkg/errors"
	"context"
	"fmt"
	"sync"
	"time"

	dto "YoudaoNoteLm/internal/model/dto/response"

	"github.com/redis/go-redis/v9"
	"github.com/wenlng/go-captcha-assets/resources/imagesv2"
	"github.com/wenlng/go-captcha-assets/resources/tiles"
	"github.com/wenlng/go-captcha/v2/base/option"
	"github.com/wenlng/go-captcha/v2/slide"
)

const (
	captchaTTL       = 5 * time.Minute // 验证码有效期
	captchaTolerance = 10              // 容差像素
)

// captchaService 验证码服务实现
type captchaService struct {
	redis       *redis.Client
	backgrounds []slide.Resource
	graphs      []slide.Resource
	once        sync.Once
}

// NewCaptchaService 创建验证码服务
func NewCaptchaService(redisClient *redis.Client) CaptchaService {
	return &captchaService{redis: redisClient}
}

// initResources 初始化资源（懒加载，只执行一次）
func (s *captchaService) initResources() error {
	var initErr error
	s.once.Do(func() {
		// 加载背景图片
		bgImages, err := imagesv2.GetImages()
		if err != nil {
			initErr = fmt.Errorf("加载背景图片失败: %w", err)
			return
		}

		// 加载滑块拼图图片
		tileImages, err := tiles.GetTiles()
		if err != nil {
			initErr = fmt.Errorf("加载滑块图片失败: %w", err)
			return
		}

		// 转换为 go-captcha slide 的 GraphImage 格式
		var graphImages []*slide.GraphImage
		for _, tile := range tileImages {
			graphImages = append(graphImages, &slide.GraphImage{
				OverlayImage: tile.OverlayImage,
				ShadowImage:  tile.ShadowImage,
				MaskImage:    tile.MaskImage,
			})
		}

		s.backgrounds = []slide.Resource{slide.WithBackgrounds(bgImages)}
		s.graphs = []slide.Resource{slide.WithGraphImages(graphImages)}
	})
	return initErr
}

// captchaKey Redis 键
func (s *captchaService) captchaKey(captchaID string) string {
	return fmt.Sprintf("captcha:%s", captchaID)
}

// Generate 生成滑块验证码
func (s *captchaService) Generate(ctx context.Context) (*dto.CaptchaData, error) {
	// 初始化资源
	if err := s.initResources(); err != nil {
		return nil, err
	}

	// 创建滑块验证码实例（通过 Builder 设置选项和资源）
	builder := slide.NewBuilder(
		slide.WithImageSize(option.Size{Width: 300, Height: 150}),
		slide.WithRangeGraphSize(option.RangeVal{Min: 40, Max: 50}),
		slide.WithGenGraphNumber(1),
	)
	builder.SetResources(s.backgrounds...)
	builder.SetResources(s.graphs...)

	captcha := builder.Make()

	// 生成验证码
	data, err := captcha.Generate()
	if err != nil {
		return nil, fmt.Errorf("生成验证码失败: %w", err)
	}

	block := data.GetData()
	if block == nil {
		return nil, fmt.Errorf("生成验证码数据为空")
	}

	// 获取主图 base64
	masterImage := data.GetMasterImage()
	masterBase64, err := masterImage.ToBase64()
	if err != nil {
		return nil, fmt.Errorf("编码主图失败: %w", err)
	}

	// 获取滑块图 base64
	tileImage := data.GetTileImage()
	tileBase64, err := tileImage.ToBase64()
	if err != nil {
		return nil, fmt.Errorf("编码滑块图失败: %w", err)
	}

	// 生成唯一 key
	key := fmt.Sprintf("%d_%d_%d", block.DX, block.DY, time.Now().UnixNano())
	// 存储缺口绝对位置和滑块起始位置
	captchaInfo := fmt.Sprintf("%d,%d", block.X, block.DX)
	if err := s.redis.Set(ctx, s.captchaKey(key), captchaInfo, captchaTTL).Err(); err != nil {
		return nil, fmt.Errorf("存储验证码失败: %w", err)
	}

	return &dto.CaptchaData{
		CaptchaID:    key,
		Background:   masterBase64,
		Slider:       tileBase64,
		SliderSize:   block.Width,
		BgWidth:      300,
		BgHeight:     150,
		SliderStartX: block.DX,
		SliderStartY: block.DY,
	}, nil
}

// Verify 校验滑块验证码
func (s *captchaService) Verify(ctx context.Context, captchaID string, userX int) error {
	key := s.captchaKey(captchaID)

	// 获取存储的验证码信息（格式："缺口绝对X,滑块起始X"）
	info, err := s.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return bizerrors.New(bizerrors.CodeInvalidParam, "验证码已过期，请重新获取")
	}
	if err != nil {
		return fmt.Errorf("查询验证码失败: %w", err)
	}

	// 删除验证码（一次性使用）
	s.redis.Del(ctx, key)

	// 解析存储的信息
	var correctX, startX int
	if _, err := fmt.Sscanf(info, "%d,%d", &correctX, &startX); err != nil {
		return fmt.Errorf("解析验证码数据失败: %w", err)
	}

	// userX 是拖拽距离，加上滑块起始位置得到绝对坐标
	absoluteX := startX + userX

	// 校验位置（允许 captchaTolerance 像素误差）
	if !slide.Validate(absoluteX, 0, correctX, 0, captchaTolerance) {
		return bizerrors.New(bizerrors.CodeInvalidParam, "滑块验证失败，请重试")
	}

	return nil
}
