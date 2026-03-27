package task

import (
	"fmt"

	"task-processor/internal/core/logger"
	"task-processor/internal/infra/clients/management/api"
)

type storePauseStatusReader interface {
	GetStorePauseStatus(storeID int64) (bool, error)
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
		logger.GetGlobalLogger("app/task").Infof("🛑 店铺 %d 已被暂停，跳过任务: TaskID=%d, ProductID=%s",
			apiTask.StoreID, apiTask.ID, apiTask.ProductID)
		return storeInfo, true, nil
	}

	return storeInfo, false, nil
}
