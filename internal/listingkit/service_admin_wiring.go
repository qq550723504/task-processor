package listingkit

func buildSettingsAdminServiceConfig(s *service) settingsAdminServiceConfig {
	wiring := buildSettingsAdminWiring(s)
	return settingsAdminServiceConfig{
		storeProfileRepo:     wiring.storeProfileRepo,
		aiCredentialStore:    wiring.aiCredentialStore,
		currentSheinSettings: wiring.currentSheinSettings,
		mutateSheinSettings:  wiring.mutateSheinSettings,
		listStoreOptions:     wiring.listStoreOptions,
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
