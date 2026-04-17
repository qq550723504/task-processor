package listingkit

import (
	"strings"

	"task-processor/internal/infra/clients/management"
	"task-processor/internal/productenrich"
	sheincategory "task-processor/internal/shein/api/category"
)

type managedSheinCategoryResolver struct {
	fallback SheinCategoryResolver
	factory  *sheinManagedAPIFactory
}

func NewManagedSheinCategoryResolver(client *management.ClientManager) SheinCategoryResolver {
	return &managedSheinCategoryResolver{
		fallback: NewSheinCategoryResolver(nil),
		factory:  newSheinManagedAPIFactory(client),
	}
}

func (r *managedSheinCategoryResolver) Resolve(req *GenerateRequest, canonical *productenrich.CanonicalProduct, pkg *SheinPackage) *SheinCategoryResolution {
	if req == nil {
		return r.fallback.Resolve(req, canonical, pkg)
	}

	api, note := r.buildAPI(req.SheinStoreID)
	resolver := NewSheinCategoryResolver(api)
	resolution := resolver.Resolve(req, canonical, pkg)
	if strings.TrimSpace(note) != "" {
		resolution.ReviewNotes = append(resolution.ReviewNotes, note)
		resolution.Status = "partial"
	}
	return resolution
}

func (r *managedSheinCategoryResolver) buildAPI(storeID int64) (SheinCategoryAPI, string) {
	baseAPIClient, note := r.factory.BuildBaseClient(storeID)
	if baseAPIClient == nil {
		return nil, note
	}
	return sheincategory.NewClient(baseAPIClient), ""
}
