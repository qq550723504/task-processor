package shein

import (
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/productenrich"
	sheincategory "task-processor/internal/shein/api/category"
)

type managedCategoryResolver struct {
	fallback        CategoryResolver
	factory         *managedAPIFactory
	suggestFallback categorySuggestFallback
	treeFallback    categoryTreeFallback
}

func NewManagedCategoryResolver(client *management.ClientManager, llmClient ...openaiclient.ChatCompleter) CategoryResolver {
	var suggestFallback categorySuggestFallback
	var treeFallback categoryTreeFallback
	if len(llmClient) > 0 {
		suggestFallback = newAICategorySuggestFallback(llmClient[0])
		treeFallback = newAICategoryTreeFallback(llmClient[0])
	}
	return &managedCategoryResolver{
		fallback:        NewCategoryResolver(nil),
		factory:         newManagedAPIFactory(client),
		suggestFallback: suggestFallback,
		treeFallback:    treeFallback,
	}
}

func (r *managedCategoryResolver) Resolve(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) *CategoryResolution {
	if req == nil {
		return r.fallback.Resolve(req, canonical, pkg)
	}

	api, note := r.buildAPI(req.SheinStoreID)
	resolver := NewCategoryResolverWithFallbacks(api, r.suggestFallback, r.treeFallback)
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

func (r *managedCategoryResolver) SuggestAlternative(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) *CategorySuggestion {
	if req == nil {
		return nil
	}
	api, _ := r.buildAPI(req.SheinStoreID)
	resolver := NewCategoryResolverWithFallbacks(api, r.suggestFallback, r.treeFallback)
	recommender, ok := resolver.(categoryRecommender)
	if !ok {
		return nil
	}
	return recommender.SuggestAlternative(req, canonical, pkg)
}
