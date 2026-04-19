package shein

import (
	"strings"

	"task-processor/internal/infra/clients/management"
	"task-processor/internal/productenrich"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type managedSaleAttributeResolver struct {
	fallback SaleAttributeResolver
	factory  *managedAPIFactory
}

func NewManagedSaleAttributeResolver(client *management.ClientManager) SaleAttributeResolver {
	return &managedSaleAttributeResolver{
		fallback: NewSaleAttributeResolver(nil),
		factory:  newManagedAPIFactory(client),
	}
}

func (r *managedSaleAttributeResolver) Resolve(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) *SaleAttributeResolution {
	if req == nil {
		return r.fallback.Resolve(req, canonical, pkg)
	}

	api, note := r.buildAPI(req.SheinStoreID)
	resolver := NewSaleAttributeResolver(api)
	resolution := resolver.Resolve(req, canonical, pkg)
	if strings.TrimSpace(note) != "" {
		resolution.ReviewNotes = append(resolution.ReviewNotes, note)
		if resolution.Status == "" || resolution.Status == "unresolved" {
			resolution.Status = "partial"
		}
	}
	return resolution
}

func (r *managedSaleAttributeResolver) buildAPI(storeID int64) (AttributeAPI, string) {
	baseAPIClient, note := r.factory.BuildBaseClient(storeID)
	if baseAPIClient == nil {
		return nil, note
	}
	return sheinattribute.NewClient(baseAPIClient), ""
}
