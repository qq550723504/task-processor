package sheinsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
	sheinproduct "task-processor/internal/shein/api/product"
)

const sheinSyncPageSize = 100
const sheinSyncCostResolutionConcurrency = 8

type SheinSyncService interface {
	SyncSheinOnShelfProducts(ctx context.Context, tenantID, storeID int64, triggerMode SheinSyncTriggerMode) (*SheinSyncJobRecord, error)
	ListSyncedProducts(ctx context.Context, query *SheinSyncedProductQuery) ([]SheinSyncedProductRecord, int64, error)
	UpdateManualCostPrice(ctx context.Context, productID int64, manualCostPrice *float64) error
}

type sheinSyncService struct {
	repo              SheinSyncRepository
	productAPI        sheinproduct.ProductAPI
	productAPIBuilder SheinSyncProductAPIBuilder
	costResolver      SheinCostResolver
	pageSize          int
}

type sheinProductSnapshots struct {
	priceSnapshot     string
	inventorySnapshot string
	priceLoaded       bool
	inventoryLoaded   bool
}

func NewSheinSyncService(repo SheinSyncRepository, productAPI sheinproduct.ProductAPI, costResolver SheinCostResolver) SheinSyncService {
	return newSheinSyncService(repo, productAPI, nil, costResolver)
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

type SheinSyncProductAPIBuilder interface {
	BuildProductAPI(ctx context.Context, storeID int64) (sheinproduct.ProductAPI, string)
}

func NewSheinSyncServiceWithBuilder(repo SheinSyncRepository, productAPIBuilder SheinSyncProductAPIBuilder, costResolver SheinCostResolver) SheinSyncService {
	return newSheinSyncService(repo, nil, productAPIBuilder, costResolver)
}

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
		if fallback == "" {
			fallback = "product API builder returned nil"
		}
		return nil, fmt.Errorf("SHEIN sync unavailable: %s", fallback)
	}
	return productAPI, nil
}

func (s *sheinSyncService) resolveCostResolver(productAPI sheinproduct.ProductAPI) SheinCostResolver {
	if s.costResolver != nil {
		return s.costResolver
	}
	return NewSheinCostResolver(productAPI)
}

func buildSyncedProductRecord(
	tenantID, storeID int64,
	product sheinproduct.ProductListItem,
	skc sheinproduct.SkcInfoItem,
) *SheinSyncedProductRecord {
	now := time.Now().UTC()
	publishTime, _ := parseSheinSyncTime(product.PublishTime)
	firstShelfTime, _ := parseSheinSyncTime(product.FirstShelfTime)

	record := &SheinSyncedProductRecord{
		TenantID:         tenantID,
		StoreID:          storeID,
		SPUName:          product.SpuName,
		SPUCode:          product.SpuCode,
		SKCName:          skc.SkcName,
		SKCCode:          skc.SkcCode,
		SupplierCode:     skc.SupplierCode,
		CategoryID:       product.CategoryID,
		BrandName:        product.BrandName,
		ProductNameMulti: product.ProductNameMulti,
		MainImageURL:     skc.MainImageThumbnailURL,
		SaleName:         skc.SaleName,
		ShelfStatus:      product.ShelfStatus,
		PublishTime:      publishTime,
		FirstShelfTime:   firstShelfTime,
		SiteSnapshot:     buildSheinSiteSnapshot(product, skc),
		LastSyncAt:       &now,
		IsActive:         true,
	}
	return record
}

func buildSheinSiteSnapshot(product sheinproduct.ProductListItem, skc sheinproduct.SkcInfoItem) string {
	payload := map[string]any{
		"spu_name":           product.SpuName,
		"spu_code":           product.SpuCode,
		"shelf_status":       product.ShelfStatus,
		"publish_time":       product.PublishTime,
		"first_shelf_time":   product.FirstShelfTime,
		"product_name_multi": product.ProductNameMulti,
		"skc_name":           skc.SkcName,
		"skc_code":           skc.SkcCode,
		"sale_name":          skc.SaleName,
		"supplier_code":      skc.SupplierCode,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func (s *sheinSyncService) fetchSupplementalSnapshots(
	_ context.Context,
	productAPI sheinproduct.ProductAPI,
	product sheinproduct.ProductListItem,
) sheinProductSnapshots {
	snapshots := sheinProductSnapshots{}
	if productAPI == nil || product.SpuName == "" {
		return snapshots
	}

	if priceResp, err := productAPI.QueryPrice(product.SpuName); err == nil {
		snapshots.priceLoaded = true
		snapshots.priceSnapshot = buildSheinPriceSnapshot(priceResp, product)
	}

	if inventoryResp, err := productAPI.QueryInventory(product.SpuName); err == nil {
		snapshots.inventoryLoaded = true
		snapshots.inventorySnapshot = buildSheinInventorySnapshot(inventoryResp, product)
	}

	return snapshots
}

func buildSheinPriceSnapshot(
	response *sheinproduct.PriceQueryResponse,
	product sheinproduct.ProductListItem,
) string {
	if response == nil {
		return ""
	}

	skcByName := make(map[string]sheinproduct.SkcPriceData, len(response.Info.Data))
	for _, skcPrice := range response.Info.Data {
		skcByName[skcPrice.SkcName] = skcPrice
	}

	for _, skc := range product.SkcInfoList {
		skcPrice, ok := skcByName[skc.SkcName]
		if !ok {
			continue
		}
		for _, skuPrice := range skcPrice.SkuInfoList {
			for _, detail := range skuPrice.PriceInfoList {
				salePrice := detail.SpecialPrice
				if salePrice <= 0 {
					salePrice = detail.ShopPrice
				}
				if salePrice <= 0 {
					continue
				}

				payload := map[string]any{
					"sale_price": salePrice,
					"currency":   detail.Currency,
					"sub_site":   detail.SubSite,
				}
				encoded, err := json.Marshal(payload)
				if err != nil {
					return ""
				}
				return string(encoded)
			}
		}
	}

	return ""
}

func buildSheinInventorySnapshot(
	response *sheinproduct.InventoryQueryResponse,
	product sheinproduct.ProductListItem,
) string {
	if response == nil {
		return ""
	}

	skcByName := make(map[string]sheinproduct.SkcInventory, len(response.Info.SkcInfo))
	for _, skcInventory := range response.Info.SkcInfo {
		skcByName[skcInventory.SkcName] = skcInventory
	}

	for _, skc := range product.SkcInfoList {
		skcInventory, ok := skcByName[skc.SkcName]
		if !ok {
			continue
		}

		total := 0
		available := 0
		for _, skuInventory := range skcInventory.SkuInfo {
			for _, warehouse := range skuInventory.InventoryInfo {
				total += warehouse.InventoryQuantity
				available += warehouse.UsableInventory
			}
		}

		payload := map[string]any{
			"total":     total,
			"available": available,
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return ""
		}
		return string(encoded)
	}

	return ""
}

func parseSheinSyncTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
		time.RFC3339,
	}
	for _, format := range formats {
		parsed, err := time.Parse(format, value)
		if err == nil {
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("parse SHEIN time %q", value)
}

func countUpsertedProducts(existingProducts map[string]SheinSyncedProductRecord, records []*SheinSyncedProductRecord) (int, int) {
	insertedCount := 0
	updatedCount := 0
	for _, record := range records {
		if record == nil {
			continue
		}
		if _, exists := existingProducts[record.SKCName]; exists {
			updatedCount++
			continue
		}
		insertedCount++
	}
	return insertedCount, updatedCount
}

func countDeactivatedProducts(existingProducts map[string]SheinSyncedProductRecord, activeSKCNames []string) int {
	activeSet := make(map[string]struct{}, len(activeSKCNames))
	for _, skcName := range activeSKCNames {
		activeSet[skcName] = struct{}{}
	}

	deactivatedCount := 0
	for skcName, row := range existingProducts {
		if !row.IsActive {
			continue
		}
		if _, stillActive := activeSet[skcName]; stillActive {
			continue
		}
		deactivatedCount++
	}
	return deactivatedCount
}

func cloneSheinSyncFloat64(v *float64) *float64 {
	if v == nil {
		return nil
	}
	copied := *v
	return &copied
}
