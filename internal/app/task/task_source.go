package task

import (
	"fmt"

	"task-processor/internal/core/logger"
	"task-processor/internal/infra/clients/management/api"
)

type TaskSource struct {
	fetcher *TaskFetcher
}

func NewTaskSource(fetcher *TaskFetcher) *TaskSource {
	return &TaskSource{fetcher: fetcher}
}

func (s *TaskSource) FetchPendingTasks(maxTasks int) ([]api.ProductImportTaskRespDTO, error) {
	if s == nil || s.fetcher == nil {
		return nil, fmt.Errorf("task source is not initialized")
	}
	if s.fetcher.managementClient == nil {
		return nil, fmt.Errorf("management client is not initialized")
	}

	importTaskClient := s.fetcher.managementClient.GetImportTaskClient()
	logger.GetGlobalLogger("app/task").Warn("using deprecated legacy polling task source; migrate this workload to RabbitMQ platform consumers")
	logger.GetGlobalLogger("app/task").Infof("🔍 任务获取参数: maxTasks=%d, userID=%d, storeIDs=%v",
		maxTasks, s.fetcher.config.Management.UserID, s.fetcher.config.Management.StoreIDs)

	return importTaskClient.GetPendingAndRetryTasks(
		maxTasks,
		s.fetcher.config.Management.UserID,
		s.fetcher.config.Management.StoreIDs,
	)
}
