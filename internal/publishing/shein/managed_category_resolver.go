package shein

import (
	"strings"

	"task-processor/internal/infra/clients/management"
	"task-processor/internal/productenrich"
	sheincategory "task-processor/internal/shein/api/category"
)

type managedCategoryResolver struct {
	fallback CategoryResolver
	factory  *managedAPIFactory
}

func NewManagedCategoryResolver(client *management.ClientManager) CategoryResolver {
	return &managedCategoryResolver{
		fallback: NewCategoryResolver(nil),
		factory:  newManagedAPIFactory(client),
	}
}

func (r *managedCategoryResolver) Resolve(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) *CategoryResolution {
	if req == nil {
		return r.fallback.Resolve(req, canonical, pkg)
	}

	api, note := r.buildAPI(req.SheinStoreID)
	resolver := NewCategoryResolver(api)
	resolution := resolver.Resolve(req, canonical, pkg)
	if strings.TrimSpace(note) != "" {
		resolution.ReviewNotes = append(resolution.ReviewNotes, note)
		resolution.Status = "partial"
	}
	return resolution
}

func (r *managedCategoryResolver) buildAPI(storeID int64) (CategoryAPI, string) {
	baseAPIClient, note := r.factory.BuildBaseClient(storeID)
	if baseAPIClient == nil {
		return nil, note
	}
	return sheincategory.NewClient(baseAPIClient), ""
}
