package listingkit

import (
	"context"
	"fmt"
	"strings"

	sheinwarehouse "task-processor/internal/shein/api/warehouse"
)

type submitRuntimeContextResolver struct {
	service *service
}

func buildSubmitRuntimeContextResolver(s *service) *submitRuntimeContextResolver {
	if s == nil {
		return nil
	}
	return &submitRuntimeContextResolver{service: s}
}

func (r *submitRuntimeContextResolver) resolveSubmitSettings(ctx context.Context, task *Task) SheinSettings {
	if r == nil || r.service == nil {
		return SheinSettings{}
	}
	settings := r.service.currentSheinSubmitSettings()
	if profile, err := r.resolveStoreProfile(ctx, task); err == nil && profile != nil {
		settings = applySubmitSettingsProfile(settings, profile)
	}
	settings = applySubmitSettingsTaskRequest(settings, task)
	if task == nil {
		return settings
	}
	return applySubmitWarehouseOverride(settings, r.resolveWarehouseCode(ctx, task, settings.Site))
}

func (r *submitRuntimeContextResolver) resolveWarehouseCode(ctx context.Context, task *Task, site string) string {
	if r == nil || r.service == nil || resolveSubmissionStoreCatalog(r.service) == nil || resolveSubmissionAPIClientFactory(r.service) == nil || task == nil {
		return ""
	}
	apiClient, storeID, err := r.newAPIClient(ctx, task)
	if err != nil {
		return ""
	}
	if !apiClient.HasCookies() {
		if err := apiClient.ForceRefreshCookies(); err != nil {
			return ""
		}
	}
	if !apiClient.HasCookies() {
		return ""
	}
	baseAPI := NewSheinRuntimeBaseAPIClient(apiClient, storeID)
	warehouseAPI := sheinwarehouse.NewClient(baseAPI)
	warehouses, err := warehouseAPI.GetWarehouses()
	if err != nil || warehouses == nil {
		return ""
	}
	return pickSheinWarehouseCode(warehouses, site)
}

func (r *submitRuntimeContextResolver) resolveStoreInfo(ctx context.Context, task *Task) (*SheinStoreInfo, error) {
	if r == nil || r.service == nil || resolveSubmissionStoreCatalog(r.service) == nil {
		return nil, fmt.Errorf("shein store catalog is unavailable")
	}
	storeID, err := r.resolveStoreID(ctx, task)
	if err != nil || storeID <= 0 {
		return nil, fmt.Errorf("shein store id is unavailable")
	}
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		tenantID = tenantIDInt64FromTask(task)
	}
	if tenantID <= 0 {
		return nil, fmt.Errorf("shein store tenant is unavailable")
	}
	validator := resolveSubmissionStoreAccessValidator(r.service)
	if validator == nil {
		return nil, NewStoreAccessError(StoreAccessUnavailable, "store is unavailable")
	}
	if _, err := validator.ValidateStoreAccess(ctx, tenantID, storeID, "SHEIN"); err != nil {
		if sheinStoreResolutionSnapshotFromTask(task) != nil && StoreAccessErrorCode(err) != "" {
			return nil, NewStoreAccessError(StoreAccessStale, "store selection is stale")
		}
		return nil, err
	}
	storeInfo, err := resolveSubmissionStoreCatalog(r.service).GetStoreInfo(ctx, tenantID, storeID)
	if err != nil {
		return nil, fmt.Errorf("load shein store info: %w", err)
	}
	if storeInfo == nil || storeInfo.ID != storeID || storeInfo.TenantID != tenantID || !strings.EqualFold(strings.TrimSpace(storeInfo.Platform), "SHEIN") {
		return nil, storeResolutionAccessError(task)
	}
	return storeInfo, nil
}

func storeResolutionAccessError(task *Task) error {
	if sheinStoreResolutionSnapshotFromTask(task) != nil {
		return NewStoreAccessError(StoreAccessStale, "store selection is stale")
	}
	return NewStoreAccessError(StoreAccessUnavailable, "store is unavailable")
}

func (r *submitRuntimeContextResolver) newAPIClient(ctx context.Context, task *Task) (*SheinRuntimeAPIClient, int64, error) {
	if r == nil || r.service == nil || resolveSubmissionAPIClientFactory(r.service) == nil {
		return nil, 0, fmt.Errorf("shein api client factory is unavailable")
	}
	storeInfo, err := r.resolveStoreInfo(ctx, task)
	if err != nil {
		return nil, 0, err
	}
	return resolveSubmissionAPIClientFactory(r.service).NewSheinAPIClient(storeInfo.ID, storeInfo), storeInfo.ID, nil
}

func (r *submitRuntimeContextResolver) resolveStoreID(ctx context.Context, task *Task) (int64, error) {
	if task != nil && task.Request != nil && task.Request.SheinStoreID > 0 {
		return task.Request.SheinStoreID, nil
	}
	if snapshot := sheinStoreResolutionSnapshotFromTask(task); snapshot != nil && snapshot.StoreID > 0 {
		return snapshot.StoreID, nil
	}
	return 0, fmt.Errorf("shein store id is unavailable")
}

func (r *submitRuntimeContextResolver) resolveStoreProfile(ctx context.Context, task *Task) (*ListingKitStoreProfile, error) {
	selection, err := r.resolveStoreSelection(ctx, task)
	if err != nil {
		return nil, err
	}
	if selection == nil {
		return nil, fmt.Errorf("store profile is unavailable")
	}
	return cloneStoreProfile(selection.Profile), nil
}

func (r *submitRuntimeContextResolver) resolveStoreSelection(ctx context.Context, task *Task) (*sheinStoreSelection, error) {
	if snapshot := sheinStoreResolutionSnapshotFromTask(task); snapshot != nil && snapshot.StoreID > 0 {
		return selectionFromSnapshot(snapshot), nil
	}
	if task == nil || task.Request == nil || task.Request.SheinStoreID <= 0 {
		return nil, fmt.Errorf("shein store id is unavailable")
	}

	selection := &sheinStoreSelection{
		Profile: &ListingKitStoreProfile{
			StoreID: task.Request.SheinStoreID,
			Enabled: true,
		},
		Strategy:       "manual",
		Reason:         "任务显式指定了 SHEIN 店铺。",
		ManualOverride: true,
	}
	if r == nil || r.service == nil || resolveSubmissionStoreProfileRepo(r.service) == nil {
		return selection, nil
	}

	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		tenantID = tenantIDInt64FromTask(task)
	}
	if tenantID <= 0 {
		return selection, nil
	}

	items, err := resolveSubmissionStoreProfileRepo(r.service).ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for idx := range items {
		if !items[idx].Enabled || items[idx].StoreID != task.Request.SheinStoreID {
			continue
		}
		selection.Profile = cloneStoreProfile(&items[idx])
		return selection, nil
	}
	return selection, nil
}
