package shein

import (
	"context"

	"task-processor/internal/product/catalog/canonical"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type runtimeSaleAttributeResolver struct {
	factory     *runtimeAPIFactory
	llm         TextGenerator
	deniedStore ResolutionCacheStore
}

func NewRuntimeSaleAttributeResolver(factory RuntimeAPIClientFactory, llm TextGenerator, stores ...ResolutionCacheStore) SaleAttributeResolver {
	return &runtimeSaleAttributeResolver{
		factory:     newRuntimeAPIFactory(factory),
		llm:         llm,
		deniedStore: firstResolutionCacheStore(stores),
	}
}

func (r *runtimeSaleAttributeResolver) Resolve(req *BuildRequest, canonical *canonical.Product, pkg *Package) *SaleAttributeResolution {
	if r == nil || req == nil {
		return nil
	}

	api := r.buildAPI(req.Context, req.SheinStoreID)
	if api == nil {
		return nil
	}
	resolver := NewSaleAttributeResolverWithDeniedStore(api, r.llm, r.deniedStore)
	return resolver.Resolve(req, canonical, pkg)
}

func (r *runtimeSaleAttributeResolver) buildAPI(ctx context.Context, storeID int64) AttributeAPI {
	if r == nil || r.factory == nil {
		return nil
	}
	baseAPIClient, _ := r.factory.BuildBaseClient(ctx, storeID)
	if baseAPIClient == nil {
		return nil
	}
	return sheinattribute.NewClient(baseAPIClient)
}
