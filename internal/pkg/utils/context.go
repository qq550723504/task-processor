// Package utils 提供工具方法
package utils

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

// ContextKey 上下文键类型
type ContextKey string

const (
	// RequestIDKey 请求ID键
	RequestIDKey ContextKey = "request_id"
	// UserIDKey 用户ID键
	UserIDKey ContextKey = "user_id"
	// TenantIDKey 租户ID键
	TenantIDKey ContextKey = "tenant_id"
)

// WithRequestID 添加请求ID到上下文
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// GetRequestID 从上下文获取请求ID
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// WithUserID 添加用户ID到上下文
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// GetUserID 从上下文获取用户ID
func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		return userID
	}
	return ""
}

// WithTenantID 添加租户ID到上下文
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, TenantIDKey, tenantID)
}

// GetTenantID 从上下文获取租户ID
func GetTenantID(ctx context.Context) string {
	if tenantID, ok := ctx.Value(TenantIDKey).(string); ok {
		return tenantID
	}
	return ""
}

// WithTimeout 创建带超时的上下文
func WithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}

// LogContextInfo 记录上下文信息到日志
func LogContextInfo(ctx context.Context, logger *logrus.Logger, message string) {
	logger.WithFields(logrus.Fields{
		"request_id": GetRequestID(ctx),
		"user_id":    GetUserID(ctx),
		"tenant_id":  GetTenantID(ctx),
	}).Info(message)
}
