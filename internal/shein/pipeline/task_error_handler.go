package pipeline

import (
	"fmt"
	"sync"
	"time"

	"task-processor/internal/core/logger"
	"task-processor/internal/core/metrics"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/api"

	"github.com/sirupsen/logrus"
)

type TaskErrorHandler struct {
	processor              *SheinProcessor
	lastCookieDeletionTime map[int64]time.Time
	cookieDeletionMutex    sync.RWMutex
}

func NewTaskErrorHandler(processor *SheinProcessor) *TaskErrorHandler {
	return &TaskErrorHandler{
		processor:              processor,
		lastCookieDeletionTime: make(map[int64]time.Time),
		cookieDeletionMutex:    sync.RWMutex{},
	}
}

func (h *TaskErrorHandler) HandleTaskFailure(task model.Task, err error) {
	if shein.IsFilteredError(err) {
		logger.GetGlobalLogger("shein/pipeline").Infof("任务被筛选规则过滤: ID=%d, Priority=%d, 原因=%v", task.ID, task.Priority, err)
		h.updateTaskStatusToAPI(fmt.Sprintf("%d", task.ID), model.TaskStatusTerminated, err.Error())
		return
	}

	isRetryable := shein.IsRetryableError(err)
	logger.GetGlobalLogger("shein/pipeline").Infof("错误类型: %T, 错误值: %v, 是否可重试: %t", err, err, isRetryable)

	if !isRetryable {
		h.updateTaskStatusToAPI(fmt.Sprintf("%d", task.ID), model.TaskStatusTerminated, err.Error())
		metrics.GlobalTaskMetrics().IncrementFailed()

		if cookieErr, ok := shein.IsCookieLoadError(err); ok {
			logger.GetGlobalLogger("shein/pipeline").Warnf("Cookie加载失败，暂停店铺 %d:%d 24小时，等待重新登录", cookieErr.TenantID, cookieErr.StoreID)
			h.pauseShopWithCacheCleanup(cookieErr.TenantID, cookieErr.StoreID, "Cookie加载失败，等待重新登录", 24*time.Hour)
		}

		logger.GetGlobalLogger("shein/pipeline").Errorf("任务处理失败且不可重试: ID=%d, Priority=%d, 错误=%v", task.ID, task.Priority, err)
		return
	}

	retryDecision := model.ApplyRetryFailure(&task, h.processor.GetConfig().Processor.MaxRetries)
	if retryDecision.Exhausted {
		h.updateTaskStatusToAPIWithTask(&task, model.TaskStatusTerminated, err.Error())
		metrics.GlobalTaskMetrics().IncrementFailed()
		logger.GetGlobalLogger("shein/pipeline").Errorf("任务处理失败且达到最大重试次数: ID=%d, Priority=%d, 重试次数=%d, 错误=%v", task.ID, task.Priority, task.RetryCount, err)
		return
	}

	h.updateTaskStatusToAPIWithTask(&task, model.TaskStatusPendingRetry, err.Error())
	logger.GetGlobalLogger("shein/pipeline").Warnf("任务处理失败，等待重试: ID=%d, Priority=%d->%d, 重试次数=%d", task.ID, retryDecision.OriginalPriority, retryDecision.CurrentPriority, task.RetryCount)
	metrics.GlobalTaskMetrics().IncrementRequeued()
}

func (h *TaskErrorHandler) HandleAuthenticationExpired(authErr *api.AuthenticationExpiredError, task model.Task) {
	tenantID := authErr.TenantID
	shopID := authErr.ShopID

	if tenantID == 0 {
		tenantID = task.TenantID
	}
	if shopID == 0 {
		shopID = task.StoreID
	}

	logger.GetGlobalLogger("shein/pipeline").Warnf("检测到店铺 %d:%d 认证过期，开始处理客户端清理和暂停", tenantID, shopID)

	if h.shouldDeleteCookie(shopID) {
		h.deleteCookieIfNeeded(shopID)
	} else {
		logger.GetGlobalLogger("shein/pipeline").Infof("店铺 %d 距离上次删除Cookie不足10分钟，跳过删除操作", shopID)
	}

	if err := h.setPauseKeyForAuthExpired(shopID, "认证过期(20302)"); err != nil {
		logrus.WithError(err).Warnf("设置店铺 %d 的认证过期暂停键失败", shopID)
	}

	h.pauseShopWithCacheCleanup(tenantID, shopID, "认证过期(20302)", 10*time.Minute)
	logger.GetGlobalLogger("shein/pipeline").Infof("已暂停店铺 %d:%d 执行任务10分钟并清理缓存，原因: 认证过期", tenantID, shopID)
	h.updateTaskStatusToAPI(fmt.Sprintf("%d", task.ID), model.TaskStatusPendingRetry, fmt.Sprintf("认证过期: %s", authErr.Message))

	logger.GetGlobalLogger("shein/pipeline").Infof("认证过期任务已标记为待重试: TaskID=%d, TenantID=%d, ShopID=%d", task.ID, tenantID, shopID)
	metrics.GlobalTaskMetrics().IncrementRequeued()
}

func (h *TaskErrorHandler) shouldDeleteCookie(shopID int64) bool {
	h.cookieDeletionMutex.RLock()
	defer h.cookieDeletionMutex.RUnlock()

	lastTime, exists := h.lastCookieDeletionTime[shopID]
	if !exists {
		return true
	}

	return time.Since(lastTime) >= 10*time.Minute
}

func (h *TaskErrorHandler) deleteCookieIfNeeded(shopID int64) {
	storeClient := h.processor.GetManagementClient().GetStoreClient()
	if storeClient != nil {
		logger.GetGlobalLogger("shein/pipeline").Infof("正在删除店铺 %d 的过期Cookie...", shopID)
		success, err := storeClient.DeleteStoreCookie(shopID)
		if err != nil {
			logger.GetGlobalLogger("shein/pipeline").Errorf("删除店铺 %d 的过期Cookie失败: %v", shopID, err)
		} else if success {
			logger.GetGlobalLogger("shein/pipeline").Infof("成功删除店铺 %d 的过期Cookie", shopID)
			h.updateCookieDeletionTime(shopID)
		} else {
			logger.GetGlobalLogger("shein/pipeline").Warnf("删除店铺 %d 的过期Cookie返回失败", shopID)
		}
	} else {
		logger.GetGlobalLogger("shein/pipeline").Warn("StoreClient未初始化，无法删除过期Cookie")
	}
}

func (h *TaskErrorHandler) updateCookieDeletionTime(shopID int64) {
	h.cookieDeletionMutex.Lock()
	defer h.cookieDeletionMutex.Unlock()
	h.lastCookieDeletionTime[shopID] = time.Now()
}

func (h *TaskErrorHandler) pauseShopWithCacheCleanup(tenantID, shopID int64, reason string, duration time.Duration) {
	logger.GetGlobalLogger("shein/pipeline").Infof("已删除店铺 %d:%d 的客户端缓存", tenantID, shopID)
	h.processor.GetMemoryManager().ShopPauseManager.PauseShopForDuration(tenantID, shopID, reason, duration)
}

func (h *TaskErrorHandler) updateTaskStatusToAPI(taskID string, status model.TaskStatus, errorMsg string) {
	statusUpdater := NewTaskStatusUpdater(h.processor)
	statusUpdater.UpdateTaskStatusAsync(taskID, status, errorMsg)
}

func (h *TaskErrorHandler) updateTaskStatusToAPIWithTask(task *model.Task, status model.TaskStatus, errorMsg string) {
	statusUpdater := NewTaskStatusUpdater(h.processor)
	statusUpdater.UpdateTaskStatusAsyncWithTask(task, status, errorMsg)
}

func (h *TaskErrorHandler) setPauseKeyForAuthExpired(shopID int64, reason string) error {
	storeClient := h.processor.GetManagementClient().GetStoreClient()
	if storeClient == nil {
		logger.GetGlobalLogger("shein/pipeline").Warn("店铺客户端未初始化，无法设置暂停键")
		return fmt.Errorf("店铺客户端未初始化")
	}

	logger.GetGlobalLogger("shein/pipeline").Infof("设置店铺 %d 的认证过期暂停键，原因: %s", shopID, reason)
	success, err := storeClient.SetStorePauseStatus(shopID, true, "auth_expired")
	if err != nil {
		logger.GetGlobalLogger("shein/pipeline").Errorf("设置店铺 %d 的暂停状态失败: %v", shopID, err)
		return fmt.Errorf("设置暂停状态失败: %w", err)
	}

	if success {
		logger.GetGlobalLogger("shein/pipeline").Infof("成功设置店铺 %d 的认证过期暂停键", shopID)
		return nil
	}

	logger.GetGlobalLogger("shein/pipeline").Warnf("设置店铺 %d 的暂停状态返回失败", shopID)
	return fmt.Errorf("设置暂停状态返回失败")
}
