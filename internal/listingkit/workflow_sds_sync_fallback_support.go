package listingkit

import (
	"context"
	"strings"

	"task-processor/internal/productimage"
)

func needsLocalSDSMockupFallback(summary *SDSSyncSummary, options *SDSSyncOptions) bool {
	if summary == nil || options == nil || len(options.MockupImageURLs) == 0 {
		return false
	}
	renderedCount := len(uniqueNonEmptyStrings(summary.MockupImageURLs))
	if renderedCount == 0 {
		return true
	}
	expectedCount := len(uniqueNonEmptyStrings(options.MockupImageURLs))
	return expectedCount > 1 && renderedCount < expectedCount
}

func (s *service) applyLocalSDSMockupFallback(ctx context.Context, result *ListingKitResult, sourceURL string, options *SDSSyncOptions) {
	if result == nil || options == nil || len(options.MockupImageURLs) == 0 {
		return
	}
	result = normalizeListingKitResultSemanticFields(result)
	defer normalizeListingKitResultSemanticFields(result)
	rendered, err := s.renderLocalSDSMockups(ctx, localSDSMockupRenderInput{
		SourceURL:        sourceURL,
		MockupImageURLs:  options.MockupImageURLs,
		BlankDesignURL:   options.BlankDesignURL,
		TemplateImageURL: options.TemplateImageURL,
		MaskImageURL:     options.MaskImageURL,
	})
	if err != nil || len(rendered) == 0 {
		if err != nil {
			appendWarning(result, "local SDS mockup render failed: "+err.Error())
		}
		return
	}
	if result.SDSDesignResult == nil {
		result.SDSDesignResult = &SDSSyncSummary{VariantID: options.VariantID}
	}
	result.SDSDesignResult.MockupImageURLs = rendered
	result.SDSDesignResult.Status = "local_rendered"
	if result.SDSDesignResult.Error == "" {
		result.SDSDesignResult.Error = "SDS render unavailable; used local SDS mockup composite"
	}
	ensureResultPodExecution(result, nil)
}

func firstImageResultURL(imageResult *productimage.ImageProcessResult) string {
	if imageResult == nil {
		return ""
	}
	for _, asset := range []*productimage.ImageAsset{
		imageResult.MainImage,
		imageResult.WhiteBgImage,
		imageResult.SubjectCutout,
	} {
		if asset != nil && strings.TrimSpace(asset.URL) != "" {
			return strings.TrimSpace(asset.URL)
		}
		if asset != nil && strings.TrimSpace(asset.SourceURL) != "" {
			return strings.TrimSpace(asset.SourceURL)
		}
	}
	for _, asset := range imageResult.GalleryImages {
		if strings.TrimSpace(asset.URL) != "" {
			return strings.TrimSpace(asset.URL)
		}
		if strings.TrimSpace(asset.SourceURL) != "" {
			return strings.TrimSpace(asset.SourceURL)
		}
	}
	return ""
}
