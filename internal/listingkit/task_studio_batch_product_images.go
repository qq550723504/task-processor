package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const studioBatchTaskLinkHeartbeatInterval = 30 * time.Second

type studioBatchTaskLinkHeartbeatContextKey struct{}

type taskDispatchCancellationContextKey struct{}

func withStudioBatchTaskLinkHeartbeat(ctx context.Context) context.Context {
	return context.WithValue(ctx, studioBatchTaskLinkHeartbeatContextKey{}, true)
}

func hasStudioBatchTaskLinkHeartbeat(ctx context.Context) bool {
	active, _ := ctx.Value(studioBatchTaskLinkHeartbeatContextKey{}).(bool)
	return active
}

func taskDispatchCancellationPreserved(ctx context.Context) bool {
	preserve, _ := ctx.Value(taskDispatchCancellationContextKey{}).(bool)
	return preserve
}

func withTaskDispatchCancellation(ctx context.Context) context.Context {
	return context.WithValue(ctx, taskDispatchCancellationContextKey{}, true)
}

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
	heartbeatStop := func() error { return nil }
	if !hasStudioBatchTaskLinkHeartbeat(ctx) {
		heartbeatStop = s.startStudioBatchTaskLinkHeartbeat(ctx, candidate, studioBatchTaskLinkHeartbeatInterval)
	}
	productImageRequest, err := s.buildStudioBatchTaskProductImageRequest(ctx, session, batch, candidate, design)
	if err != nil {
		_ = heartbeatStop()
		return err
	}
	colorRepresentatives := studioBatchTaskColorRepresentatives(candidate.SelectionSnapshot)
	if len(colorRepresentatives) > 1 {
		productImageRequest.ProductReferenceImageURLs = studioBatchTaskProductReferenceImageURLsForVariant(candidate.SelectionSnapshot, colorRepresentatives[0])
	}
	firstProductImageRequest := productImageRequest
	if len(colorRepresentatives) > 1 {
		// Keep the shared request pristine so each later color clone receives
		// only its own directive. The first representative needs the same
		// color-specific instruction as the later requests.
		firstProductImageRequest = cloneStudioBatchProductImageRequest(productImageRequest)
		appendStudioProductImageColorDirective(firstProductImageRequest, colorRepresentatives[0].Color)
	}
	if err := s.authorizeStudioBatchProductImageUsage(ctx, batch, 1); err != nil {
		_ = heartbeatStop()
		return fmt.Errorf("authorize studio product image usage: %w", err)
	}
	response, err := s.generateProductImages(ctx, firstProductImageRequest)
	if err != nil {
		_ = heartbeatStop()
		return fmt.Errorf("generate studio product images: %w", err)
	}
	productImageURLs := studioGeneratedProductImageURLs(response)
	if len(productImageURLs) == 0 {
		_ = heartbeatStop()
		return fmt.Errorf("studio product image generator returned no images")
	}
	productImageURLs, err = s.publicizeStudioBatchProductImageURLs(ctx, productImageURLs)
	if err != nil {
		_ = heartbeatStop()
		return err
	}
	if err := s.recordStudioBatchProductImageUsage(ctx, batch, 1); err != nil {
		_ = heartbeatStop()
		return fmt.Errorf("record studio product image usage: %w", err)
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
			if err := s.authorizeStudioBatchProductImageUsage(ctx, batch, 1); err != nil {
				_ = heartbeatStop()
				return fmt.Errorf("authorize studio product image usage for color %q: %w", variant.Color, err)
			}
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
			variantURLs, variantErr = s.publicizeStudioBatchProductImageURLs(ctx, variantURLs)
			if variantErr != nil {
				_ = heartbeatStop()
				return fmt.Errorf("publicize studio product images for color %q: %w", variant.Color, variantErr)
			}
			if variantErr = s.recordStudioBatchProductImageUsage(ctx, batch, 1); variantErr != nil {
				_ = heartbeatStop()
				return fmt.Errorf("record studio product image usage for color %q: %w", variant.Color, variantErr)
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

func (s *taskStudioBatchService) authorizeStudioBatchProductImageUsage(ctx context.Context, batch *StudioBatchRecord, quantity int) error {
	if s == nil || s.productImageUsage == nil {
		return nil
	}
	tenantID := studioBatchTaskGateTenantID(ctx, batch)
	if strings.TrimSpace(tenantID) == "" {
		return fmt.Errorf("tenant id is required")
	}
	return s.productImageUsage.AuthorizeProductImageUsage(ctx, tenantID, quantity)
}

func (s *taskStudioBatchService) recordStudioBatchProductImageUsage(ctx context.Context, batch *StudioBatchRecord, quantity int) error {
	if s == nil || s.productImageUsage == nil {
		return nil
	}
	tenantID := studioBatchTaskGateTenantID(ctx, batch)
	if strings.TrimSpace(tenantID) == "" {
		return fmt.Errorf("tenant id is required")
	}
	return s.productImageUsage.RecordProductImageUsage(ctx, tenantID, quantity)
}

func (s *taskStudioBatchService) publicizeStudioBatchProductImageURLs(ctx context.Context, urls []string) ([]string, error) {
	publicURLs := append([]string(nil), urls...)
	if s == nil {
		return publicURLs, nil
	}
	for index, rawURL := range publicURLs {
		key, ok := studioReferenceUploadedImageKeyFromURL(rawURL)
		if !ok {
			continue
		}
		if s.resolveUploadedImagePublicURL == nil {
			return nil, fmt.Errorf("resolve generated upload %q: uploaded image public url resolver is not configured", key)
		}
		resolved, err := s.resolveUploadedImagePublicURL(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("resolve generated upload %q: %w", key, err)
		}
		resolved = strings.TrimSpace(resolved)
		if resolved == "" {
			return nil, fmt.Errorf("resolve generated upload %q: public url is empty", key)
		}
		publicURLs[index] = resolved
	}
	return publicURLs, nil
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
	if len(request.ImagePrompts) == 0 {
		if strings.TrimSpace(request.CustomPrompt) != "" {
			request.CustomPrompt = strings.TrimSpace(strings.Join([]string{
				strings.TrimSpace(request.CustomPrompt),
				directive,
			}, "\n"))
		} else {
			request.Prompt = strings.TrimSpace(strings.Join([]string{
				strings.TrimSpace(request.Prompt),
				directive,
			}, "\n"))
		}
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
	_, stop := s.startStudioBatchTaskLinkHeartbeatContext(ctx, candidate, interval)
	return stop
}

func (s *taskStudioBatchService) startStudioBatchTaskLinkHeartbeatContext(
	ctx context.Context,
	candidate studioBatchTaskCandidate,
	interval time.Duration,
) (context.Context, func() error) {
	if s == nil || s.batchTaskLinkRepo == nil || interval <= 0 {
		return ctx, func() error { return nil }
	}
	leaseRepo, ok := s.batchTaskLinkRepo.(studioBatchTaskLinkLeaseRepository)
	if !ok || strings.TrimSpace(candidate.ClaimToken) == "" {
		return ctx, func() error { return fmt.Errorf("studio batch task claim lease token is unavailable") }
	}
	// The caller may disconnect while inline task execution is still expected
	// to finish. Keep the lease heartbeat and dispatch alive independently of
	// that request context; refresh failure remains the explicit cancellation
	// signal for a lost claim.
	heartbeatCtx, cancel := context.WithCancel(DetachedRequestContext(ctx))
	terminalStateCtx := DetachedRequestContext(ctx)
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
					// A lost lease must stop the in-flight task dispatch. Without
					// cancellation the stale worker can still create a duplicate
					// before the guarded terminal update notices the loss.
					cancel()
					return
				}
			}
		}
	}()
	return heartbeatCtx, func() error {
		cancel()
		<-done
		select {
		case err := <-errCh:
			if errors.Is(err, context.Canceled) {
				return nil
			}
			// The worker may have completed the terminal compare-and-update
			// while a refresh was in flight. That transition intentionally
			// makes the creating predicate false; it is not a lost lease.
			if s.studioBatchTaskHeartbeatEndedInTerminalState(terminalStateCtx, candidate) {
				return nil
			}
			return fmt.Errorf("refresh studio batch task claim: %w", err)
		default:
			return nil
		}
	}
}

func (s *taskStudioBatchService) studioBatchTaskHeartbeatEndedInTerminalState(ctx context.Context, candidate studioBatchTaskCandidate) bool {
	if s == nil || s.batchTaskLinkRepo == nil {
		return false
	}
	link, err := s.batchTaskLinkRepo.GetStudioBatchTaskLinkByCandidateKey(ctx, candidate.CandidateKey)
	if err != nil || link == nil {
		return false
	}
	if strings.TrimSpace(link.Status) == studioBatchTaskLinkStatusCreating {
		return false
	}
	claimToken := strings.TrimSpace(candidate.ClaimToken)
	return claimToken != "" && strings.TrimSpace(link.ClaimToken) == claimToken
}
