package shein

import (
	"context"

	"task-processor/internal/product/catalog/canonical"
	sheincategory "task-processor/internal/shein/api/category"
)

type runtimeCategoryResolver struct {
	factory          *runtimeAPIFactory
	suggestFallback  categorySuggestFallback
	treeFallback     categoryTreeFallback
	semanticVerifier categorySemanticVerifier
}

func NewRuntimeCategoryResolver(factory RuntimeAPIClientFactory, aiConfig CategoryAIConfig) CategoryResolver {
	return &runtimeCategoryResolver{
		factory:          newRuntimeAPIFactory(factory),
		suggestFallback:  buildAICategorySuggestFallback(aiConfig),
		treeFallback:     buildAICategoryTreeFallback(aiConfig),
		semanticVerifier: buildAICategorySemanticVerifier(aiConfig),
	}
}

func (r *runtimeCategoryResolver) Resolve(req *BuildRequest, canonical *canonical.Product, pkg *Package) *CategoryResolution {
	if r == nil || req == nil {
		return nil
	}

	api := r.buildAPI(req.Context, req.SheinStoreID)
	if api == nil {
		return nil
	}
	resolver := NewCategoryResolverWithSemanticVerifier(api, r.suggestFallback, r.treeFallback, r.semanticVerifier)
	return resolver.Resolve(req, canonical, pkg)
}

func buildAICategorySuggestFallback(aiConfig CategoryAIConfig) categorySuggestFallback {
	if aiConfig.Selector == nil {
		return nil
	}
	return newAICategorySuggestFallback(aiConfig.Selector)
}

func buildAICategoryTreeFallback(aiConfig CategoryAIConfig) categoryTreeFallback {
	if aiConfig.Selector == nil {
		return nil
	}
	return newAICategoryTreeFallback(aiConfig.Selector)
}

func buildAICategorySemanticVerifier(aiConfig CategoryAIConfig) categorySemanticVerifier {
	if aiConfig.SemanticVerifier == nil {
		return nil
	}
	return newAICategorySemanticVerifier(aiConfig.SemanticVerifier)
}

func (r *runtimeCategoryResolver) buildAPI(ctx context.Context, storeID int64) CategoryAPI {
	if r == nil || r.factory == nil {
		return nil
	}
	baseAPIClient, _ := r.factory.BuildBaseClient(ctx, storeID)
	if baseAPIClient == nil {
		return nil
	}
	return sheincategory.NewClient(baseAPIClient)
}
