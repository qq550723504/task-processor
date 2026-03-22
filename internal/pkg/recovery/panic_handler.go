// Package recovery 提供统一的panic恢复和错误处理工具
package recovery

import (
	"fmt"
	"runtime/debug"

	"github.com/sirupsen/logrus"
)

// PanicHandler panic处理器接口
type PanicHandler interface {
	// HandlePanic 处理panic
	HandlePanic(r any, context string)
}

// LoggerPanicHandler 基于logger的panic处理器
type LoggerPanicHandler struct {
	logger *logrus.Entry
}

// NewLoggerPanicHandler 创建基于logger的panic处理器
func NewLoggerPanicHandler(logger *logrus.Entry) *LoggerPanicHandler {
	return &LoggerPanicHandler{
		logger: logger,
	}
}

// HandlePanic 处理panic
func (h *LoggerPanicHandler) HandlePanic(r any, context string) {
	h.logger.WithFields(logrus.Fields{
		"panic":   r,
		"context": context,
		"stack":   string(debug.Stack()),
	}).Error("发生panic")
}

// Recover 通用的panic恢复函数
// 用法: defer recovery.Recover("操作描述", logger)
func Recover(context string, logger *logrus.Entry) {
	if r := recover(); r != nil {
		if logger == nil {
			fmt.Printf("[PANIC] context=%s panic=%v\n", context, r)
			return
		}
		logger.WithFields(logrus.Fields{
			"panic":   r,
			"context": context,
		}).Error("发生panic")
	}
}

// RecoverWithStack 带堆栈信息的panic恢复函数
// 用法: defer recovery.RecoverWithStack("操作描述", logger)
func RecoverWithStack(context string, logger *logrus.Entry) {
	if r := recover(); r != nil {
		if logger == nil {
			fmt.Printf("[PANIC] context=%s panic=%v\nstack=%s\n", context, r, string(debug.Stack()))
			return
		}
		logger.WithFields(logrus.Fields{
			"panic":   r,
			"context": context,
			"stack":   string(debug.Stack()),
		}).Error("发生panic")
	}
}

// RecoverWithError 带错误返回的panic恢复函数
// 用法: defer recovery.RecoverWithError("操作描述", logger, &err)
func RecoverWithError(context string, logger *logrus.Entry, errPtr *error) {
	if r := recover(); r != nil {
		logger.WithFields(logrus.Fields{
			"panic":   r,
			"context": context,
			"stack":   string(debug.Stack()),
		}).Error("发生panic")

		if errPtr != nil {
			*errPtr = fmt.Errorf("%s时发生panic: %v", context, r)
		}
	}
}

// RecoverWithCallback 带回调的panic恢复函数
// 用法: defer recovery.RecoverWithCallback("操作描述", logger, func(r any) { ... })
func RecoverWithCallback(context string, logger *logrus.Entry, callback func(r any)) {
	if r := recover(); r != nil {
		logger.WithFields(logrus.Fields{
			"panic":   r,
			"context": context,
			"stack":   string(debug.Stack()),
		}).Error("发生panic")

		if callback != nil {
			callback(r)
		}
	}
}

// SafeExecute 安全执行函数,自动处理panic
func SafeExecute(context string, logger *logrus.Entry, fn func() error) (err error) {
	defer RecoverWithError(context, logger, &err)
	return fn()
}

// SafeExecuteWithResult 安全执行函数并返回结果,自动处理panic
func SafeExecuteWithResult[T any](context string, logger *logrus.Entry, fn func() (T, error)) (result T, err error) {
	defer RecoverWithError(context, logger, &err)
	return fn()
}
