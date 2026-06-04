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
	if r == nil || r.service == nil || r.service.sheinStoreCatalog == nil || r.service.sheinAPIClientFactory == nil || task == nil {
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
	if r == nil || r.service == nil || r.service.sheinStoreCatalog == nil {
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
	storeInfo, err := r.service.sheinStoreCatalog.GetStoreInfo(ctx, tenantID, storeID)
	if err != nil {
		return nil, fmt.Errorf("load shein store info: %w", err)
	}
	if storeInfo == nil || storeInfo.ID <= 0 {
		return nil, fmt.Errorf("shein store info is unavailable")
	}
	if storeInfo.TenantID <= 0 {
		return nil, fmt.Errorf("shein store tenant is unavailable")
	}
	return storeInfo, nil
}

func (r *submitRuntimeContextResolver) newAPIClient(ctx context.Context, task *Task) (*SheinRuntimeAPIClient, int64, error) {
	if r == nil || r.service == nil || r.service.sheinAPIClientFactory == nil {
		return nil, 0, fmt.Errorf("shein api client factory is unavailable")
	}
	storeInfo, err := r.resolveStoreInfo(ctx, task)
	if err != nil {
		return nil, 0, err
	}
	return r.service.sheinAPIClientFactory.NewSheinAPIClient(storeInfo.ID, storeInfo), storeInfo.ID, nil
}

func (r *submitRuntimeContextResolver) resolveStoreID(ctx context.Context, task *Task) (int64, error) {
	if task != nil && task.Request != nil && task.Request.SheinStoreID > 0 {
		return task.Request.SheinStoreID, nil
	}
	if selection, err := r.resolveStoreSelection(ctx, task); err == nil && selection != nil && selection.Profile != nil && selection.Profile.StoreID > 0 {
		return selection.Profile.StoreID, nil
	}
	if r == nil || r.service == nil {
		return 0, nil
	}
	r.service.sheinSettingsMu.RLock()
	defer r.service.sheinSettingsMu.RUnlock()
	return r.service.sheinSettings.DefaultStoreID, nil
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
	if r == nil || r.service == nil || r.service.storeProfileRepo == nil {
		return nil, fmt.Errorf("store profile repository is not configured")
	}
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		tenantID = tenantIDInt64FromTask(task)
	}
	if tenantID <= 0 {
		return nil, fmt.Errorf("tenant id is unavailable")
	}
	items, err := r.service.storeProfileRepo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("store profile is unavailable")
	}
	enabled := make([]ListingKitStoreProfile, 0, len(items))
	for idx := range items {
		if items[idx].Enabled {
			enabled = append(enabled, items[idx])
		}
	}
	if len(enabled) == 0 {
		return nil, fmt.Errorf("store profile is unavailable")
	}
	if task != nil && task.Request != nil && task.Request.SheinStoreID > 0 {
		for idx := range enabled {
			if enabled[idx].StoreID == task.Request.SheinStoreID {
				return &sheinStoreSelection{
					Profile:        cloneStoreProfile(&enabled[idx]),
					Strategy:       "manual",
					Reason:         "任务显式指定了 SHEIN 店铺。",
					ManualOverride: true,
				}, nil
			}
		}
	}
	settings, err := r.service.routingSettingsRepo.GetByTenant(ctx, tenantID)
	var fallback *ListingKitStoreProfile
	for idx := range enabled {
		if enabled[idx].IsFallback && fallback == nil {
			fallback = cloneStoreProfile(&enabled[idx])
		}
	}
	if err == nil && settings != nil && settings.FallbackStoreID > 0 {
		for idx := range enabled {
			if enabled[idx].StoreID == settings.FallbackStoreID {
				fallback = cloneStoreProfile(&enabled[idx])
				break
			}
		}
	}
	if settings != nil && settings.SelectionStrategy != "manual" {
		if matched := matchStoreProfileForTask(enabled, task, settings.SelectionStrategy); matched != nil {
			return &sheinStoreSelection{
				Profile:          cloneStoreProfile(matched.profile),
				Strategy:         settings.SelectionStrategy,
				Reason:           routeSelectionReason(settings.SelectionStrategy, matched.kinds),
				MatchedRuleKinds: append([]string(nil), matched.kinds...),
			}, nil
		}
		if settings.AllowFallback && fallback != nil {
			return &sheinStoreSelection{
				Profile:  cloneStoreProfile(fallback),
				Strategy: settings.SelectionStrategy,
				Reason:   "没有命中路由规则，已回退到 fallback 店铺。",
				Fallback: true,
			}, nil
		}
	}
	if settings != nil && settings.AllowFallback && settings.FallbackStoreID > 0 && fallback != nil {
		return &sheinStoreSelection{
			Profile:  cloneStoreProfile(fallback),
			Strategy: firstNonEmpty(strings.TrimSpace(settings.SelectionStrategy), "manual"),
			Reason:   "当前使用配置的 fallback 店铺作为兜底。",
			Fallback: true,
		}, nil
	}
	if len(enabled) > 0 {
		strategy := "priority"
		reason := "当前未显式指定店铺，使用已启用店铺里的最高优先级项。"
		if settings != nil && settings.SelectionStrategy == "manual" {
			strategy = "manual"
			reason = "当前未显式指定店铺，按优先级选择默认店铺。"
		}
		return &sheinStoreSelection{
			Profile:  cloneStoreProfile(&enabled[0]),
			Strategy: strategy,
			Reason:   reason,
		}, nil
	}
	return nil, fmt.Errorf("store profile is unavailable")
}
