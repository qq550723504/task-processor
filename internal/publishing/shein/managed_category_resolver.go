package shein

import (
	"strings"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/infra/clients/management"
	openaiclient "task-processor/internal/infra/clients/openai"
	sheincategory "task-processor/internal/shein/api/category"
)

type managedCategoryResolver struct {
	fallback         CategoryResolver
	factory          *managedAPIFactory
	suggestFallback  categorySuggestFallback
	treeFallback     categoryTreeFallback
	semanticVerifier categorySemanticVerifier
}

func NewManagedCategoryResolver(client *management.ClientManager, llmClient ...openaiclient.ChatCompleter) CategoryResolver {
	var suggestFallback categorySuggestFallback
	var treeFallback categoryTreeFallback
	var semanticVerifier categorySemanticVerifier
	if len(llmClient) > 0 {
		suggestFallback = newAICategorySuggestFallback(llmClient[0])
		treeFallback = newAICategoryTreeFallback(llmClient[0])
		semanticVerifier = newAICategorySemanticVerifier(llmClient[0])
	}
	return &managedCategoryResolver{
		fallback:         NewCategoryResolver(nil),
		factory:          newManagedAPIFactory(client),
		suggestFallback:  suggestFallback,
		treeFallback:     treeFallback,
		semanticVerifier: semanticVerifier,
	}
}

func (r *managedCategoryResolver) Resolve(req *BuildRequest, canonical *canonical.Product, pkg *Package) *CategoryResolution {
	if req == nil {
		return r.fallback.Resolve(req, canonical, pkg)
	}

	api, note := r.buildAPI(req.SheinStoreID)
	resolver := NewCategoryResolverWithSemanticVerifier(api, r.suggestFallback, r.treeFallback, r.semanticVerifier)
	resolution := resolver.Resolve(req, canonical, pkg)
	if strings.TrimSpace(note) != "" {
		resolution.ReviewNotes = append(resolution.ReviewNotes, note)
		resolution.Status = "partial"
	}
	return resolution
}

func (r *managedCategoryResolver) buildAPI(storeID int64) (CategoryAPI, string) {
	baseAPIClient, note := r.factory.BuildBaseClient(storeID)
	if baseAPIClient == nil {
		return nil, note
	}
	return sheincategory.NewClient(baseAPIClient), ""
}
