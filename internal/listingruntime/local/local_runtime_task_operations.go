package local

import (
	"context"
	"fmt"

	"task-processor/internal/app/taskstatus"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
)

func (r *LocalRuntime) GetRuntimeImportTask(taskID int64) (*listingruntime.ImportTask, error) {
	if r == nil || r.resources == nil {
		return nil, fmt.Errorf("local listing runtime is not initialized")
	}
	repo := r.resources.ImportTaskRepository()
	if repo == nil || taskID <= 0 {
		return nil, nil
	}
	item, err := repo.GetImportTaskByID(context.Background(), taskID)
	if err != nil || item == nil {
		return nil, err
	}
	return importTaskToRuntime(item), nil
}

func (r *LocalRuntime) UpdateRuntimeTaskStatus(req *listingruntime.TaskStatusUpdate) error {
	if r == nil || r.resources == nil {
		return fmt.Errorf("local listing runtime is not initialized")
	}
	if req == nil {
		return fmt.Errorf("runtime task status update request is nil")
	}
	repo := r.resources.ImportTaskRepository()
	if repo == nil {
		return nil
	}
	_, err := repo.UpdateImportTaskStatus(context.Background(), &listingadmin.ImportTaskStatusUpdate{
		ID:                    req.ID,
		Status:                req.Status,
		ErrorMessage:          req.ErrorMessage,
		ReasonCode:            req.ReasonCode,
		Stage:                 req.Stage,
		ExpectedCurrentStatus: req.ExpectedCurrentStatus,
		RetryCount:            req.RetryCount,
		Priority:              req.Priority,
	})
	return err
}

func (r *LocalRuntime) GetTaskStatus(taskID int64) (*taskstatus.TaskStatusSnapshot, error) {
	if r == nil || r.resources == nil {
		return nil, fmt.Errorf("local task rpc provider is not configured")
	}
	taskRPC := NewLocalTaskRPCProvider(r.resources.Database())
	if taskRPC == nil {
		return nil, fmt.Errorf("local task rpc provider is not configured")
	}
	status, found, err := taskRPC.GetTaskStatus(taskID)
	if err != nil || !found || status == nil {
		return nil, err
	}
	return taskStatusSnapshotFromDTO(status), nil
}

func (r *LocalRuntime) RuntimePublishedProductExists(ctx context.Context, storeID int64, platform, region, productID string) (bool, error) {
	repo := r.productImportMappingRepository()
	if repo == nil {
		return false, nil
	}
	return repo.ExistsPublishedProduct(ctx, storeID, platform, region, productID)
}

func (r *LocalRuntime) FindRuntimeProductImportMappingByTaskAndSKU(ctx context.Context, importTaskID int64, sku string) (*listingruntime.ProductImportMapping, error) {
	repo := r.productImportMappingRepository()
	if repo == nil {
		return nil, nil
	}
	mapping, err := repo.FindLatest(ctx, listingadmin.ProductImportMappingQuery{ImportTaskID: &importTaskID, SKU: sku})
	if err != nil {
		return nil, err
	}
	return runtimeProductImportMappingFromListing(mapping), nil
}

func (r *LocalRuntime) CreateRuntimeProductImportMapping(ctx context.Context, req *listingruntime.ProductImportMappingUpsert) (int64, error) {
	if req == nil {
		return 0, fmt.Errorf("runtime product import mapping request is nil")
	}
	repo := r.productImportMappingRepository()
	if repo == nil {
		return 0, nil
	}
	mapping, err := repo.CreateProductImportMapping(ctx, listingProductImportMappingFromRuntime(req))
	if err != nil || mapping == nil {
		return 0, err
	}
	return mapping.ID, nil
}

func (r *LocalRuntime) UpdateRuntimeProductImportMapping(ctx context.Context, req *listingruntime.ProductImportMappingUpsert) error {
	if req == nil {
		return fmt.Errorf("runtime product import mapping request is nil")
	}
	repo := r.productImportMappingRepository()
	if repo == nil {
		return nil
	}
	_, err := repo.UpdateProductImportMapping(ctx, listingProductImportMappingFromRuntime(req))
	return err
}
