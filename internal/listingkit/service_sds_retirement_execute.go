package listingkit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	sheinproduct "task-processor/internal/shein/api/product"
)

type SheinProductAPIResolver interface {
	ResolveProductAPI(ctx context.Context, storeID int64) (sheinproduct.ProductAPI, error)
}

func (s *sdsRetirementService) ConfirmSDSRetirementRun(ctx context.Context, runID string, req *ConfirmSDSRetirementRunRequest) (*SDSRetirementRunDetail, error) {
	if req == nil {
		return nil, fmt.Errorf("confirm request is required")
	}
	return s.executeSDSRetirementRun(ctx, runID, req, false)
}

func (s *sdsRetirementService) RetrySDSRetirementRun(ctx context.Context, runID string) (*SDSRetirementRunDetail, error) {
	return s.executeSDSRetirementRun(ctx, runID, nil, true)
}

func (s *sdsRetirementService) executeSDSRetirementRun(ctx context.Context, runID string, req *ConfirmSDSRetirementRunRequest, retryOnly bool) (*SDSRetirementRunDetail, error) {
	repo, ok := s.repo.(SDSRetirementRepository)
	if !ok {
		return nil, fmt.Errorf("SDS retirement repository is unavailable")
	}
	if err := requireSDSRetirementTenantScope(ctx); err != nil {
		return nil, err
	}
	run, items, err := repo.GetSDSRetirementRun(ctx, strings.TrimSpace(runID))
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("SDS retirement run %q not found", runID)
	}
	if strings.ToLower(strings.TrimSpace(run.Platform)) != "shein" {
		return nil, fmt.Errorf("unsupported SDS retirement platform %q", run.Platform)
	}
	switch run.Status {
	case SDSRetirementRunStatusReady, SDSRetirementRunStatusPartiallySucceeded, SDSRetirementRunStatusFailed:
	default:
		return nil, fmt.Errorf("SDS retirement run %s cannot execute in status %s", runID, run.Status)
	}
	if !sdsRetirementSupportsImmediateRefresh(s.sheinSyncService) {
		return nil, fmt.Errorf("%s", sdsRetirementAsyncRefreshSafetyReason)
	}
	tenantID, err := parseSDSRetirementExecutionTenantID(ctx, run)
	if err != nil {
		return nil, err
	}
	if _, err := s.sheinSyncService.SyncSheinOnShelfProducts(ctx, tenantID, run.StoreID, SheinSyncTriggerModeManual); err != nil {
		return nil, err
	}
	currentProducts, err := s.listCurrentSyncedProductsByID(ctx, tenantID, run.StoreID)
	if err != nil {
		return nil, err
	}
	apiResolver, ok := s.sheinSyncService.(SheinProductAPIResolver)
	if !ok {
		return nil, fmt.Errorf("SHEIN product API resolver is unavailable")
	}
	var (
		productAPI         sheinproduct.ProductAPI
		productAPIResolved bool
	)

	now := time.Now().UTC()
	run.Status = SDSRetirementRunStatusRunning
	run.StartedAt = &now
	if !retryOnly {
		run.ConfirmedBy = strings.TrimSpace(req.ConfirmedBy)
		run.ConfirmedAt = &now
	}

	var successCount, failureCount, processedCount int
	for i := range items {
		item := &items[i]
		if retryOnly {
			if item.Status != SDSRetirementItemStatusFailed {
				continue
			}
		} else if !item.Selected {
			item.Status = SDSRetirementItemStatusSkipped
			item.Error = ""
			item.StartedAt = nil
			item.FinishedAt = nil
			continue
		}

		processedCount++
		startedAt := time.Now().UTC()
		item.Status = SDSRetirementItemStatusRunning
		item.StartedAt = &startedAt
		item.FinishedAt = nil
		item.Error = ""
		item.RequestSnapshot = ""

		currentProduct, refreshErr := sdsRetirementCurrentProductForItem(*item, currentProducts)
		if refreshErr != nil {
			finishedAt := time.Now().UTC()
			item.Status = SDSRetirementItemStatusFailed
			item.Error = refreshErr.Error()
			item.FinishedAt = &finishedAt
			failureCount++
			continue
		}
		applySDSRetirementCurrentProduct(item, currentProduct)
		if !strings.EqualFold(strings.TrimSpace(item.ShelfStatusBefore), "ON_SHELF") {
			finishedAt := time.Now().UTC()
			item.Status = SDSRetirementItemStatusSucceededAlreadyOffShelf
			item.Error = ""
			item.FinishedAt = &finishedAt
			successCount++
			continue
		}

		if !productAPIResolved {
			productAPI, err = apiResolver.ResolveProductAPI(ctx, run.StoreID)
			if err != nil {
				return nil, err
			}
			productAPIResolved = true
		}

		request, buildErr := buildSDSRetirementShelfRequest(*item, item.BusinessModel)
		if buildErr != nil {
			finishedAt := time.Now().UTC()
			item.Status = SDSRetirementItemStatusFailed
			item.Error = buildErr.Error()
			item.FinishedAt = &finishedAt
			failureCount++
			continue
		}
		if rawRequest, marshalErr := json.Marshal(request); marshalErr == nil {
			item.RequestSnapshot = string(rawRequest)
		}

		if err := productAPI.OffShelf(request); err != nil {
			finishedAt := time.Now().UTC()
			item.Status = SDSRetirementItemStatusFailed
			item.Error = err.Error()
			item.FinishedAt = &finishedAt
			failureCount++
			continue
		}

		finishedAt := time.Now().UTC()
		if item.SyncedProductID > 0 {
			if err := repo.MarkSyncedProductOffShelf(ctx, item.SyncedProductID, finishedAt); err != nil {
				item.Status = SDSRetirementItemStatusFailed
				item.Error = err.Error()
				item.FinishedAt = &finishedAt
				failureCount++
				continue
			}
		}
		item.Status = SDSRetirementItemStatusSucceeded
		item.Error = ""
		item.FinishedAt = &finishedAt
		successCount++
	}

	finishedAt := time.Now().UTC()
	run.FinishedAt = &finishedAt
	switch {
	case processedCount == 0:
		run.Status = SDSRetirementRunStatusFailed
		run.Reason = "No selected SDS retirement items were executable."
	case failureCount == 0:
		run.Status = SDSRetirementRunStatusSucceeded
		run.Reason = ""
	case successCount > 0:
		run.Status = SDSRetirementRunStatusPartiallySucceeded
		run.Reason = ""
	default:
		run.Status = SDSRetirementRunStatusFailed
		run.Reason = "All selected SDS retirement items failed."
	}

	if err := repo.SaveSDSRetirementExecution(ctx, run, items); err != nil {
		return nil, err
	}
	return &SDSRetirementRunDetail{Run: *run, Items: items}, nil
}

func (s *sdsRetirementService) listCurrentSyncedProductsByID(ctx context.Context, tenantID, storeID int64) (map[int64]SheinSyncedProductRecord, error) {
	if s == nil || s.sheinSyncService == nil {
		return nil, fmt.Errorf("SHEIN sync service is unavailable")
	}
	productsByID := make(map[int64]SheinSyncedProductRecord)
	for page := 1; ; page++ {
		rows, total, err := s.sheinSyncService.ListSyncedProducts(ctx, &SheinSyncedProductQuery{
			TenantID: tenantID,
			StoreID:  storeID,
			Page:     page,
			PageSize: sdsRetirementSyncedProductPageSize,
		})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			productsByID[row.ID] = row
		}
		if len(rows) == 0 || int64(page*sdsRetirementSyncedProductPageSize) >= total {
			break
		}
	}
	return productsByID, nil
}

func sdsRetirementCurrentProductForItem(item SDSRetirementItemRecord, productsByID map[int64]SheinSyncedProductRecord) (SheinSyncedProductRecord, error) {
	if item.SyncedProductID <= 0 {
		return SheinSyncedProductRecord{}, fmt.Errorf("synced product id is required for item %s", item.ID)
	}
	product, ok := productsByID[item.SyncedProductID]
	if !ok {
		return SheinSyncedProductRecord{}, fmt.Errorf("refreshed synced product %d not found for item %s", item.SyncedProductID, item.ID)
	}
	return product, nil
}

func applySDSRetirementCurrentProduct(item *SDSRetirementItemRecord, product SheinSyncedProductRecord) {
	if item == nil {
		return
	}
	item.SPUName = product.SPUName
	item.SKCName = product.SKCName
	item.SKCCode = product.SKCCode
	item.SupplierCode = strings.TrimSpace(product.SupplierCode)
	item.BusinessModel = product.BusinessModel
	item.ShelfStatusBefore = strings.TrimSpace(product.ShelfStatus)
}

func parseSDSRetirementExecutionTenantID(ctx context.Context, run *SDSRetirementRunRecord) (int64, error) {
	if run != nil {
		if tenantID, err := parseSDSRetirementTenantID(run.TenantID); err == nil {
			return tenantID, nil
		}
	}
	return parseSDSRetirementTenantID(TenantIDFromContext(ctx))
}
