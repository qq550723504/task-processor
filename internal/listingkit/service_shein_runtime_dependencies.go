package listingkit

import sheinpub "task-processor/internal/publishing/shein"

type sheinRuntimeDependencies struct {
	resolutionCacheStore   sheinpub.ResolutionCacheStore
	categoryResolver       sheinpub.CategoryResolver
	attributeResolver      sheinpub.AttributeResolver
	freshAttributeResolver sheinpub.FreshAttributeResolver
	saleAttributeResolver  sheinpub.SaleAttributeResolver
	sizeHeaderResolver     sheinpub.SizeAttributeHeaderResolver
	pricingPolicy          sheinpub.PricingPolicy
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

func resolveSheinFreshAttributeResolver(s *service) sheinpub.FreshAttributeResolver {
	if s == nil {
		return nil
	}
	if s.sheinRuntimeDeps.freshAttributeResolver != nil {
		return s.sheinRuntimeDeps.freshAttributeResolver
	}
	fresh, _ := s.sheinRuntimeDeps.attributeResolver.(sheinpub.FreshAttributeResolver)
	return fresh
}

func resolveSheinSaleAttributeResolver(s *service) sheinpub.SaleAttributeResolver {
	if s == nil {
		return nil
	}
	return s.sheinRuntimeDeps.saleAttributeResolver
}

func resolveSheinSizeHeaderResolver(s *service) sheinpub.SizeAttributeHeaderResolver {
	if s == nil {
		return nil
	}
	return s.sheinRuntimeDeps.sizeHeaderResolver
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
