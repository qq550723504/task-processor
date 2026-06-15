package sheinsync

import (
	"context"
	"fmt"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/shared/tenantctx"
	sheinproduct "task-processor/internal/shein/api/product"
)

type asyncSheinSyncService struct {
	sync *sheinSyncService
}

func NewAsyncSheinSyncService(repo SheinSyncRepository, productAPI sheinproduct.ProductAPI, costResolver SheinCostResolver) SheinSyncService {
	return &asyncSheinSyncService{
		sync: newSheinSyncService(repo, productAPI, nil, costResolver),
	}
}

func NewAsyncSheinSyncServiceWithBuilder(repo SheinSyncRepository, productAPIBuilder SheinSyncProductAPIBuilder, costResolver SheinCostResolver) SheinSyncService {
	return &asyncSheinSyncService{
		sync: newSheinSyncService(repo, nil, productAPIBuilder, costResolver),
	}
}

func (s *asyncSheinSyncService) SyncSheinOnShelfProducts(ctx context.Context, tenantID, storeID int64, triggerMode SheinSyncTriggerMode) (*SheinSyncJobRecord, error) {
	if s == nil || s.sync == nil {
		return nil, fmt.Errorf("SHEIN sync service is required")
	}
	if err := s.sync.validateDependencies(); err != nil {
		return nil, err
	}

	job, err := s.sync.createPendingSyncJob(ctx, tenantID, storeID, triggerMode)
	if err != nil {
		return nil, err
	}

	backgroundCtx := detachedSheinSyncContext(ctx)
	backgroundJob := *job
	go func() {
		_, _ = s.sync.runSyncJob(backgroundCtx, &backgroundJob)
	}()

	return job, nil
}

func (s *asyncSheinSyncService) ListSyncedProducts(ctx context.Context, query *SheinSyncedProductQuery) ([]SheinSyncedProductRecord, int64, error) {
	return s.sync.ListSyncedProducts(ctx, query)
}

func (s *asyncSheinSyncService) UpdateManualCostPrice(ctx context.Context, productID int64, manualCostPrice *float64) error {
	return s.sync.UpdateManualCostPrice(ctx, productID, manualCostPrice)
}

func detachedSheinSyncContext(ctx context.Context) context.Context {
	detached := tenantctx.WithTenantID(context.Background(), tenantctx.TenantIDFromContext(ctx))
	return openaiclient.WithIdentity(detached, openaiclient.IdentityFromContext(ctx))
}
