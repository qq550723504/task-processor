// Package utils 提供日志辅助工具，统一日志记录模式
package utils

import (
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
)

// LoggerHelper 日志辅助器
type LoggerHelper struct {
	logger *logrus.Logger
}

// NewLoggerHelper 创建日志辅助器
func NewLoggerHelper(logger *logrus.Logger) *LoggerHelper {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	return &LoggerHelper{
		logger: logger,
	}
}

// LogTaskStart 记录任务开始日志
func (h *LoggerHelper) LogTaskStart(taskID interface{}, productID string, operation string) {
	h.logger.WithFields(logrus.Fields{
		"task_id":    taskID,
		"product_id": productID,
		"operation":  operation,
		"phase":      "start",
	}).Infof("🚀 开始%s: TaskID=%v, ProductID=%s", operation, taskID, productID)
}

// LogTaskComplete 记录任务完成日志
func (h *LoggerHelper) LogTaskComplete(taskID interface{}, productID string, operation string, duration time.Duration) {
	h.logger.WithFields(logrus.Fields{
		"task_id":    taskID,
		"product_id": productID,
		"operation":  operation,
		"phase":      "complete",
		"duration":   duration.String(),
	}).Infof("✅ %s完成: TaskID=%v, ProductID=%s, 耗时=%v", operation, taskID, productID, duration.Truncate(time.Millisecond))
}

// LogTaskError 记录任务错误日志
func (h *LoggerHelper) LogTaskError(taskID interface{}, productID string, operation string, err error) {
	h.logger.WithFields(logrus.Fields{
		"task_id":    taskID,
		"product_id": productID,
		"operation":  operation,
		"phase":      "error",
		"error":      err.Error(),
	}).Errorf("❌ %s失败: TaskID=%v, ProductID=%s, Error=%v", operation, taskID, productID, err)
}

// LogWorkerStart 记录工作协程启动日志
func (h *LoggerHelper) LogWorkerStart(workerID int, poolName string) {
	h.logger.WithFields(logrus.Fields{
		"worker_id": workerID,
		"pool_name": poolName,
		"phase":     "start",
	}).Infof("🔧 工作协程 %d 已启动 [%s]", workerID, poolName)
}

// LogWorkerStop 记录工作协程停止日志
func (h *LoggerHelper) LogWorkerStop(workerID int, poolName string) {
	h.logger.WithFields(logrus.Fields{
		"worker_id": workerID,
		"pool_name": poolName,
		"phase":     "stop",
	}).Infof("🛑 工作协程 %d 正在停止 [%s]", workerID, poolName)
}

// LogPanic 记录panic恢复日志
func (h *LoggerHelper) LogPanic(component string, panicValue interface{}) {
	// 获取堆栈信息
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)

	h.logger.WithFields(logrus.Fields{
		"component": component,
		"panic":     panicValue,
		"stack":     string(buf[:n]),
	}).Errorf("💥 %s发生panic: %v", component, panicValue)
}

// LogHTTPRequest 记录HTTP请求日志
func (h *LoggerHelper) LogHTTPRequest(method, url string, statusCode int, duration time.Duration) {
	level := logrus.InfoLevel
	if statusCode >= 400 {
		level = logrus.WarnLevel
	}
	if statusCode >= 500 {
		level = logrus.ErrorLevel
	}

	h.logger.WithFields(logrus.Fields{
		"method":      method,
		"url":         url,
		"status_code": statusCode,
		"duration":    duration.String(),
	}).Logf(level, "🌐 HTTP %s %s -> %d (%v)", method, url, statusCode, duration.Truncate(time.Millisecond))
}

// LogServiceStart 记录服务启动日志
func (h *LoggerHelper) LogServiceStart(serviceName string) {
	h.logger.WithFields(logrus.Fields{
		"service": serviceName,
		"phase":   "start",
	}).Infof("🎯 启动服务: %s", serviceName)
}

// LogServiceStop 记录服务停止日志
func (h *LoggerHelper) LogServiceStop(serviceName string) {
	h.logger.WithFields(logrus.Fields{
		"service": serviceName,
		"phase":   "stop",
	}).Infof("🔴 停止服务: %s", serviceName)
}

// LogMetrics 记录指标日志
func (h *LoggerHelper) LogMetrics(component string, metrics map[string]interface{}) {
	h.logger.WithFields(logrus.Fields{
		"component": component,
		"metrics":   metrics,
	}).Infof("📊 [%s] 指标统计: %+v", component, metrics)
}

// LogConfigLoad 记录配置加载日志
func (h *LoggerHelper) LogConfigLoad(configFile string, success bool, err error) {
	if success {
		h.logger.WithFields(logrus.Fields{
			"config_file": configFile,
			"status":      "success",
		}).Infof("⚙️ 配置加载成功: %s", configFile)
	} else {
		h.logger.WithFields(logrus.Fields{
			"config_file": configFile,
			"status":      "failed",
			"error":       err.Error(),
		}).Errorf("⚙️ 配置加载失败: %s, Error=%v", configFile, err)
	}
}

// 全局日志辅助器
var globalLoggerHelper = NewLoggerHelper(nil)

// 全局便捷函数
func LogTaskStart(taskID interface{}, productID string, operation string) {
	globalLoggerHelper.LogTaskStart(taskID, productID, operation)
}

func LogTaskComplete(taskID interface{}, productID string, operation string, duration time.Duration) {
	globalLoggerHelper.LogTaskComplete(taskID, productID, operation, duration)
}

func LogTaskError(taskID interface{}, productID string, operation string, err error) {
	globalLoggerHelper.LogTaskError(taskID, productID, operation, err)
}

func LogPanic(component string, panicValue interface{}) {
	globalLoggerHelper.LogPanic(component, panicValue)
}

func LogServiceStart(serviceName string) {
	globalLoggerHelper.LogServiceStart(serviceName)
}

func LogServiceStop(serviceName string) {
	globalLoggerHelper.LogServiceStop(serviceName)
}
