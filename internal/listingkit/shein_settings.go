package listingkit

import (
	"context"
	"strings"
	"time"
)

func (s *service) GetSheinSettings(ctx context.Context) (*SheinSettings, error) {
	s.sheinSettingsMu.RLock()
	defer s.sheinSettingsMu.RUnlock()
	settings := s.sheinSettings
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
