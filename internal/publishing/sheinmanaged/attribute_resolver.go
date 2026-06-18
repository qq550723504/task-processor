package sheinmanaged

import (
	"task-processor/internal/catalog/canonical"
	"task-processor/internal/infra/clients/management"
	openaiclient "task-processor/internal/infra/clients/openai"
	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type attributeResolver struct {
	fallback sheinpub.AttributeResolver
	factory  *apiFactory
	llm      openaiclient.ChatCompleter
}

func NewAttributeResolver(client *management.ClientManager, llm openaiclient.ChatCompleter) sheinpub.AttributeResolver {
	return &attributeResolver{
		fallback: sheinpub.NewAttributeResolver(nil, llm),
		factory:  newAPIFactory(client),
		llm:      llm,
	}
}

func (r *attributeResolver) Resolve(req *sheinpub.BuildRequest, canonicalProduct *canonical.Product, pkg *sheinpub.Package) *sheinpub.AttributeResolution {
	if req == nil {
		return r.fallback.Resolve(req, canonicalProduct, pkg)
	}

	api, note := r.buildAPI(req.SheinStoreID)
	resolver := sheinpub.NewAttributeResolver(api, r.llm)
	resolution := resolver.Resolve(req, canonicalProduct, pkg)
	if note != "" {
		resolution.ReviewNotes = append(resolution.ReviewNotes, note)
		if resolution.Status == "" || resolution.Status == "unresolved" {
			resolution.Status = "partial"
		}
	}
	return resolution
}

func (r *attributeResolver) buildAPI(storeID int64) (sheinpub.AttributeAPI, string) {
	baseAPIClient, note := r.factory.BuildBaseClient(storeID)
	if baseAPIClient == nil {
		return nil, note
	}
	return sheinattribute.NewClient(baseAPIClient), ""
}
