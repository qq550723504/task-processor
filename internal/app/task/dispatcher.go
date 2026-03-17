// Package task 提供任务分发功能
package task

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"task-processor/internal/infra/clients/management/api"
	types "task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

// fetchAndDispatchTasks 获取并分发任务
func (f *TaskFetcher) fetchAndDispatchTasks(ctx context.Context) {
	// 检查管理客户端
	if f.managementClient == nil {
		logrus.Debug("管理客户端为空，跳过任务获取")
		return
	}

	// 第1步：检查队列压力
	totalAvailableSlots, shouldSkipFetch := f.checkQueuePressure()
	if shouldSkipFetch {
		logrus.Info("🛑 队列压力过大，本次跳过任务获取，等待队列消化")
		return
	}

	if totalAvailableSlots <= 0 {
		logrus.Debug("所有平台队列已满，跳过任务获取")
		return
	}

	// 第2步：计算获取数量
	maxTasks := f.calculateFetchCount(totalAvailableSlots)

	// 第3步：获取任务
	apiTasks, err := f.fetchTasksFromAPI(maxTasks)
	if err != nil {
		logrus.Errorf("获取任务失败: %v", err)
		return
	}

	if len(apiTasks) == 0 {
		logrus.Debugf("没有待处理任务")
		return
	}

	logrus.Infof("📥 获取到 %d 个待处理任务", len(apiTasks))

	// 第4步：分发任务
	f.dispatchTasks(ctx, apiTasks)
}

// checkQueuePressure 检查队列压力
func (f *TaskFetcher) checkQueuePressure() (totalAvailableSlots int, shouldSkipFetch bool) {
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

	logrus.Debugf("📊 可用槽位: %d, 本次获取: %d (策略: 50%%, 上限: %d)",
		totalAvailableSlots, maxTasks, maxFetchPerCycle)

	return maxTasks
}

// dispatchTasks 分发任务
func (f *TaskFetcher) dispatchTasks(ctx context.Context, apiTasks []api.ProductImportTaskRespDTO) {
	platformCounts := make(map[string]int)
	errorCount := 0
	queueFullCount := 0

	// 使用适配器包装管理客户端
	storeClient := f.managementClient.GetStoreClient()

	for _, apiTask := range apiTasks {
		// 直接使用任务信息，无需类型断言
		apiTaskPtr := f.extractAPITask(apiTask)

		taskID := fmt.Sprintf("%d", apiTaskPtr.ID)
		logrus.Debugf("🔍 处理任务: TaskID=%s, StoreID=%d, ProductID=%s",
			taskID, apiTaskPtr.StoreID, apiTaskPtr.ProductID)

		// 去重检查：跳过已在处理中的任务
		if f.isTaskProcessing(taskID) {
			continue
		}

		// 🚀 关键优化：获取到任务后立即标记为处理中，防止重复获取
		f.markTaskAsProcessingImmediately(taskID, apiTaskPtr.ID)

		// 获取店铺信息判断平台
		storeInfo, err := f.getStoreInfo(apiTaskPtr.StoreID, storeClient)
		if err != nil {
			// 如果获取店铺信息失败，需要回滚处理中状态
			f.rollbackProcessingStatus(taskID)
			errorCount++
			continue
		}

		// 检查店铺是否被暂停
		isPaused, err := storeClient.GetStorePauseStatus(apiTaskPtr.StoreID)
		if err != nil {
			logrus.Warnf("获取店铺 %d 暂停状态失败: %v，跳过任务", apiTaskPtr.StoreID, err)
			f.rollbackProcessingStatus(taskID)
			errorCount++
			continue
		}

		if isPaused {
			logrus.Infof("🛑 店铺 %d 已被暂停，跳过任务: TaskID=%s, ProductID=%s",
				apiTaskPtr.StoreID, taskID, apiTaskPtr.ProductID)
			f.rollbackProcessingStatus(taskID)
			continue
		}

		// 转换为内部任务格式并提交
		success, isQueueFull := f.submitTask(ctx, apiTaskPtr, storeInfo)
		if success {
			platform := strings.ToLower(storeInfo.Platform)
			platformCounts[platform]++
			logrus.Debugf("✅ 任务已成功提交并标记为处理中: TaskID=%s", taskID)
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
	logrus.Infof("✅ 任务分发完成: 成功=%v, 队列满=%d, 错误=%d",
		platformCounts, queueFullCount, errorCount)

	if queueFullCount > 0 {
		logrus.Warnf("⚠️ 有 %d 个任务因队列满未提交，将在下次获取时重试", queueFullCount)
	}
}

// submitTask 提交单个任务
func (f *TaskFetcher) submitTask(ctx context.Context, apiTask *api.ProductImportTaskRespDTO, storeInfo *api.StoreRespDTO) (success bool, isQueueFull bool) {
	// 转换为内部任务格式
	internalTask := types.Task{
		ID:         apiTask.ID,
		TenantID:   apiTask.TenantID,
		ProductID:  apiTask.ProductID,
		Platform:   apiTask.Platform,
		Region:     apiTask.Region,
		StoreID:    apiTask.StoreID,
		CategoryID: apiTask.CategoryID,
		CreateTime: apiTask.CreateTime / 1000, // 转换毫秒时间戳为秒
		RetryCount: apiTask.RetryCount,
		Priority:   apiTask.Priority,
		Creator:    apiTask.Creator,
	}

	taskData, err := json.Marshal(internalTask)
	if err != nil {
		logrus.Errorf("序列化任务失败: %v", err)
		return false, false
	}

	// 根据店铺平台分发任务
	platform := strings.ToLower(storeInfo.Platform)
	submitter, exists := f.submitters[platform]
	if !exists {
		logrus.Errorf("❌ 未找到平台处理器: TaskID=%d, StoreID=%d, Platform=%s, 可用平台=%v",
			internalTask.ID, apiTask.StoreID, platform, f.getSubmitterKeys())
		return false, false
	}

	// 提交任务
	if err := submitter.SubmitTask(ctx, string(taskData)); err != nil {
		// 检查是否是队列满的错误
		if err.Error() == "工作队列已满" || err.Error() == "工作池已满" {
			logrus.Debugf("[%s] 队列已满，任务将重试: TaskID=%d", platform, internalTask.ID)
			return false, true
		} else {
			logrus.Errorf("[%s] 提交失败: TaskID=%d, Error=%v", platform, internalTask.ID, err)
			return false, false
		}
	}

	logrus.Debugf("[%s] 任务已提交: ID=%d, ProductID=%s", platform, internalTask.ID, internalTask.ProductID)
	return true, false
}
