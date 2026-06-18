package sheinmanaged

import (
	"task-processor/internal/catalog/canonical"
	"task-processor/internal/infra/clients/management"
	openaiclient "task-processor/internal/infra/clients/openai"
	sheinpub "task-processor/internal/publishing/shein"
)

type saleAttributeResolver struct {
	fallback sheinpub.SaleAttributeResolver
	factory  *apiFactory
	llm      openaiclient.ChatCompleter
}

func NewSaleAttributeResolver(client *management.ClientManager, llm openaiclient.ChatCompleter) sheinpub.SaleAttributeResolver {
	return &saleAttributeResolver{
		fallback: sheinpub.NewSaleAttributeResolver(nil, llm),
		factory:  newAPIFactory(client),
		llm:      llm,
	}
}

func (r *saleAttributeResolver) Resolve(req *sheinpub.BuildRequest, canonicalProduct *canonical.Product, pkg *sheinpub.Package) *sheinpub.SaleAttributeResolution {
	if req == nil {
		return r.fallback.Resolve(req, canonicalProduct, pkg)
	}

	api, note := r.buildAPI(req.SheinStoreID)
	resolver := sheinpub.NewSaleAttributeResolver(api, r.llm)
	resolution := resolver.Resolve(req, canonicalProduct, pkg)
	if note != "" {
		resolution.ReviewNotes = append(resolution.ReviewNotes, note)
		if resolution.Status == "" || resolution.Status == "unresolved" {
			resolution.Status = "partial"
		}
	}
	return resolution
}

func (r *saleAttributeResolver) buildAPI(storeID int64) (sheinpub.AttributeAPI, string) {
	return buildAttributeAPI(r.factory, storeID)
}
