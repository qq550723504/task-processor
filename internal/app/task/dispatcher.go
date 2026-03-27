// Package task 提供任务分发功能
package task

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/core/logger"
	"task-processor/internal/infra/clients/management/api"
)

// fetchAndDispatchTasks 获取并分发任务
func (f *TaskFetcher) fetchAndDispatchTasks(ctx context.Context) {
	// 检查管理客户端
	if f.managementClient == nil {
		logger.GetGlobalLogger("app/task").Debug("管理客户端为空，跳过任务获取")
		return
	}

	// 第1步：检查队列压力
	totalAvailableSlots, shouldSkipFetch := f.checkQueuePressure()
	if shouldSkipFetch {
		logger.GetGlobalLogger("app/task").Info("🛑 队列压力过大，本次跳过任务获取，等待队列消化")
		return
	}

	if totalAvailableSlots <= 0 {
		logger.GetGlobalLogger("app/task").Debug("所有平台队列已满，跳过任务获取")
		return
	}

	// 第2步：计算获取数量
	maxTasks := f.calculateFetchCount(totalAvailableSlots)

	// 第3步：获取任务
	apiTasks, err := f.fetchTasksFromAPI(maxTasks)
	if err != nil {
		logger.GetGlobalLogger("app/task").Errorf("获取任务失败: %v", err)
		return
	}

	if len(apiTasks) == 0 {
		logger.GetGlobalLogger("app/task").Debugf("没有待处理任务")
		return
	}

	logger.GetGlobalLogger("app/task").Infof("📥 获取到 %d 个待处理任务", len(apiTasks))

	// 第4步：分发任务
	f.dispatchTasks(ctx, apiTasks)
}

// checkQueuePressure 检查队列压力
func (f *TaskFetcher) checkQueuePressure() (totalAvailableSlots int, shouldSkipFetch bool) {
	for platform, submitter := range f.submitters {
		stats := submitter.GetQueueStats()
		totalAvailableSlots += stats.AvailableSlots

		logger.GetGlobalLogger("app/task").Debugf("[%s] 队列状态: %d/%d (%.1f%%), 可用: %d",
			platform, stats.QueueSize, stats.BufferSize,
			stats.UsagePercent, stats.AvailableSlots)

		// 检查队列使用率
		threshold := float64(f.config.Worker.QueueThreshold)
		if threshold <= 0 {
			threshold = 75 // 默认75%
		}

		if stats.UsagePercent > threshold {
			logger.GetGlobalLogger("app/task").Warnf("[%s] ⚠️ 队列使用率过高 (%.1f%% > %.1f%%)，暂停获取",
				platform, stats.UsagePercent, threshold)
			shouldSkipFetch = true
		}
	}
	return
}

// calculateFetchCount 计算获取数量（保守策略）
func (f *TaskFetcher) calculateFetchCount(totalAvailableSlots int) int {
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

	logger.GetGlobalLogger("app/task").Debugf("📊 可用槽位: %d, 本次获取: %d (策略: 50%%, 上限: %d)",
		totalAvailableSlots, maxTasks, maxFetchPerCycle)

	return maxTasks
}

// dispatchTasks 分发任务
func (f *TaskFetcher) dispatchTasks(ctx context.Context, apiTasks []api.ProductImportTaskRespDTO) {
	platformCounts := make(map[string]int)
	errorCount := 0
	queueFullCount := 0
	claimService := NewTaskClaimService(f)
	taskDispatcher := NewTaskDispatcher(f)

	// 使用适配器包装管理客户端
	storeClient := f.managementClient.GetStoreClient()
	dispatchGuard := NewTaskDispatchGuard(f, storeClient)

	for _, apiTask := range apiTasks {
		// 直接使用任务信息，无需类型断言
		apiTaskPtr := f.extractAPITask(apiTask)

		taskID := fmt.Sprintf("%d", apiTaskPtr.ID)
		logger.GetGlobalLogger("app/task").Debugf("🔍 处理任务: TaskID=%s, StoreID=%d, ProductID=%s",
			taskID, apiTaskPtr.StoreID, apiTaskPtr.ProductID)

		// 🚀 关键优化：获取到任务后立即标记为处理中，防止重复获取
		if _, ok := claimService.Claim(apiTaskPtr); !ok {
			continue
		}

		// 获取店铺信息并检查是否允许分发
		storeInfo, isPaused, err := dispatchGuard.Check(apiTaskPtr)
		if err != nil {
			f.rollbackProcessingStatus(taskID)
			errorCount++
			continue
		}

		if isPaused {
			f.rollbackProcessingStatus(taskID)
			continue
		}

		// 转换为内部任务格式并提交
		success, isQueueFull := taskDispatcher.Dispatch(ctx, apiTaskPtr, storeInfo)
		if success {
			platform := strings.ToLower(storeInfo.Platform)
			platformCounts[platform]++
			logger.GetGlobalLogger("app/task").Debugf("✅ 任务已成功提交并标记为处理中: TaskID=%s", taskID)
		} else if isQueueFull {
			// 队列满时，回滚处理中状态，让任务可以在下次获取时重试
			f.rollbackProcessingStatus(taskID)
			queueFullCount++
		} else {
			// 提交失败时，回滚处理中状态
			f.rollbackProcessingStatus(taskID)
			errorCount++
		}
	}

	// 输出统计
	logger.GetGlobalLogger("app/task").Infof("✅ 任务分发完成: 成功=%v, 队列满=%d, 错误=%d",
		platformCounts, queueFullCount, errorCount)

	if queueFullCount > 0 {
		logger.GetGlobalLogger("app/task").Warnf("⚠️ 有 %d 个任务因队列满未提交，将在下次获取时重试", queueFullCount)
	}
}
