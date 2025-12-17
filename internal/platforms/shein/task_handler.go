package shein

import (
	"context"
	"fmt"
	"sync"
	"task-processor/internal/common/management"
	management_api "task-processor/internal/common/management/api"
	"task-processor/internal/common/memory"
	shops "task-processor/internal/common/shein"
	"task-processor/internal/common/shein/api"
	"task-processor/internal/config"
	"task-processor/internal/model"
	"task-processor/internal/platforms/shein/modules"
	"task-processor/internal/utils"
	"time"

	"github.com/sirupsen/logrus"
)

// TaskHandler 任务处理器
type TaskHandler struct {
	config           *config.Config
	memoryManager    *memory.MemoryManager
	shopClientMgr    *shops.ClientManager
	managementClient *management.ClientManager
	// 记录每个店铺上次删除cookie的时间
	lastCookieDeletionTime map[int64]time.Time
	cookieDeletionMutex    sync.RWMutex
}

// NewTaskHandler 创建任务处理器
func NewTaskHandler(
	cfg *config.Config,
	memoryManager *memory.MemoryManager,
	shopClientMgr *shops.ClientManager,
	managementClient *management.ClientManager,
) *TaskHandler {
	return &TaskHandler{
		config:                 cfg,
		memoryManager:          memoryManager,
		shopClientMgr:          shopClientMgr,
		managementClient:       managementClient,
		lastCookieDeletionTime: make(map[int64]time.Time),
		cookieDeletionMutex:    sync.RWMutex{},
	}
}

// ProcessTask 处理任务
func (h *TaskHandler) ProcessTask(ctx context.Context, task modules.Task, pipeline *Pipeline) error {
	logrus.Infof("开始处理任务: ID=%s, ProductID=%s", task.ID, task.ProductID)

	taskCtx := &modules.TaskContext{
		Context:             ctx,
		Task:                &task,
		MemoryManager:       h.memoryManager,
		ShopClientMgr:       h.shopClientMgr,
		ManagementClientMgr: h.managementClient,
	}

	if err := pipeline.Process(taskCtx); err != nil {
		logrus.Errorf("任务处理失败: %v", err)

		if authErr, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			logrus.Warnf("检测到认证过期错误: %v", authErr)
			h.handleAuthenticationExpired(authErr, task)
			return err
		}

		h.handleTaskFailure(task, err)
		return err
	}

	if taskCtx.Task.CreateTime > 0 {
		processTime := time.Since(time.Unix(taskCtx.Task.CreateTime/1000, 0))
		utils.GetGlobalMetrics().RecordProcessTime(processTime)
	}

	// Pipeline包含完整的发品流程，成功后状态应为已上架而非已抓取
	h.updateTaskStatusToAPI(task.ID, model.TaskStatusPublished, "")
	utils.GetGlobalMetrics().IncrementCompleted()

	logrus.Infof("任务处理成功: ID=%s, TenantID=%d, StoreID=%d, ProductID=%s", taskCtx.Task.ID, taskCtx.Task.TenantID, taskCtx.Task.StoreID, taskCtx.Task.ProductID)
	return nil
}

// handleTaskFailure 处理任务失败
func (h *TaskHandler) handleTaskFailure(task modules.Task, err error) {
	isRetryable := modules.IsRetryableError(err)
	logrus.Infof("错误类型: %T, 错误值: %v, 是否可重试: %t", err, err, isRetryable)

	if !isRetryable {
		h.updateTaskStatusToAPI(task.ID, model.TaskStatusTerminated, err.Error())
		utils.GetGlobalMetrics().IncrementFailed()

		// 区分业务过滤和真正的错误
		if modules.IsFilteredError(err) {
			logrus.Infof("✓ 任务被筛选规则过滤: ID=%s, Priority=%d, 原因=%v", task.ID, task.Priority, err)
		} else {
			logrus.Errorf("任务处理失败且不可重试: ID=%s, Priority=%d, 错误=%v", task.ID, task.Priority, err)
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

	if task.RetryCount >= h.config.Processor.MaxRetries {
		h.updateTaskStatusToAPI(task.ID, model.TaskStatusTerminated, err.Error())
		utils.GetGlobalMetrics().IncrementFailed()
		logrus.Errorf("任务处理失败且达到最大重试次数: ID=%s, Priority=%d, 重试次数=%d, 错误=%v", task.ID, task.Priority, task.RetryCount, err)
	} else {
		h.updateTaskStatusToAPI(task.ID, model.TaskStatusPendingRetry, err.Error())
		logrus.Warnf("任务处理失败，等待重试: ID=%s, Priority=%d->%d, 重试次数=%d", task.ID, originalPriority, task.Priority, task.RetryCount)
		utils.GetGlobalMetrics().IncrementRequeued()
	}
}

// handleAuthenticationExpired 处理认证过期错误
func (h *TaskHandler) handleAuthenticationExpired(authErr *api.AuthenticationExpiredError, task modules.Task) {
	tenantID := authErr.TenantID
	shopID := authErr.ShopID

	if tenantID == 0 {
		tenantID = task.TenantID
	}
	if shopID == 0 {
		shopID = task.StoreID
	}

	logrus.Warnf("检测到店铺 %d:%d 认证过期，开始处理客户端清理和暂停", tenantID, shopID)

	// 检查是否需要删除Cookie（距离上次删除至少10分钟）
	if h.shouldDeleteCookie(shopID) {
		// 调用API删除过期的Cookie
		storeClient := h.managementClient.GetStoreClient()
		if storeClient != nil {
			logrus.Infof("正在删除店铺 %d 的过期Cookie...", shopID)
			success, err := storeClient.DeleteStoreCookie(shopID)
			if err != nil {
				logrus.Errorf("删除店铺 %d 的过期Cookie失败: %v", shopID, err)
			} else if success {
				logrus.Infof("成功删除店铺 %d 的过期Cookie", shopID)
				// 记录删除时间
				h.updateCookieDeletionTime(shopID)
			} else {
				logrus.Warnf("删除店铺 %d 的过期Cookie返回失败", shopID)
			}
		} else {
			logrus.Warn("StoreClient未初始化，无法删除过期Cookie")
		}
	} else {
		logrus.Infof("店铺 %d 距离上次删除Cookie不足10分钟，跳过删除操作", shopID)
	}

	if err := h.pauseShopWithCacheCleanup(tenantID, shopID, "认证过期(20302)", 10*time.Minute); err != nil {
		logrus.WithError(err).Warnf("暂停店铺并清理缓存失败: tenantID=%d, shopID=%d", tenantID, shopID)
	} else {
		logrus.Infof("已暂停店铺 %d:%d 执行任务10分钟并清理缓存，原因: 认证过期", tenantID, shopID)
	}

	// 标记任务为待重试状态，等待店铺恢复后自动重试
	h.updateTaskStatusToAPI(task.ID, model.TaskStatusPendingRetry, fmt.Sprintf("认证过期: %s", authErr.Message))

	logrus.Infof("认证过期任务已标记为待重试: TaskID=%s, TenantID=%d, ShopID=%d", task.ID, tenantID, shopID)
	utils.GetGlobalMetrics().IncrementRequeued()
}

// pauseShopWithCacheCleanup 暂停店铺并清理相关缓存
func (h *TaskHandler) pauseShopWithCacheCleanup(tenantID, shopID int64, reason string, duration time.Duration) error {
	// 清理客户端缓存
	h.shopClientMgr.RemoveClient(tenantID, shopID)
	logrus.Infof("已删除店铺 %d:%d 的客户端缓存", tenantID, shopID)

	// 暂停店铺（认证过期类型，内部会调用API设置暂停状态）
	h.memoryManager.ShopPauseManager.PauseShopForDuration(tenantID, shopID, reason, duration)

	return nil
}

// updateTaskStatusToAPI 更新任务状态到API（异步）
func (h *TaskHandler) updateTaskStatusToAPI(taskID string, status model.TaskStatus, errorMsg string) {
	h.updateTaskStatusToAPIWithMode(taskID, status, errorMsg, false)
}

// updateTaskStatusToAPIWithMode 更新任务状态到API（支持同步/异步模式）
func (h *TaskHandler) updateTaskStatusToAPIWithMode(taskID string, status model.TaskStatus, errorMsg string, sync bool) error {
	var id int64
	if _, err := fmt.Sscanf(taskID, "%d", &id); err != nil {
		logrus.Errorf("解析任务ID失败: %v", err)
		return fmt.Errorf("解析任务ID失败: %w", err)
	}

	importTaskClient := h.managementClient.GetImportTaskClient()
	if importTaskClient == nil {
		err := fmt.Errorf("导入任务客户端未初始化")
		logrus.Warn(err.Error())
		return err
	}

	req := &management_api.ProductImportTaskUpdateReqDTO{
		ID:           id,
		Status:       status.Int16(),
		ErrorMessage: errorMsg,
	}

	updateFunc := func() error {
		maxRetries := 5 // 增加重试次数
		var lastErr error

		for i := 0; i < maxRetries; i++ {
			if err := importTaskClient.UpdateTaskStatus(req); err != nil {
				lastErr = err
				if i < maxRetries-1 {
					retryDelay := time.Second * time.Duration(i+1)
					logrus.Warnf("更新任务状态到API失败 (TaskID: %s, Status: %s, 重试 %d/%d): %v, %v后重试",
						taskID, status.String(), i+1, maxRetries, err, retryDelay)
					time.Sleep(retryDelay)
					continue
				}
				logrus.Errorf("⚠️ 更新任务状态到API失败，已达最大重试次数 (TaskID: %s, Status: %s): %v",
					taskID, status.String(), err)
				return fmt.Errorf("更新任务状态失败: %w", err)
			} else {
				logrus.Infof("✅ 成功更新任务状态到API (TaskID: %s, Status: %s)", taskID, status.String())
				return nil
			}
		}
		return lastErr
	}

	if sync {
		// 同步模式：直接执行并返回结果
		return updateFunc()
	} else {
		// 异步模式：在新协程中执行
		go func() {
			if err := updateFunc(); err != nil {
				logrus.Errorf("异步更新任务状态最终失败 (TaskID: %s, Status: %s): %v", taskID, status.String(), err)
			}
		}()
		return nil
	}
}

// shouldDeleteCookie 检查是否应该删除Cookie（距离上次删除至少5分钟）
func (h *TaskHandler) shouldDeleteCookie(shopID int64) bool {
	h.cookieDeletionMutex.RLock()
	defer h.cookieDeletionMutex.RUnlock()

	lastTime, exists := h.lastCookieDeletionTime[shopID]
	if !exists {
		return true // 如果没有记录，允许删除
	}

	return time.Since(lastTime) >= 10*time.Minute
}

// updateCookieDeletionTime 更新Cookie删除时间
func (h *TaskHandler) updateCookieDeletionTime(shopID int64) {
	h.cookieDeletionMutex.Lock()
	defer h.cookieDeletionMutex.Unlock()

	h.lastCookieDeletionTime[shopID] = time.Now()
}
