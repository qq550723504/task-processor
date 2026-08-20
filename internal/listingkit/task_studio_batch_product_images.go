package listingkit

import (
	"context"
	"fmt"
)

func (s *taskStudioBatchService) attachStudioBatchProductImages(
	ctx context.Context,
	request *GenerateRequest,
	session *SheinStudioSession,
	batch *StudioBatchRecord,
	candidate studioBatchTaskCandidate,
	design StudioMaterializedDesignRecord,
) error {
	if request == nil || request.Options == nil || request.Options.ImageStrategy != sheinImageStrategyAIGenerated {
		return nil
	}
	if request.Options.SheinStudio == nil {
		return fmt.Errorf("studio product image options are not configured")
	}
	if s == nil || s.generateProductImages == nil {
		return fmt.Errorf("studio product image generator is not configured")
	}
	response, err := s.generateProductImages(ctx, buildStudioBatchTaskProductImageRequest(session, batch, candidate, design))
	if err != nil {
		return fmt.Errorf("generate studio product images: %w", err)
	}
	productImageURLs := studioGeneratedProductImageURLs(response)
	if len(productImageURLs) == 0 {
		return fmt.Errorf("studio product image generator returned no images")
	}
	request.Options.SheinStudio.ProductImageURLs = productImageURLs
	return nil
}
