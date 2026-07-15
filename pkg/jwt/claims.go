package jwt

import "github.com/golang-jwt/jwt/v5"

// TokenType token 类型
type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

// CustomClaims 自定义 JWT Claims
type CustomClaims struct {
	UserID    uint      `json:"user_id"`
	Username  string    `json:"username"`
	TokenType TokenType `json:"token_type"`
	jwt.RegisteredClaims
}

// GetUserID 获取用户 ID
func (c *CustomClaims) GetUserID() uint {
	return c.UserID
}

// GetUsername 获取用户名
func (c *CustomClaims) GetUsername() string {
	return c.Username
}

// GetTokenType 获取 token 类型
func (c *CustomClaims) GetTokenType() TokenType {
	return c.TokenType
}
