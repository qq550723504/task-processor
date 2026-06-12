package listingkit

func buildSettingsAdminServiceConfig(s *service) settingsAdminServiceConfig {
	return settingsAdminServiceConfig{
		storeProfileRepo:     s.storeProfileRepo,
		aiCredentialStore:    s.aiCredentialStore,
		currentSheinSettings: s.currentSheinSubmitSettings,
		mutateSheinSettings: func(mutate func(*SheinSettings)) SheinSettings {
			s.sheinSettingsMu.Lock()
			defer s.sheinSettingsMu.Unlock()
			settings := s.sheinSettings
			if mutate != nil {
				mutate(&settings)
			}
			s.sheinSettings = settings
			return settings
		},
		listStoreOptions: s.listSheinStoreOptions,
	}
}

func buildSheinAdminServiceConfig(s *service) sheinAdminServiceConfig {
	return sheinAdminServiceConfig{
		repo:                  s.repo,
		mutateTaskResult:      s.mutateTaskResult,
		currentPricingRule:    s.currentSheinPricingRule,
		newSheinAPIClient:     s.newSheinAPIClient,
		buildTaskPreview:      s.buildTaskPreview,
		categoryResolver:      s.sheinCategoryResolver,
		attributeResolver:     s.sheinAttributeResolver,
		saleAttributeResolver: s.sheinSaleAttributeResolver,
		clearPricingCache:     s.clearSheinPricingCache,
	}
}
