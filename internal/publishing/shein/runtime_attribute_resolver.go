package shein

import (
	"context"

	"task-processor/internal/product/catalog/canonical"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type runtimeAttributeResolver struct {
	factory *runtimeAPIFactory
	llm     TextGenerator
}

func NewRuntimeAttributeResolver(factory RuntimeAPIClientFactory, llm TextGenerator) AttributeResolver {
	return &runtimeAttributeResolver{
		factory: newRuntimeAPIFactory(factory),
		llm:     llm,
	}
}

func (r *runtimeAttributeResolver) Resolve(req *BuildRequest, canonical *canonical.Product, pkg *Package) *AttributeResolution {
	if r == nil || req == nil {
		return nil
	}

	api := r.buildAPI(req.Context, req.SheinStoreID)
	if api == nil {
		return nil
	}
	resolver := NewAttributeResolver(api, r.llm)
	return resolver.Resolve(req, canonical, pkg)
}

func (r *runtimeAttributeResolver) buildAPI(ctx context.Context, storeID int64) AttributeAPI {
	if r == nil || r.factory == nil {
		return nil
	}
	baseAPIClient, _ := r.factory.BuildBaseClient(ctx, storeID)
	if baseAPIClient == nil {
		return nil
	}
	return sheinattribute.NewClient(baseAPIClient)
}
