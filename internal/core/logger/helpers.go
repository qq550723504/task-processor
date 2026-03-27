package logger

import (
	"time"

	"github.com/sirupsen/logrus"
)

// StandardFields 标准字段名称常量
const (
	FieldComponent  = "component"
	FieldPlatform   = "platform"
	FieldTaskID     = "task_id"
	FieldProductID  = "product_id"
	FieldTenantID   = "tenant_id"
	FieldStoreID    = "store_id"
	FieldTraceID    = "trace_id"
	FieldRequestID  = "request_id"
	FieldDurationMs = "duration_ms"
	FieldRetryCount = "retry_count"
	FieldErrorCode  = "error_code"
	FieldErrorType  = "error_type"
	FieldOperation  = "operation"
	FieldStatus     = "status"
	FieldUserID     = "user_id"
	FieldSessionID  = "session_id"
)

// LoggerHelper 日志辅助工具
type LoggerHelper struct {
	logger *logrus.Entry
}

// NewLoggerHelper 创建日志辅助工具
func NewLoggerHelper(logger *logrus.Entry) *LoggerHelper {
	return &LoggerHelper{logger: logger}
}

// LogOperation 记录操作日志（带计时）
func (h *LoggerHelper) LogOperation(operation string, fn func() error) error {
	start := time.Now()
	h.logger.WithField(FieldOperation, operation).Info("开始操作")

	err := fn()
	duration := time.Since(start)

	fields := logrus.Fields{
		FieldOperation:  operation,
		FieldDurationMs: duration.Milliseconds(),
	}

	if err != nil {
		h.logger.WithError(err).WithFields(fields).Error("操作失败")
		return err
	}

	h.logger.WithFields(fields).Info("操作完成")
	return nil
}

// LogOperationWithResult 记录操作日志并返回结果
func (h *LoggerHelper) LogOperationWithResult(operation string, fn func() (any, error)) (any, error) {
	start := time.Now()
	h.logger.WithField(FieldOperation, operation).Info("开始操作")

	result, err := fn()
	duration := time.Since(start)

	fields := logrus.Fields{
		FieldOperation:  operation,
		FieldDurationMs: duration.Milliseconds(),
	}

	if err != nil {
		h.logger.WithError(err).WithFields(fields).Error("操作失败")
		return nil, err
	}

	h.logger.WithFields(fields).Info("操作完成")
	return result, nil
}

// LogProgress 记录进度日志
func (h *LoggerHelper) LogProgress(current, total int, message string) {
	if total == 0 {
		return
	}

	progress := float64(current) / float64(total) * 100
	h.logger.WithFields(logrus.Fields{
		"current":      current,
		"total":        total,
		"progress_pct": progress,
	}).Info(message)
}

// LogRetry 记录重试日志
func (h *LoggerHelper) LogRetry(operation string, retryCount, maxRetries int, err error) {
	h.logger.WithError(err).WithFields(logrus.Fields{
		FieldOperation:  operation,
		FieldRetryCount: retryCount,
		"max_retries":   maxRetries,
	}).Warn("操作失败，准备重试")
}

// LogTaskStart 记录任务开始
func (h *LoggerHelper) LogTaskStart(taskID int64, productID string) {
	h.logger.WithFields(logrus.Fields{
		FieldTaskID:    taskID,
		FieldProductID: productID,
	}).Info("任务开始")
}

// LogTaskComplete 记录任务完成
func (h *LoggerHelper) LogTaskComplete(taskID int64, duration time.Duration) {
	h.logger.WithFields(logrus.Fields{
		FieldTaskID:     taskID,
		FieldDurationMs: duration.Milliseconds(),
	}).Info("任务完成")
}

// LogTaskFailed 记录任务失败
func (h *LoggerHelper) LogTaskFailed(taskID int64, err error) {
	h.logger.WithError(err).WithField(FieldTaskID, taskID).Error("任务失败")
}

// LogAPICall 记录API调用
func (h *LoggerHelper) LogAPICall(method, url string, statusCode int, duration time.Duration) {
	fields := logrus.Fields{
		"method":        method,
		"url":           url,
		"status_code":   statusCode,
		FieldDurationMs: duration.Milliseconds(),
	}

	if statusCode >= 200 && statusCode < 300 {
		h.logger.WithFields(fields).Info("API调用成功")
	} else if statusCode >= 400 {
		h.logger.WithFields(fields).Error("API调用失败")
	} else {
		h.logger.WithFields(fields).Warn("API调用异常")
	}
}

// LogCacheHit 记录缓存命中
func (h *LoggerHelper) LogCacheHit(key string, hit bool) {
	h.logger.WithFields(logrus.Fields{
		"cache_key": key,
		"hit":       hit,
	}).Debug("缓存查询")
}

// LogDatabaseQuery 记录数据库查询
func (h *LoggerHelper) LogDatabaseQuery(query string, duration time.Duration, rowsAffected int64) {
	h.logger.WithFields(logrus.Fields{
		"query":         query,
		FieldDurationMs: duration.Milliseconds(),
		"rows_affected": rowsAffected,
	}).Debug("数据库查询")
}

// LogStateChange 记录状态变更
func (h *LoggerHelper) LogStateChange(entity string, entityID any, oldState, newState string) {
	h.logger.WithFields(logrus.Fields{
		"entity":    entity,
		"entity_id": entityID,
		"old_state": oldState,
		"new_state": newState,
	}).Info("状态变更")
}

// LogMetric 记录指标
func (h *LoggerHelper) LogMetric(metricName string, value any, tags map[string]string) {
	fields := logrus.Fields{
		"metric": metricName,
		"value":  value,
	}
	for k, v := range tags {
		fields[k] = v
	}
	h.logger.WithFields(fields).Info("指标记录")
}

// WithComponent 创建带组件名的logger
func WithComponent(component string) *logrus.Entry {
	return GetGlobalLogger(component)
}

// WithPlatform 创建带平台名的logger
func WithPlatform(platform string) *logrus.Entry {
	return GetGlobalLogger("core/logger").WithField(FieldPlatform, platform)
}

// WithTaskContext 创建带任务上下文的logger
func WithTaskContext(taskID int64, productID string) *logrus.Entry {
	return GetGlobalLogger("core/logger").WithFields(logrus.Fields{
		FieldTaskID:    taskID,
		FieldProductID: productID,
	})
}

// WithStoreContext 创建带店铺上下文的logger
func WithStoreContext(tenantID, storeID int64) *logrus.Entry {
	return GetGlobalLogger("core/logger").WithFields(logrus.Fields{
		FieldTenantID: tenantID,
		FieldStoreID:  storeID,
	})
}

// ShouldLog 判断是否应该记录日志（用于采样）
func ShouldLog(sampleRate float64) bool {
	if sampleRate <= 0 {
		return false
	}
	if sampleRate >= 1.0 {
		return true
	}
	// 使用纳秒时间戳的最后几位作为随机源
	return float64(time.Now().UnixNano()%10000)/10000.0 < sampleRate
}
