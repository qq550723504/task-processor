package listingkit

import (
	"context"
	"strings"
	"time"

	"task-processor/internal/infra/clients/management/api"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/tenantbridge"
)

func (s *service) GetSheinSettings(ctx context.Context) (*SheinSettings, error) {
	s.sheinSettingsMu.RLock()
	defer s.sheinSettingsMu.RUnlock()
	settings := s.sheinSettings
	settings.AvailableStores = s.listSheinStoreOptions(ctx)
	return &settings, nil
}

func (s *service) UpdateSheinSettings(ctx context.Context, req *SheinSettings) (*SheinSettings, error) {
	if req == nil {
		return s.GetSheinSettings(ctx)
	}
	s.sheinSettingsMu.Lock()
	defer s.sheinSettingsMu.Unlock()
	settings := s.sheinSettings
	if req.DefaultStoreID > 0 {
		settings.DefaultStoreID = req.DefaultStoreID
	}
	if value := strings.ToUpper(strings.TrimSpace(req.Site)); value != "" {
		settings.Site = value
	}
	if value := strings.TrimSpace(req.WarehouseCode); value != "" {
		settings.WarehouseCode = value
	}
	if req.DefaultStock > 0 {
		settings.DefaultStock = req.DefaultStock
	}
	if value := strings.ToLower(strings.TrimSpace(req.DefaultSubmitMode)); value == "publish" || value == "save_draft" {
		settings.DefaultSubmitMode = value
	}
	settings.Pricing = normalizeSheinPricingRule(req.Pricing, settings.Pricing)
	now := time.Now()
	settings.UpdatedAt = &now
	s.sheinSettings = settings
	return &settings, nil
}

func (s *service) listSheinStoreOptions(ctx context.Context) []SheinStoreOption {
	if s == nil || s.sheinManagementClient == nil {
		return nil
	}
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		return nil
	}
	page, err := s.sheinManagementClient.GetStoreClient().PageStores(&api.StorePageReqDTO{
		Platform: "shein",
		TenantID: tenantID,
		PageNo:   1,
		PageSize: 200,
	})
	if err != nil || page == nil || len(page.List) == 0 {
		return nil
	}
	options := make([]SheinStoreOption, 0, len(page.List))
	for _, item := range page.List {
		if item == nil || item.ID <= 0 {
			continue
		}
		options = append(options, SheinStoreOption{
			ID:       item.ID,
			StoreID:  strings.TrimSpace(item.StoreID),
			Name:     strings.TrimSpace(item.Name),
			Platform: strings.TrimSpace(item.Platform),
			Region:   strings.TrimSpace(item.Region),
		})
	}
	return options
}

func tenantIDInt64FromContext(ctx context.Context) (int64, bool) {
	identity := openaiclient.IdentityFromContext(ctx)
	tenantID := strings.TrimSpace(identity.TenantID)
	if tenantID == "" {
		return 0, false
	}
	value, err := tenantbridge.ResolveLegacyTenantID(ctx, tenantID)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func tenantIDInt64FromTask(task *Task) int64 {
	if task == nil {
		return 0
	}
	value, err := tenantbridge.ResolveLegacyTenantID(context.Background(), strings.TrimSpace(task.TenantID))
	if err != nil || value <= 0 {
		return 0
	}
	return value
}
