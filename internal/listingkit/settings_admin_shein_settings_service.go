package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *settingsAdminService) GetSheinSettings(ctx context.Context) (*SheinSettings, error) {
	if s == nil || s.currentSheinSettings == nil {
		return nil, fmt.Errorf("shein settings are not configured")
	}
	settings := s.currentSheinSettings()
	settings.AvailableStores = s.listStoreOptions(ctx)
	return &settings, nil
}

func (s *settingsAdminService) UpdateSheinSettings(ctx context.Context, req *SheinSettings) (*SheinSettings, error) {
	if req == nil {
		return s.GetSheinSettings(ctx)
	}
	if s == nil || s.mutateSheinSettings == nil {
		return nil, fmt.Errorf("shein settings are not configured")
	}
	settings := s.mutateSheinSettings(func(settings *SheinSettings) {
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
	})
	settings.AvailableStores = s.listStoreOptions(ctx)
	return &settings, nil
}
