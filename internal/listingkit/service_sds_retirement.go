package listingkit

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

const (
	sdsRetirementTaskDiscoveryPageSize    = 100
	sdsRetirementSyncedProductPageSize    = 100
	sdsRetirementAsyncRefreshSafetyReason = "cannot guarantee refreshed SHEIN preview data with async-only sync service"
)

type sdsRetirementService struct {
	repo             Repository
	baselineService  TaskLifecycleService
	sheinSyncService SheinSyncService
}

func NewSDSRetirementService(repo Repository, baselineService TaskLifecycleService, sheinSyncService SheinSyncService) SDSRetirementService {
	return &sdsRetirementService{
		repo:             repo,
		baselineService:  baselineService,
		sheinSyncService: sheinSyncService,
	}
}

func (s *sdsRetirementService) CreateSDSRetirementRun(ctx context.Context, req *CreateSDSRetirementRunRequest) (*SDSRetirementRunDetail, error) {
	if req == nil {
		return nil, fmt.Errorf("SDS retirement request is required")
	}
	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	if platform != "shein" {
		return nil, fmt.Errorf("unsupported SDS retirement platform %q", req.Platform)
	}
	if req.StoreID <= 0 {
		return nil, fmt.Errorf("store_id must be positive")
	}
	identity := SDSBaselineIdentity{
		ParentProductID:    req.ParentProductID,
		PrototypeGroupID:   req.PrototypeGroupID,
		VariantID:          req.VariantID,
		SelectedVariantIDs: normalizedSDSBaselineVariantIDs(req.SelectedVariantIDs),
	}
	if isEmptySDSBaselineIdentity(identity) {
		return nil, fmt.Errorf("SDS retirement identity is required")
	}
	repo, ok := s.repo.(SDSRetirementRepository)
	if !ok {
		return nil, fmt.Errorf("SDS retirement repository is unavailable")
	}

	tenantID := strings.TrimSpace(req.TenantID)
	if tenantID == "" {
		tenantID = strings.TrimSpace(TenantIDFromContext(ctx))
	}
	ctx = WithTenantID(ctx, tenantID)
	readiness := s.sdsBaselineReadiness(ctx, tenantID, req)
	tasks, err := s.listSDSRetirementTasks(ctx, identity, strings.TrimSpace(req.SourceTaskID))
	if err != nil {
		return nil, err
	}
	items, err := s.buildSDSRetirementItems(ctx, tenantID, req.StoreID, tasks)
	if err != nil {
		return nil, err
	}

	selectedVariantIDs, err := json.Marshal(identity.SelectedVariantIDs)
	if err != nil {
		return nil, fmt.Errorf("marshal selected variant ids: %w", err)
	}
	status := SDSRetirementRunStatusDraft
	if len(items) > 0 {
		status = SDSRetirementRunStatusReady
	}
	run := &SDSRetirementRunRecord{
		ID:                 uuid.NewString(),
		TenantID:           tenantID,
		Platform:           platform,
		StoreID:            req.StoreID,
		ParentProductID:    req.ParentProductID,
		PrototypeGroupID:   req.PrototypeGroupID,
		VariantID:          req.VariantID,
		SelectedVariantIDs: string(selectedVariantIDs),
		BaselineKey: sdsBaselineKey(tenantID, (&SDSBaselineReadinessQuery{
			ParentProductID:    req.ParentProductID,
			PrototypeGroupID:   req.PrototypeGroupID,
			VariantID:          req.VariantID,
			SelectedVariantIDs: req.SelectedVariantIDs,
		}).BaselineOptions()),
		ValidationStatus: readiness.ValidationStatus,
		ReasonCode:       readiness.ReasonCode,
		Reason:           readiness.Reason,
		Status:           status,
		CreatedBy:        strings.TrimSpace(req.CreatedBy),
	}
	if err := repo.CreateSDSRetirementRun(ctx, run, items); err != nil {
		return nil, err
	}
	return &SDSRetirementRunDetail{
		Run:    *run,
		Items:  items,
		Tasks:  tasks,
		Reason: readiness.Reason,
	}, nil
}

func (s *sdsRetirementService) GetSDSRetirementRun(ctx context.Context, runID string) (*SDSRetirementRunDetail, error) {
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
	return &SDSRetirementRunDetail{Run: *run, Items: items}, nil
}

func (s *sdsRetirementService) UpdateSDSRetirementSelection(ctx context.Context, runID string, req *UpdateSDSRetirementSelectionRequest) (*SDSRetirementRunDetail, error) {
	repo, ok := s.repo.(SDSRetirementRepository)
	if !ok {
		return nil, fmt.Errorf("SDS retirement repository is unavailable")
	}
	if err := requireSDSRetirementTenantScope(ctx); err != nil {
		return nil, err
	}
	run, _, err := repo.GetSDSRetirementRun(ctx, strings.TrimSpace(runID))
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("SDS retirement run %q not found", runID)
	}
	if run.Status != SDSRetirementRunStatusDraft && run.Status != SDSRetirementRunStatusReady {
		return nil, fmt.Errorf("SDS retirement run %s cannot be edited in status %s", runID, run.Status)
	}
	if req == nil {
		return nil, fmt.Errorf("selection request is required")
	}
	if err := repo.UpdateSDSRetirementItems(ctx, runID, req.Items); err != nil {
		return nil, err
	}
	return s.GetSDSRetirementRun(ctx, runID)
}

func (s *sdsRetirementService) sdsBaselineReadiness(ctx context.Context, tenantID string, req *CreateSDSRetirementRunRequest) *SDSBaselineReadiness {
	readiness := &SDSBaselineReadiness{
		Status:           SDSBaselineStatusMissing,
		ValidationStatus: SDSBaselineValidationStatusUnknown,
	}
	if s == nil || s.baselineService == nil {
		return readiness
	}
	got, err := s.baselineService.GetSDSBaselineReadiness(ctx, &SDSBaselineReadinessQuery{
		TenantID:           tenantID,
		ParentProductID:    req.ParentProductID,
		PrototypeGroupID:   req.PrototypeGroupID,
		VariantID:          req.VariantID,
		SelectedVariantIDs: req.SelectedVariantIDs,
	})
	if err != nil || got == nil {
		return readiness
	}
	return got
}

func (s *sdsRetirementService) listSDSRetirementTasks(ctx context.Context, identity SDSBaselineIdentity, sourceTaskID string) ([]Task, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("listing task repository is unavailable")
	}
	out := make([]Task, 0)
	for page := 1; ; page++ {
		tasks, total, err := s.repo.ListTasks(ctx, &TaskListQuery{
			Page:     page,
			PageSize: sdsRetirementTaskDiscoveryPageSize,
			Platform: "shein",
		})
		if err != nil {
			return nil, err
		}
		for i := range tasks {
			if sourceTaskID != "" && tasks[i].ID != sourceTaskID {
				continue
			}
			if sdsRetirementTaskMatchesIdentity(&tasks[i], identity) {
				out = append(out, tasks[i])
			}
		}
		if len(tasks) == 0 || int64(page*sdsRetirementTaskDiscoveryPageSize) >= total {
			break
		}
	}
	return out, nil
}

func (s *sdsRetirementService) buildSDSRetirementItems(ctx context.Context, tenantID string, storeID int64, tasks []Task) ([]SDSRetirementItemRecord, error) {
	sourceSKUs := sdsRetirementSourceSKUsFromTasks(tasks)
	sourceSKUSet := sdsRetirementSourceSKUSet(sourceSKUs)
	if len(sourceSKUSet) == 0 {
		return nil, fmt.Errorf("no source SDS SKUs found for retirement identity")
	}
	if s == nil || s.sheinSyncService == nil {
		return nil, fmt.Errorf("SHEIN sync service is unavailable")
	}
	tenantIDInt64, err := parseSDSRetirementTenantID(tenantID)
	if err != nil {
		return nil, err
	}
	if !sdsRetirementSupportsImmediateRefresh(s.sheinSyncService) {
		return nil, fmt.Errorf("%s", sdsRetirementAsyncRefreshSafetyReason)
	}

	if _, err := s.sheinSyncService.SyncSheinOnShelfProducts(ctx, tenantIDInt64, storeID, SheinSyncTriggerModeManual); err != nil {
		return nil, err
	}
	active := true
	items := make([]SDSRetirementItemRecord, 0)
	for page := 1; ; page++ {
		products, total, err := s.sheinSyncService.ListSyncedProducts(ctx, &SheinSyncedProductQuery{
			TenantID: tenantIDInt64,
			StoreID:  storeID,
			IsActive: &active,
			Page:     page,
			PageSize: sdsRetirementSyncedProductPageSize,
		})
		if err != nil {
			return nil, err
		}

		for _, product := range products {
			if strings.TrimSpace(product.ShelfStatus) != "ON_SHELF" {
				continue
			}
			supplierCode := strings.TrimSpace(product.SupplierCode)
			if supplierCode == "" {
				continue
			}
			if _, ok := sourceSKUSet[supplierCode]; !ok {
				continue
			}
			items = append(items, SDSRetirementItemRecord{
				ID:                uuid.NewString(),
				TenantID:          tenantID,
				Platform:          "shein",
				StoreID:           storeID,
				SyncedProductID:   product.ID,
				SPUName:           product.SPUName,
				SKCName:           product.SKCName,
				SKCCode:           product.SKCCode,
				SupplierCode:      supplierCode,
				BusinessModel:     product.BusinessModel,
				ShelfStatusBefore: product.ShelfStatus,
				Selected:          true,
				Status:            SDSRetirementItemStatusSelected,
			})
		}
		if len(products) == 0 || int64(page*sdsRetirementSyncedProductPageSize) >= total {
			break
		}
	}
	return items, nil
}

func sdsRetirementSourceSKUsFromTasks(tasks []Task) []string {
	set := map[string]struct{}{}
	for i := range tasks {
		if tasks[i].Result == nil {
			continue
		}
		for _, sku := range sdsRetirementSourceSKUs(tasks[i].Result.CanonicalProduct) {
			set[sku] = struct{}{}
		}
		if snapshot := tasks[i].Result.StandardProductSnapshot; snapshot != nil {
			for _, sku := range sdsRetirementSourceSKUs(snapshot.CanonicalProduct) {
				set[sku] = struct{}{}
			}
		}
	}
	return sdsRetirementSortedSetValues(set)
}

func parseSDSRetirementTenantID(value string) (int64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("tenant id is required")
	}
	parsed, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("tenant id must be numeric")
	}
	return parsed, nil
}

type sdsRetirementImmediateRefreshAware interface {
	SupportsImmediateRefresh() bool
}

func sdsRetirementSupportsImmediateRefresh(service SheinSyncService) bool {
	aware, ok := service.(sdsRetirementImmediateRefreshAware)
	if !ok {
		return true
	}
	return aware.SupportsImmediateRefresh()
}

func requireSDSRetirementTenantScope(ctx context.Context) error {
	if strings.TrimSpace(TenantIDFromContext(ctx)) == "" {
		return fmt.Errorf("tenant scope is required")
	}
	return nil
}
