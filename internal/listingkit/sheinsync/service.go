package sheinsync

import (
	"context"

	"task-processor/internal/listingadmin"
	sheinproduct "task-processor/internal/shein/api/product"
)

const sheinSyncPageSize = 100
const sheinSyncCostResolutionConcurrency = 1

type SheinSyncService interface {
	SyncSheinOnShelfProducts(ctx context.Context, tenantID, storeID int64, triggerMode SheinSyncTriggerMode) (*SheinSyncJobRecord, error)
	SyncSheinSourceSDSProduct(ctx context.Context, tenantID, storeID int64, sourceCode string) (int, error)
	ListSyncedProducts(ctx context.Context, query *SheinSyncedProductQuery) ([]SheinSyncedProductRecord, int64, error)
	UpdateManualCostPrice(ctx context.Context, productID int64, manualCostPrice *float64) error
	ResolveProductAPI(ctx context.Context, storeID int64) (sheinproduct.ProductAPI, error)
}

type SheinSyncImmediateRefreshAware interface {
	SupportsImmediateRefresh() bool
}

type sheinSyncService struct {
	repo                   SheinSyncRepository
	productAPI             sheinproduct.ProductAPI
	productAPIBuilder      SheinSyncProductAPIBuilder
	costResolver           SheinCostResolver
	inventoryMappingSource SheinInventoryMappingSource
	pageSize               int
}

func NewSheinSyncService(repo SheinSyncRepository, productAPI sheinproduct.ProductAPI, costResolver SheinCostResolver) SheinSyncService {
	return newSheinSyncService(repo, productAPI, nil, costResolver)
}

func (s *sheinSyncService) SupportsImmediateRefresh() bool {
	return true
}

func (s *sheinSyncService) ResolveProductAPI(ctx context.Context, storeID int64) (sheinproduct.ProductAPI, error) {
	return s.resolveProductAPI(ctx, storeID)
}

func newSheinSyncService(repo SheinSyncRepository, productAPI sheinproduct.ProductAPI, productAPIBuilder SheinSyncProductAPIBuilder, costResolver SheinCostResolver) *sheinSyncService {
	if costResolver == nil && productAPI != nil {
		costResolver = NewSheinCostResolver(productAPI)
	}
	return &sheinSyncService{
		repo:              repo,
		productAPI:        productAPI,
		productAPIBuilder: productAPIBuilder,
		costResolver:      costResolver,
		pageSize:          sheinSyncPageSize,
	}
}

type SheinInventoryMappingSource interface {
	FindLatest(ctx context.Context, query listingadmin.ProductImportMappingQuery) (*listingadmin.ProductImportMapping, error)
}

func NewSheinSyncServiceWithInventoryMappingSource(repo SheinSyncRepository, productAPI sheinproduct.ProductAPI, costResolver SheinCostResolver, mappingSource SheinInventoryMappingSource) SheinSyncService {
	service := newSheinSyncService(repo, productAPI, nil, costResolver)
	service.inventoryMappingSource = mappingSource
	return service
}

type SheinSyncProductAPIBuilder interface {
	BuildProductAPI(ctx context.Context, storeID int64) (sheinproduct.ProductAPI, string)
}

func NewSheinSyncServiceWithBuilder(repo SheinSyncRepository, productAPIBuilder SheinSyncProductAPIBuilder, costResolver SheinCostResolver) SheinSyncService {
	return newSheinSyncService(repo, nil, productAPIBuilder, costResolver)
}

func NewSheinSyncServiceWithBuilderAndInventoryMappingSource(repo SheinSyncRepository, productAPIBuilder SheinSyncProductAPIBuilder, costResolver SheinCostResolver, mappingSource SheinInventoryMappingSource) SheinSyncService {
	service := newSheinSyncService(repo, nil, productAPIBuilder, costResolver)
	service.inventoryMappingSource = mappingSource
	return service
}
