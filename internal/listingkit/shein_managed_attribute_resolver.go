package listingkit

import (
	"strings"

	"task-processor/internal/infra/clients/management"
	"task-processor/internal/productenrich"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type managedSheinAttributeResolver struct {
	fallback SheinAttributeResolver
	factory  *sheinManagedAPIFactory
}

func NewManagedSheinAttributeResolver(client *management.ClientManager) SheinAttributeResolver {
	return &managedSheinAttributeResolver{
		fallback: NewSheinAttributeResolver(nil),
		factory:  newSheinManagedAPIFactory(client),
	}
}

func (r *managedSheinAttributeResolver) Resolve(req *GenerateRequest, canonical *productenrich.CanonicalProduct, pkg *SheinPackage) *SheinAttributeResolution {
	if req == nil {
		return r.fallback.Resolve(req, canonical, pkg)
	}

	api, note := r.buildAPI(req.SheinStoreID)
	resolver := NewSheinAttributeResolver(api)
	resolution := resolver.Resolve(req, canonical, pkg)
	if strings.TrimSpace(note) != "" {
		resolution.ReviewNotes = append(resolution.ReviewNotes, note)
		if resolution.Status == "" || resolution.Status == "unresolved" {
			resolution.Status = "partial"
		}
	}
	return resolution
}

func (r *managedSheinAttributeResolver) buildAPI(storeID int64) (SheinAttributeAPI, string) {
	baseAPIClient, note := r.factory.BuildBaseClient(storeID)
	if baseAPIClient == nil {
		return nil, note
	}
	return sheinattribute.NewClient(baseAPIClient), ""
}
