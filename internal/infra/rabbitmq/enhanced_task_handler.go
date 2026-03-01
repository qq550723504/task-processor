// Package rabbitmq 提供增强的任务处理器，集成结果上报功能
package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"task-processor/internal/app/worker"
	"task-processor/internal/domain/model"

	"github.com/sirupsen/logrus"
)

// EnhancedTaskHandler 增强的任务处理器，集成结果上报
type EnhancedTaskHandler struct {
	processor      worker.Processor
	resultReporter *ResultReporter
	adapter        *TaskMessageAdapter
	platform       string
	logger         *logrus.Logger
}

// NewEnhancedTaskHandler 创建增强的任务处理器
func NewEnhancedTaskHandler(
	platform string,
	processor worker.Processor,
	resultReporter *ResultReporter,
	logger *logrus.Logger,
) *EnhancedTaskHandler {
	return &EnhancedTaskHandler{
		processor:      processor,
		resultReporter: resultReporter,
		adapter:        NewTaskMessageAdapter(),
		platform:       platform,
		logger:         logger,
	}
}

// HandleMessage 处理消息并上报结果
func (eth *EnhancedTaskHandler) HandleMessage(ctx context.Context, msg *Message) error {
	startTime := time.Now()

	eth.logger.Debugf("[%s] 开始处理任务消息: ID=%s", eth.platform, msg.ID)

	// 将消息转换为任务对象
	task, err := eth.adapter.MessageToTask(msg)
	if err != nil {
		eth.logger.Errorf("[%s] 转换消息为任务失败: ID=%s, Error=%v",
			eth.platform, msg.ID, err)
		return fmt.Errorf("转换消息为任务失败: %w", err)
	}

	// 验证平台匹配
	if task.Platform != eth.platform {
		err := fmt.Errorf("任务平台 %s 与处理器平台 %s 不匹配", task.Platform, eth.platform)
		eth.logger.Errorf("[%s] %v", eth.platform, err)
		return err
	}

	eth.logger.Infof("[%s] 处理任务: ID=%d, ProductID=%s, Priority=%d",
		eth.platform, task.ID, task.ProductID, task.Priority)

	// 处理任务
	err = eth.processTaskWithReporting(ctx, task, startTime)
	if err != nil {
		eth.logger.Errorf("[%s] 任务处理失败: ID=%d, Error=%v",
			eth.platform, task.ID, err)
		return err
	}

	processingTime := time.Since(startTime)
	eth.logger.Infof("[%s] 任务处理成功: ID=%d, Duration=%v",
		eth.platform, task.ID, processingTime)

	return nil
}

// processTaskWithReporting 处理任务并上报结果
func (eth *EnhancedTaskHandler) processTaskWithReporting(
	ctx context.Context,
	task *model.Task,
	startTime time.Time,
) error {
	defer func() {
		if r := recover(); r != nil {
			processingTime := time.Since(startTime)
			panicErr := fmt.Errorf("任务处理发生panic: %v", r)

			eth.logger.Errorf("[%s] 任务处理panic: ID=%d, Panic=%v",
				eth.platform, task.ID, r)

			// 上报panic错误
			if eth.resultReporter != nil {
				if reportErr := eth.resultReporter.ReportFailure(task, panicErr, processingTime); reportErr != nil {
					eth.logger.Errorf("[%s] 上报panic结果失败: ID=%d, Error=%v",
						eth.platform, task.ID, reportErr)
				}
			}
		}
	}()

	// 处理任务
	err := eth.processor.ProcessTask(ctx, task)
	processingTime := time.Since(startTime)

	if err != nil {
		// 处理失败
		eth.logger.Errorf("[%s] 任务处理失败: ID=%d, Error=%v",
			eth.platform, task.ID, err)

		// 判断是否需要重试
		if eth.shouldRetry(task, err) {
			// 上报重试结果
			if eth.resultReporter != nil {
				if reportErr := eth.resultReporter.ReportRetry(task, err, processingTime); reportErr != nil {
					eth.logger.Errorf("[%s] 上报重试结果失败: ID=%d, Error=%v",
						eth.platform, task.ID, reportErr)
				}
			}
		} else {
			// 上报失败结果
			if eth.resultReporter != nil {
				if reportErr := eth.resultReporter.ReportFailure(task, err, processingTime); reportErr != nil {
					eth.logger.Errorf("[%s] 上报失败结果失败: ID=%d, Error=%v",
						eth.platform, task.ID, reportErr)
				}
			}
		}

		return err
	}

	// 处理成功
	eth.logger.Infof("[%s] 任务处理成功: ID=%d", eth.platform, task.ID)

	// 上报成功结果
	if eth.resultReporter != nil {
		successData := map[string]interface{}{
			"platform":   task.Platform,
			"product_id": task.ProductID,
			"store_id":   task.StoreID,
		}

		if reportErr := eth.resultReporter.ReportSuccess(task, successData, processingTime); reportErr != nil {
			eth.logger.Errorf("[%s] 上报成功结果失败: ID=%d, Error=%v",
				eth.platform, task.ID, reportErr)
			// 注意：上报失败不影响任务处理结果
		}
	}

	return nil
}

// shouldRetry 判断是否应该重试
func (eth *EnhancedTaskHandler) shouldRetry(task *model.Task, err error) bool {
	// 检查重试次数
	if task.RetryCount >= task.MaxRetryCount {
		return false
	}

	// 检查错误类型，某些错误不需要重试
	errorMsg := err.Error()

	// 不重试的错误类型
	nonRetryableErrors := []string{
		"invalid product id",
		"product not found",
		"access denied",
		"unauthorized",
		"forbidden",
		"bad request",
	}

	for _, nonRetryable := range nonRetryableErrors {
		if contains(errorMsg, nonRetryable) {
			eth.logger.Infof("[%s] 错误不可重试: ID=%d, Error=%s",
				eth.platform, task.ID, errorMsg)
			return false
		}
	}

	return true
}

// contains 检查字符串是否包含子字符串（忽略大小写）
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				containsIgnoreCase(s, substr)))
}

// containsIgnoreCase 忽略大小写检查包含关系
func containsIgnoreCase(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)

	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// toLower 转换为小写
func toLower(s string) string {
	result := make([]byte, len(s))
	for i, b := range []byte(s) {
		if b >= 'A' && b <= 'Z' {
			result[i] = b + 32
		} else {
			result[i] = b
		}
	}
	return string(result)
}

// EnhancedTaskProcessorRegistry 增强的任务处理器注册表
type EnhancedTaskProcessorRegistry struct {
	processors     map[string]worker.Processor
	handlers       map[string]MessageHandler
	resultReporter *ResultReporter
	logger         *logrus.Logger
	mutex          sync.RWMutex
}

// NewEnhancedTaskProcessorRegistry 创建增强的任务处理器注册表
func NewEnhancedTaskProcessorRegistry(resultReporter *ResultReporter, logger *logrus.Logger) *EnhancedTaskProcessorRegistry {
	return &EnhancedTaskProcessorRegistry{
		processors:     make(map[string]worker.Processor),
		handlers:       make(map[string]MessageHandler),
		resultReporter: resultReporter,
		logger:         logger,
	}
}

// RegisterProcessor 注册处理器
func (etpr *EnhancedTaskProcessorRegistry) RegisterProcessor(platform string, processor worker.Processor) {
	etpr.mutex.Lock()
	defer etpr.mutex.Unlock()

	etpr.processors[platform] = processor

	// 创建增强的消息处理器
	handler := NewEnhancedTaskHandler(platform, processor, etpr.resultReporter, etpr.logger)
	etpr.handlers[platform] = handler

	etpr.logger.Infof("注册增强处理器: 平台=%s", platform)
}

// GetHandler 获取消息处理器
func (etpr *EnhancedTaskProcessorRegistry) GetHandler(platform string) (MessageHandler, bool) {
	etpr.mutex.RLock()
	defer etpr.mutex.RUnlock()

	handler, exists := etpr.handlers[platform]
	return handler, exists
}

// GetAllHandlers 获取所有消息处理器
func (etpr *EnhancedTaskProcessorRegistry) GetAllHandlers() map[string]MessageHandler {
	etpr.mutex.RLock()
	defer etpr.mutex.RUnlock()

	handlers := make(map[string]MessageHandler)
	for platform, handler := range etpr.handlers {
		handlers[platform] = handler
	}

	return handlers
}

// GetQueueName 根据平台获取队列名称
func (etpr *EnhancedTaskProcessorRegistry) GetQueueName(platform string) string {
	adapter := NewTaskMessageAdapter()
	return adapter.GetQueueName(platform)
}

// GetStats 获取统计信息
func (etpr *EnhancedTaskProcessorRegistry) GetStats() map[string]interface{} {
	etpr.mutex.RLock()
	defer etpr.mutex.RUnlock()

	stats := make(map[string]interface{})
	stats["total_processors"] = len(etpr.processors)

	platformStats := make(map[string]interface{})
	for platform := range etpr.processors {
		platformStats[platform] = map[string]interface{}{
			"status": "registered",
		}
	}
	stats["platforms"] = platformStats

	// 添加结果上报器统计
	if etpr.resultReporter != nil {
		stats["result_reporter"] = etpr.resultReporter.GetStats()
	}

	return stats
}
