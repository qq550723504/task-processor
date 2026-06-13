package listingkit

import sheinpub "task-processor/internal/publishing/shein"

type sheinRuntimeDependencies struct {
	resolutionCacheStore  sheinpub.ResolutionCacheStore
	storeCatalog          SheinStoreCatalog
	apiClientFactory      SheinAPIClientFactory
	categoryResolver      sheinpub.CategoryResolver
	attributeResolver     sheinpub.AttributeResolver
	saleAttributeResolver sheinpub.SaleAttributeResolver
	pricingPolicy         sheinpub.PricingPolicy
}

func resolveSheinResolutionCacheStore(s *service) sheinpub.ResolutionCacheStore {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.sheinRuntimeDeps.resolutionCacheStore, &s.mirrors.sheinResolutionCacheStore)
}

func resolveSheinStoreCatalog(s *service) SheinStoreCatalog {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.sheinRuntimeDeps.storeCatalog, &s.mirrors.sheinStoreCatalog)
}

func resolveSheinAPIClientFactory(s *service) SheinAPIClientFactory {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.sheinRuntimeDeps.apiClientFactory, &s.mirrors.sheinAPIClientFactory)
}

func resolveSheinCategoryResolver(s *service) sheinpub.CategoryResolver {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.sheinRuntimeDeps.categoryResolver, &s.mirrors.sheinCategoryResolver)
}

func resolveSheinAttributeResolver(s *service) sheinpub.AttributeResolver {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.sheinRuntimeDeps.attributeResolver, &s.mirrors.sheinAttributeResolver)
}

func resolveSheinSaleAttributeResolver(s *service) sheinpub.SaleAttributeResolver {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.sheinRuntimeDeps.saleAttributeResolver, &s.mirrors.sheinSaleAttributeResolver)
}

func resolveSheinPricingPolicy(s *service) sheinpub.PricingPolicy {
	if s == nil {
		return sheinpub.PricingPolicy{}
	}
	policy := syncGroupedDependency(&s.sheinRuntimeDeps.pricingPolicy, &s.mirrors.sheinPricingPolicy)
	if isZeroSheinPricingPolicy(policy) {
		return sheinpub.PricingPolicy{}
	}
	return policy
}

func isZeroSheinPricingPolicy(policy sheinpub.PricingPolicy) bool {
	return policy == (sheinpub.PricingPolicy{})
}
