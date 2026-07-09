package sourcing

import "strings"

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

// NormalizeSourceRequest trims source request fields and normalizes platform
// and region tokens for cross-crawler identity comparisons.
func NormalizeSourceRequest(req SourceRequest) SourceRequest {
	req.Platform = strings.ToLower(strings.TrimSpace(req.Platform))
	req.Region = strings.ToLower(strings.TrimSpace(req.Region))
	req.ProductID = strings.TrimSpace(req.ProductID)
	req.Zipcode = strings.TrimSpace(req.Zipcode)
	req.Creator = strings.TrimSpace(req.Creator)
	return req
}

// Identity returns the normalized source identity for a request.
func (req SourceRequest) Identity() SourceIdentity {
	normalized := NormalizeSourceRequest(req)
	return NormalizeSourceIdentity(SourceIdentity{
		SourceType:     SourceTypeCrawler,
		SourcePlatform: normalized.Platform,
		SourceID:       normalized.ProductID,
		Platform:       normalized.Platform,
		Region:         normalized.Region,
		ProductID:      normalized.ProductID,
		StoreID:        normalized.StoreID,
	})
}

// VariantSourceRequest builds a source request for one variant while preserving
// source-scoped fields from the parent product request.
func VariantSourceRequest(base SourceRequest, productID string) SourceRequest {
	base = NormalizeSourceRequest(base)
	base.ProductID = strings.TrimSpace(productID)
	return base
}
