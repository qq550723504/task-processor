package listingkit

import (
	"context"

	sheinpub "task-processor/internal/publishing/shein"
	sheinclient "task-processor/internal/shein/client"
)

type settingsAdminWiring struct {
	storeProfileRepo     StoreProfileRepository
	aiCredentialStore    AIClientCredentialStore
	currentSheinSettings func() SheinSettings
	mutateSheinSettings  func(func(*SheinSettings)) SheinSettings
	listStoreOptions     func(context.Context) []SheinStoreOption
}

type sheinAdminWiring struct {
	repo                  Repository
	mutateTaskResult      func(context.Context, string, TaskResultMutation) (*Task, error)
	currentPricingRule    func() sheinpub.PricingRule
	newSheinAPIClient     func(context.Context, *Task) (*sheinclient.APIClient, int64, error)
	buildTaskPreview      func(context.Context, *Task, string) (*ListingKitPreview, error)
	categoryResolver      sheinpub.CategoryResolver
	attributeResolver     sheinpub.AttributeResolver
	saleAttributeResolver sheinpub.SaleAttributeResolver
	clearPricingCache     func(*sheinpub.BuildRequest, *sheinpub.Package) error
}

func buildSettingsAdminWiring(s *service) settingsAdminWiring {
	return settingsAdminWiring{
		storeProfileRepo:     resolveAdminStoreProfileRepo(s),
		aiCredentialStore:    resolveAdminAICredentialStore(s),
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

func buildSheinAdminWiring(s *service) sheinAdminWiring {
	repository := buildServiceRepositoryWiring(s)
	return sheinAdminWiring{
		repo:                  repository.repo,
		mutateTaskResult:      s.mutateTaskResult,
		currentPricingRule:    s.currentSheinPricingRule,
		newSheinAPIClient:     s.newSheinAPIClient,
		buildTaskPreview:      s.buildTaskPreview,
		categoryResolver:      resolveSheinCategoryResolver(s),
		attributeResolver:     resolveSheinAttributeResolver(s),
		saleAttributeResolver: resolveSheinSaleAttributeResolver(s),
		clearPricingCache:     s.clearSheinPricingCache,
	}
}

func resolveAdminStoreProfileRepo(s *service) StoreProfileRepository {
	if s == nil {
		return nil
	}
	if s.adminDeps.storeProfileRepo != nil {
		s.storeProfileRepo = s.adminDeps.storeProfileRepo
		return s.adminDeps.storeProfileRepo
	}
	s.adminDeps.storeProfileRepo = s.storeProfileRepo
	return s.storeProfileRepo
}

func resolveAdminAICredentialStore(s *service) AIClientCredentialStore {
	if s == nil {
		return nil
	}
	if s.adminDeps.aiCredentialStore != nil {
		s.aiCredentialStore = s.adminDeps.aiCredentialStore
		return s.adminDeps.aiCredentialStore
	}
	s.adminDeps.aiCredentialStore = s.aiCredentialStore
	return s.aiCredentialStore
}
