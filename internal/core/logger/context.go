package logger

import (
	"context"

	"github.com/sirupsen/logrus"
)

// contextKey 用于在context中存储logger的key类型
type contextKey string

const (
	loggerContextKey contextKey = "logger"
	traceIDKey       contextKey = "trace_id"
	requestIDKey     contextKey = "request_id"
)

// WithLogger 将logger添加到context中
func WithLogger(ctx context.Context, logger *logrus.Entry) context.Context {
	return context.WithValue(ctx, loggerContextKey, logger)
}

// FromContext 从context中获取logger
// 如果context中没有logger，返回全局logger
func FromContext(ctx context.Context, component string) *logrus.Entry {
	if logger, ok := ctx.Value(loggerContextKey).(*logrus.Entry); ok {
		return logger
	}
	return GetGlobalLogger(component)
}

// WithTraceID 将trace_id添加到context中
func WithTraceID(ctx context.Context, traceID string) context.Context {
	// 同时更新logger
	if logger, ok := ctx.Value(loggerContextKey).(*logrus.Entry); ok {
		newLogger := logger.WithField("trace_id", traceID)
		ctx = context.WithValue(ctx, loggerContextKey, newLogger)
	}
	return context.WithValue(ctx, traceIDKey, traceID)
}

// GetTraceID 从context中获取trace_id
func GetTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value(traceIDKey).(string); ok {
		return traceID
	}
	return ""
}

// WithRequestID 将request_id添加到context中
func WithRequestID(ctx context.Context, requestID string) context.Context {
	// 同时更新logger
	if logger, ok := ctx.Value(loggerContextKey).(*logrus.Entry); ok {
		newLogger := logger.WithField("request_id", requestID)
		ctx = context.WithValue(ctx, loggerContextKey, newLogger)
	}
	return context.WithValue(ctx, requestIDKey, requestID)
}

// GetRequestID 从context中获取request_id
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(requestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// WithFields 为context中的logger添加字段
func WithFields(ctx context.Context, fields logrus.Fields) context.Context {
	logger := FromContext(ctx, "")
	newLogger := logger.WithFields(fields)
	return context.WithValue(ctx, loggerContextKey, newLogger)
}

// WithField 为context中的logger添加单个字段
func WithField(ctx context.Context, key string, value any) context.Context {
	logger := FromContext(ctx, "")
	newLogger := logger.WithField(key, value)
	return context.WithValue(ctx, loggerContextKey, newLogger)
}
