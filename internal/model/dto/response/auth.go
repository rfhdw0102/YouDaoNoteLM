package response

import "time"

// UserResponse 用户响应
type UserResponse struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Nickname  string    `json:"nickname"`
	Avatar    string    `json:"avatar"`
	Status    int       `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// LoginResponse 登录响应（双 token）
type LoginResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User         UserResponse `json:"user"`
}

// SendCodeResponse 发送验证码响应
type SendCodeResponse struct {
	RetryAfter int `json:"retry_after"` // 剩余冷却秒数
}

// CaptchaData 滑块验证码数据
type CaptchaData struct {
	CaptchaID    string `json:"captcha_id"`     // 验证码 ID
	Background   string `json:"background"`     // 背景图 base64
	Slider       string `json:"slider"`         // 滑块图 base64
	SliderSize   int    `json:"slider_size"`    // 滑块大小（像素）
	BgWidth      int    `json:"bg_width"`       // 背景图宽度（像素）
	BgHeight     int    `json:"bg_height"`      // 背景图高度（像素）
	SliderStartX int    `json:"slider_start_x"` // 滑块起始 X 坐标（相对背景图，左边缘为 0）
	SliderStartY int    `json:"slider_start_y"` // 滑块起始 Y 坐标（相对背景图，顶部为 0）
}
