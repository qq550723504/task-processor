// Package consumer 提供任务处理器，集成结果上报、去重和店铺亲和性功能
package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"task-processor/internal/core/metrics"
	"time"

	apptask "task-processor/internal/app/task"
	"task-processor/internal/app/taskstatus"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"
	"task-processor/internal/model"
	"task-processor/internal/pkg/strx"

	"github.com/sirupsen/logrus"
)

const (
	storeStatusEnabled  int16 = 0
	storeStatusDisabled int16 = 1
)

// TaskHandler 增强的任务处理器，集成结果上报、去重和店铺亲和性
type TaskHandler struct {
	processor      worker.Processor
	resultReporter *ResultReporter
	adapter        *apptask.MessageAdapter
	platform       string
	logger         *logrus.Logger

	// 店铺亲和性支持
	storeAPI       api.StoreAPI
	ownedStores    []int64
	useStoreQueues bool
	deduplicator   *apptask.DeduplicationManager
}

type managementClientProvider interface {
	GetManagementClient() *management.ClientManager
}

type staleTaskMessageError struct {
	taskID           int64
	messageStatus    int16
	currentStatus    string
	currentStatusKey string
	reason           string
}

func (e *staleTaskMessageError) Error() string {
	return fmt.Sprintf("stale task message discarded: task_id=%d message_status=%d current_status=%s current_status_key=%s reason=%s",
		e.taskID, e.messageStatus, e.currentStatus, e.currentStatusKey, e.reason)
}

func (e *staleTaskMessageError) IsRetryable() bool {
	return false
}

func (e *staleTaskMessageError) ShouldDiscard() bool {
	return true
}

// TaskHandlerConfig 任务处理器配置
type TaskHandlerConfig struct {
	Platform       string
	Processor      worker.Processor
	ResultReporter *ResultReporter
	StoreAPI       api.StoreAPI
	OwnedStores    []int64
	UseStoreQueues bool
	Deduplicator   *apptask.DeduplicationManager
	Logger         *logrus.Logger
}

// NewTaskHandler 创建增强的任务处理器
func NewTaskHandler(cfg TaskHandlerConfig) *TaskHandler {
	return &TaskHandler{
		processor:      cfg.Processor,
		resultReporter: cfg.ResultReporter,
		adapter:        apptask.NewMessageAdapter(),
		platform:       cfg.Platform,
		logger:         cfg.Logger,
		storeAPI:       cfg.StoreAPI,
		ownedStores:    cfg.OwnedStores,
		useStoreQueues: cfg.UseStoreQueues,
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
	canProcess, err := eth.validateStoreAccess(task)
	if err != nil {
		// 非本节点店铺，静默跳过（不算错误，消息正常 ack）
		if apptask.IsStoreNotOwnedError(err) {
			return nil
		}
		return err
	}
	if !canProcess {
		return nil
	}

	// 4. 验证平台匹配
	err = eth.validatePlatform(task)
	if err != nil {
		return err
	}

	// 4.1 在真正执行前先 claim 远端任务状态，避免成功结果回写时仍停留在 queued。
	if err = eth.claimTaskStatus(task); err != nil {
		return err
	}

	eth.logger.Infof("[%s] 处理任务: ID=%d, ProductID=%s, Priority=%d, TargetPlatform=%s, SourcePlatform=%s",
		eth.platform, task.ID, task.ProductID, task.Priority, task.Platform, task.GetSourcePlatformOrDefault())

	eth.recordListingTaskMetricsOnStart(task, startTime)

	// 5. 处理任务
	err = eth.processTaskWithReporting(ctx, task, originalPayload, startTime)
	if err != nil {
		eth.logger.Errorf("[%s] 任务处理失败: ID=%d, Error=%v",
			eth.platform, task.ID, err)
		return err
	}

	processingTime := time.Since(startTime)
	if !task.IsCrawlerTask() {
		metrics.GlobalTaskMetrics().RecordProcessTime(processingTime)
	}

	eth.logger.Infof("[%s] 任务处理成功: ID=%d, Duration=%v",
		eth.platform, task.ID, processingTime)

	return nil
}

func (eth *TaskHandler) recordListingTaskMetricsOnStart(task *model.Task, startTime time.Time) {
	if task == nil || task.IsCrawlerTask() {
		return
	}

	taskMetrics := metrics.GlobalTaskMetrics()
	taskMetrics.IncrementProcessing()
	taskMetrics.RecordPriority(task.Priority)

	if task.CreateTime <= 0 {
		return
	}

	createdAt := time.UnixMilli(task.CreateTime)
	if createdAt.IsZero() || createdAt.After(startTime) {
		return
	}

	taskMetrics.RecordWaitTime(startTime.Sub(createdAt))
}

func (eth *TaskHandler) claimTaskStatus(task *model.Task) error {
	if task == nil || task.IsCrawlerTask() {
		return nil
	}
	if task.Status == model.TaskStatusPaused.Int16() {
		eth.logger.WithFields(logrus.Fields{
			"task_id":            task.ID,
			"message_status":     task.Status,
			"current_status":     model.TaskStatusPaused.String(),
			"current_status_key": "PAUSED",
		}).Warn("discarding paused task message before claim")
		return &staleTaskMessageError{
			taskID:           task.ID,
			messageStatus:    task.Status,
			currentStatus:    model.TaskStatusPaused.String(),
			currentStatusKey: "PAUSED",
			reason:           "paused_message",
		}
	}
	if task.Status == model.TaskStatusProcessing.Int16() {
		return nil
	}

	provider, ok := eth.processor.(managementClientProvider)
	if !ok {
		eth.logger.Debugf("[%s] processor does not expose management client, skip remote claim: ID=%d, Status=%d",
			eth.platform, task.ID, task.Status)
		return nil
	}

	managementClient := provider.GetManagementClient()
	if managementClient == nil {
		return fmt.Errorf("management client is not initialized for platform %s", eth.platform)
	}

	statusService := taskstatus.NewService("app/consumer", func() taskstatus.ImportTaskStatusClient {
		return managementClient.GetImportTaskClient()
	})
	expectedStatuses := []int16{task.Status}
	if shouldRetryClaimFromQueued(task.Status) {
		expectedStatuses = append(expectedStatuses, model.TaskStatusQueued.Int16())
	}

	var lastErr error
	for idx, expectedStatus := range expectedStatuses {
		if err := statusService.TransitionFromCodeSync(task.ID, expectedStatus, model.TaskStatusProcessing, ""); err != nil {
			lastErr = err
			if idx < len(expectedStatuses)-1 {
				eth.logger.WithError(err).WithFields(logrus.Fields{
					"task_id":         task.ID,
					"original_status": task.Status,
					"expected_status": expectedStatus,
					"fallback_status": model.TaskStatusQueued.Int16(),
					"target_status":   model.TaskStatusProcessing.Int16(),
				}).Warn("claim task status failed, retrying with queued fallback")
				continue
			}
			return eth.resolveClaimFailure(managementClient, task, expectedStatus, err)
		}

		task.Status = model.TaskStatusProcessing.Int16()
		return nil
	}

	return fmt.Errorf("claim task %d as processing failed: %w", task.ID, lastErr)
}

func (eth *TaskHandler) resolveClaimFailure(managementClient *management.ClientManager, task *model.Task, expectedStatus int16, claimErr error) error {
	if !isClaimConflictError(claimErr) || managementClient == nil {
		return fmt.Errorf("claim task %d as processing failed from status %d: %w", task.ID, expectedStatus, claimErr)
	}

	taskRPCClient := managementClient.GetTaskRPCClient()
	if taskRPCClient == nil {
		return fmt.Errorf("claim task %d as processing failed from status %d: %w", task.ID, expectedStatus, claimErr)
	}

	statusResp, err := taskRPCClient.GetTaskStatus(task.ID)
	if err != nil {
		eth.logger.WithError(err).WithFields(logrus.Fields{
			"task_id":         task.ID,
			"message_status":  task.Status,
			"expected_status": expectedStatus,
		}).Warn("failed to query current task status after claim conflict")
		return fmt.Errorf("claim task %d as processing failed from status %d: %w", task.ID, expectedStatus, claimErr)
	}

	eth.logger.WithFields(logrus.Fields{
		"task_id":            task.ID,
		"message_status":     task.Status,
		"expected_status":    expectedStatus,
		"current_status":     statusResp.CanonicalStatus,
		"current_status_key": statusResp.StatusKey,
		"processing_node":    statusResp.ProcessingNode,
	}).Warn("discarding stale task message after claim conflict")

	return &staleTaskMessageError{
		taskID:           task.ID,
		messageStatus:    task.Status,
		currentStatus:    statusResp.CanonicalStatus,
		currentStatusKey: statusResp.StatusKey,
		reason:           "claim_conflict",
	}
}

func isClaimConflictError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "Management API error 409") ||
		strings.Contains(message, "管理端拒绝更新任务状态")
}

func shouldRetryClaimFromQueued(status int16) bool {
	return status == model.TaskStatusPending.Int16() ||
		status == model.TaskStatusPendingRetry.Int16() ||
		status == model.TaskStatusCrawled.Int16()
}

// convertAndValidateMessage 转换并验证消息
func (eth *TaskHandler) convertAndValidateMessage(msg *rabbitmq.Message) (*model.Task, map[string]any, error) {
	// 将消息转换为领域消息
	domainMsg := &apptask.Message{
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
		return nil, nil, apptask.NewConversionError(0, err)
	}

	// 验证任务的关键字段
	if !task.IsValid() {
		eth.logger.Errorf("[%s] 收到无效任务消息: ID=%d, ProductID=%s, TargetPlatform=%s, SourcePlatform=%s, MessageID=%s",
			eth.platform, task.ID, task.ProductID, task.Platform, task.GetSourcePlatformOrDefault(), msg.ID)
		return nil, nil, apptask.NewInvalidTaskError(task.ID,
			fmt.Sprintf("invalid task data: ID=%d, ProductID=%s", task.ID, task.ProductID))
	}

	return task, originalPayload, nil
}

// extractNestedPayload 提取嵌套的 payload（分布式爬虫消息格式）
func (eth *TaskHandler) extractNestedPayload(domainMsg *apptask.Message) map[string]any {
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

	taskID := fmt.Sprintf("%d", task.ID)
	canSubmit, err := eth.deduplicator.CanSubmitTask(taskID)
	if err != nil || !canSubmit {
		eth.logger.Warnf("[%s] 检测到重复任务，跳过处理: ID=%d", eth.platform, task.ID)
		return true
	}

	// 标记任务处理中（防止重复）
	_ = eth.deduplicator.MarkTaskAsProcessing(taskID, eth.platform, task.MaxRetryCount)
	return false
}

// validateStoreAccess 验证店铺访问权限，并在共享队列模式下提前丢弃已禁用店铺任务。
func (eth *TaskHandler) validateStoreAccess(task *model.Task) (bool, error) {
	if eth.storeAPI == nil {
		return true, nil
	}

	storeInfo, err := eth.storeAPI.GetStore(task.StoreID)
	if err != nil {
		eth.logger.Errorf("[%s] 获取店铺 %d 配置失败: %v", eth.platform, task.StoreID, err)
		return false, apptask.NewStoreNotFoundError(task.ID, task.StoreID, err)
	}
	if storeInfo == nil {
		eth.logger.Warnf("[%s] 店铺信息为空，跳过任务: ID=%d, StoreID=%d", eth.platform, task.ID, task.StoreID)
		return false, nil
	}

	if !eth.isStoreDispatchEnabled(storeInfo) {
		eth.logger.Warnf("[%s] 丢弃已禁用店铺任务: ID=%d, StoreID=%d, StoreName=%s, Status=%d, EnableAutoListing=%s",
			eth.platform, task.ID, task.StoreID, storeInfo.Name, storeInfo.Status, formatBoolPointer(storeInfo.EnableAutoListing))
		return false, nil
	}

	// 默认共享消费；仅在显式启用店铺亲和模式时才限制 ownedStores
	if !eth.useStoreQueues || len(eth.ownedStores) == 0 {
		return true, nil
	}

	isOwned := eth.isOwnedStore(task.StoreID)
	if !isOwned {
		eth.logger.Debugf("[%s] 跳过非本节点店铺的任务: ID=%d, StoreID=%d", eth.platform, task.ID, task.StoreID)
		return false, apptask.NewStoreNotOwnedError(task.ID, task.StoreID)
	}

	eth.logger.Infof("[%s] 🎯 处理自己店铺的任务: ID=%d, StoreID=%d, StoreName=%s",
		eth.platform, task.ID, task.StoreID, storeInfo.Name)
	return true, nil
}

func (eth *TaskHandler) isStoreDispatchEnabled(storeInfo *api.StoreRespDTO) bool {
	if storeInfo == nil {
		return false
	}
	if storeInfo.Status != storeStatusEnabled {
		return false
	}
	return storeInfo.EnableAutoListing != nil && *storeInfo.EnableAutoListing
}

func formatBoolPointer(value *bool) string {
	if value == nil {
		return "nil"
	}
	if *value {
		return "true"
	}
	return "false"
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
		return apptask.NewPlatformMismatchError(task.ID, task.Platform, eth.platform)
	}

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

	// 检查是否是爬虫任务：优先用注册的队列平台名判断（含 "crawler"），
	// 兼容 task.Platform 字段（分布式爬虫消息里 sourcePlatform 是 "amazon" 不含 "crawler"）
	isCrawler := strings.Contains(strings.ToLower(eth.platform), "crawler") || task.IsCrawlerTask()
	if isCrawler {
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
		successData := apptask.NewSuccessData(task.Platform, task.GetSourcePlatformOrDefault(), task.ProductID, task.StoreID)

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
	if taskErr, ok := err.(*apptask.TaskError); ok {
		return taskErr.IsRetryable()
	}

	// 检查错误类型，某些错误不需要重试
	errorMsg := err.Error()

	// 不重试的错误类型
	nonRetryableErrors := []string{
		"nonretryable:",
		"terminated:",
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
