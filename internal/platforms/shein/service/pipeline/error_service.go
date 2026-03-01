// Package shein 提供SHEIN平台的任务错误处理功能
package pipeline

import (
	"fmt"
	"sync"
	"task-processor/internal/domain/model"
	"task-processor/internal/pkg/utils"
	"task-processor/internal/platforms/shein/api"
	shein_model "task-processor/internal/platforms/shein/model"
	"time"

	"github.com/sirupsen/logrus"
)

// TaskErrorHandler SHEIN任务错误处理器
type TaskErrorHandler struct {
	processor              *SheinProcessor
	lastCookieDeletionTime map[int64]time.Time
	cookieDeletionMutex    sync.RWMutex
}

// NewTaskErrorHandler 创建任务错误处理器
func NewTaskErrorHandler(processor *SheinProcessor) *TaskErrorHandler {
	return &TaskErrorHandler{
		processor:              processor,
		lastCookieDeletionTime: make(map[int64]time.Time),
		cookieDeletionMutex:    sync.RWMutex{},
	}
}

// HandleTaskFailure 处理任务失败
func (h *TaskErrorHandler) HandleTaskFailure(task model.Task, err error) {
	isRetryable := shein_model.IsRetryableError(err)

	logrus.Infof("错误类型: %T, 错误值: %v, 是否可重试: %t", err, err, isRetryable)

	if !isRetryable {
		h.updateTaskStatusToAPI(fmt.Sprintf("%d", task.ID), model.TaskStatusTerminated, err.Error())
		utils.GetGlobalMetrics().IncrementFailed()

		// 区分业务过滤和真正的错误
		if shein_model.IsFilteredError(err) {
			logrus.Infof("✓ 任务被筛选规则过滤: ID=%d, Priority=%d, 原因=%v", task.ID, task.Priority, err)
		} else {
			logrus.Errorf("任务处理失败且不可重试: ID=%d, Priority=%d, 错误=%v", task.ID, task.Priority, err)
		}
		return
	}

	task.RetryCount++
	originalPriority := task.Priority
	if task.RetryCount > 0 && task.Priority > 10 {
		task.Priority = task.Priority - 10
		if task.Priority < 0 {
			task.Priority = 0
		}
	}

	maxRetries := h.processor.GetConfig().Processor.MaxRetries
	if task.RetryCount >= maxRetries {
		h.updateTaskStatusToAPI(fmt.Sprintf("%d", task.ID), model.TaskStatusTerminated, err.Error())
		utils.GetGlobalMetrics().IncrementFailed()
		logrus.Errorf("任务处理失败且达到最大重试次数: ID=%d, Priority=%d, 重试次数=%d, 错误=%v", task.ID, task.Priority, task.RetryCount, err)
	} else {
		h.updateTaskStatusToAPI(fmt.Sprintf("%d", task.ID), model.TaskStatusPendingRetry, err.Error())
		logrus.Warnf("任务处理失败，等待重试: ID=%d, Priority=%d->%d, 重试次数=%d", task.ID, originalPriority, task.Priority, task.RetryCount)
		utils.GetGlobalMetrics().IncrementRequeued()
	}
}

// HandleAuthenticationExpired 处理认证过期错误
func (h *TaskErrorHandler) HandleAuthenticationExpired(authErr *api.AuthenticationExpiredError, task model.Task) {
	tenantID := authErr.TenantID
	shopID := authErr.ShopID

	if tenantID == 0 {
		tenantID = task.TenantID
	}
	if shopID == 0 {
		shopID = task.StoreID
	}

	logrus.Warnf("检测到店铺 %d:%d 认证过期，开始处理客户端清理和暂停", tenantID, shopID)

	// 检查是否需要删除Cookie
	if h.shouldDeleteCookie(shopID) {
		h.deleteCookieIfNeeded(shopID)
	} else {
		logrus.Infof("店铺 %d 距离上次删除Cookie不足10分钟，跳过删除操作", shopID)
	}

	// 设置店铺暂停状态到管理系统
	if err := h.setPauseKeyForAuthExpired(shopID, "认证过期(20302)"); err != nil {
		logrus.WithError(err).Warnf("设置店铺 %d 的认证过期暂停键失败", shopID)
	}

	if err := h.pauseShopWithCacheCleanup(tenantID, shopID, "认证过期(20302)", 10*time.Minute); err != nil {
		logrus.WithError(err).Warnf("暂停店铺并清理缓存失败: tenantID=%d, shopID=%d", tenantID, shopID)
	} else {
		logrus.Infof("已暂停店铺 %d:%d 执行任务10分钟并清理缓存，原因: 认证过期", tenantID, shopID)
	}

	// 标记任务为待重试状态
	h.updateTaskStatusToAPI(fmt.Sprintf("%d", task.ID), model.TaskStatusPendingRetry, fmt.Sprintf("认证过期: %s", authErr.Message))

	logrus.Infof("认证过期任务已标记为待重试: TaskID=%d, TenantID=%d, ShopID=%d", task.ID, tenantID, shopID)
	utils.GetGlobalMetrics().IncrementRequeued()
}

// shouldDeleteCookie 检查是否应该删除Cookie
func (h *TaskErrorHandler) shouldDeleteCookie(shopID int64) bool {
	h.cookieDeletionMutex.RLock()
	defer h.cookieDeletionMutex.RUnlock()

	lastTime, exists := h.lastCookieDeletionTime[shopID]
	if !exists {
		return true
	}

	return time.Since(lastTime) >= 10*time.Minute
}

// deleteCookieIfNeeded 删除过期Cookie
func (h *TaskErrorHandler) deleteCookieIfNeeded(shopID int64) {
	storeClient := h.processor.GetManagementClient().GetStoreClient()
	if storeClient != nil {
		logrus.Infof("正在删除店铺 %d 的过期Cookie...", shopID)
		success, err := storeClient.DeleteStoreCookie(shopID)
		if err != nil {
			logrus.Errorf("删除店铺 %d 的过期Cookie失败: %v", shopID, err)
		} else if success {
			logrus.Infof("成功删除店铺 %d 的过期Cookie", shopID)
			h.updateCookieDeletionTime(shopID)
		} else {
			logrus.Warnf("删除店铺 %d 的过期Cookie返回失败", shopID)
		}
	} else {
		logrus.Warn("StoreClient未初始化，无法删除过期Cookie")
	}
}

// updateCookieDeletionTime 更新Cookie删除时间
func (h *TaskErrorHandler) updateCookieDeletionTime(shopID int64) {
	h.cookieDeletionMutex.Lock()
	defer h.cookieDeletionMutex.Unlock()

	h.lastCookieDeletionTime[shopID] = time.Now()
}

// pauseShopWithCacheCleanup 暂停店铺并清理相关缓存
func (h *TaskErrorHandler) pauseShopWithCacheCleanup(tenantID, shopID int64, reason string, duration time.Duration) error {
	logrus.Infof("已删除店铺 %d:%d 的客户端缓存", tenantID, shopID)

	// 暂停店铺
	h.processor.GetMemoryManager().ShopPauseManager.PauseShopForDuration(tenantID, shopID, reason, duration)

	return nil
}

// updateTaskStatusToAPI 更新任务状态到API
func (h *TaskErrorHandler) updateTaskStatusToAPI(taskID string, status model.TaskStatus, errorMsg string) {
	// 委托给状态更新器处理
	statusUpdater := NewTaskStatusUpdater(h.processor)
	statusUpdater.UpdateTaskStatusAsync(taskID, status, errorMsg)
}

// setPauseKeyForAuthExpired 设置认证过期暂停键到管理系统
func (h *TaskErrorHandler) setPauseKeyForAuthExpired(shopID int64, reason string) error {
	storeClient := h.processor.GetManagementClient().GetStoreClient()
	if storeClient == nil {
		logrus.Warn("店铺客户端未初始化，无法设置暂停键")
		return fmt.Errorf("店铺客户端未初始化")
	}

	logrus.Infof("设置店铺 %d 的认证过期暂停键，原因: %s", shopID, reason)
	success, err := storeClient.SetStorePauseStatus(shopID, true, "auth_expired")
	if err != nil {
		logrus.Errorf("设置店铺 %d 的暂停状态失败: %v", shopID, err)
		return fmt.Errorf("设置暂停状态失败: %w", err)
	}

	if success {
		logrus.Infof("✓ 成功设置店铺 %d 的认证过期暂停键", shopID)
	} else {
		logrus.Warnf("设置店铺 %d 的暂停状态返回失败", shopID)
		return fmt.Errorf("设置暂停状态返回失败")
	}

	return nil
}
