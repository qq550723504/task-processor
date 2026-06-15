package listingkit

import openaiclient "task-processor/internal/infra/clients/openai"

type sheinSharedDependencies struct {
	storeCatalog     SheinStoreCatalog
	apiClientFactory SheinAPIClientFactory
	contentOptimizer openaiclient.ChatCompleter
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

func resolveSheinContentOptimizer(s *service) openaiclient.ChatCompleter {
	if s == nil {
		return nil
	}
	return s.sheinSharedDeps.contentOptimizer
}
