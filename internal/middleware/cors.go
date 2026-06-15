package middleware

import (
	"YoudaoNoteLm/pkg/config"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS 跨域中间件
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Get().CORS

		// 如果未启用 CORS，直接放行
		if !cfg.Enabled {
			c.Next()
			return
		}

		// 获取 Origin
		origin := c.Request.Header.Get("Origin")

		// 检查 Origin 是否在允许列表中
		allowOrigin := ""
		for _, allowed := range cfg.AllowOrigins {
			if allowed == "*" || allowed == origin {
				allowOrigin = allowed
				break
			}
		}

		// 设置 CORS 头
		if allowOrigin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		}

		c.Writer.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowMethods, ", "))
		c.Writer.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowHeaders, ", "))
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		// 处理 OPTIONS 预检请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
