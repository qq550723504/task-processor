// Package client 提供TEMU平台认证配置管理
package client

import "time"

// AuthConfig 认证相关配置
type AuthConfig struct {
	// 重试配置
	MaxRetries               int           // 最大重试次数
	RetryDelay               time.Duration // 重试延迟
	MaxConsecutiveAuthErrors int           // 最大连续认证错误次数

	// 认证错误码配置
	TemuAuthErrorCodes []string // TEMU特定认证错误码
	AuthErrorKeywords  []string // 通用认证错误关键词
}

// DefaultAuthConfig 返回默认认证配置
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		MaxRetries:               3,
		RetryDelay:               time.Second * 2,
		MaxConsecutiveAuthErrors: 2,
		TemuAuthErrorCodes: []string{
			"40001", // TEMU认证失效错误码
			"40002", // 可能的其他认证错误码
			"40003", // 可能的其他认证错误码
		},
		AuthErrorKeywords: []string{
			"401", "403", "unauthorized", "forbidden",
			"登录", "认证", "权限", "cookie", "signature",
			"expired", "签名", "过期",
		},
	}
}

// AuthContext 认证上下文
type AuthContext struct {
	StoreID               int64  // 店铺ID
	AttemptCount          int    // 当前尝试次数
	ConsecutiveAuthErrors int    // 连续认证错误次数
	LastError             error  // 最后一次错误
	Reason                string // 失败原因
}

// NewAuthContext 创建新的认证上下文
func NewAuthContext(storeID int64) *AuthContext {
	return &AuthContext{
		StoreID:               storeID,
		AttemptCount:          0,
		ConsecutiveAuthErrors: 0,
	}
}

// IncrementAttempt 增加尝试次数
func (ctx *AuthContext) IncrementAttempt() {
	ctx.AttemptCount++
}

// IncrementAuthError 增加认证错误次数
func (ctx *AuthContext) IncrementAuthError(err error) {
	ctx.ConsecutiveAuthErrors++
	ctx.LastError = err
}

// ResetAuthError 重置认证错误次数
func (ctx *AuthContext) ResetAuthError() {
	ctx.ConsecutiveAuthErrors = 0
}

// ShouldPause 判断是否应该暂停
func (ctx *AuthContext) ShouldPause(config *AuthConfig) bool {
	return ctx.ConsecutiveAuthErrors >= config.MaxConsecutiveAuthErrors
}

// IsMaxRetryReached 判断是否达到最大重试次数
func (ctx *AuthContext) IsMaxRetryReached(config *AuthConfig) bool {
	return ctx.AttemptCount >= config.MaxRetries
}
