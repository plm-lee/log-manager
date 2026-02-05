package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// APIKeyMiddleware API Key 认证中间件
// apiKey 为空时不做认证；否则检查 X-API-Key 或 Authorization: Bearer <key>
func APIKeyMiddleware(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiKey == "" {
			c.Next()
			return
		}

		key := c.GetHeader("X-API-Key")
		if key == "" {
			auth := c.GetHeader("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				key = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		if key != apiKey {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "未授权",
				"message": "请提供有效的 API Key（X-API-Key 或 Authorization: Bearer <key>）",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
