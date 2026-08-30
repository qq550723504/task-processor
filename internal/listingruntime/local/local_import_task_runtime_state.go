package local

import (
	"context"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
)

// localImportTaskRuntimeState adapts resource-owned task persistence to the
// runtime task contract without depending on LocalDataProvider.
type localImportTaskRuntimeState struct {
	repository *listingadmin.GormImportTaskRepository
}

func newLocalImportTaskRuntimeState(resources *RuntimeResources) *localImportTaskRuntimeState {
	if resources == nil {
		return nil
	}
	repository := resources.ImportTaskRepository()
	if repository == nil {
		return nil
	}
	return &localImportTaskRuntimeState{repository: repository}
}

func (s *localImportTaskRuntimeState) GetPendingAndRetryTasks(limit int, tenantID int64, storeIDs []int64) ([]listingruntime.ImportTask, bool, error) {
	if s == nil || s.repository == nil {
		return nil, false, nil
	}
	tasks, err := s.repository.ListPendingAndRetryTasks(context.Background(), limit, tenantID, storeIDs)
	if err != nil {
		return nil, true, err
	}
	result := make([]listingruntime.ImportTask, 0, len(tasks))
	for index := range tasks {
		if runtimeTask := importTaskToRuntime(&tasks[index]); runtimeTask != nil {
			result = append(result, *runtimeTask)
		}
	}
	return result, true, nil
}

func (s *localImportTaskRuntimeState) GetImportTaskByID(taskID int64) (*listingruntime.ImportTask, bool, error) {
	if s == nil || s.repository == nil || taskID <= 0 {
		return nil, false, nil
	}
	task, err := s.repository.GetImportTaskByID(context.Background(), taskID)
	if err != nil {
		return nil, true, err
	}
	return importTaskToRuntime(task), true, nil
}

func (s *localImportTaskRuntimeState) UpdateImportTaskStatus(req *listingadmin.ImportTaskStatusUpdate) (bool, error) {
	if s == nil || s.repository == nil || req == nil {
		return false, nil
	}
	return s.repository.UpdateImportTaskStatus(context.Background(), req)
}

func importTaskToRuntime(task *listingadmin.ImportTask) *listingruntime.ImportTask {
	if task == nil {
		return nil
	}
	meta := localTaskStatusMetadata(task.Status)
	return &listingruntime.ImportTask{
		ID:              task.ID,
		TenantID:        task.TenantID,
		StoreID:         int64FromPtr(task.StoreID),
		Platform:        task.Platform,
		SourcePlatform:  task.SourcePlatform,
		TargetPlatform:  task.TargetPlatform,
		Region:          task.Region,
		CategoryID:      int64FromPtr(task.CategoryID),
		ProductID:       task.ProductID,
		Status:          task.Status,
		ErrorMessage:    task.ErrorMessage,
		RetryCount:      task.RetryCount,
		MaxRetryCount:   task.MaxRetryCount,
		Priority:        task.Priority,
		CreateTime:      timeToUnixMillis(task.CreateTime),
		PublishedTime:   timeToUnixMillis(task.PublishedTime),
		StatusKey:       meta.Key,
		CanonicalStatus: meta.Canonical,
	}
}
