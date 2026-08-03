package response

import "time"

// AdminUserResponse 管理员用户列表响应
type AdminUserResponse struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Nickname  string    `json:"nickname"`
	Avatar    string    `json:"avatar"`
	Role      string    `json:"role"`
	Status    int       `json:"status"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ConfigStatusGroupResponse 配置状态组响应
type ConfigStatusGroupResponse struct {
	Group       string `json:"group"`
	Total       int64  `json:"total"`
	Enabled     int64  `json:"enabled"`
	Description string `json:"description"`
}
