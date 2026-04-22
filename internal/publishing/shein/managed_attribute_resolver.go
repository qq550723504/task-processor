package shein

import (
	"strings"

	"task-processor/internal/infra/clients/management"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/productenrich"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type managedAttributeResolver struct {
	fallback AttributeResolver
	factory  *managedAPIFactory
	llm      openaiclient.ChatCompleter
}

func NewManagedAttributeResolver(client *management.ClientManager, llm openaiclient.ChatCompleter) AttributeResolver {
	return &managedAttributeResolver{
		fallback: NewAttributeResolver(nil, llm),
		factory:  newManagedAPIFactory(client),
		llm:      llm,
	}
}

func (r *managedAttributeResolver) Resolve(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) *AttributeResolution {
	if req == nil {
		return r.fallback.Resolve(req, canonical, pkg)
	}

	api, note := r.buildAPI(req.SheinStoreID)
	resolver := NewAttributeResolver(api, r.llm)
	resolution := resolver.Resolve(req, canonical, pkg)
	if strings.TrimSpace(note) != "" {
		resolution.ReviewNotes = append(resolution.ReviewNotes, note)
		if resolution.Status == "" || resolution.Status == "unresolved" {
			resolution.Status = "partial"
		}
	}
	return resolution
}

func (r *managedAttributeResolver) buildAPI(storeID int64) (AttributeAPI, string) {
	baseAPIClient, note := r.factory.BuildBaseClient(storeID)
	if baseAPIClient == nil {
		return nil, note
	}
	return sheinattribute.NewClient(baseAPIClient), ""
}
