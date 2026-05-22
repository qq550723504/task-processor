package shein

import (
	"context"
	"strings"

	"task-processor/internal/catalog/canonical"
	openaiclient "task-processor/internal/infra/clients/openai"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type runtimeSaleAttributeResolver struct {
	fallback SaleAttributeResolver
	factory  *runtimeAPIFactory
	llm      openaiclient.ChatCompleter
}

func NewRuntimeSaleAttributeResolver(factory RuntimeAPIClientFactory, llm openaiclient.ChatCompleter) SaleAttributeResolver {
	return &runtimeSaleAttributeResolver{
		fallback: NewSaleAttributeResolver(nil, llm),
		factory:  newRuntimeAPIFactory(factory),
		llm:      llm,
	}
}

func (r *runtimeSaleAttributeResolver) Resolve(req *BuildRequest, canonical *canonical.Product, pkg *Package) *SaleAttributeResolution {
	if req == nil {
		return r.fallback.Resolve(req, canonical, pkg)
	}

	api, note := r.buildAPI(req.Context, req.SheinStoreID)
	resolver := NewSaleAttributeResolver(api, r.llm)
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
