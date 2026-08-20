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
	if s.batchTaskLinkRepo != nil {
		if _, ok := s.batchTaskLinkRepo.(studioBatchTaskLinkLeaseRepository); !ok || strings.TrimSpace(candidate.ClaimToken) == "" {
			return fmt.Errorf("studio batch task claim lease token is unavailable")
		}
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
			appendStudioProductImageColorDirective(variantRequest, variant.Color)
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

func appendStudioProductImageColorDirective(request *StudioProductImageRequest, color string) {
	if request == nil {
		return
	}
	directive := fmt.Sprintf("Generate the product image for the SDS color variant %q. Keep the approved artwork identical, but match the base product color and material from this variant's SDS reference image.", firstNonEmpty(strings.TrimSpace(color), "this color variant"))
	for index := range request.ImagePrompts {
		request.ImagePrompts[index].Prompt = strings.TrimSpace(strings.Join([]string{
			strings.TrimSpace(request.ImagePrompts[index].Prompt),
			directive,
		}, "\n"))
	}
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
	leaseRepo, ok := s.batchTaskLinkRepo.(studioBatchTaskLinkLeaseRepository)
	if !ok || strings.TrimSpace(candidate.ClaimToken) == "" {
		return func() error { return fmt.Errorf("studio batch task claim lease token is unavailable") }
	}
	heartbeatCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	errCh := make(chan error, 1)
	refresh := func() error {
		if s.currentTime != nil {
			updatedAt := s.currentTime().UTC()
			refreshed, err := leaseRepo.RefreshStudioBatchTaskLink(heartbeatCtx, candidate.CandidateKey, candidate.ClaimToken, updatedAt)
			if err != nil {
				return err
			}
			if !refreshed {
				return fmt.Errorf("studio batch task claim is no longer owned")
			}
			return nil
		} else {
			refreshed, err := leaseRepo.RefreshStudioBatchTaskLink(heartbeatCtx, candidate.CandidateKey, candidate.ClaimToken, time.Now().UTC())
			if err != nil {
				return err
			}
			if !refreshed {
				return fmt.Errorf("studio batch task claim is no longer owned")
			}
			return nil
		}
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
