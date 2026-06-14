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
	wiring := buildSheinAdminWiring(s)
	return sheinAdminServiceConfig{
		repo:                  wiring.repo,
		recovery:              wiring.recovery,
		currentPricingRule:    wiring.currentPricingRule,
		newSheinAPIClient:     wiring.newSheinAPIClient,
		buildTaskPreview:      wiring.buildTaskPreview,
		categoryResolver:      wiring.categoryResolver,
		attributeResolver:     wiring.attributeResolver,
		saleAttributeResolver: wiring.saleAttributeResolver,
		clearPricingCache:     wiring.clearPricingCache,
	}
}
