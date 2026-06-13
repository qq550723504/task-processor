package sourcing

// SourceRequest carries source-identifying fields shared by product fetch paths.
type SourceRequest struct {
	TenantID   int64
	Platform   string
	Region     string
	ProductID  string
	Zipcode    string
	StoreID    int64
	CategoryID int64
	Creator    string
}

// VariantSourceRequest builds a source request for one variant while preserving
// source-scoped fields from the parent product request.
func VariantSourceRequest(base SourceRequest, productID string) SourceRequest {
	base.ProductID = productID
	return base
}
