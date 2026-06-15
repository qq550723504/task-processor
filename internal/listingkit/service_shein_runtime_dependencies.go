package listingkit

import sheinpub "task-processor/internal/publishing/shein"

type sheinRuntimeDependencies struct {
	resolutionCacheStore  sheinpub.ResolutionCacheStore
	categoryResolver      sheinpub.CategoryResolver
	attributeResolver     sheinpub.AttributeResolver
	saleAttributeResolver sheinpub.SaleAttributeResolver
	pricingPolicy         sheinpub.PricingPolicy
}

func resolveSheinResolutionCacheStore(s *service) sheinpub.ResolutionCacheStore {
	if s == nil {
		return nil
	}
	return s.sheinRuntimeDeps.resolutionCacheStore
}

func resolveSheinCategoryResolver(s *service) sheinpub.CategoryResolver {
	if s == nil {
		return nil
	}
	return s.sheinRuntimeDeps.categoryResolver
}

func resolveSheinAttributeResolver(s *service) sheinpub.AttributeResolver {
	if s == nil {
		return nil
	}
	return s.sheinRuntimeDeps.attributeResolver
}

func resolveSheinSaleAttributeResolver(s *service) sheinpub.SaleAttributeResolver {
	if s == nil {
		return nil
	}
	return s.sheinRuntimeDeps.saleAttributeResolver
}

func resolveSheinPricingPolicy(s *service) sheinpub.PricingPolicy {
	if s == nil {
		return sheinpub.PricingPolicy{}
	}
	policy := s.sheinRuntimeDeps.pricingPolicy
	if isZeroSheinPricingPolicy(policy) {
		return sheinpub.PricingPolicy{}
	}
	return policy
}

func isZeroSheinPricingPolicy(policy sheinpub.PricingPolicy) bool {
	return policy == (sheinpub.PricingPolicy{})
}
