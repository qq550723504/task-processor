package shein

import (
	"context"
	"strings"

	"task-processor/internal/catalog/canonical"
	openaiclient "task-processor/internal/infra/clients/openai"
	sheincategory "task-processor/internal/shein/api/category"
)

type runtimeCategoryResolver struct {
	fallback         CategoryResolver
	factory          *runtimeAPIFactory
	suggestFallback  categorySuggestFallback
	treeFallback     categoryTreeFallback
	semanticVerifier categorySemanticVerifier
}

func NewRuntimeCategoryResolver(factory RuntimeAPIClientFactory, llmClient ...openaiclient.ChatCompleter) CategoryResolver {
	var suggestFallback categorySuggestFallback
	var treeFallback categoryTreeFallback
	var semanticVerifier categorySemanticVerifier
	if len(llmClient) > 0 {
		suggestFallback = newAICategorySuggestFallback(llmClient[0])
		treeFallback = newAICategoryTreeFallback(llmClient[0])
		semanticVerifier = newAICategorySemanticVerifier(llmClient[0])
	}
	return &runtimeCategoryResolver{
		fallback:         NewCategoryResolver(nil),
		factory:          newRuntimeAPIFactory(factory),
		suggestFallback:  suggestFallback,
		treeFallback:     treeFallback,
		semanticVerifier: semanticVerifier,
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

func (r *runtimeCategoryResolver) buildAPI(ctx context.Context, storeID int64) (CategoryAPI, string) {
	baseAPIClient, note := r.factory.BuildBaseClient(ctx, storeID)
	if baseAPIClient == nil {
		return nil, note
	}
	return sheincategory.NewClient(baseAPIClient), ""
}
