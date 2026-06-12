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
	if s.sheinRuntimeDeps.resolutionCacheStore != nil {
		s.sheinResolutionCacheStore = s.sheinRuntimeDeps.resolutionCacheStore
		return s.sheinRuntimeDeps.resolutionCacheStore
	}
	s.sheinRuntimeDeps.resolutionCacheStore = s.sheinResolutionCacheStore
	return s.sheinResolutionCacheStore
}

func resolveSheinStoreCatalog(s *service) SheinStoreCatalog {
	if s == nil {
		return nil
	}
	if s.sheinRuntimeDeps.storeCatalog != nil {
		s.sheinStoreCatalog = s.sheinRuntimeDeps.storeCatalog
		return s.sheinRuntimeDeps.storeCatalog
	}
	s.sheinRuntimeDeps.storeCatalog = s.sheinStoreCatalog
	return s.sheinStoreCatalog
}

func resolveSheinAPIClientFactory(s *service) SheinAPIClientFactory {
	if s == nil {
		return nil
	}
	if s.sheinRuntimeDeps.apiClientFactory != nil {
		s.sheinAPIClientFactory = s.sheinRuntimeDeps.apiClientFactory
		return s.sheinRuntimeDeps.apiClientFactory
	}
	s.sheinRuntimeDeps.apiClientFactory = s.sheinAPIClientFactory
	return s.sheinAPIClientFactory
}

func resolveSheinCategoryResolver(s *service) sheinpub.CategoryResolver {
	if s == nil {
		return nil
	}
	if s.sheinRuntimeDeps.categoryResolver != nil {
		s.sheinCategoryResolver = s.sheinRuntimeDeps.categoryResolver
		return s.sheinRuntimeDeps.categoryResolver
	}
	s.sheinRuntimeDeps.categoryResolver = s.sheinCategoryResolver
	return s.sheinCategoryResolver
}

func resolveSheinAttributeResolver(s *service) sheinpub.AttributeResolver {
	if s == nil {
		return nil
	}
	if s.sheinRuntimeDeps.attributeResolver != nil {
		s.sheinAttributeResolver = s.sheinRuntimeDeps.attributeResolver
		return s.sheinRuntimeDeps.attributeResolver
	}
	s.sheinRuntimeDeps.attributeResolver = s.sheinAttributeResolver
	return s.sheinAttributeResolver
}

func resolveSheinSaleAttributeResolver(s *service) sheinpub.SaleAttributeResolver {
	if s == nil {
		return nil
	}
	if s.sheinRuntimeDeps.saleAttributeResolver != nil {
		s.sheinSaleAttributeResolver = s.sheinRuntimeDeps.saleAttributeResolver
		return s.sheinRuntimeDeps.saleAttributeResolver
	}
	s.sheinRuntimeDeps.saleAttributeResolver = s.sheinSaleAttributeResolver
	return s.sheinSaleAttributeResolver
}

func resolveSheinPricingPolicy(s *service) sheinpub.PricingPolicy {
	if s == nil {
		return sheinpub.PricingPolicy{}
	}
	if !isZeroSheinPricingPolicy(s.sheinRuntimeDeps.pricingPolicy) {
		s.sheinPricingPolicy = s.sheinRuntimeDeps.pricingPolicy
		return s.sheinRuntimeDeps.pricingPolicy
	}
	if isZeroSheinPricingPolicy(s.sheinPricingPolicy) {
		return sheinpub.PricingPolicy{}
	}
	s.sheinRuntimeDeps.pricingPolicy = s.sheinPricingPolicy
	return s.sheinPricingPolicy
}

func isZeroSheinPricingPolicy(policy sheinpub.PricingPolicy) bool {
	return policy == (sheinpub.PricingPolicy{})
}
