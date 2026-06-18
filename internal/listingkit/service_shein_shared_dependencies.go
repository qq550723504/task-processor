package listingkit

type sheinSharedDependencies struct {
	storeCatalog     SheinStoreCatalog
	apiClientFactory SheinAPIClientFactory
	contentOptimizer AIChatCompleter
}

func resolveSheinStoreCatalog(s *service) SheinStoreCatalog {
	if s == nil {
		return nil
	}
	return s.sheinSharedDeps.storeCatalog
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
