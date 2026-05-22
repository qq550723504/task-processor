package shein

import (
	"context"
	"strings"

	"task-processor/internal/catalog/canonical"
	openaiclient "task-processor/internal/infra/clients/openai"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type runtimeAttributeResolver struct {
	fallback AttributeResolver
	factory  *runtimeAPIFactory
	llm      openaiclient.ChatCompleter
}

func NewRuntimeAttributeResolver(factory RuntimeAPIClientFactory, llm openaiclient.ChatCompleter) AttributeResolver {
	return &runtimeAttributeResolver{
		fallback: NewAttributeResolver(nil, llm),
		factory:  newRuntimeAPIFactory(factory),
		llm:      llm,
	}
}

func (r *runtimeAttributeResolver) Resolve(req *BuildRequest, canonical *canonical.Product, pkg *Package) *AttributeResolution {
	if req == nil {
		return r.fallback.Resolve(req, canonical, pkg)
	}

	api, note := r.buildAPI(req.Context, req.SheinStoreID)
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

func (r *runtimeAttributeResolver) buildAPI(ctx context.Context, storeID int64) (AttributeAPI, string) {
	baseAPIClient, note := r.factory.BuildBaseClient(ctx, storeID)
	if baseAPIClient == nil {
		return nil, note
	}
	return sheinattribute.NewClient(baseAPIClient), ""
}
