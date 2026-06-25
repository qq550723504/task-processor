package task

import (
	"fmt"

	"task-processor/internal/core/logger"
)

type TaskSource struct {
	fetcher *TaskFetcher
}

func NewTaskSource(fetcher *TaskFetcher) *TaskSource {
	return &TaskSource{fetcher: fetcher}
}

func (s *TaskSource) FetchPendingTasks(maxTasks int) ([]ImportTaskRecord, error) {
	if s == nil || s.fetcher == nil {
		return nil, fmt.Errorf("task source is not initialized")
	}
	source := s.fetcher.pendingRuntimeTaskSource()
	if source == nil {
		return nil, fmt.Errorf("pending task source is not initialized")
	}

	logger.GetGlobalLogger("app/task").Warn("using deprecated legacy polling task source; migrate this workload to RabbitMQ platform consumers")
	logger.GetGlobalLogger("app/task").Infof("🔍 任务获取参数: maxTasks=%d, userID=%d, storeIDs=%v",
		maxTasks, s.fetcher.config.Management.UserID, s.fetcher.config.Management.StoreIDs)

	apiTasks, err := source.GetPendingRuntimeTasks(
		maxTasks,
		s.fetcher.config.Management.UserID,
		s.fetcher.config.Management.StoreIDs,
	)
	if err != nil {
		return nil, err
	}
	items := make([]ImportTaskRecord, 0, len(apiTasks))
	for _, task := range apiTasks {
		items = append(items, toImportTaskRecord(task))
	}
	return items, nil
}
