package request

import "encoding/json"

// UserConfigRequest 统一用户配置请求
type UserConfigRequest struct {
	Name        string          `json:"name" binding:"required"`
	Provider    string          `json:"provider" binding:"required"`
	APIKey      string          `json:"api_key"`
	APIURL      string          `json:"api_url"`
	Model       string          `json:"model"`
	Dimensions  *int            `json:"dimensions"`
	DailyQuota  *int            `json:"daily_quota"`
	ExtraConfig json.RawMessage `json:"extra_config"`
	Enabled     *bool           `json:"enabled"` // 使用指针以区分未传和 false
}
