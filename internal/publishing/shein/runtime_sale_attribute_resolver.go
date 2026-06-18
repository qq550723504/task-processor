package shein

import (
	"context"
	"strings"

	"task-processor/internal/catalog/canonical"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type runtimeSaleAttributeResolver struct {
	fallback    SaleAttributeResolver
	factory     *runtimeAPIFactory
	llm         TextGenerator
	deniedStore ResolutionCacheStore
}

func NewRuntimeSaleAttributeResolver(factory RuntimeAPIClientFactory, llm TextGenerator, stores ...ResolutionCacheStore) SaleAttributeResolver {
	return &runtimeSaleAttributeResolver{
		fallback:    NewSaleAttributeResolverWithDeniedStore(nil, llm, firstResolutionCacheStore(stores)),
		factory:     newRuntimeAPIFactory(factory),
		llm:         llm,
		deniedStore: firstResolutionCacheStore(stores),
	}
}

func (r *runtimeSaleAttributeResolver) Resolve(req *BuildRequest, canonical *canonical.Product, pkg *Package) *SaleAttributeResolution {
	if req == nil {
		return r.fallback.Resolve(req, canonical, pkg)
	}

	api, note := r.buildAPI(req.Context, req.SheinStoreID)
	resolver := NewSaleAttributeResolverWithDeniedStore(api, r.llm, r.deniedStore)
	resolution := resolver.Resolve(req, canonical, pkg)
	if strings.TrimSpace(note) != "" {
		resolution.ReviewNotes = append(resolution.ReviewNotes, note)
		if resolution.Status == "" || resolution.Status == "unresolved" {
			resolution.Status = "partial"
		}
	}
	return resolution
}

func (r *runtimeSaleAttributeResolver) buildAPI(ctx context.Context, storeID int64) (AttributeAPI, string) {
	baseAPIClient, note := r.factory.BuildBaseClient(ctx, storeID)
	if baseAPIClient == nil {
		return nil, note
	}
	return sheinattribute.NewClient(baseAPIClient), ""
}
