package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Claims JWT 声明
type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// GenerateToken 生成 JWT
// secret: 签名密钥
// username: 用户名
// expireHours: 有效期（小时）
func GenerateToken(secret, username string, expireHours int) (string, error) {
	claims := Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// parseToken 解析并校验 JWT
func parseToken(secret, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrTokenInvalidClaims
}

// JWTAuthMiddleware JWT 校验中间件
// 从 Authorization: Bearer <token> 解析 JWT，校验通过则设置 c.Set("user", username)
func JWTAuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secret == "" {
			c.Next()
			return
		}
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "未授权",
				"message": "请提供有效的 JWT（Authorization: Bearer <token>）",
			})
			c.Abort()
			return
		}
		tokenString := strings.TrimPrefix(auth, "Bearer ")
		claims, err := parseToken(secret, tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "未授权",
				"message": "JWT 无效或已过期",
			})
			c.Abort()
			return
		}
		c.Set("user", claims.Username)
		c.Next()
	}
}

// APIKeyOrJWTMiddleware Admin 路由认证：API Key 或 JWT 任一有效即可
// 用于保护 Web 管理界面调用的 API
func APIKeyOrJWTMiddleware(apiKey, jwtSecret string, loginEnabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !loginEnabled {
			c.Next()
			return
		}

		// 1. 校验 X-API-Key
		if apiKey != "" {
			if key := c.GetHeader("X-API-Key"); key == apiKey {
				c.Next()
				return
			}
		}

		// 2. 校验 Authorization: Bearer <apiKey 或 JWT>
		auth := c.GetHeader("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			val := strings.TrimPrefix(auth, "Bearer ")
			if apiKey != "" && val == apiKey {
				c.Next()
				return
			}
			if claims, err := parseToken(jwtSecret, val); err == nil {
				c.Set("user", claims.Username)
				c.Next()
				return
			}
		}

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "未授权",
			"message": "请登录或提供有效的 API Key",
		})
		c.Abort()
	}
}
