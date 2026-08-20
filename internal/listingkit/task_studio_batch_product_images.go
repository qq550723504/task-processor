package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const studioBatchTaskLinkHeartbeatInterval = 30 * time.Second

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
	heartbeatStop := s.startStudioBatchTaskLinkHeartbeat(ctx, candidate, studioBatchTaskLinkHeartbeatInterval)
	productImageRequest, err := s.buildStudioBatchTaskProductImageRequest(ctx, session, batch, candidate, design)
	if err != nil {
		_ = heartbeatStop()
		return err
	}
	colorRepresentatives := studioBatchTaskColorRepresentatives(candidate.SelectionSnapshot)
	if len(colorRepresentatives) > 1 {
		productImageRequest.ProductReferenceImageURLs = studioBatchTaskProductReferenceImageURLsForVariant(candidate.SelectionSnapshot, colorRepresentatives[0])
	}
	response, err := s.generateProductImages(ctx, productImageRequest)
	if err != nil {
		_ = heartbeatStop()
		return fmt.Errorf("generate studio product images: %w", err)
	}
	productImageURLs := studioGeneratedProductImageURLs(response)
	if len(productImageURLs) == 0 {
		_ = heartbeatStop()
		return fmt.Errorf("studio product image generator returned no images")
	}
	request.Options.SheinStudio.ProductImageURLs = productImageURLs
	if len(colorRepresentatives) > 1 {
		request.Options.SheinStudio.VariantProductImages = append(request.Options.SheinStudio.VariantProductImages, SheinStudioVariantImageSet{
			VariantSKU: colorRepresentatives[0].VariantSKU,
			Color:      colorRepresentatives[0].Color,
			ImageURLs:  append([]string(nil), productImageURLs...),
		})
		for _, variant := range colorRepresentatives[1:] {
			variantRequest := cloneStudioBatchProductImageRequest(productImageRequest)
			variantRequest.StyleName = firstNonEmpty(productImageRequest.StyleName, "Style") + " " + firstNonEmpty(strings.TrimSpace(variant.Color), "this color variant")
			variantRequest.ProductReferenceImageURLs = studioBatchTaskProductReferenceImageURLsForVariant(candidate.SelectionSnapshot, variant)
			variantRequest.CustomPrompt = strings.TrimSpace(strings.Join([]string{
				strings.TrimSpace(productImageRequest.CustomPrompt),
				fmt.Sprintf("Generate the product image for the SDS color variant %q. Keep the approved artwork identical, but match the base product color and material from this variant's SDS reference image.", firstNonEmpty(strings.TrimSpace(variant.Color), "this color variant")),
			}, "\n"))
			variantResponse, variantErr := s.generateProductImages(ctx, variantRequest)
			if variantErr != nil {
				_ = heartbeatStop()
				return fmt.Errorf("generate studio product images for color %q: %w", variant.Color, variantErr)
			}
			variantURLs := studioGeneratedProductImageURLs(variantResponse)
			if len(variantURLs) == 0 {
				_ = heartbeatStop()
				return fmt.Errorf("studio product image generator returned no images for color %q", variant.Color)
			}
			request.Options.SheinStudio.VariantProductImages = append(request.Options.SheinStudio.VariantProductImages, SheinStudioVariantImageSet{
				VariantSKU: variant.VariantSKU,
				Color:      variant.Color,
				ImageURLs:  variantURLs,
			})
		}
	}
	if err := heartbeatStop(); err != nil {
		return err
	}
	return nil
}

func (s *taskStudioBatchService) buildStudioBatchTaskProductImageRequest(
	ctx context.Context,
	session *SheinStudioSession,
	batch *StudioBatchRecord,
	candidate studioBatchTaskCandidate,
	design StudioMaterializedDesignRecord,
) (*StudioProductImageRequest, error) {
	request := buildStudioBatchTaskProductImageRequest(session, batch, candidate, design)
	if s == nil || s.sdsProductDetailProvider == nil || candidate.SelectionSnapshot.ParentProductID <= 0 {
		return request, nil
	}
	detail, err := s.sdsProductDetailProvider.GetProductDetail(ctx, candidate.SelectionSnapshot.ParentProductID)
	if err != nil {
		return nil, fmt.Errorf("load SDS product category %d: %w", candidate.SelectionSnapshot.ParentProductID, err)
	}
	request.CategoryPath = studioProductImageCategoryPath(detail)
	return request, nil
}

func cloneStudioBatchProductImageRequest(input *StudioProductImageRequest) *StudioProductImageRequest {
	if input == nil {
		return nil
	}
	cloned := *input
	cloned.CategoryPath = append([]string(nil), input.CategoryPath...)
	cloned.ProductReferenceImageURLs = append([]string(nil), input.ProductReferenceImageURLs...)
	cloned.ImagePrompts = append([]StudioProductImagePrompt(nil), input.ImagePrompts...)
	return &cloned
}

func (s *taskStudioBatchService) startStudioBatchTaskLinkHeartbeat(
	ctx context.Context,
	candidate studioBatchTaskCandidate,
	interval time.Duration,
) func() error {
	if s == nil || s.batchTaskLinkRepo == nil || interval <= 0 {
		return func() error { return nil }
	}
	heartbeatCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	errCh := make(chan error, 1)
	refresh := func() error {
		link, err := s.batchTaskLinkRepo.GetStudioBatchTaskLinkByCandidateKey(heartbeatCtx, candidate.CandidateKey)
		if err != nil {
			return err
		}
		if link == nil || link.Status != studioBatchTaskLinkStatusCreating || strings.TrimSpace(link.ListingKitTaskID) != "" {
			return fmt.Errorf("studio batch task claim is no longer owned")
		}
		if s.currentTime != nil {
			link.UpdatedAt = s.currentTime().UTC()
		} else {
			link.UpdatedAt = time.Now().UTC()
		}
		return s.batchTaskLinkRepo.UpdateStudioBatchTaskLink(heartbeatCtx, link)
	}
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				if err := refresh(); err != nil {
					select {
					case errCh <- err:
					default:
					}
					return
				}
			}
		}
	}()
	return func() error {
		cancel()
		<-done
		select {
		case err := <-errCh:
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("refresh studio batch task claim: %w", err)
		default:
			return nil
		}
	}
}
