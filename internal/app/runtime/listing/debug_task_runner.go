package listing

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/infra/worker"
	"task-processor/internal/listingadmin"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

type appLogger interface {
	Infof(format string, args ...any)
}

type debugTaskLoader interface {
	GetTaskByID(ctx context.Context, taskID int64) (*managementapi.ProductImportTaskRespDTO, error)
}

type listingAdminDebugTaskLoader struct {
	repo listingadmin.ImportTaskRepository
}

func (l listingAdminDebugTaskLoader) GetTaskByID(ctx context.Context, taskID int64) (*managementapi.ProductImportTaskRespDTO, error) {
	if l.repo == nil {
		return nil, fmt.Errorf("local import task repository is not configured")
	}
	task, err := l.repo.GetImportTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return listingAdminImportTaskToDebugDTO(task), nil
}

type staticDebugTaskLoader struct {
	task *managementapi.ProductImportTaskRespDTO
}

func (l staticDebugTaskLoader) GetTaskByID(_ context.Context, _ int64) (*managementapi.ProductImportTaskRespDTO, error) {
	return l.task, nil
}

type debugTaskRunner struct {
	displayName string
	logger      appLogger
	taskLoader  debugTaskLoader
	processor   worker.Processor
}

func (r debugTaskRunner) run(ctx context.Context, taskID int64) error {
	if r.processor == nil {
		return fmt.Errorf("%s debug processor is not available", r.displayName)
	}
	if r.taskLoader == nil {
		return fmt.Errorf("%s debug task loader is not available", r.displayName)
	}

	taskDTO, err := r.taskLoader.GetTaskByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("load debug task %d: %w", taskID, err)
	}
	if taskDTO == nil {
		return fmt.Errorf("debug task %d not found", taskID)
	}

	task := buildDebugModelTask(taskDTO)
	r.logger.Infof(
		"running %s debug task directly: taskID=%d, productID=%s, storeID=%d",
		r.displayName,
		task.ID,
		task.ProductID,
		task.StoreID,
	)

	if err := r.processor.Start(ctx); err != nil {
		return fmt.Errorf("start %s debug processor failed: %w", r.displayName, err)
	}
	defer r.processor.Close(ctx)

	job, err := buildDebugWorkerJob(task)
	if err != nil {
		return fmt.Errorf("marshal debug task: %w", err)
	}
	if err := r.processor.ProcessTask(ctx, job); err != nil {
		return fmt.Errorf("process debug task %d failed: %w", taskID, err)
	}

	r.logger.Infof("%s debug single-task mode completed: taskID=%d", r.displayName, taskID)
	return nil
}

func buildDebugModelTask(taskDTO *managementapi.ProductImportTaskRespDTO) model.Task {
	if taskDTO == nil {
		return model.Task{}
	}

	return model.Task{
		ID:            taskDTO.ID,
		TenantID:      taskDTO.TenantID,
		StoreID:       taskDTO.StoreID,
		Platform:      taskDTO.Platform,
		Region:        taskDTO.Region,
		CategoryID:    taskDTO.CategoryID,
		ProductID:     taskDTO.ProductID,
		Status:        taskDTO.Status,
		ErrorMessage:  taskDTO.ErrorMessage,
		RetryCount:    taskDTO.RetryCount,
		MaxRetryCount: taskDTO.MaxRetryCount,
		Remark:        taskDTO.Remark,
		Priority:      taskDTO.Priority,
		CreateTime:    taskDTO.CreateTime,
		UpdateTime:    taskDTO.UpdateTime,
		Creator:       taskDTO.Creator,
		Updater:       taskDTO.Updater,
	}
}

func listingAdminImportTaskToDebugDTO(task *listingadmin.ImportTask) *managementapi.ProductImportTaskRespDTO {
	if task == nil {
		return nil
	}
	return &managementapi.ProductImportTaskRespDTO{
		ID:            task.ID,
		TenantID:      task.TenantID,
		StoreID:       int64FromPtr(task.StoreID),
		Platform:      task.Platform,
		Region:        task.Region,
		CategoryID:    int64FromPtr(task.CategoryID),
		ProductID:     task.ProductID,
		Status:        task.Status,
		ErrorMessage:  task.ErrorMessage,
		ReasonCode:    task.ReasonCode,
		Stage:         task.Stage,
		RetryCount:    task.RetryCount,
		MaxRetryCount: task.MaxRetryCount,
		Remark:        task.Remark,
		Priority:      task.Priority,
		CreateTime:    timeToUnixMillis(task.CreateTime),
		UpdateTime:    timeToUnixMillis(task.UpdateTime),
		PublishedTime: timeToUnixMillis(task.PublishedTime),
		Creator:       task.Creator,
		Updater:       task.Updater,
	}
}

func int64FromPtr(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func timeToUnixMillis(value *time.Time) int64 {
	if value == nil {
		return 0
	}
	return value.UnixMilli()
}

func buildDebugWorkerJob(task model.Task) (worker.WorkerJob, error) {
	taskData, err := json.Marshal(task)
	if err != nil {
		return worker.WorkerJob{}, err
	}

	return worker.WorkerJob{
		TenantID: fmt.Sprintf("%d", task.TenantID),
		ShopID:   fmt.Sprintf("%d", task.StoreID),
		TaskData: string(taskData),
	}, nil
}

type debugProcessorRegistrar struct {
	processor worker.Processor
}

func (r *debugProcessorRegistrar) RegisterProcessor(_ string, processor worker.Processor) error {
	r.processor = processor
	return nil
}

var _ appLogger = (*logrus.Logger)(nil)
