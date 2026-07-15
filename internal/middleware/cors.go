package middleware

import (
	"strconv"
	"strings"

	"YoudaoNoteLm/pkg/config"

	"github.com/gin-gonic/gin"
)

// CORS handles cross-origin requests according to config.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Get().CORS
		if !cfg.Enabled {
			c.Next()
			return
		}

		origin := c.Request.Header.Get("Origin")
		allowOrigin := ""
		for _, allowed := range cfg.AllowOrigins {
			allowed = strings.TrimSpace(allowed)
			if allowed == "*" || allowed == origin {
				allowOrigin = allowedOriginValue(allowed, origin, cfg.AllowCredentials)
				break
			}
		}

		if allowOrigin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			c.Writer.Header().Add("Vary", "Origin")
		}

		c.Writer.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowMethods, ", "))
		c.Writer.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowHeaders, ", "))
		c.Writer.Header().Set("Access-Control-Expose-Headers", strings.Join(cfg.ExposeHeaders, ", "))
		if cfg.AllowCredentials {
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if cfg.MaxAge > 0 {
			c.Writer.Header().Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func allowedOriginValue(allowed string, origin string, allowCredentials bool) string {
	if allowed != "*" {
		return allowed
	}
	if allowCredentials && origin != "" {
		return origin
	}
	return "*"
}
