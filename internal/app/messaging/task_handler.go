// Package messaging 提供任务处理器，集成结果上报、去重和店铺亲和性功能
package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"task-processor/internal/domain/message"
	"task-processor/internal/domain/model"
	domaintask "task-processor/internal/domain/task"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"
	"task-processor/internal/pkg/strx"

	"github.com/sirupsen/logrus"
)

// TaskHandler 增强的任务处理器，集成结果上报、去重和店铺亲和性
type TaskHandler struct {
	processor      worker.Processor
	resultReporter *ResultReporter
	adapter        *domaintask.MessageAdapter
	platform       string
	logger         *logrus.Logger

	// 店铺亲和性支持
	storeAPI     api.StoreAPI
	ownedStores  []int64
	deduplicator *domaintask.Deduplicator
}

// TaskHandlerConfig 任务处理器配置
type TaskHandlerConfig struct {
	Platform       string
	Processor      worker.Processor
	ResultReporter *ResultReporter
	StoreAPI       api.StoreAPI
	OwnedStores    []int64
	Deduplicator   *domaintask.Deduplicator
	Logger         *logrus.Logger
}

// NewTaskHandler 创建增强的任务处理器
func NewTaskHandler(cfg TaskHandlerConfig) *TaskHandler {
	return &TaskHandler{
		processor:      cfg.Processor,
		resultReporter: cfg.ResultReporter,
		adapter:        domaintask.NewMessageAdapter(),
		platform:       cfg.Platform,
		logger:         cfg.Logger,
		storeAPI:       cfg.StoreAPI,
		ownedStores:    cfg.OwnedStores,
		deduplicator:   cfg.Deduplicator,
	}
}

// HandleMessage 处理消息并上报结果(支持去重和店铺亲和性)
func (eth *TaskHandler) HandleMessage(ctx context.Context, msg *rabbitmq.Message) error {
	startTime := time.Now()
	eth.logger.Debugf("[%s] 开始处理任务消息: ID=%s", eth.platform, msg.ID)

	// 1. 转换并验证消息
	task, originalPayload, err := eth.convertAndValidateMessage(msg)
	if err != nil {
		return err
	}

	// 2. 去重检查
	if eth.shouldSkipDuplicate(task) {
		return nil
	}

	// 3. 验证店铺访问权限
	isOwned, err := eth.validateStoreAccess(task)
	if err != nil {
		return err
	}

	// 4. 验证平台匹配
	err = eth.validatePlatform(task)
	if err != nil {
		return err
	}

	eth.logger.Infof("[%s] 处理任务: ID=%d, ProductID=%s, Priority=%d",
		eth.platform, task.ID, task.ProductID, task.Priority)

	// 5. 处理任务
	err = eth.processTaskWithReporting(ctx, task, originalPayload, startTime)
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

// convertAndValidateMessage 转换并验证消息
func (eth *TaskHandler) convertAndValidateMessage(msg *rabbitmq.Message) (*model.Task, map[string]any, error) {
	// 将消息转换为领域消息
	domainMsg := &domaintask.Message{
		ID:         msg.ID,
		Type:       msg.Type,
		Payload:    msg.Payload,
		Priority:   msg.Priority,
		Timestamp:  msg.Timestamp,
		RetryCount: msg.RetryCount,
		MaxRetries: msg.MaxRetries,
	}

	// 提取嵌套的 payload（如果存在）
	originalPayload := eth.extractNestedPayload(domainMsg)

	// 转换为任务对象
	task, err := eth.adapter.MessageToTask(domainMsg)
	if err != nil {
		eth.logger.Errorf("[%s] 转换消息为任务失败: ID=%s, Error=%v",
			eth.platform, msg.ID, err)
		return nil, nil, domaintask.NewConversionError(0, err)
	}

	// 验证任务的关键字段
	if !task.IsValid() {
		eth.logger.Errorf("[%s] 收到无效任务消息: ID=%d, ProductID=%s, Platform=%s, MessageID=%s",
			eth.platform, task.ID, task.ProductID, task.Platform, msg.ID)
		return nil, nil, domaintask.NewInvalidTaskError(task.ID,
			fmt.Sprintf("invalid task data: ID=%d, ProductID=%s", task.ID, task.ProductID))
	}

	return task, originalPayload, nil
}

// extractNestedPayload 提取嵌套的 payload（分布式爬虫消息格式）
func (eth *TaskHandler) extractNestedPayload(domainMsg *domaintask.Message) map[string]any {
	if nestedPayload, ok := domainMsg.Payload["payload"]; ok {
		if payloadMap, ok := nestedPayload.(map[string]any); ok {
			eth.logger.Debugf("[%s] 检测到嵌套 payload，提取内层数据", eth.platform)

			// 字段名映射：分布式爬虫使用 "id"，但 TaskMessage 期望 "taskId"
			if id, exists := payloadMap["id"]; exists {
				payloadMap["taskId"] = id
			}

			domainMsg.Payload = payloadMap
			return payloadMap
		}
	}
	return domainMsg.Payload
}

// shouldSkipDuplicate 检查是否应该跳过重复任务
func (eth *TaskHandler) shouldSkipDuplicate(task *model.Task) bool {
	if eth.deduplicator == nil {
		return false
	}

	if eth.deduplicator.IsDuplicate(task.ID) {
		eth.logger.Warnf("[%s] 检测到重复任务，跳过处理: ID=%d", eth.platform, task.ID)
		return true
	}

	// 标记任务已处理（防止重复）
	eth.deduplicator.MarkProcessed(task.ID)
	return false
}

// validateStoreAccess 验证店铺访问权限
func (eth *TaskHandler) validateStoreAccess(task *model.Task) (bool, error) {
	if eth.storeAPI == nil {
		return false, nil
	}

	storeInfo, err := eth.storeAPI.GetStore(task.StoreID)
	if err != nil {
		eth.logger.Errorf("[%s] 获取店铺 %d 配置失败: %v", eth.platform, task.StoreID, err)
		return false, domaintask.NewStoreNotFoundError(task.ID, task.StoreID, err)
	}

	isOwned := eth.isOwnedStore(task.StoreID)
	if isOwned {
		eth.logger.Infof("[%s] 🎯 处理自己店铺的任务: ID=%d, StoreID=%d, StoreName=%s",
			eth.platform, task.ID, task.StoreID, storeInfo.Name)
	} else {
		eth.logger.Infof("[%s] 🔄 处理其他店铺的任务: ID=%d, StoreID=%d, StoreName=%s",
			eth.platform, task.ID, task.StoreID, storeInfo.Name)
	}

	return isOwned, nil
}

// validatePlatform 验证平台匹配
func (eth *TaskHandler) validatePlatform(task *model.Task) error {
	// 如果任务平台为空，使用处理器平台（容错处理旧数据）
	if task.Platform == "" {
		eth.logger.Warnf("[%s] 任务平台为空，使用处理器平台: ID=%d", eth.platform, task.ID)
		task.Platform = eth.platform
		return nil
	}

	// 比较任务平台与处理器平台（使用领域对象方法）
	if !task.PlatformMatches(eth.platform) {
		return domaintask.NewPlatformMismatchError(task.ID, task.Platform, eth.platform)
	}

	return nil
}

// getBasePlatform 获取基础平台名称（移除 .crawler 后缀）
func (eth *TaskHandler) getBasePlatform() string {
	if strings.Contains(eth.platform, ".crawler") {
		return strings.TrimSuffix(eth.platform, ".crawler")
	}
	return eth.platform
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

	// 检查是否是爬虫任务（使用领域对象方法）
	if task.IsCrawlerTask() {
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
		successData := message.NewSuccessData(task.Platform, task.ProductID, task.StoreID)

		if reportErr := eth.resultReporter.ReportSuccess(task, successData.ToMap(), processingTime); reportErr != nil {
			eth.logger.Errorf("[%s] 上报成功结果失败: ID=%d, Error=%v",
				eth.platform, task.ID, reportErr)
			// 注意：上报失败不影响任务处理结果
		}
	}

	return nil
}

// shouldRetry 判断是否应该重试
func (eth *TaskHandler) shouldRetry(task *model.Task, err error) bool {
	// 检查重试次数（使用领域对象方法）
	if !task.CanRetry() {
		return false
	}

	// 如果是 TaskError，使用其 IsRetryable 方法
	if taskErr, ok := err.(*domaintask.TaskError); ok {
		return taskErr.IsRetryable()
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
		if strx.ContainsIgnoreCase(errorMsg, nonRetryable) {
			eth.logger.Infof("[%s] 错误不可重试: ID=%d, Error=%s",
				eth.platform, task.ID, errorMsg)
			return false
		}
	}

	return true
}

// isOwnedStore 判断是否是该节点拥有的店铺
func (eth *TaskHandler) isOwnedStore(storeID int64) bool {
	return slices.Contains(eth.ownedStores, storeID)
}
