package sheinmanaged

import (
	"context"

	"task-processor/internal/catalog/canonical"
	openaiclient "task-processor/internal/infra/clients/openai"
	sheinpub "task-processor/internal/publishing/shein"
)

type categoryResolver struct {
	fallback sheinpub.CategoryResolver
	factory  *apiFactory
	aiConfig sheinpub.CategoryAIConfig
}

func NewCategoryResolver(llmClient ...openaiclient.ChatCompleter) sheinpub.CategoryResolver {
	var aiConfig sheinpub.CategoryAIConfig
	if len(llmClient) > 0 && llmClient[0] != nil {
		aiConfig.Selector = newCategorySelectorAdapter(llmClient[0])
		aiConfig.SemanticVerifier = llmClient[0]
	}
	return &categoryResolver{
		fallback: sheinpub.NewCategoryResolver(nil),
		factory:  newAPIFactory(),
		aiConfig: aiConfig,
	}
}

func (r *categoryResolver) Resolve(req *sheinpub.BuildRequest, canonicalProduct *canonical.Product, pkg *sheinpub.Package) *sheinpub.CategoryResolution {
	if req == nil {
		return r.fallback.Resolve(req, canonicalProduct, pkg)
	}

	api, note := r.buildAPI(req.Context, req.SheinStoreID)
	resolver := sheinpub.NewCategoryResolverWithAI(api, r.aiConfig)
	resolution := resolver.Resolve(req, canonicalProduct, pkg)
	if note != "" {
		resolution.ReviewNotes = append(resolution.ReviewNotes, note)
		resolution.Status = "partial"
	}
	return resolution
}

func (r *categoryResolver) buildAPI(_ context.Context, storeID int64) (sheinpub.CategoryAPI, string) {
	return buildCategoryAPI(r.factory, storeID)
}
