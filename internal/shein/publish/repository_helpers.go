package publish

import (
	"context"
	"fmt"

	"task-processor/internal/listingruntime"
)

type publishRuntimeRepository interface {
	RuntimePublishedProductExists(ctx context.Context, storeID int64, platform, region, productID string) (bool, error)
	FindRuntimeProductImportMappingByTaskAndSKU(ctx context.Context, importTaskID int64, sku string) (*listingruntime.ProductImportMapping, error)
	CreateRuntimeProductImportMapping(ctx context.Context, req *listingruntime.ProductImportMappingUpsert) (int64, error)
	UpdateRuntimeProductImportMapping(ctx context.Context, req *listingruntime.ProductImportMappingUpsert) error
	GetRuntimeStorePauseStatusDetail(storeID int64) (*listingruntime.StorePauseStatusDetail, error)
}

func checkPublishedProductExists(
	ctx context.Context,
	repository publishRuntimeRepository,
	storeID int64,
	platform string,
	region string,
	productID string,
) (bool, error) {
	if repository == nil {
		return false, fmt.Errorf("publish runtime repository is nil")
	}
	return repository.RuntimePublishedProductExists(ctx, storeID, platform, region, productID)
}

func findMappingByTaskAndSKU(
	ctx context.Context,
	repository publishRuntimeRepository,
	importTaskID int64,
	sku string,
) (*listingruntime.ProductImportMapping, error) {
	if repository == nil {
		return nil, fmt.Errorf("publish runtime repository is nil")
	}
	return repository.FindRuntimeProductImportMappingByTaskAndSKU(ctx, importTaskID, sku)
}

func publishMappingDTO(mapping *listingruntime.ProductImportMapping) *listingruntime.ProductImportMapping {
	if mapping == nil {
		return nil
	}
	copy := *mapping
	return &copy
}
