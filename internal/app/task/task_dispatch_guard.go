package task

import (
	"fmt"

	"task-processor/internal/core/logger"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/pkg/timex"
)

type storePauseStatusReader interface {
	GetStorePauseStatus(storeID int64) (bool, error)
}

type storePauseStatusDetailReader interface {
	GetStorePauseStatusDetail(storeID int64) (*api.StorePauseStatusRespDTO, error)
}

type storePauseStatusWriter interface {
	SetStorePauseStatus(storeID int64, pause bool, pauseType string) (bool, error)
}

type TaskDispatchGuard struct {
	fetcher     *TaskFetcher
	storeClient any
}

func NewTaskDispatchGuard(fetcher *TaskFetcher, storeClient any) *TaskDispatchGuard {
	return &TaskDispatchGuard{
		fetcher:     fetcher,
		storeClient: storeClient,
	}
}

func (g *TaskDispatchGuard) Check(apiTask *api.ProductImportTaskRespDTO) (*api.StoreRespDTO, bool, error) {
	if g == nil || g.fetcher == nil || apiTask == nil {
		return nil, false, fmt.Errorf("task dispatch guard is not initialized")
	}

	storeInfo, err := g.fetcher.getStoreInfo(apiTask.StoreID, g.storeClient)
	if err != nil {
		return nil, false, err
	}

	pauseReader, ok := g.storeClient.(storePauseStatusReader)
	if !ok {
		return nil, false, fmt.Errorf("store client does not support pause status check: %T", g.storeClient)
	}

	isPaused, err := pauseReader.GetStorePauseStatus(apiTask.StoreID)
	if err != nil {
		logger.GetGlobalLogger("app/task").Warnf("获取店铺 %d 暂停状态失败: %v，跳过任务", apiTask.StoreID, err)
		return nil, false, err
	}

	if isPaused {
		if g.tryResumeStaleQuotaPause(apiTask, storeInfo) {
			return storeInfo, false, nil
		}

		logger.GetGlobalLogger("app/task").Infof("🛑 店铺 %d 已被暂停，跳过任务: TaskID=%d, ProductID=%s",
			apiTask.StoreID, apiTask.ID, apiTask.ProductID)
		return storeInfo, true, nil
	}

	return storeInfo, false, nil
}

func (g *TaskDispatchGuard) tryResumeStaleQuotaPause(apiTask *api.ProductImportTaskRespDTO, storeInfo *api.StoreRespDTO) bool {
	if g == nil || g.fetcher == nil || g.fetcher.managementClient == nil || apiTask == nil || storeInfo == nil {
		return false
	}
	if storeInfo.DailyLimit == nil || *storeInfo.DailyLimit <= 0 {
		return false
	}

	pauseReader, ok := g.storeClient.(storePauseStatusDetailReader)
	if !ok {
		return false
	}
	pauseWriter, ok := g.storeClient.(storePauseStatusWriter)
	if !ok {
		return false
	}

	pauseDetail, err := pauseReader.GetStorePauseStatusDetail(apiTask.StoreID)
	if err != nil {
		logger.GetGlobalLogger("app/task").Warnf("获取店铺 %d 暂停详情失败，保留暂停状态: %v", apiTask.StoreID, err)
		return false
	}
	if !isQuotaPause(pauseDetail) {
		return false
	}

	countClient := g.fetcher.managementClient.GetDailyListingCountClient()
	if countClient == nil {
		return false
	}

	currentDate := timex.NowDate()
	dailyCount, err := countClient.GetDailyListingCount(apiTask.TenantID, apiTask.StoreID, apiTask.TenantID, currentDate)
	if err != nil {
		logger.GetGlobalLogger("app/task").Warnf("获取店铺 %d 当日上品计数失败，保留暂停状态: %v", apiTask.StoreID, err)
		return false
	}
	if dailyCount == nil {
		logger.GetGlobalLogger("app/task").Warnf("获取店铺 %d 当日上品计数为空，保留暂停状态", apiTask.StoreID)
		return false
	}
	if dailyCount.Count >= int64(*storeInfo.DailyLimit) {
		return false
	}

	success, err := pauseWriter.SetStorePauseStatus(apiTask.StoreID, false, "")
	if err != nil {
		logger.GetGlobalLogger("app/task").Warnf("恢复店铺 %d 配额暂停失败，保留暂停状态: %v", apiTask.StoreID, err)
		return false
	}
	if !success {
		logger.GetGlobalLogger("app/task").Warnf("恢复店铺 %d 配额暂停返回失败，保留暂停状态", apiTask.StoreID)
		return false
	}

	logger.GetGlobalLogger("app/task").Infof(
		"店铺 %d 检测到过期 quota_limit 暂停，已自动恢复并继续派发: count=%d, limit=%d, date=%s",
		apiTask.StoreID,
		dailyCount.Count,
		*storeInfo.DailyLimit,
		currentDate,
	)
	return true
}

func isQuotaPause(pauseDetail *api.StorePauseStatusRespDTO) bool {
	if pauseDetail == nil || !pauseDetail.Paused {
		return false
	}
	return pauseDetail.PauseType == "quota_limit" || pauseDetail.Reason == "quota_limit"
}
