// Package task provides task dispatch orchestration.
package task

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/core/logger"
	"task-processor/internal/infra/clients/management/api"
)

// fetchAndDispatchTasks pulls tasks from management and dispatches them to platform workers.
func (f *TaskFetcher) fetchAndDispatchTasks(ctx context.Context) {
	if f.managementClient == nil {
		logger.GetGlobalLogger("app/task").Debug("管理客户端为空，跳过任务获取")
		return
	}

	totalAvailableSlots, shouldSkipFetch := f.checkQueuePressure()
	if shouldSkipFetch {
		logger.GetGlobalLogger("app/task").Info("队列压力过大，本轮跳过任务获取")
		return
	}

	if totalAvailableSlots <= 0 {
		logger.GetGlobalLogger("app/task").Debug("所有平台队列已满，跳过任务获取")
		return
	}

	maxTasks := f.calculateFetchCount(totalAvailableSlots)
	apiTasks, err := f.fetchTasksFromAPI(maxTasks)
	if err != nil {
		logger.GetGlobalLogger("app/task").Errorf("获取任务失败: %v", err)
		return
	}

	if len(apiTasks) == 0 {
		logger.GetGlobalLogger("app/task").Debug("没有待处理任务")
		return
	}

	logger.GetGlobalLogger("app/task").Infof("获取到 %d 个待处理任务", len(apiTasks))
	f.dispatchTasks(ctx, apiTasks)
}

// checkQueuePressure inspects submitter queue pressure and available capacity.
func (f *TaskFetcher) checkQueuePressure() (totalAvailableSlots int, shouldSkipFetch bool) {
	for platform, submitter := range f.submitters {
		stats := submitter.GetQueueStats()
		totalAvailableSlots += stats.AvailableSlots

		logger.GetGlobalLogger("app/task").Debugf("[%s] 队列状态: %d/%d (%.1f%%), 可用: %d",
			platform, stats.QueueSize, stats.BufferSize, stats.UsagePercent, stats.AvailableSlots)

		threshold := float64(f.config.Worker.QueueThreshold)
		if threshold <= 0 {
			threshold = 75
		}

		if stats.UsagePercent > threshold {
			logger.GetGlobalLogger("app/task").Warnf("[%s] 队列使用率过高(%.1f%% > %.1f%%)，暂停获取",
				platform, stats.UsagePercent, threshold)
			shouldSkipFetch = true
		}
	}
	return
}

// calculateFetchCount computes the task batch size using a conservative strategy.
func (f *TaskFetcher) calculateFetchCount(totalAvailableSlots int) int {
	maxTasks := totalAvailableSlots / 2

	maxFetchPerCycle := f.config.Worker.MaxFetchPerCycle
	if maxFetchPerCycle <= 0 {
		maxFetchPerCycle = 5
	}
	if maxTasks > maxFetchPerCycle {
		maxTasks = maxFetchPerCycle
	}

	if maxTasks < 1 {
		maxTasks = 1
	}

	logger.GetGlobalLogger("app/task").Debugf(
		"可用槽位: %d, 本次获取: %d (策略: 50%%, 上限: %d)",
		totalAvailableSlots,
		maxTasks,
		maxFetchPerCycle,
	)
	return maxTasks
}

// dispatchTasks orchestrates claim, guard, and submit for fetched tasks.
func (f *TaskFetcher) dispatchTasks(ctx context.Context, apiTasks []api.ProductImportTaskRespDTO) {
	platformCounts := make(map[string]int)
	errorCount := 0
	queueFullCount := 0
	claimService := NewTaskClaimService(f)
	taskDispatcher := NewTaskDispatcher(f)

	storeClient := f.managementClient.GetStoreClient()
	dispatchGuard := NewTaskDispatchGuard(f, storeClient)

	for _, apiTask := range apiTasks {
		apiTaskPtr := f.extractAPITask(apiTask)
		taskID := fmt.Sprintf("%d", apiTaskPtr.ID)

		logger.GetGlobalLogger("app/task").Debugf(
			"处理任务: TaskID=%s, StoreID=%d, ProductID=%s",
			taskID,
			apiTaskPtr.StoreID,
			apiTaskPtr.ProductID,
		)

		if _, ok := claimService.Claim(apiTaskPtr); !ok {
			continue
		}

		storeInfo, isPaused, err := dispatchGuard.Check(apiTaskPtr)
		if err != nil {
			f.rollbackClaimState(taskID, apiTaskPtr, "dispatch guard check failed")
			errorCount++
			continue
		}

		if isPaused {
			f.rollbackClaimState(taskID, apiTaskPtr, "store paused before dispatch")
			continue
		}

		success, isQueueFull := taskDispatcher.Dispatch(ctx, apiTaskPtr, storeInfo)
		if success {
			platform := strings.ToLower(storeInfo.Platform)
			platformCounts[platform]++
			logger.GetGlobalLogger("app/task").Debugf("任务已成功提交并保持 processing: TaskID=%s", taskID)
			continue
		}

		if isQueueFull {
			f.rollbackClaimState(taskID, apiTaskPtr, "submitter queue full")
			queueFullCount++
			continue
		}

		f.rollbackClaimState(taskID, apiTaskPtr, "submitter dispatch failed")
		errorCount++
	}

	logger.GetGlobalLogger("app/task").Infof(
		"任务分发完成: 成功=%v, 队列满=%d, 错误=%d",
		platformCounts,
		queueFullCount,
		errorCount,
	)

	if queueFullCount > 0 {
		logger.GetGlobalLogger("app/task").Warnf("%d 个任务因队列满未提交，将在下次获取时重试", queueFullCount)
	}
}
