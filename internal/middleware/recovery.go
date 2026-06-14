package middleware

import (
	"YoudaoNoteLm/pkg/logger"
	"YoudaoNoteLm/pkg/response"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Recovery Panic 恢复中间件
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 获取堆栈信息
				stack := debug.Stack()

				// 记录错误日志
				logger.Error("Panic recovered",
					zap.Any("error", err),
					zap.String("stack", string(stack)),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
					zap.String("ip", c.ClientIP()),
				)

				// 响应客户端
				response.InternalError(c, "服务器内部错误")
				c.Abort()
			}
		}()

		c.Next()
	}
}

// RecoveryWithLogger 自定义日志的 Panic 恢复中间件
func RecoveryWithLogger(customLogger any) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()

				// 调用自定义日志记录器
				if customLogger != nil {
					logger.Error("Panic recovered",
						zap.Any("error", err),
						zap.String("stack", string(stack)),
					)
				}

				response.InternalError(c, "服务器内部错误")
				c.Abort()
			}
		}()

		c.Next()
	}
}
