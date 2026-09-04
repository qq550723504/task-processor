package temporal

import (
	"context"
	"errors"
	"fmt"
	sdktemporal "go.temporal.io/sdk/temporal"
	"strings"
	"task-processor/internal/imageagent"
)

// ReviewStagedSlotV3 retries only the read-only reviewer call for candidates
// already captured by the durable staging manifest. A successful review then
// resumes the existing staging attempt and lets ExecuteSlotV3 finalize it;
// generation and provider quoting are never called here.
func (a *Activities) ReviewStagedSlotV3(ctx context.Context, input ExecuteSlotV3ActivityInput) (imageagent.SlotEffectV3PublishedResult, error) {
	if a.slotEffectsV3 == nil || a.stagedSlotExecutor == nil || a.artifactStore == nil {
		return imageagent.SlotEffectV3PublishedResult{}, fmt.Errorf("image agent staged review dependencies are incomplete")
	}
	reviewer, ok := a.stagedSlotExecutor.(imageagent.StagedSlotReviewer)
	if !ok {
		return imageagent.SlotEffectV3PublishedResult{}, sdktemporal.NewNonRetryableApplicationError("image agent executor cannot review staged candidates", imageagent.SlotReviewTransportRequiredCode, imageagent.ErrValidation)
	}
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return imageagent.SlotEffectV3PublishedResult{}, err
	}
	executionInput := slotExecutionInputV3(input)
	reservation := slotEffectReservationV3(executionInput)
	effect, err := a.slotEffectsV3.GetSlotExternalEffectV3(ctx, reservation.Identity)
	if err != nil {
		return imageagent.SlotEffectV3PublishedResult{}, persistedSlotEffectV3RepositoryError(err)
	}
	if err := validatePersistedSlotEffectV3(effect); err != nil {
		return imageagent.SlotEffectV3PublishedResult{}, err
	}
	if effect.Phase != imageagent.SlotEffectV3ReviewTransportRequired && effect.Phase != imageagent.SlotEffectV3StagingPrepared {
		return imageagent.SlotEffectV3PublishedResult{}, sdktemporal.NewNonRetryableApplicationError("staged review is not pending", imageagent.SlotReviewTransportRequiredCode, imageagent.ErrRevisionConflict)
	}
	prepared, err := a.artifactStore.RecoverSlotArtifacts(ctx, reservation.Identity, effect.StagingManifest)
	if err != nil {
		return imageagent.SlotEffectV3PublishedResult{}, err
	}
	if err := a.artifactStore.EnsureStaged(ctx, prepared); err != nil {
		return imageagent.SlotEffectV3PublishedResult{}, err
	}
	staged, err := stagedOutputFromManifest(input, effect.StagingManifest, a.artifactStore)
	if err != nil {
		return imageagent.SlotEffectV3PublishedResult{}, err
	}
	var reviewReservation imageagent.SlotReviewUsageReservation
	budgetedReview, budgeted := a.stagedSlotExecutor.(imageagent.BudgetedStagedSlotReviewer)
	reviewAlreadySettled := false
	if input.BudgetAuthorization {
		if !budgeted {
			return imageagent.SlotEffectV3PublishedResult{}, sdktemporal.NewNonRetryableApplicationError("image agent reviewer cannot produce a conservative usage quote", imageagent.BudgetQuoteUnavailableCode, imageagent.ErrBudgetQuoteUnavailable)
		}
		if strings.TrimSpace(input.ReviewActionID) == "" {
			return imageagent.SlotEffectV3PublishedResult{}, sdktemporal.NewNonRetryableApplicationError("staged review action identity is missing", imageagent.SlotReviewTransportRequiredCode, imageagent.ErrValidation)
		}
		quote, quoteErr := budgetedReview.QuoteStagedReview(ctx, executionInput, effect.Policy)
		if quoteErr != nil {
			return imageagent.SlotEffectV3PublishedResult{}, sdktemporal.NewNonRetryableApplicationError("image agent review usage quote is unavailable", imageagent.BudgetQuoteUnavailableCode, quoteErr)
		}
		reviewReservation = imageagent.SlotReviewUsageReservation{
			Identity: reservation.Identity, ActionID: input.ReviewActionID,
			InputFingerprint: imageagent.SlotExecutionFingerprint(executionInput), Policy: effect.Policy, Quote: quote,
		}
		reservedEffect, acquired, reserveErr := a.slotEffectsV3.ReserveSlotReviewV3(ctx, reviewReservation)
		if reserveErr != nil {
			if errors.Is(reserveErr, imageagent.ErrBudgetExceeded) || errors.Is(reserveErr, imageagent.ErrBudgetOverflow) {
				return imageagent.SlotEffectV3PublishedResult{}, sdktemporal.NewNonRetryableApplicationError("image agent review budget is exhausted", imageagent.BudgetExhaustedCode, reserveErr)
			}
			return imageagent.SlotEffectV3PublishedResult{}, persistedSlotEffectV3RepositoryError(reserveErr)
		}
		if !acquired {
			priorStatus := imageagent.SlotBudgetStatus("")
			for _, prior := range reservedEffect.ReviewUsage {
				if prior.ActionID == input.ReviewActionID {
					priorStatus = prior.BudgetStatus
					if prior.BudgetStatus == imageagent.SlotBudgetCommitted && prior.Outcome == imageagent.SlotReviewOutcomeAccepted {
						reviewAlreadySettled = true
					}
					break
				}
			}
			if !reviewAlreadySettled {
				if priorStatus == imageagent.SlotBudgetReserved {
					if _, unknownErr := a.slotEffectsV3.MarkSlotReviewBudgetUnknownV3(ctx, reviewReservation); unknownErr != nil {
						return imageagent.SlotEffectV3PublishedResult{}, persistedSlotEffectV3RepositoryError(unknownErr)
					}
				}
				return imageagent.SlotEffectV3PublishedResult{}, sdktemporal.NewApplicationError("image reviewer budget reservation outcome is unknown", imageagent.SlotReviewTransportRequiredCode, imageagent.ErrRevisionConflict)
			}
		}
	}
	var reviewErr error
	var reviewReceipt imageagent.SlotUsageReceipt
	reviewCommitted := false
	if !reviewAlreadySettled {
		if input.BudgetAuthorization && budgeted {
			reviewReceipt, reviewErr = budgetedReview.ReviewStagedSlotQuoted(ctx, executionInput, staged, reviewReservation.Quote)
		} else {
			reviewErr = reviewer.ReviewStagedSlot(ctx, executionInput, staged)
		}
		if input.BudgetAuthorization && budgeted {
			if reviewErr == nil || errors.Is(reviewErr, imageagent.ErrReviewDecision) || imageagent.ProviderDispatchStateOf(reviewErr) == imageagent.ProviderDispatchedUnknown {
				if _, settleErr := a.slotEffectsV3.SettleSlotReviewV3(ctx, reviewReservation, reviewReceipt); settleErr != nil {
					_, _ = a.slotEffectsV3.MarkSlotReviewBudgetUnknownV3(ctx, reviewReservation)
					return imageagent.SlotEffectV3PublishedResult{}, fmt.Errorf("settle staged review budget: %w", persistedSlotEffectV3RepositoryError(settleErr))
				}
				reviewCommitted = true
			} else {
				if _, releaseErr := a.slotEffectsV3.ReleaseSlotReviewBudgetV3(ctx, reviewReservation); releaseErr != nil {
					return imageagent.SlotEffectV3PublishedResult{}, fmt.Errorf("release staged review budget: %w", persistedSlotEffectV3RepositoryError(releaseErr))
				}
			}
			outcome := imageagent.SlotReviewOutcomeAccepted
			if errors.Is(reviewErr, imageagent.ErrReviewDecision) {
				outcome = imageagent.SlotReviewOutcomeNeedsHuman
			} else if reviewErr != nil {
				outcome = imageagent.SlotReviewOutcomeTransportErr
			}
			if reviewCommitted {
				if _, outcomeErr := a.slotEffectsV3.RecordSlotReviewOutcomeV3(ctx, reviewReservation, outcome); outcomeErr != nil {
					return imageagent.SlotEffectV3PublishedResult{}, fmt.Errorf("record staged review outcome: %w", persistedSlotEffectV3RepositoryError(outcomeErr))
				}
			}
		}
	}
	if reviewErr != nil {
		if errors.Is(reviewErr, imageagent.ErrReviewDecision) {
			if effect.Phase == imageagent.SlotEffectV3ReviewTransportRequired {
				if _, err := resumeReviewRetrySlot(ctx, a.slotEffectsV3, reservation); err != nil {
					return imageagent.SlotEffectV3PublishedResult{}, persistedSlotEffectV3RepositoryError(err)
				}
				effect, err = a.slotEffectsV3.GetSlotExternalEffectV3(ctx, reservation.Identity)
				if err != nil {
					return imageagent.SlotEffectV3PublishedResult{}, persistedSlotEffectV3RepositoryError(err)
				}
			}
			if _, err := a.slotEffectsV3.BlockSlotEffectV3(ctx, imageagent.SlotEffectV3BlockTransition{
				Reservation: reservation, Phase: imageagent.SlotEffectV3ReviewRequired,
				Code: imageagent.SlotReviewRequiredCode,
			}); err != nil {
				return imageagent.SlotEffectV3PublishedResult{}, persistedSlotEffectV3RepositoryError(err)
			}
			return imageagent.SlotEffectV3PublishedResult{}, sdktemporal.NewNonRetryableApplicationError("staged image review requires human intervention", imageagent.SlotReviewRequiredCode, reviewErr)
		}
		if imageagent.ProviderDispatchStateOf(reviewErr) == imageagent.ProviderDispatchedUnknown {
			return imageagent.SlotEffectV3PublishedResult{}, sdktemporal.NewApplicationError("image reviewer transport failed", imageagent.SlotReviewTransportRequiredCode, reviewErr)
		}
		return imageagent.SlotEffectV3PublishedResult{}, reviewErr
	}
	if effect.Phase == imageagent.SlotEffectV3ReviewTransportRequired {
		if _, err := resumeReviewRetrySlot(ctx, a.slotEffectsV3, reservation); err != nil {
			return imageagent.SlotEffectV3PublishedResult{}, persistedSlotEffectV3RepositoryError(err)
		}
	}
	return a.ExecuteSlotV3(ctx, input)
}

func resumeReviewRetrySlot(ctx context.Context, repository imageagent.SlotExternalEffectV3Repository, reservation imageagent.SlotEffectV3Reservation) (imageagent.SlotEffectV3Attempt, error) {
	resumer, ok := repository.(imageagent.ReviewRetrySlotEffectV3Repository)
	if !ok {
		return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRevisionConflict
	}
	return resumer.ResumeReviewRetrySlotV3(ctx, reservation)
}

func stagedOutputFromManifest(input ExecuteSlotV3ActivityInput, manifest imageagent.StagingManifest, store DurableArtifactStore) (imageagent.SlotGeneratedOutput, error) {
	output := imageagent.SlotGeneratedOutput{SlotID: input.Slot.ID, Attempt: input.Attempt}
	for _, staged := range manifest.Assets {
		url := store.PublicURL(staged.ObjectKey)
		sourceURL, err := stagedSourceURL(input.AssetCatalog, staged.SourceAssetID)
		if err != nil {
			return imageagent.SlotGeneratedOutput{}, err
		}
		output.Assets = append(output.Assets, imageagent.GeneratedAsset{
			URL: url, ContentType: staged.ContentType, SourceURL: sourceURL,
			Operations: append([]string(nil), staged.Operations...), Width: staged.Width, Height: staged.Height,
			ProviderReceiptID: staged.ProviderReceiptID,
		})
		if output.SourceAssetID == "" {
			output.SourceAssetID = staged.SourceAssetID
		}
	}
	return output, nil
}

func stagedSourceURL(catalog imageagent.AssetCatalog, sourceAssetID string) (string, error) {
	for _, asset := range catalog.Assets {
		if asset.ID != sourceAssetID || asset.Type != imageagent.AuthorizedAssetSource {
			continue
		}
		for _, candidate := range []string{asset.SourceURL, asset.URL, asset.DisplayURL} {
			if strings.TrimSpace(candidate) == "" {
				continue
			}
			validated, err := imageagent.ValidateSafeImageURL(candidate)
			if err != nil {
				return "", err
			}
			return validated, nil
		}
	}
	return "", fmt.Errorf("%w: staged source asset %q is missing from the immutable catalog", imageagent.ErrValidation, sourceAssetID)
}
