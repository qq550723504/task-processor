package shein

import (
	listingsubmission "task-processor/internal/listing/submission"
	sheinproduct "task-processor/internal/shein/api/product"
)

type SubmitRemoteResult struct {
	Raw      *sheinproduct.SheinResponse
	Response *SubmissionResponse
}

func ExecuteSubmitRemote(productAPI sheinproduct.ProductAPI, action string, product *sheinproduct.Product) (*SubmitRemoteResult, error) {
	switch action {
	case listingsubmission.SubmitActionSaveDraft:
		raw, _, err := productAPI.SaveDraftProduct(product)
		return &SubmitRemoteResult{
			Raw:      raw,
			Response: BuildSubmissionResponseSummary(raw),
		}, err
	case listingsubmission.SubmitActionPublish:
		raw, _, err := productAPI.PublishProduct(product)
		return &SubmitRemoteResult{
			Raw:      raw,
			Response: BuildSubmissionResponseSummary(raw),
		}, err
	default:
		return nil, listingsubmission.UnsupportedSubmitActionError(action)
	}
}
