package task

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"task-processor/common"
	"task-processor/common/config"
	"task-processor/common/types"

	"github.com/sirupsen/logrus"
)

// UnifiedTaskFetcher 统一任务获取器
type UnifiedTaskFetcher struct {
	config           *config.Config
	managementClient ManagementClientProvider
	submitters       map[string]TaskSubmitter // platform -> submitter
	interval         time.Duration
	processingTasks  map[string]time.Time // taskID -> 提交时间，用于去重
	tasksMutex       sync.RWMutex         // 保护 processingTasks
}

// NewUnifiedTaskFetcher 创建统一任务获取器
func NewUnifiedTaskFetcher(
	cfg *config.Config,
	managementClient ManagementClientProvider,
	submitters map[string]TaskSubmitter,
) *UnifiedTaskFetcher {
	interval := time.Duration(cfg.Worker.TaskInterval) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}

	fetcher := &UnifiedTaskFetcher{
		config:           cfg,
		managementClient: managementClient,
		submitters:       submitters,
		interval:         interval,
		processingTasks:  make(map[string]time.Time),
	}

	// 启动定期清理过期任务的协程（每5分钟清理一次超过30分钟的任务）
	go fetcher.cleanupExpiredTasks()

	return fetcher
}

// Start 启动任务获取
func (f *UnifiedTaskFetcher) Start(ctx context.Context) {
	logrus.Infof("统一任务获取器启动，间隔: %v", f.interval)

	// 启动时立即执行一次任务获取
	logrus.Info("🚀 启动时立即执行首次任务获取")
	f.fetchAndDispatchTasks()

	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logrus.Info("统一任务获取器停止")
			return
		case <-ticker.C:
			f.fetchAndDispatchTasks()
		}
	}
}

// fetchAndDispatchTasks 获取并分发任务
func (f *UnifiedTaskFetcher) fetchAndDispatchTasks() {
	// 检查管理客户端
	if f.managementClient == nil {
		logrus.Debug("管理客户端为空，跳过任务获取")
		return
	}

	// ========== 第1步：检查队列压力 ==========
	totalAvailableSlots := 0
	shouldSkipFetch := false

	for platform, submitter := range f.submitters {
		stats := submitter.GetQueueStats()
		totalAvailableSlots += stats.AvailableSlots

		logrus.Debugf("[%s] 队列状态: %d/%d (%.1f%%), 可用: %d",
			platform, stats.QueueSize, stats.BufferSize,
			stats.UsagePercent, stats.AvailableSlots)

		// 检查队列使用率
		threshold := float64(f.config.Worker.QueueThreshold)
		if threshold <= 0 {
			threshold = 75 // 默认75%
		}

		if stats.UsagePercent > threshold {
			logrus.Warnf("[%s] ⚠️ 队列使用率过高 (%.1f%% > %.1f%%)，暂停获取",
				platform, stats.UsagePercent, threshold)
			shouldSkipFetch = true
		}
	}

	// ========== 第2步：决定是否跳过本次获取 ==========
	if shouldSkipFetch {
		logrus.Info("🛑 队列压力过大，本次跳过任务获取，等待队列消化")
		return
	}

	if totalAvailableSlots <= 0 {
		logrus.Debug("所有平台队列已满，跳过任务获取")
		return
	}

	// ========== 第3步：计算获取数量（保守策略）==========
	// 策略1：只获取可用槽位的50%
	maxTasks := totalAvailableSlots / 2

	// 策略2：应用配置的上限
	maxFetchPerCycle := f.config.Worker.MaxFetchPerCycle
	if maxFetchPerCycle <= 0 {
		maxFetchPerCycle = 5 // 默认5个
	}
	if maxTasks > maxFetchPerCycle {
		maxTasks = maxFetchPerCycle
	}

	// 策略3：至少获取1个
	if maxTasks < 1 {
		maxTasks = 1
	}

	logrus.Debugf("📊 可用槽位: %d, 本次获取: %d (策略: 50%%, 上限: %d)",
		totalAvailableSlots, maxTasks, maxFetchPerCycle)

	// 获取任务
	importTaskClient := f.managementClient.GetImportTaskClient()
	apiTasks, err := importTaskClient.GetPendingAndRetryTasks(
		maxTasks,
		f.config.Management.UserID,
		f.config.Management.StoreIDs,
	)
	if err != nil {
		logrus.Errorf("获取任务失败: %v", err)
		return
	}

	if len(apiTasks) == 0 {
		logrus.Debugf("没有待处理任务")
		return
	}

	logrus.Infof("📥 获取到 %d 个待处理任务", len(apiTasks))

	// ========== 第5步：分发任务 ==========
	platformCounts := make(map[string]int)
	errorCount := 0
	queueFullCount := 0
	storeClient := f.managementClient.GetStoreClient()

	for _, apiTask := range apiTasks {
		taskID := fmt.Sprintf("%d", apiTask.ID)
		logrus.Debugf("🔍 处理任务: TaskID=%s, StoreID=%d, ProductID=%s", taskID, apiTask.StoreID, apiTask.ProductID)

		// ========== 去重检查：跳过已在处理中的任务 ==========
		f.tasksMutex.RLock()
		submitTime, isProcessing := f.processingTasks[taskID]
		f.tasksMutex.RUnlock()

		if isProcessing {
			logrus.Debugf("⏭️ 跳过重复任务: TaskID=%s (已在队列中，提交时间: %v)", taskID, submitTime)
			continue
		}

		// 获取店铺信息判断平台
		logrus.Debugf("📞 正在获取店铺信息: StoreID=%d", apiTask.StoreID)
		storeInfo, err := storeClient.GetStore(apiTask.StoreID)
		if err != nil {
			logrus.Errorf("❌ 获取店铺信息失败: StoreID=%d, Error=%v", apiTask.StoreID, err)
			errorCount++
			continue
		}
		logrus.Debugf("✅ 获取店铺信息成功: StoreID=%d, Platform=%s, Name=%s", apiTask.StoreID, storeInfo.Platform, storeInfo.Name)

		// 转换为内部任务格式
		internalTask := types.Task{
			ID:         taskID,
			TenantID:   apiTask.TenantID,
			ProductID:  apiTask.ProductID,
			Platform:   apiTask.Platform,
			Region:     apiTask.Region,
			StoreID:    apiTask.StoreID,
			CategoryID: apiTask.CategoryID,
			CreateTime: apiTask.CreateTime,
			RetryCount: apiTask.RetryCount,
			Priority:   apiTask.Priority,
			Creator:    apiTask.Creator,
		}

		taskData, err := json.Marshal(internalTask)
		if err != nil {
			logrus.Errorf("序列化任务失败: %v", err)
			errorCount++
			continue
		}

		// 根据店铺平台分发任务
		platform := storeInfo.Platform
		logrus.Debugf("🔎 查找平台处理器: Platform=%s", platform)
		submitter, exists := f.submitters[platform]
		if !exists {
			logrus.Errorf("❌ 未找到平台处理器: TaskID=%s, StoreID=%d, Platform=%s, 可用平台=%v",
				internalTask.ID, apiTask.StoreID, platform, getMapKeys(f.submitters))
			errorCount++
			continue
		}

		// 提交任务
		if err := submitter.SubmitTask(string(taskData)); err != nil {
			// 如果是队列满的错误，记录但不计入错误（任务会在下次获取时重试）
			if err.Error() == "工作队列已满" || err.Error() == "工作池已满" {
				logrus.Debugf("[%s] 队列已满，任务将重试: TaskID=%s", platform, internalTask.ID)
				queueFullCount++
			} else {
				logrus.Errorf("[%s] 提交失败: TaskID=%s, Error=%v", platform, internalTask.ID, err)
				errorCount++
			}
		} else {
			// ========== 提交成功后，立即更新任务状态为"处理中" ==========
			f.tasksMutex.Lock()
			f.processingTasks[internalTask.ID] = time.Now()
			f.tasksMutex.Unlock()

			// 立即更新API端任务状态，防止重复获取
			f.updateTaskStatusToProcessing(apiTask.ID)

			platformCounts[platform]++
			logrus.Debugf("[%s] 任务已提交: ID=%s, ProductID=%s", platform, internalTask.ID, internalTask.ProductID)
		}
	}

	// ========== 第6步：输出统计 ==========
	logrus.Infof("✅ 任务分发完成: 成功=%v, 队列满=%d, 错误=%d",
		platformCounts, queueFullCount, errorCount)

	if queueFullCount > 0 {
		logrus.Warnf("⚠️ 有 %d 个任务因队列满未提交，将在下次获取时重试", queueFullCount)
	}
}

// cleanupExpiredTasks 定期清理过期的任务记录
func (f *UnifiedTaskFetcher) cleanupExpiredTasks() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		f.tasksMutex.Lock()
		now := time.Now()
		expiredCount := 0

		// 清理超过30分钟的任务记录（任务处理超时应该在15分钟内）
		for taskID, submitTime := range f.processingTasks {
			if now.Sub(submitTime) > 30*time.Minute {
				delete(f.processingTasks, taskID)
				expiredCount++
			}
		}

		totalTasks := len(f.processingTasks)
		f.tasksMutex.Unlock()

		if expiredCount > 0 {
			logrus.Infof("🧹 清理过期任务记录: 清理=%d, 剩余=%d", expiredCount, totalTasks)
		}
	}
}

// RemoveProcessingTask 从处理中任务列表移除（任务完成或失败时调用）
func (f *UnifiedTaskFetcher) RemoveProcessingTask(taskID string) {
	f.tasksMutex.Lock()
	delete(f.processingTasks, taskID)
	f.tasksMutex.Unlock()
	logrus.Infof("✅ 任务已从处理队列移除: TaskID=%s", taskID)
}

// OnTaskCompleted 实现 TaskCompletionNotifier 接口
func (f *UnifiedTaskFetcher) OnTaskCompleted(taskID string) {
	f.RemoveProcessingTask(taskID)
}

// getMapKeys 获取map的所有key（用于日志输出）
func getMapKeys(m map[string]TaskSubmitter) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// updateTaskStatusToProcessing 更新任务状态为"处理中"
func (f *UnifiedTaskFetcher) updateTaskStatusToProcessing(taskID int64) {
	// 异步更新，避免阻塞任务分发
	go func() {
		importTaskClient := f.managementClient.GetImportTaskClient()
		if importTaskClient == nil {
			logrus.Error("导入任务客户端未初始化，无法更新任务状态")
			return
		}

		// 更新状态为"处理中"（状态码1）
		if err := importTaskClient.UpdateTaskStatus(taskID, common.TaskStatusProcessing.Int16(), ""); err != nil {
			logrus.Warnf("更新任务状态为处理中失败: TaskID=%d, Error=%v", taskID, err)
		} else {
			logrus.Debugf("✅ 任务状态已更新为处理中: TaskID=%d", taskID)
		}
	}()
}
