// Package messaging 提供任务处理器，集成结果上报、去重和店铺亲和性功能
package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"task-processor/internal/domain/model"
	"task-processor/internal/domain/task"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"
	"task-processor/internal/pkg/management/api"
	"task-processor/internal/pkg/strutil"

	"github.com/sirupsen/logrus"
)

// TaskHandler 增强的任务处理器，集成结果上报、去重和店铺亲和性
type TaskHandler struct {
	processor      worker.Processor
	resultReporter *ResultReporter
	adapter        *task.MessageAdapter
	platform       string
	logger         *logrus.Logger

	// 店铺亲和性支持
	storeAPI     api.StoreAPI
	ownedStores  []int64
	deduplicator *task.Deduplicator
}

// NewTaskHandler 创建增强的任务处理器
func NewTaskHandler(
	platform string,
	processor worker.Processor,
	resultReporter *ResultReporter,
	storeAPI api.StoreAPI,
	ownedStores []int64,
	deduplicator *task.Deduplicator,
	logger *logrus.Logger,
) *TaskHandler {
	return &TaskHandler{
		processor:      processor,
		resultReporter: resultReporter,
		adapter:        task.NewMessageAdapter(),
		platform:       platform,
		logger:         logger,
		storeAPI:       storeAPI,
		ownedStores:    ownedStores,
		deduplicator:   deduplicator,
	}
}

// HandleMessage 处理消息并上报结果(支持去重和店铺亲和性)
func (eth *TaskHandler) HandleMessage(ctx context.Context, msg *rabbitmq.Message) error {
	startTime := time.Now()

	eth.logger.Debugf("[%s] 开始处理任务消息: ID=%s", eth.platform, msg.ID)

	// 将消息转换为任务对象
	domainMsg := &task.Message{
		ID:         msg.ID,
		Type:       msg.Type,
		Payload:    msg.Payload,
		Priority:   msg.Priority,
		Timestamp:  msg.Timestamp,
		RetryCount: msg.RetryCount,
		MaxRetries: msg.MaxRetries,
	}

	// 特殊处理：如果 Payload 中有嵌套的 payload 字段（分布式爬虫消息格式）
	// 提取内层的 payload 作为真正的任务数据
	if nestedPayload, ok := msg.Payload["payload"]; ok {
		if payloadMap, ok := nestedPayload.(map[string]any); ok {
			eth.logger.Debugf("[%s] 检测到嵌套 payload，提取内层数据", eth.platform)

			// 字段名映射：分布式爬虫使用 "id"，但 TaskMessage 期望 "taskId"
			if id, exists := payloadMap["id"]; exists {
				payloadMap["taskId"] = id
			}

			domainMsg.Payload = payloadMap
		}
	}

	task, err := eth.adapter.MessageToTask(domainMsg)
	if err != nil {
		eth.logger.Errorf("[%s] 转换消息为任务失败: ID=%s, Error=%v",
			eth.platform, msg.ID, err)
		return fmt.Errorf("转换消息为任务失败: %w", err)
	}

	// 验证任务的关键字段
	if task.ID == 0 || task.ProductID == "" {
		eth.logger.Errorf("[%s] 收到无效任务消息: ID=%d, ProductID=%s, Platform=%s, MessageID=%s, Payload=%+v",
			eth.platform, task.ID, task.ProductID, task.Platform, msg.ID, msg.Payload)
		return fmt.Errorf("任务数据无效: ID=%d, ProductID=%s", task.ID, task.ProductID)
	}

	// 1. 去重检查（如果启用）
	if eth.deduplicator != nil && eth.deduplicator.IsDuplicate(task.ID) {
		eth.logger.Warnf("[%s] 检测到重复任务，跳过处理: ID=%d", eth.platform, task.ID)
		return nil // 返回nil表示消息已处理，可以ACK
	}

	// 2. 标记任务已处理（防止重复）
	if eth.deduplicator != nil {
		eth.deduplicator.MarkProcessed(task.ID)
	}

	// 3. 获取店铺配置（如果启用店铺亲和性）
	var isOwned bool
	if eth.storeAPI != nil {
		storeInfo, err := eth.storeAPI.GetStore(task.StoreID)
		if err != nil {
			eth.logger.Errorf("[%s] 获取店铺 %d 配置失败: %v", eth.platform, task.StoreID, err)
			return fmt.Errorf("获取店铺配置失败: %w", err)
		}

		isOwned = eth.isOwnedStore(task.StoreID)
		if isOwned {
			eth.logger.Infof("[%s] 🎯 处理自己店铺的任务: ID=%d, StoreID=%d, StoreName=%s",
				eth.platform, task.ID, task.StoreID, storeInfo.Name)
		} else {
			eth.logger.Infof("[%s] 🔄 处理其他店铺的任务: ID=%d, StoreID=%d, StoreName=%s",
				eth.platform, task.ID, task.StoreID, storeInfo.Name)
		}
	}

	// 4. 验证平台匹配（忽略大小写）
	// 如果任务平台为空，使用处理器平台（容错处理旧数据）
	if task.Platform == "" {
		eth.logger.Warnf("[%s] 任务平台为空，使用处理器平台: ID=%d", eth.platform, task.ID)
		task.Platform = eth.platform
	} else {
		// 对于爬虫处理器（如 amazon.crawler），提取基础平台名进行比较
		processorBasePlatform := eth.platform
		if strings.Contains(eth.platform, ".crawler") {
			processorBasePlatform = strings.TrimSuffix(eth.platform, ".crawler")
		}

		// 比较任务平台与处理器基础平台（忽略大小写）
		if !strings.EqualFold(task.Platform, processorBasePlatform) {
			err := fmt.Errorf("任务平台 %s 与处理器平台 %s 不匹配", task.Platform, eth.platform)
			eth.logger.Errorf("[%s] %v", eth.platform, err)
			return err
		}
	}

	eth.logger.Infof("[%s] 处理任务: ID=%d, ProductID=%s, Priority=%d",
		eth.platform, task.ID, task.ProductID, task.Priority)

	// 5. 处理任务
	err = eth.processTaskWithReporting(ctx, task, domainMsg.Payload, startTime)
	if err != nil {
		eth.logger.Errorf("[%s] 任务处理失败: ID=%d, Error=%v",
			eth.platform, task.ID, err)
		return err
	}

	processingTime := time.Since(startTime)
	eth.logger.Infof("[%s] 任务处理成功: ID=%d, Duration=%v, IsOwned=%v",
		eth.platform, task.ID, processingTime, isOwned)

	return nil
}

// processTaskWithReporting 处理任务并上报结果
func (eth *TaskHandler) processTaskWithReporting(
	ctx context.Context,
	task *model.Task,
	originalPayload map[string]any,
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

	// 将任务转换为 WorkerJob
	// 注意：对于爬虫任务，需要保留原始 payload（包含 reply_to 等额外字段）
	var taskData []byte
	var err error

	// 检查是否是爬虫任务（通过平台名称判断）
	if strings.Contains(strings.ToLower(eth.platform), "crawler") {
		// 爬虫任务：使用原始 payload（包含 reply_to 字段）
		taskData, err = json.Marshal(originalPayload)
	} else {
		// 普通任务：使用 Task 对象
		taskData, err = json.Marshal(task)
	}

	if err != nil {
		return fmt.Errorf("序列化任务数据失败: %w", err)
	}

	job := worker.WorkerJob{
		TenantID: fmt.Sprintf("%d", task.TenantID),
		ShopID:   fmt.Sprintf("%d", task.StoreID),
		TaskData: string(taskData),
	}

	// 处理任务
	err = eth.processor.ProcessTask(ctx, job)
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
		successData := map[string]any{
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
func (eth *TaskHandler) shouldRetry(task *model.Task, err error) bool {
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
		if strutil.ContainsIgnoreCase(errorMsg, nonRetryable) {
			eth.logger.Infof("[%s] 错误不可重试: ID=%d, Error=%s",
				eth.platform, task.ID, errorMsg)
			return false
		}
	}

	return true
}

// TaskProcessorRegistry 增强的任务处理器注册表
type TaskProcessorRegistry struct {
	processors     map[string]worker.Processor
	handlers       map[string]rabbitmq.MessageHandler
	resultReporter *ResultReporter
	storeAPI       api.StoreAPI
	ownedStores    []int64
	deduplicator   *task.Deduplicator
	logger         *logrus.Logger
	mutex          sync.RWMutex
}

// NewTaskProcessorRegistry 创建增强的任务处理器注册表
func NewTaskProcessorRegistry(
	resultReporter *ResultReporter,
	storeAPI api.StoreAPI,
	ownedStores []int64,
	deduplicator *task.Deduplicator,
	logger *logrus.Logger,
) *TaskProcessorRegistry {
	return &TaskProcessorRegistry{
		processors:     make(map[string]worker.Processor),
		handlers:       make(map[string]rabbitmq.MessageHandler),
		resultReporter: resultReporter,
		storeAPI:       storeAPI,
		ownedStores:    ownedStores,
		deduplicator:   deduplicator,
		logger:         logger,
	}
}

// RegisterProcessor 注册处理器
func (etpr *TaskProcessorRegistry) RegisterProcessor(platform string, processor worker.Processor) {
	etpr.mutex.Lock()
	defer etpr.mutex.Unlock()

	etpr.processors[platform] = processor

	// 创建增强的消息处理器（支持结果上报、去重和店铺亲和性）
	handler := NewTaskHandler(
		platform,
		processor,
		etpr.resultReporter,
		etpr.storeAPI,
		etpr.ownedStores,
		etpr.deduplicator,
		etpr.logger,
	)
	etpr.handlers[platform] = handler

	etpr.logger.Infof("注册增强处理器: 平台=%s", platform)
}

// GetHandler 获取消息处理器
func (etpr *TaskProcessorRegistry) GetHandler(platform string) (rabbitmq.MessageHandler, bool) {
	etpr.mutex.RLock()
	defer etpr.mutex.RUnlock()

	handler, exists := etpr.handlers[platform]
	return handler, exists
}

// GetAllHandlers 获取所有消息处理器
func (etpr *TaskProcessorRegistry) GetAllHandlers() map[string]rabbitmq.MessageHandler {
	etpr.mutex.RLock()
	defer etpr.mutex.RUnlock()

	handlers := make(map[string]rabbitmq.MessageHandler)
	maps.Copy(handlers, etpr.handlers)

	return handlers
}

// GetQueueName 根据平台获取队列名称
func (etpr *TaskProcessorRegistry) GetQueueName(platform string) string {
	adapter := task.NewMessageAdapter()
	return adapter.GetQueueName(platform)
}

// GetStats 获取统计信息
func (etpr *TaskProcessorRegistry) GetStats() map[string]any {
	etpr.mutex.RLock()
	defer etpr.mutex.RUnlock()

	stats := make(map[string]any)
	stats["total_processors"] = len(etpr.processors)

	platformStats := make(map[string]any)
	for platform := range etpr.processors {
		platformStats[platform] = map[string]any{
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

// isOwnedStore 判断是否是该节点拥有的店铺
func (eth *TaskHandler) isOwnedStore(storeID int64) bool {
	return slices.Contains(eth.ownedStores, storeID)
}
