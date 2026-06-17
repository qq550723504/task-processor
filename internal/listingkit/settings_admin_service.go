package listingkit

import (
	"context"
)

type settingsAdminServiceConfig struct {
	storeProfileRepo     StoreProfileRepository
	aiCredentialStore    AIClientCredentialStore
	currentSheinSettings func() SheinSettings
	mutateSheinSettings  func(func(*SheinSettings)) SheinSettings
	listStoreOptions     func(context.Context) []SheinStoreOption
	settingsHealthProbes SettingsHealthProbes
}

type settingsAdminService struct {
	storeProfileRepo     StoreProfileRepository
	aiCredentialStore    AIClientCredentialStore
	currentSheinSettings func() SheinSettings
	mutateSheinSettings  func(func(*SheinSettings)) SheinSettings
	listStoreOptions     func(context.Context) []SheinStoreOption
	settingsHealthProbes SettingsHealthProbes
}

func newSettingsAdminService(config settingsAdminServiceConfig) *settingsAdminService {
	return &settingsAdminService{
		storeProfileRepo:     config.storeProfileRepo,
		aiCredentialStore:    config.aiCredentialStore,
		currentSheinSettings: config.currentSheinSettings,
		mutateSheinSettings:  config.mutateSheinSettings,
		listStoreOptions:     config.listStoreOptions,
		settingsHealthProbes: config.settingsHealthProbes,
	}
}

func (s *settingsAdminService) GetSettingsHealthProbes(context.Context) SettingsHealthProbes {
	if s == nil {
		return SettingsHealthProbes{}
	}
	return s.settingsHealthProbes
}

func (s *settingsAdminService) attachStoreOptions(ctx context.Context, items []ListingKitStoreProfile) []ListingKitStoreProfile {
	if len(items) == 0 || s.listStoreOptions == nil {
		return items
	}
	options := s.listStoreOptions(ctx)
	if len(options) == 0 {
		return items
	}
	byID := make(map[int64]SheinStoreOption, len(options))
	for _, option := range options {
		byID[option.ID] = option
	}
	for idx := range items {
		option, ok := byID[items[idx].StoreID]
		if !ok {
			continue
		}
		copyOption := option
		items[idx].Store = &copyOption
	}
	return items
}
