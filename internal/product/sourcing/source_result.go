package sourcing

import "task-processor/internal/model"

// SourceProductResult is a normalized product result with its source identity.
type SourceProductResult struct {
	Identity SourceIdentity
	Product  *model.Product
	Error    error
}

// NormalizeAmazonBatchResults aligns raw Amazon crawler batch results with the
// requested source identities. Missing source results are represented as empty
// product results so callers can keep request/result accounting stable.
func NormalizeAmazonBatchResults(input AmazonCrawlRequestInput, productIDs []string, results []model.ProductResult) []SourceProductResult {
	input = normalizeAmazonCrawlRequestInput(input)
	normalized := make([]SourceProductResult, 0, len(productIDs))
	for index, productID := range productIDs {
		req := VariantSourceRequest(SourceRequest{
			Platform:  "amazon",
			Region:    input.Region,
			ProductID: input.ProductID,
			Zipcode:   input.Zipcode,
		}, productID)
		item := SourceProductResult{Identity: req.Identity()}
		if index < len(results) {
			item.Product = results[index].Product
			item.Error = results[index].Error
		}
		normalized = append(normalized, item)
	}
	return normalized
}
