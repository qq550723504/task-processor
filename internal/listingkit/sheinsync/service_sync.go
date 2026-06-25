package sheinsync

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	sheinproduct "task-processor/internal/shein/api/product"
)

func (s *sheinSyncService) SyncSheinOnShelfProducts(ctx context.Context, tenantID, storeID int64, triggerMode SheinSyncTriggerMode) (*SheinSyncJobRecord, error) {
	if err := s.validateDependencies(); err != nil {
		return nil, err
	}

	job, err := s.createPendingSyncJob(ctx, tenantID, storeID, triggerMode)
	if err != nil {
		return nil, err
	}
	return s.runSyncJob(ctx, job)
}

func (s *sheinSyncService) createPendingSyncJob(ctx context.Context, tenantID, storeID int64, triggerMode SheinSyncTriggerMode) (*SheinSyncJobRecord, error) {
	job := &SheinSyncJobRecord{
		TenantID:    tenantID,
		StoreID:     storeID,
		TriggerMode: triggerMode,
		Status:      SheinSyncJobStatusPending,
	}
	if err := s.repo.SaveSyncJob(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *sheinSyncService) runSyncJob(ctx context.Context, job *SheinSyncJobRecord) (*SheinSyncJobRecord, error) {
	if job == nil {
		return nil, fmt.Errorf("SHEIN sync job is required")
	}
	startedAt := time.Now().UTC()
	job.StartedAt = &startedAt
	job.Status = SheinSyncJobStatusRunning
	if err := s.repo.SaveSyncJob(ctx, job); err != nil {
		return nil, err
	}

	existingProducts, err := s.listExistingProducts(ctx, job.TenantID, job.StoreID)
	if err != nil {
		return nil, s.failSyncJob(ctx, job, fmt.Errorf("list existing synced products: %w", err))
	}

	productAPI, err := s.resolveProductAPI(ctx, job.StoreID)
	if err != nil {
		return nil, s.failSyncJob(ctx, job, err)
	}
	costResolver := s.resolveCostResolver(productAPI)

	records, activeSKCNames, fetchedCount, err := s.fetchOnShelfProducts(ctx, job.TenantID, job.StoreID, existingProducts, productAPI, costResolver)
	if err != nil {
		return nil, s.failSyncJob(ctx, job, err)
	}

	insertedCount, updatedCount := countUpsertedProducts(existingProducts, records)
	deactivatedCount := countDeactivatedProducts(existingProducts, activeSKCNames)

	if err := s.repo.UpsertSyncedProducts(ctx, records); err != nil {
		return nil, s.failSyncJob(ctx, job, fmt.Errorf("persist synced products: %w", err))
	}
	if err := s.repo.MarkMissingSyncedProductsInactive(ctx, job.TenantID, job.StoreID, activeSKCNames); err != nil {
		return nil, s.failSyncJob(ctx, job, fmt.Errorf("mark missing synced products inactive: %w", err))
	}

	finishedAt := time.Now().UTC()
	job.Status = SheinSyncJobStatusSucceeded
	job.FinishedAt = &finishedAt
	job.FetchedCount = fetchedCount
	job.InsertedCount = insertedCount
	job.UpdatedCount = updatedCount
	job.DeactivatedCount = deactivatedCount
	job.ErrorSummary = ""
	if err := s.repo.SaveSyncJob(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *sheinSyncService) ListSyncedProducts(ctx context.Context, query *SheinSyncedProductQuery) ([]SheinSyncedProductRecord, int64, error) {
	if err := s.validateDependencies(); err != nil {
		return nil, 0, err
	}
	return s.repo.ListSyncedProducts(ctx, query)
}

func (s *sheinSyncService) UpdateManualCostPrice(ctx context.Context, productID int64, manualCostPrice *float64) error {
	if err := s.validateDependencies(); err != nil {
		return err
	}
	return s.repo.UpdateManualCostPrice(ctx, productID, manualCostPrice)
}

type sheinSDSCostGroupRepository interface {
	UpsertSDSCostGroup(ctx context.Context, record *SheinSDSCostGroupRecord) error
	ListSDSCostGroups(ctx context.Context, query *SheinSDSCostGroupQuery) ([]SheinSDSCostGroupRecord, int64, error)
}

func (s *sheinSyncService) ListSDSCostGroups(ctx context.Context, query *SheinSDSCostGroupQuery) ([]SheinSDSCostGroupRecord, int64, error) {
	if err := s.validateDependencies(); err != nil {
		return nil, 0, err
	}
	repo, ok := s.repo.(sheinSDSCostGroupRepository)
	if !ok {
		return nil, 0, fmt.Errorf("SHEIN SDS cost group repository is unavailable")
	}
	return repo.ListSDSCostGroups(ctx, query)
}

func (s *sheinSyncService) UpdateSDSCostGroupManualCost(ctx context.Context, tenantID, storeID int64, groupKey, groupLabel string, manualCostPrice *float64) (*SheinSDSCostGroupRecord, error) {
	if err := s.validateDependencies(); err != nil {
		return nil, err
	}
	repo, ok := s.repo.(sheinSDSCostGroupRepository)
	if !ok {
		return nil, fmt.Errorf("SHEIN SDS cost group repository is unavailable")
	}
	groupKey = strings.TrimSpace(groupKey)
	if groupKey == "" {
		return nil, fmt.Errorf("SHEIN SDS cost group key is required")
	}
	row := &SheinSDSCostGroupRecord{
		TenantID:        tenantID,
		StoreID:         storeID,
		GroupKey:        groupKey,
		GroupLabel:      strings.TrimSpace(groupLabel),
		ManualCostPrice: cloneSheinSyncFloat64(manualCostPrice),
	}
	if err := repo.UpsertSDSCostGroup(ctx, row); err != nil {
		return nil, err
	}
	rows, _, err := repo.ListSDSCostGroups(ctx, &SheinSDSCostGroupQuery{
		TenantID:  tenantID,
		StoreID:   storeID,
		GroupKeys: []string{groupKey},
		Page:      1,
		PageSize:  1,
	})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return row, nil
	}
	return &rows[0], nil
}

func (s *sheinSyncService) listExistingProducts(ctx context.Context, tenantID, storeID int64) (map[string]SheinSyncedProductRecord, error) {
	items := make(map[string]SheinSyncedProductRecord)
	page := 1
	for {
		rows, total, err := s.repo.ListSyncedProducts(ctx, &SheinSyncedProductQuery{
			TenantID: tenantID,
			StoreID:  storeID,
			Page:     page,
			PageSize: s.pageSize,
		})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			items[row.SKCName] = row
		}
		if len(rows) == 0 || int64(page*s.pageSize) >= total {
			break
		}
		page++
	}
	return items, nil
}

func (s *sheinSyncService) fetchOnShelfProducts(
	ctx context.Context,
	tenantID, storeID int64,
	existingProducts map[string]SheinSyncedProductRecord,
	productAPI sheinproduct.ProductAPI,
	costResolver SheinCostResolver,
) ([]*SheinSyncedProductRecord, []string, int, error) {
	records := make([]*SheinSyncedProductRecord, 0)
	activeSKCNames := make([]string, 0)
	activeSet := make(map[string]struct{})
	page := 1

	for {
		response, err := productAPI.ListProducts(page, s.pageSize, &sheinproduct.ProductListRequest{
			Language:  "en",
			ShelfType: "ON_SHELF",
			SortType:  1,
		})
		if err != nil {
			return nil, nil, 0, fmt.Errorf("list SHEIN on-shelf products page %d: %w", page, err)
		}
		if response == nil {
			return nil, nil, 0, fmt.Errorf("list SHEIN on-shelf products page %d returned nil response", page)
		}

		resolvedCostsByProduct := make([]map[string]resolvedSheinCost, len(response.Info.Data))
		snapshotsByProduct := make([]sheinProductSnapshots, len(response.Info.Data))
		resolveGroup, resolveCtx := errgroup.WithContext(ctx)
		resolveGroup.SetLimit(sheinSyncCostResolutionConcurrency)

		for idx, product := range response.Info.Data {
			idx := idx
			product := product
			resolveGroup.Go(func() error {
				resolvedCosts, resolveErr := costResolver.ResolveAutoCosts(resolveCtx, product)
				if resolveErr != nil {
					return fmt.Errorf("resolve SHEIN cost price for spu %s: %w", product.SpuName, resolveErr)
				}
				resolvedCostsByProduct[idx] = resolvedCosts
				snapshotsByProduct[idx] = s.fetchSupplementalSnapshots(resolveCtx, productAPI, product)
				return nil
			})
		}

		if err := resolveGroup.Wait(); err != nil {
			return nil, nil, 0, err
		}

		for idx, product := range response.Info.Data {
			resolvedCosts := resolvedCostsByProduct[idx]
			snapshots := snapshotsByProduct[idx]
			for _, skc := range product.SkcInfoList {
				if skc.SkcName == "" {
					continue
				}

				record := buildSyncedProductRecord(tenantID, storeID, product, skc)
				if existing, ok := existingProducts[skc.SkcName]; ok {
					record.ID = existing.ID
					record.ManualCostPrice = cloneSheinSyncFloat64(existing.ManualCostPrice)
					record.AutoCostPrice = cloneSheinSyncFloat64(existing.AutoCostPrice)
					record.CreatedAt = existing.CreatedAt
					if existing.Currency != "" {
						record.Currency = existing.Currency
					}
					if !snapshots.priceLoaded {
						record.PriceSnapshot = existing.PriceSnapshot
					}
					if !snapshots.inventoryLoaded {
						record.InventorySnapshot = existing.InventorySnapshot
					}
				}
				if resolved, ok := resolvedCosts[skc.SkcName]; ok {
					record.AutoCostPrice = cloneSheinSyncFloat64(resolved.CostPrice)
					if resolved.Currency != "" {
						record.Currency = resolved.Currency
					}
				}
				if snapshots.priceLoaded {
					record.PriceSnapshot = snapshots.priceSnapshot
				}
				if snapshots.inventoryLoaded {
					record.InventorySnapshot = snapshots.inventorySnapshot
				}
				ApplyEffectiveCostPrice(record)
				records = append(records, record)
				if _, seen := activeSet[record.SKCName]; !seen {
					activeSet[record.SKCName] = struct{}{}
					activeSKCNames = append(activeSKCNames, record.SKCName)
				}
			}
		}

		if len(response.Info.Data) == 0 || len(response.Info.Data) < s.pageSize || int64(page*s.pageSize) >= int64(response.Info.Meta.Count) {
			break
		}
		page++
	}

	return records, activeSKCNames, len(records), nil
}

func (s *sheinSyncService) failSyncJob(ctx context.Context, job *SheinSyncJobRecord, syncErr error) error {
	if job == nil {
		return syncErr
	}

	finishedAt := time.Now().UTC()
	job.Status = SheinSyncJobStatusFailed
	job.FinishedAt = &finishedAt
	job.ErrorSummary = syncErr.Error()
	if err := s.repo.SaveSyncJob(ctx, job); err != nil {
		return errors.Join(syncErr, fmt.Errorf("persist failed SHEIN sync job state: %w", err))
	}
	return syncErr
}

func (s *sheinSyncService) validateDependencies() error {
	switch {
	case s == nil:
		return fmt.Errorf("SHEIN sync service is required")
	case s.repo == nil:
		return fmt.Errorf("SHEIN sync repository is required")
	case s.productAPI == nil && s.productAPIBuilder == nil:
		return fmt.Errorf("SHEIN product API is required")
	default:
		return nil
	}
}

func (s *sheinSyncService) resolveProductAPI(ctx context.Context, storeID int64) (sheinproduct.ProductAPI, error) {
	if s.productAPI != nil {
		return s.productAPI, nil
	}
	if s.productAPIBuilder == nil {
		return nil, fmt.Errorf("SHEIN product API is required")
	}
	productAPI, fallback := s.productAPIBuilder.BuildProductAPI(ctx, storeID)
	if productAPI == nil {
		return nil, fmt.Errorf("SHEIN sync unavailable: %s", sheinSyncProductAPIFallbackMessage(fallback))
	}
	return productAPI, nil
}

func sheinSyncProductAPIFallbackMessage(fallback string) string {
	fallback = strings.TrimSpace(fallback)
	if fallback == "" {
		return "product API builder returned nil"
	}
	if strings.Contains(fallback, "已降级为离线解析") {
		return "SHEIN 店铺 cookie 不可用，无法同步 SHEIN 商品；请先完成店铺登录或验证码"
	}
	if strings.Contains(fallback, "在线解析") {
		return strings.ReplaceAll(fallback, "在线解析", "商品同步")
	}
	return fallback
}

func (s *sheinSyncService) resolveCostResolver(productAPI sheinproduct.ProductAPI) SheinCostResolver {
	if s.costResolver != nil {
		return s.costResolver
	}
	return NewSheinCostResolver(productAPI)
}
