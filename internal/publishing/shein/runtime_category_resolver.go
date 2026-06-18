package shein

import (
	"context"
	"strings"

	"task-processor/internal/catalog/canonical"
	sheincategory "task-processor/internal/shein/api/category"
)

type runtimeCategoryResolver struct {
	fallback         CategoryResolver
	factory          *runtimeAPIFactory
	suggestFallback  categorySuggestFallback
	treeFallback     categoryTreeFallback
	semanticVerifier categorySemanticVerifier
}

func NewRuntimeCategoryResolver(factory RuntimeAPIClientFactory, aiConfig CategoryAIConfig) CategoryResolver {
	return &runtimeCategoryResolver{
		fallback:         NewCategoryResolver(nil),
		factory:          newRuntimeAPIFactory(factory),
		suggestFallback:  buildAICategorySuggestFallback(aiConfig),
		treeFallback:     buildAICategoryTreeFallback(aiConfig),
		semanticVerifier: buildAICategorySemanticVerifier(aiConfig),
	}
}

func (r *runtimeCategoryResolver) Resolve(req *BuildRequest, canonical *canonical.Product, pkg *Package) *CategoryResolution {
	if req == nil {
		return r.fallback.Resolve(req, canonical, pkg)
	}

	api, note := r.buildAPI(req.Context, req.SheinStoreID)
	resolver := NewCategoryResolverWithSemanticVerifier(api, r.suggestFallback, r.treeFallback, r.semanticVerifier)
	resolution := resolver.Resolve(req, canonical, pkg)
	if strings.TrimSpace(note) != "" {
		resolution.ReviewNotes = append(resolution.ReviewNotes, note)
		resolution.Status = "partial"
	}
	return resolution
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

func (r *runtimeCategoryResolver) buildAPI(ctx context.Context, storeID int64) (CategoryAPI, string) {
	baseAPIClient, note := r.factory.BuildBaseClient(ctx, storeID)
	if baseAPIClient == nil {
		return nil, note
	}
	return sheincategory.NewClient(baseAPIClient), ""
}
