package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"task-processor/internal/listingsubscription"
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
	if s.productImageUsage == nil {
		return listingsubscription.ErrSubscriptionRequired
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
	productImageGenerationCount := studioBatchTaskProductImageGenerationCount(candidate.SelectionSnapshot)
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
	if err := s.authorizeStudioBatchProductImageUsage(ctx, batch, candidate, productImageGenerationCount); err != nil {
		_ = heartbeatStop()
		return fmt.Errorf("authorize studio product image usage for %d generation jobs: %w", productImageGenerationCount, err)
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
			variantURLs, variantErr = s.publicizeStudioBatchProductImageURLs(ctx, variantURLs)
			if variantErr != nil {
				_ = heartbeatStop()
				return fmt.Errorf("publicize studio product images for color %q: %w", variant.Color, variantErr)
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

func studioBatchTaskProductImageGenerationCount(selection SheinStudioSelection) int {
	if count := len(studioBatchTaskColorRepresentatives(selection)); count > 0 {
		return count
	}
	return 1
}

func studioBatchTaskProductImageUsageReservationID(candidate studioBatchTaskCandidate) string {
	reservationID := strings.TrimSpace(candidate.CandidateKey)
	if token := strings.TrimSpace(candidate.ClaimToken); token != "" {
		reservationID += "|" + token
	}
	return reservationID
}

// settleStudioBatchProductImageUsage is called only after the generated task
// and its terminal durable link have committed. Generation and authorization
// happen earlier, but a failed task/link must not consume the quota.
func (s *taskStudioBatchService) settleStudioBatchProductImageUsage(ctx context.Context, batch *StudioBatchRecord, candidate studioBatchTaskCandidate) error {
	if normalizeSheinImageStrategy(candidate.ImageStrategy) != sheinImageStrategyAIGenerated {
		return nil
	}
	if settled, err := s.studioBatchProductImageUsageAlreadySettled(ctx, candidate); err != nil {
		return err
	} else if settled {
		return nil
	}
	var err error
	settlementClaimed := false
	if reservation, ok := s.productImageUsageReservation(); ok {
		tenantID := studioBatchTaskGateTenantID(ctx, batch)
		if strings.TrimSpace(tenantID) == "" {
			return fmt.Errorf("tenant id is required")
		}
		reservationID := studioBatchTaskProductImageUsageReservationID(candidate)
		if reservationID == "" {
			return fmt.Errorf("product image usage reservation id is required")
		}
		err = reservation.CommitProductImageUsage(ctx, tenantID, reservationID)
	} else {
		quantity := studioBatchTaskProductImageGenerationCount(candidate.SelectionSnapshot)
		if _, idempotent := s.productImageUsage.(StudioProductImageUsageIdempotent); idempotent {
			// The operation identity is durable in the legacy counter repository,
			// so a crash before the link marker is persisted can be retried safely.
			err = s.recordStudioBatchProductImageUsageOnce(ctx, batch, quantity, candidate)
		} else {
			// Keep the atomic claim fallback for older adapters that do not expose
			// an idempotent counter operation.
			if s.batchTaskLinkRepo != nil {
				settlementClaimed, err = s.markStudioBatchProductImageUsageSettled(ctx, candidate)
				if err != nil {
					return err
				}
				if !settlementClaimed {
					return nil
				}
			}
			err = s.recordStudioBatchProductImageUsage(ctx, batch, quantity)
			if err != nil && s.batchTaskLinkRepo != nil {
				if clearErr := s.clearStudioBatchProductImageUsageSettled(ctx, candidate); clearErr != nil {
					err = errors.Join(err, fmt.Errorf("clear legacy settlement claim: %w", clearErr))
				}
			}
		}
	}
	if err != nil {
		return err
	}
	if settlementClaimed {
		return nil
	}
	_, err = s.markStudioBatchProductImageUsageSettled(ctx, candidate)
	return err
}

func (s *taskStudioBatchService) studioBatchProductImageUsageAlreadySettled(ctx context.Context, candidate studioBatchTaskCandidate) (bool, error) {
	if s == nil || s.batchTaskLinkRepo == nil {
		return false, nil
	}
	link, err := s.batchTaskLinkRepo.GetStudioBatchTaskLinkByCandidateKey(ctx, strings.TrimSpace(candidate.CandidateKey))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return link != nil && link.ProductImageUsageSettled, nil
}

func (s *taskStudioBatchService) markStudioBatchProductImageUsageSettled(ctx context.Context, candidate studioBatchTaskCandidate) (bool, error) {
	if s == nil || s.batchTaskLinkRepo == nil {
		return true, nil
	}
	claimer, ok := s.batchTaskLinkRepo.(studioBatchTaskLinkUsageSettlementRepository)
	if !ok {
		return false, fmt.Errorf("studio batch task link repository lacks atomic usage settlement claim")
	}
	claimed, err := claimer.ClaimStudioBatchProductImageUsageSettled(ctx, strings.TrimSpace(candidate.CandidateKey), s.currentTime().UTC())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if claimed {
		return true, nil
	}
	link, err := s.batchTaskLinkRepo.GetStudioBatchTaskLinkByCandidateKey(ctx, strings.TrimSpace(candidate.CandidateKey))
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && link != nil && link.ProductImageUsageSettled) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return false, fmt.Errorf("studio batch task link settlement claim was lost")
}

func (s *taskStudioBatchService) authorizeStudioBatchProductImageUsage(ctx context.Context, batch *StudioBatchRecord, candidate studioBatchTaskCandidate, quantity int) error {
	if s == nil || s.productImageUsage == nil {
		return listingsubscription.ErrSubscriptionRequired
	}
	tenantID := studioBatchTaskGateTenantID(ctx, batch)
	if strings.TrimSpace(tenantID) == "" {
		return fmt.Errorf("tenant id is required")
	}
	ledgerAdmission := s.generationUsageAdmission == nil || s.generationUsageAdmission.AllowsGenerationUsage(tenantID)
	if ledgerAdmission {
		if reservation, ok := s.productImageUsageReservation(); ok {
			reservationID := studioBatchTaskProductImageUsageReservationID(candidate)
			if reservationID == "" {
				return fmt.Errorf("product image usage reservation id is required")
			}
			return reservation.ReserveProductImageUsage(ctx, tenantID, reservationID, quantity)
		}
	}
	return s.productImageUsage.AuthorizeProductImageUsage(ctx, tenantID, quantity)
}

func (s *taskStudioBatchService) productImageUsageReservation() (StudioProductImageUsageReservation, bool) {
	if s == nil || s.productImageUsage == nil {
		return nil, false
	}
	if availability, ok := s.productImageUsage.(StudioProductImageUsageReservationAvailability); ok && !availability.StudioProductImageUsageReservationEnabled() {
		return nil, false
	}
	reservation, ok := s.productImageUsage.(StudioProductImageUsageReservation)
	return reservation, ok && reservation != nil
}

func (s *taskStudioBatchService) releaseStudioBatchProductImageUsage(ctx context.Context, batch *StudioBatchRecord, candidate studioBatchTaskCandidate, reason string) error {
	reservation, ok := s.productImageUsageReservation()
	if !ok || normalizeSheinImageStrategy(candidate.ImageStrategy) != sheinImageStrategyAIGenerated {
		return nil
	}
	tenantID := studioBatchTaskGateTenantID(ctx, batch)
	if strings.TrimSpace(tenantID) == "" {
		return fmt.Errorf("tenant id is required")
	}
	reservationID := studioBatchTaskProductImageUsageReservationID(candidate)
	if reservationID == "" {
		return fmt.Errorf("product image usage reservation id is required")
	}
	return reservation.ReleaseProductImageUsage(ctx, tenantID, reservationID, reason)
}

func (s *taskStudioBatchService) releasePendingStudioBatchProductImageUsage(ctx context.Context, batch *StudioBatchRecord, candidate studioBatchTaskCandidate) error {
	if s == nil || s.batchTaskLinkRepo == nil {
		return nil
	}
	link, err := s.batchTaskLinkRepo.GetStudioBatchTaskLinkByCandidateKey(ctx, strings.TrimSpace(candidate.CandidateKey))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if link == nil || strings.TrimSpace(link.PendingProductImageUsageReleaseClaimToken) == "" {
		return nil
	}
	pending := candidate
	pending.ClaimToken = link.PendingProductImageUsageReleaseClaimToken
	if err := s.releaseStudioBatchProductImageUsage(ctx, batch, pending, "pending_release_retry"); err != nil {
		return err
	}
	link.PendingProductImageUsageReleaseClaimToken = ""
	link.UpdatedAt = s.currentTime().UTC()
	if leaseRepo, ok := s.batchTaskLinkRepo.(studioBatchTaskLinkLeaseRepository); ok && link.Status == studioBatchTaskLinkStatusCreating && strings.TrimSpace(link.ClaimToken) != "" {
		updated, updateErr := leaseRepo.UpdateStudioBatchTaskLinkWithClaimToken(ctx, link, link.ClaimToken)
		if updateErr != nil {
			return updateErr
		}
		if !updated {
			return fmt.Errorf("studio batch task claim is no longer owned while clearing pending release")
		}
		return nil
	}
	return s.batchTaskLinkRepo.UpdateStudioBatchTaskLink(ctx, link)
}

func (s *taskStudioBatchService) clearStudioBatchProductImageUsageSettled(ctx context.Context, candidate studioBatchTaskCandidate) error {
	if s == nil || s.batchTaskLinkRepo == nil {
		return nil
	}
	link, err := s.batchTaskLinkRepo.GetStudioBatchTaskLinkByCandidateKey(ctx, strings.TrimSpace(candidate.CandidateKey))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if link == nil || !link.ProductImageUsageSettled {
		return nil
	}
	link.ProductImageUsageSettled = false
	link.UpdatedAt = s.currentTime().UTC()
	return s.batchTaskLinkRepo.UpdateStudioBatchTaskLink(ctx, link)
}

func (s *taskStudioBatchService) clearPendingStudioBatchProductImageUsageRelease(ctx context.Context, candidate studioBatchTaskCandidate) error {
	if s == nil || s.batchTaskLinkRepo == nil {
		return nil
	}
	link, err := s.batchTaskLinkRepo.GetStudioBatchTaskLinkByCandidateKey(ctx, strings.TrimSpace(candidate.CandidateKey))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if link == nil || strings.TrimSpace(link.PendingProductImageUsageReleaseClaimToken) == "" {
		return nil
	}
	link.PendingProductImageUsageReleaseClaimToken = ""
	link.UpdatedAt = s.currentTime().UTC()
	if leaseRepo, ok := s.batchTaskLinkRepo.(studioBatchTaskLinkLeaseRepository); ok && link.Status == studioBatchTaskLinkStatusCreating && strings.TrimSpace(link.ClaimToken) != "" {
		updated, updateErr := leaseRepo.UpdateStudioBatchTaskLinkWithClaimToken(ctx, link, link.ClaimToken)
		if updateErr != nil {
			return updateErr
		}
		if !updated {
			return fmt.Errorf("studio batch task claim is no longer owned while clearing pending release")
		}
		return nil
	}
	return s.batchTaskLinkRepo.UpdateStudioBatchTaskLink(ctx, link)
}

func (s *taskStudioBatchService) persistPendingStudioBatchProductImageUsageRelease(ctx context.Context, candidate studioBatchTaskCandidate, previousClaimToken string) error {
	if s == nil || s.batchTaskLinkRepo == nil || strings.TrimSpace(previousClaimToken) == "" {
		return nil
	}
	link, err := s.batchTaskLinkRepo.GetStudioBatchTaskLinkByCandidateKey(ctx, strings.TrimSpace(candidate.CandidateKey))
	if err != nil {
		return err
	}
	link.PendingProductImageUsageReleaseClaimToken = strings.TrimSpace(previousClaimToken)
	link.UpdatedAt = s.currentTime().UTC()
	if leaseRepo, ok := s.batchTaskLinkRepo.(studioBatchTaskLinkLeaseRepository); ok && link.Status == studioBatchTaskLinkStatusCreating && strings.TrimSpace(candidate.ClaimToken) != "" {
		link.ClaimToken = strings.TrimSpace(candidate.ClaimToken)
		updated, updateErr := leaseRepo.UpdateStudioBatchTaskLinkWithClaimToken(ctx, link, candidate.ClaimToken)
		if updateErr != nil {
			return updateErr
		}
		if !updated {
			return fmt.Errorf("studio batch task claim is no longer owned while persisting pending release")
		}
		return nil
	}
	return s.batchTaskLinkRepo.UpdateStudioBatchTaskLink(ctx, link)
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

func (s *taskStudioBatchService) recordStudioBatchProductImageUsageOnce(ctx context.Context, batch *StudioBatchRecord, quantity int, candidate studioBatchTaskCandidate) error {
	if s == nil || s.productImageUsage == nil {
		return nil
	}
	usage, ok := s.productImageUsage.(StudioProductImageUsageIdempotent)
	if !ok {
		return s.recordStudioBatchProductImageUsage(ctx, batch, quantity)
	}
	tenantID := studioBatchTaskGateTenantID(ctx, batch)
	if strings.TrimSpace(tenantID) == "" {
		return fmt.Errorf("tenant id is required")
	}
	operationKey := "listingkit:legacy_product_image_settlement:" + strings.TrimSpace(candidate.CandidateKey)
	return usage.RecordProductImageUsageOnce(ctx, tenantID, quantity, operationKey)
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

func (s *taskStudioBatchService) buildStudioBatchTaskProductImageRequest(
	ctx context.Context,
	session *SheinStudioSession,
	batch *StudioBatchRecord,
	candidate studioBatchTaskCandidate,
	design StudioMaterializedDesignRecord,
) (*StudioProductImageRequest, error) {
	request := buildStudioBatchTaskProductImageRequest(session, batch, candidate, design)
	if len(candidate.ProductImageCategoryPath) > 0 {
		request.CategoryPath = append([]string(nil), candidate.ProductImageCategoryPath...)
		return request, nil
	}
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

func (s *taskStudioBatchService) revalidateStudioBatchTaskLinkLease(ctx context.Context, candidate studioBatchTaskCandidate) error {
	claimToken := strings.TrimSpace(candidate.ClaimToken)
	if claimToken == "" {
		// Legacy candidates created before lease tokens were introduced do not
		// have an ownership predicate to revalidate. Keep their existing path;
		// token-bearing candidates below are always checked synchronously.
		return nil
	}
	if s == nil || s.batchTaskLinkRepo == nil {
		return fmt.Errorf("studio batch task link repository is not configured")
	}
	leaseRepo, ok := s.batchTaskLinkRepo.(studioBatchTaskLinkLeaseRepository)
	if !ok {
		return fmt.Errorf("studio batch task link repository does not support lease refresh")
	}
	updatedAt := time.Now().UTC()
	if s.currentTime != nil {
		updatedAt = s.currentTime().UTC()
	}
	refreshed, err := leaseRepo.RefreshStudioBatchTaskLink(DetachedRequestContext(ctx), candidate.CandidateKey, claimToken, updatedAt)
	if err != nil {
		return fmt.Errorf("refresh studio batch task claim before create: %w", err)
	}
	if !refreshed {
		return fmt.Errorf("studio batch task claim is no longer owned")
	}
	return nil
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
