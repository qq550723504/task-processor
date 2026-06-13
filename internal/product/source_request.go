package product

import "task-processor/internal/product/sourcing"

// SourceRequestFromFetch converts a product fetch request into source-scoped
// request data used by sourcing helpers.
func SourceRequestFromFetch(req *FetchRequest) sourcing.SourceRequest {
	if req == nil {
		return sourcing.SourceRequest{}
	}
	return sourcing.SourceRequest{
		TenantID:   req.TenantID,
		Platform:   req.Platform,
		Region:     req.Region,
		ProductID:  req.ProductID,
		Zipcode:    req.Zipcode,
		StoreID:    req.StoreID,
		CategoryID: req.CategoryID,
		Creator:    req.Creator,
	}
}

// FetchRequestFromSource converts source-scoped request data back into the
// product fetch request shape used by product fetchers.
func FetchRequestFromSource(req sourcing.SourceRequest) *FetchRequest {
	return &FetchRequest{
		TenantID:   req.TenantID,
		Platform:   req.Platform,
		Region:     req.Region,
		ProductID:  req.ProductID,
		Zipcode:    req.Zipcode,
		StoreID:    req.StoreID,
		CategoryID: req.CategoryID,
		Creator:    req.Creator,
	}
}
