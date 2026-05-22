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
	return &runtimeCategoryResolver{
		fallback:         NewCategoryResolver(nil),
		factory:          newRuntimeAPIFactory(factory),
		suggestFallback:  buildAICategorySuggestFallback(llmClient...),
		treeFallback:     buildAICategoryTreeFallback(llmClient...),
		semanticVerifier: buildAICategorySemanticVerifier(llmClient...),
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

func buildAICategorySuggestFallback(llmClient ...openaiclient.ChatCompleter) categorySuggestFallback {
	if len(llmClient) == 0 {
		return nil
	}
	return newAICategorySuggestFallback(llmClient[0])
}

func buildAICategoryTreeFallback(llmClient ...openaiclient.ChatCompleter) categoryTreeFallback {
	if len(llmClient) == 0 {
		return nil
	}
	return newAICategoryTreeFallback(llmClient[0])
}

func buildAICategorySemanticVerifier(llmClient ...openaiclient.ChatCompleter) categorySemanticVerifier {
	if len(llmClient) == 0 {
		return nil
	}
	return newAICategorySemanticVerifier(llmClient[0])
}

func (r *runtimeCategoryResolver) buildAPI(ctx context.Context, storeID int64) (CategoryAPI, string) {
	baseAPIClient, note := r.factory.BuildBaseClient(ctx, storeID)
	if baseAPIClient == nil {
		return nil, note
	}
	return sheincategory.NewClient(baseAPIClient), ""
}
