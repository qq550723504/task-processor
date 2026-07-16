package listingkit

type sheinSharedDependencies struct {
	storeCatalog         SheinStoreCatalog
	storeAccessValidator StoreAccessValidator
	apiClientFactory     SheinAPIClientFactory
	contentOptimizer     AIChatCompleter
}

func resolveSheinStoreCatalog(s *service) SheinStoreCatalog {
	if s == nil {
		return nil
	}
	return s.sheinSharedDeps.storeCatalog
}

func resolveSheinStoreAccessValidator(s *service) StoreAccessValidator {
	if s == nil {
		return nil
	}
	return s.sheinSharedDeps.storeAccessValidator
}

func resolveSheinAPIClientFactory(s *service) SheinAPIClientFactory {
	if s == nil {
		return nil
	}
	return s.sheinSharedDeps.apiClientFactory
}

func resolveSheinContentOptimizer(s *service) AIChatCompleter {
	if s == nil {
		return nil
	}
	return s.sheinSharedDeps.contentOptimizer
}
