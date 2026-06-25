package task

import (
	"fmt"

	"task-processor/internal/core/logger"
	"task-processor/internal/pkg/timex"
)

type TaskDispatchGuard struct {
	fetcher     *TaskFetcher
	storeClient storeDispatchRuntime
}

func NewTaskDispatchGuard(fetcher *TaskFetcher, storeClient storeDispatchRuntime) *TaskDispatchGuard {
	return &TaskDispatchGuard{
		fetcher:     fetcher,
		storeClient: storeClient,
	}
}

func (g *TaskDispatchGuard) Check(task *ImportTaskRecord) (*StoreInfo, bool, error) {
	if g == nil || g.fetcher == nil || task == nil {
		return nil, false, fmt.Errorf("task dispatch guard is not initialized")
	}

	storeInfo, err := g.fetcher.getStoreInfo(task.StoreID, g.storeClient)
	if err != nil {
		return nil, false, err
	}

	if g.storeClient == nil {
		return nil, false, fmt.Errorf("store dispatch runtime is not initialized")
	}

	isPaused, err := g.storeClient.GetStorePauseStatus(task.StoreID)
	if err != nil {
		logger.GetGlobalLogger("app/task").Warnf("获取店铺 %d 暂停状态失败: %v，跳过任务", task.StoreID, err)
		return nil, false, err
	}

	if isPaused {
		if g.tryResumeStaleQuotaPause(task, storeInfo) {
			return storeInfo, false, nil
		}

		logger.GetGlobalLogger("app/task").Infof("🛑 店铺 %d 已被暂停，跳过任务: TaskID=%d, ProductID=%s",
			task.StoreID, task.ID, task.ProductID)
		return storeInfo, true, nil
	}

	return storeInfo, false, nil
}

func (g *TaskDispatchGuard) tryResumeStaleQuotaPause(task *ImportTaskRecord, storeInfo *StoreInfo) bool {
	if g == nil || g.fetcher == nil || task == nil || storeInfo == nil {
		return false
	}
	if storeInfo.DailyLimit == nil || *storeInfo.DailyLimit <= 0 {
		return false
	}

	pauseDetail, err := g.storeClient.GetStorePauseStatusDetail(task.StoreID)
	if err != nil {
		logger.GetGlobalLogger("app/task").Warnf("获取店铺 %d 暂停详情失败，保留暂停状态: %v", task.StoreID, err)
		return false
	}
	if !isQuotaPause(pauseDetail) {
		return false
	}

	countReader := g.fetcher.dailyListingCountReader()
	if countReader == nil {
		return false
	}

	currentDate := timex.NowDate()
	dailyCount, err := countReader.GetDailyListingCount(task.TenantID, task.StoreID, task.TenantID, currentDate)
	if err != nil {
		logger.GetGlobalLogger("app/task").Warnf("获取店铺 %d 当日上品计数失败，保留暂停状态: %v", task.StoreID, err)
		return false
	}
	if dailyCount == nil {
		logger.GetGlobalLogger("app/task").Warnf("获取店铺 %d 当日上品计数为空，保留暂停状态", task.StoreID)
		return false
	}
	if dailyCount.Count >= int64(*storeInfo.DailyLimit) {
		return false
	}

	success, err := g.storeClient.SetStorePauseStatus(task.StoreID, false, "")
	if err != nil {
		logger.GetGlobalLogger("app/task").Warnf("恢复店铺 %d 配额暂停失败，保留暂停状态: %v", task.StoreID, err)
		return false
	}
	if !success {
		logger.GetGlobalLogger("app/task").Warnf("恢复店铺 %d 配额暂停返回失败，保留暂停状态", task.StoreID)
		return false
	}

	logger.GetGlobalLogger("app/task").Infof(
		"店铺 %d 检测到过期 quota_limit 暂停，已自动恢复并继续派发: count=%d, limit=%d, date=%s",
		task.StoreID,
		dailyCount.Count,
		*storeInfo.DailyLimit,
		currentDate,
	)
	return true
}

func isQuotaPause(pauseDetail *StorePauseStatusDetail) bool {
	if pauseDetail == nil || !pauseDetail.Paused {
		return false
	}
	return pauseDetail.PauseType == "quota_limit" || pauseDetail.Reason == "quota_limit"
}
