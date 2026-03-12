// Package productjson 提供认证中间件
package productjson

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthConfig 认证配置
type AuthConfig struct {
	// API Key 认证
	APIKeys map[string]string // key: API Key, value: User ID

	// Bearer Token 认证
	BearerTokens map[string]string // key: Token, value: User ID

	// 是否启用认证
	Enabled bool
}

// APIKeyAuthMiddleware API Key 认证中间件
func APIKeyAuthMiddleware(config *AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.Enabled {
			c.Next()
			return
		}

		// 从 Header 获取 API Key
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "缺少 API Key",
			})
			c.Abort()
			return
		}

		// 验证 API Key
		userID, valid := validateAPIKey(apiKey, config.APIKeys)
		if !valid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "无效的 API Key",
			})
			c.Abort()
			return
		}

		// 设置用户 ID 到上下文
		c.Set("user_id", userID)
		c.Set("auth_method", "api_key")

		c.Next()
	}
}

// BearerTokenAuthMiddleware Bearer Token 认证中间件
func BearerTokenAuthMiddleware(config *AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.Enabled {
			c.Next()
			return
		}

		// 从 Header 获取 Authorization
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "缺少 Authorization Header",
			})
			c.Abort()
			return
		}

		// 解析 Bearer Token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "无效的 Authorization Header 格式",
			})
			c.Abort()
			return
		}

		token := parts[1]

		// 验证 Token
		userID, valid := validateBearerToken(token, config.BearerTokens)
		if !valid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "无效的 Token",
			})
			c.Abort()
			return
		}

		// 设置用户 ID 到上下文
		c.Set("user_id", userID)
		c.Set("auth_method", "bearer_token")

		c.Next()
	}
}

// MultiAuthMiddleware 多种认证方式中间件（API Key 或 Bearer Token）
func MultiAuthMiddleware(config *AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.Enabled {
			c.Next()
			return
		}

		// 尝试 API Key 认证
		apiKey := c.GetHeader("X-API-Key")
		if apiKey != "" {
			userID, valid := validateAPIKey(apiKey, config.APIKeys)
			if valid {
				c.Set("user_id", userID)
				c.Set("auth_method", "api_key")
				c.Next()
				return
			}
		}

		// 尝试 Bearer Token 认证
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				token := parts[1]
				userID, valid := validateBearerToken(token, config.BearerTokens)
				if valid {
					c.Set("user_id", userID)
					c.Set("auth_method", "bearer_token")
					c.Next()
					return
				}
			}
		}

		// 认证失败
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "认证失败，请提供有效的 API Key 或 Bearer Token",
		})
		c.Abort()
	}
}

// validateAPIKey 验证 API Key（使用常量时间比较防止时序攻击）
func validateAPIKey(apiKey string, validKeys map[string]string) (string, bool) {
	for key, userID := range validKeys {
		if subtle.ConstantTimeCompare([]byte(apiKey), []byte(key)) == 1 {
			return userID, true
		}
	}
	return "", false
}

// validateBearerToken 验证 Bearer Token（使用常量时间比较防止时序攻击）
func validateBearerToken(token string, validTokens map[string]string) (string, bool) {
	for validToken, userID := range validTokens {
		if subtle.ConstantTimeCompare([]byte(token), []byte(validToken)) == 1 {
			return userID, true
		}
	}
	return "", false
}

// OptionalAuthMiddleware 可选认证中间件（不强制要求认证）
func OptionalAuthMiddleware(config *AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.Enabled {
			c.Next()
			return
		}

		// 尝试认证，但不强制要求
		apiKey := c.GetHeader("X-API-Key")
		if apiKey != "" {
			userID, valid := validateAPIKey(apiKey, config.APIKeys)
			if valid {
				c.Set("user_id", userID)
				c.Set("auth_method", "api_key")
				c.Set("authenticated", true)
			}
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				token := parts[1]
				userID, valid := validateBearerToken(token, config.BearerTokens)
				if valid {
					c.Set("user_id", userID)
					c.Set("auth_method", "bearer_token")
					c.Set("authenticated", true)
				}
			}
		}

		c.Next()
	}
}
