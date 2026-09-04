package temporal

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	sdkactivity "go.temporal.io/sdk/activity"
	sdktemporal "go.temporal.io/sdk/temporal"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/objectstore"
)

const slotProviderOutcomeUnknownErrorType = "imageagent_slot_provider_outcome_unknown"

const (
	slotProviderOutcomeUnknownCode        = imageagent.SlotProviderOutcomeUnknownCode
	slotStagingOutcomeUnknownCode         = imageagent.SlotStagingOutcomeUnknownCode
	slotPublicationOutcomeUnknownCode     = imageagent.SlotPublicationOutcomeUnknownCode
	slotPublicationRecoveryErrorType      = "imageagent_slot_publication_recovery"
	slotEffectPhaseInvalidCode            = "imageagent_slot_effect_phase_invalid"
	slotEffectPolicyInvalidCode           = "imageagent_slot_effect_policy_invalid"
	invalidMainCandidateCountCode         = "invalid_main_candidate_count"
	publicationLeaseRetrySafetyMargin     = time.Second
	providerFinalizationTimeout           = time.Minute
	externalEffectHeartbeatInterval       = time.Second
	externalEffectHeartbeatTimeout        = 5 * time.Second
	recoveryBundlePersistenceAttempts     = 3
	recoveryBundlePersistenceInitialDelay = 100 * time.Millisecond
	recoveryBundlePersistenceMaxDelay     = time.Second
)

func providerFinalizationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), providerFinalizationTimeout)
}

func startExternalEffectHeartbeat(ctx context.Context) func() {
	if !sdkactivity.IsActivity(ctx) || sdkactivity.GetInfo(ctx).HeartbeatTimeout <= 0 {
		return func() {}
	}
	heartbeatCtx := context.WithoutCancel(ctx)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(externalEffectHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sdkactivity.RecordHeartbeat(heartbeatCtx)
			case <-stop:
				return
			}
		}
	}()
	return func() {
		close(stop)
		<-done
	}
}

type slotPublicationRecoveryDetails struct {
	RetryDelay     time.Duration `json:"retry_delay"`
	LeaseExpiresAt time.Time     `json:"lease_expires_at"`
	Owner          string        `json:"owner"`
	Fence          int64         `json:"fence"`
}

func (a *Activities) ExecuteSlot(ctx context.Context, input ExecuteSlotActivityInput) (imageagent.SlotExecutionResult, error) {
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return imageagent.SlotExecutionResult{}, err
	}
	return a.slotExecutor.ExecuteSlot(ctx, imageagent.SlotExecutionInput{
		RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID,
		PlanRevision: input.PlanRevision, Slot: input.Slot, Attempt: input.Attempt,
		IdempotencyKey: input.IdempotencyKey,
		AssetCatalog:   input.AssetCatalog,
		ProductContext: input.AssetCatalog.ProductContext,
	})
}

func (a *Activities) ExecuteSlotV3(ctx context.Context, input ExecuteSlotV3ActivityInput) (v3Result imageagent.SlotEffectV3PublishedResult, err error) {
	if a.slotEffectsV3 == nil || a.stagedSlotExecutor == nil || a.artifactStore == nil || a.publicationOwner == nil {
		return v3Result, fmt.Errorf("image agent v3 activity dependencies are incomplete")
	}
	ctx, err = restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return v3Result, err
	}
	stopHeartbeat := startExternalEffectHeartbeat(ctx)
	defer stopHeartbeat()
	executionInput := slotExecutionInputV3(input)
	reservation := slotEffectReservationV3(executionInput)
	var budgeted imageagent.BudgetedStagedSlotExecutor
	providerCtx := ctx
	var cancelProvider context.CancelFunc
	persisted, getErr := a.slotEffectsV3.GetSlotExternalEffectV3(ctx, reservation.Identity)
	hasPersistedEffect := getErr == nil
	if getErr != nil && !errors.Is(getErr, imageagent.ErrRunNotFound) {
		return v3Result, persistedSlotEffectV3RepositoryError(getErr)
	}
	if hasPersistedEffect {
		if err := validatePersistedSlotEffectV3(persisted); err != nil {
			return v3Result, err
		}
		persistedBudgetAuthorization := persisted.Quote.Fingerprint != ""
		if persistedBudgetAuthorization != input.BudgetAuthorization {
			return v3Result, imageagent.ErrRevisionConflict
		}
		reservation.Policy = persisted.Policy
		reservation.Quote = persisted.Quote
	}
	providerDispatchPossible := !hasPersistedEffect || persisted.Phase == imageagent.SlotEffectV3ProviderNotDispatched || (persisted.Phase == imageagent.SlotEffectV3ProviderClaimed && persisted.BudgetStatus == imageagent.SlotBudgetReleased)
	if input.BudgetAuthorization && providerDispatchPossible {
		var ok bool
		budgeted, ok = a.stagedSlotExecutor.(imageagent.BudgetedStagedSlotExecutor)
		if !ok {
			return v3Result, sdktemporal.NewNonRetryableApplicationError("image agent provider cannot produce a conservative usage quote", imageagent.BudgetQuoteUnavailableCode, imageagent.ErrBudgetQuoteUnavailable)
		}
	}
	if providerDispatchPossible && !input.LifecycleDeadlineAt.IsZero() && !time.Now().UTC().Before(input.LifecycleDeadlineAt) {
		return v3Result, sdktemporal.NewNonRetryableApplicationError("image agent lifecycle deadline elapsed", imageagent.WorkflowLifecycleElapsedCode, imageagent.ErrBudgetExceeded)
	}
	if input.BudgetAuthorization && providerDispatchPossible && !input.DeadlineAt.IsZero() && !time.Now().UTC().Before(input.DeadlineAt) {
		return v3Result, sdktemporal.NewNonRetryableApplicationError("image agent budget deadline elapsed", imageagent.BudgetElapsedCode, imageagent.ErrBudgetExceeded)
	}
	if input.BudgetAuthorization && providerDispatchPossible && !input.DeadlineAt.IsZero() {
		providerCtx, cancelProvider = context.WithDeadline(ctx, input.DeadlineAt)
		defer cancelProvider()
	}
	if input.BudgetAuthorization && !hasPersistedEffect {
		quote, quoteErr := budgeted.QuoteSlot(providerCtx, executionInput, input.BudgetPolicy)
		if quoteErr != nil {
			if errors.Is(quoteErr, context.DeadlineExceeded) {
				return v3Result, sdktemporal.NewNonRetryableApplicationError("image agent budget deadline elapsed", imageagent.BudgetElapsedCode, quoteErr)
			}
			return v3Result, sdktemporal.NewNonRetryableApplicationError("image agent provider usage quote is unavailable", imageagent.BudgetQuoteUnavailableCode, quoteErr)
		}
		reservation.Policy = input.BudgetPolicy
		reservation.Quote = quote
	}
	effect, claimed, err := a.slotEffectsV3.ReserveSlotProviderV3(ctx, reservation)
	if err != nil {
		if errors.Is(err, imageagent.ErrBudgetExceeded) || errors.Is(err, imageagent.ErrBudgetOverflow) {
			return v3Result, sdktemporal.NewNonRetryableApplicationError("image agent budget is exhausted", imageagent.BudgetExhaustedCode, err)
		}
		return v3Result, persistedSlotEffectV3RepositoryError(err)
	}
	if err := validatePersistedSlotEffectV3(effect); err != nil {
		return v3Result, err
	}

	var prepared objectstore.PreparedSlotArtifacts
	var generated imageagent.SlotGeneratedOutput
	var finalizationCtx context.Context
	var cancelFinalization context.CancelFunc
	postProviderContext := func() context.Context {
		if finalizationCtx == nil {
			finalizationCtx, cancelFinalization = providerFinalizationContext(ctx)
		}
		return finalizationCtx
	}
	defer func() {
		if cancelFinalization != nil {
			cancelFinalization()
		}
	}()
	if effect.Phase == imageagent.SlotEffectV3ProviderClaimed {
		if claimed {
			var generateErr error
			if budgeted != nil {
				generated, generateErr = budgeted.GenerateQuotedSlot(providerCtx, executionInput, reservation.Quote)
			} else {
				generated, generateErr = a.stagedSlotExecutor.GenerateSlot(ctx, executionInput)
			}
			if generateErr != nil {
				if reviewOutput, reviewRequired := imageagent.ReviewRequiredOutput(generateErr); reviewRequired {
					providerStagingCtx := postProviderContext()
					if budgeted != nil {
						if _, settleErr := a.slotEffectsV3.SettleSlotProviderV3(providerStagingCtx, reservation, reviewOutput.UsageReceipt); settleErr != nil {
							if _, unknownErr := a.slotEffectsV3.MarkSlotProviderBudgetUnknownV3(providerStagingCtx, reservation); unknownErr != nil {
								return v3Result, fmt.Errorf("retain review provider reservation: %w", persistedSlotEffectV3RepositoryError(unknownErr))
							}
							return v3Result, a.blockSlotEffectV3(providerStagingCtx, reservation, imageagent.SlotEffectV3ProviderUnknown, slotProviderOutcomeUnknownCode, imageagent.PublicationClaim{})
						}
					}
					prepared, err = prepareGeneratedSlotArtifacts(executionInput, reviewOutput, a.artifactStore)
					if err != nil {
						return v3Result, a.blockSlotEffectV3(providerStagingCtx, reservation, imageagent.SlotEffectV3StagingUnknown, slotStagingOutcomeUnknownCode, imageagent.PublicationClaim{})
					}
					if err := a.preserveSlotRecoveryBundle(providerStagingCtx, reservation.Identity, prepared); err != nil {
						cleanupGeneratedSlotTemporaryAssets(&reviewOutput)
						return v3Result, a.blockSlotEffectV3(providerStagingCtx, reservation, imageagent.SlotEffectV3StagingUnknown, slotStagingOutcomeUnknownCode, imageagent.PublicationClaim{})
					}
					cleanupGeneratedSlotTemporaryAssets(&reviewOutput)
					if _, err = a.slotEffectsV3.PrepareSlotStagingV3(providerStagingCtx, reservation, prepared.Manifest); err != nil {
						return v3Result, fmt.Errorf("persist review staging manifest: %w", persistedSlotEffectV3RepositoryError(err))
					}
					return v3Result, a.blockSlotEffectV3(providerStagingCtx, reservation, imageagent.SlotEffectV3ReviewRequired, imageagent.SlotReviewRequiredCode, imageagent.PublicationClaim{})
				}
				if reviewOutput, transportFailure := imageagent.ReviewTransportOutput(generateErr); transportFailure {
					providerStagingCtx := postProviderContext()
					if budgeted != nil {
						if _, settleErr := a.slotEffectsV3.SettleSlotProviderV3(providerStagingCtx, reservation, reviewOutput.UsageReceipt); settleErr != nil {
							if _, unknownErr := a.slotEffectsV3.MarkSlotProviderBudgetUnknownV3(providerStagingCtx, reservation); unknownErr != nil {
								return v3Result, fmt.Errorf("retain review provider reservation: %w", persistedSlotEffectV3RepositoryError(unknownErr))
							}
							return v3Result, a.blockSlotEffectV3(providerStagingCtx, reservation, imageagent.SlotEffectV3ProviderUnknown, slotProviderOutcomeUnknownCode, imageagent.PublicationClaim{})
						}
					}
					prepared, err = prepareGeneratedSlotArtifacts(executionInput, reviewOutput, a.artifactStore)
					if err != nil {
						return v3Result, a.blockSlotEffectV3(providerStagingCtx, reservation, imageagent.SlotEffectV3StagingUnknown, slotStagingOutcomeUnknownCode, imageagent.PublicationClaim{})
					}
					if err := a.preserveSlotRecoveryBundle(providerStagingCtx, reservation.Identity, prepared); err != nil {
						cleanupGeneratedSlotTemporaryAssets(&reviewOutput)
						return v3Result, a.blockSlotEffectV3(providerStagingCtx, reservation, imageagent.SlotEffectV3StagingUnknown, slotStagingOutcomeUnknownCode, imageagent.PublicationClaim{})
					}
					cleanupGeneratedSlotTemporaryAssets(&reviewOutput)
					if _, err = a.slotEffectsV3.PrepareSlotStagingV3(providerStagingCtx, reservation, prepared.Manifest); err != nil {
						return v3Result, fmt.Errorf("persist review staging manifest: %w", persistedSlotEffectV3RepositoryError(err))
					}
					return v3Result, a.blockSlotEffectV3(providerStagingCtx, reservation, imageagent.SlotEffectV3ReviewTransportRequired, imageagent.SlotReviewTransportRequiredCode, imageagent.PublicationClaim{})
				}
				switch imageagent.ProviderDispatchStateOf(generateErr) {
				case imageagent.ProviderNotDispatched, imageagent.ProviderRejectedBeforeEffect:
					finalizationCtx, cancelFinalization := providerFinalizationContext(ctx)
					defer cancelFinalization()
					if _, recordErr := a.slotEffectsV3.RecordSlotProviderNotDispatchedV3(finalizationCtx, reservation); recordErr != nil {
						return v3Result, fmt.Errorf("record provider rejection before dispatch: %w", persistedSlotEffectV3RepositoryError(recordErr))
					}
					return v3Result, sdktemporal.NewApplicationError("image agent provider did not dispatch", imageagent.SlotProviderNotDispatchedCode, generateErr)
				default:
					finalizationCtx, cancelFinalization := providerFinalizationContext(ctx)
					defer cancelFinalization()
					if budgeted != nil {
						if _, unknownErr := a.slotEffectsV3.MarkSlotProviderBudgetUnknownV3(finalizationCtx, reservation); unknownErr != nil {
							return v3Result, fmt.Errorf("retain unknown provider reservation: %w", persistedSlotEffectV3RepositoryError(unknownErr))
						}
					}
					return v3Result, a.blockSlotEffectV3(finalizationCtx, reservation, imageagent.SlotEffectV3ProviderUnknown, slotProviderOutcomeUnknownCode, imageagent.PublicationClaim{})
				}
			}
			providerStagingCtx := postProviderContext()
			if budgeted != nil {
				if _, settleErr := a.slotEffectsV3.SettleSlotProviderV3(providerStagingCtx, reservation, generated.UsageReceipt); settleErr != nil {
					if _, unknownErr := a.slotEffectsV3.MarkSlotProviderBudgetUnknownV3(providerStagingCtx, reservation); unknownErr != nil {
						return v3Result, fmt.Errorf("retain unsettled provider reservation: %w", persistedSlotEffectV3RepositoryError(unknownErr))
					}
					return v3Result, a.blockSlotEffectV3(providerStagingCtx, reservation, imageagent.SlotEffectV3ProviderUnknown, slotProviderOutcomeUnknownCode, imageagent.PublicationClaim{})
				}
			}
			prepared, err = prepareGeneratedSlotArtifacts(executionInput, generated, a.artifactStore)
			if err != nil {
				return v3Result, a.blockSlotEffectV3(providerStagingCtx, reservation, imageagent.SlotEffectV3ProviderUnknown, slotProviderOutcomeUnknownCode, imageagent.PublicationClaim{})
			}
			if err := a.preserveSlotRecoveryBundle(providerStagingCtx, reservation.Identity, prepared); err != nil {
				if errors.Is(err, objectstore.ErrArtifactUnavailable) || errors.Is(err, objectstore.ErrObjectConflict) {
					cleanupGeneratedSlotTemporaryAssets(&generated)
					return v3Result, a.blockSlotEffectV3(providerStagingCtx, reservation, imageagent.SlotEffectV3ProviderUnknown, slotProviderOutcomeUnknownCode, imageagent.PublicationClaim{})
				}
				cleanupGeneratedSlotTemporaryAssets(&generated)
				return v3Result, a.blockSlotEffectV3(providerStagingCtx, reservation, imageagent.SlotEffectV3ProviderUnknown, slotProviderOutcomeUnknownCode, imageagent.PublicationClaim{})
			}
			cleanupGeneratedSlotTemporaryAssets(&generated)
		} else {
			providerStagingCtx := postProviderContext()
			prepared, err = a.artifactStore.RecoverSlotArtifacts(providerStagingCtx, reservation.Identity, imageagent.StagingManifest{})
			if err != nil {
				if errors.Is(err, objectstore.ErrArtifactUnavailable) || errors.Is(err, objectstore.ErrObjectConflict) {
					return v3Result, a.blockSlotEffectV3(providerStagingCtx, reservation, imageagent.SlotEffectV3ProviderUnknown, slotProviderOutcomeUnknownCode, imageagent.PublicationClaim{})
				}
				return v3Result, fmt.Errorf("recover generated artifact bundle before staging transition: %w", err)
			}
		}
		providerStagingCtx := postProviderContext()
		effect, err = a.slotEffectsV3.PrepareSlotStagingV3(providerStagingCtx, reservation, prepared.Manifest)
		if err != nil {
			transitionErr := err
			effect, err = a.slotEffectsV3.GetSlotExternalEffectV3(providerStagingCtx, reservation.Identity)
			if err != nil {
				return v3Result, fmt.Errorf("reconcile persisted staging manifest: %w", persistedSlotEffectV3RepositoryError(err))
			}
			if effect.Phase == imageagent.SlotEffectV3ProviderClaimed {
				return v3Result, fmt.Errorf("persist prepared staging manifest: %w", persistedSlotEffectV3RepositoryError(transitionErr))
			}
		}
	}

	if effect.Phase == imageagent.SlotEffectV3StagingPrepared {
		providerStagingCtx := postProviderContext()
		if len(prepared.Manifest.Assets) == 0 {
			prepared = objectstore.PreparedSlotArtifacts{Manifest: effect.StagingManifest}
		}
		ensureErr := a.artifactStore.EnsureStaged(providerStagingCtx, prepared)
		if errors.Is(ensureErr, objectstore.ErrArtifactUnavailable) {
			recovered, recoverErr := a.artifactStore.RecoverSlotArtifacts(providerStagingCtx, reservation.Identity, effect.StagingManifest)
			if recoverErr == nil {
				prepared = recovered
				ensureErr = a.artifactStore.EnsureStaged(providerStagingCtx, prepared)
			} else if errors.Is(recoverErr, objectstore.ErrArtifactUnavailable) || errors.Is(recoverErr, objectstore.ErrObjectConflict) {
				return v3Result, a.blockSlotEffectV3(providerStagingCtx, reservation, imageagent.SlotEffectV3StagingUnknown, slotStagingOutcomeUnknownCode, imageagent.PublicationClaim{})
			} else {
				return v3Result, fmt.Errorf("rehydrate prepared staging bytes: %w", recoverErr)
			}
		}
		if ensureErr != nil {
			if errors.Is(ensureErr, objectstore.ErrArtifactUnavailable) || errors.Is(ensureErr, objectstore.ErrObjectConflict) {
				return v3Result, a.blockSlotEffectV3(providerStagingCtx, reservation, imageagent.SlotEffectV3StagingUnknown, slotStagingOutcomeUnknownCode, imageagent.PublicationClaim{})
			}
			return v3Result, fmt.Errorf("ensure staged artifacts: %w", ensureErr)
		}
		effect, err = a.slotEffectsV3.CommitSlotStagedV3(providerStagingCtx, reservation, effect.StagingManifestFingerprint)
		if err != nil {
			effect, err = a.slotEffectsV3.GetSlotExternalEffectV3(providerStagingCtx, reservation.Identity)
			if err != nil {
				return v3Result, fmt.Errorf("reconcile staged commit: %w", persistedSlotEffectV3RepositoryError(err))
			}
			if effect.Phase == imageagent.SlotEffectV3StagingPrepared {
				return v3Result, fmt.Errorf("commit staged artifacts: %w", imageagent.ErrRevisionConflict)
			}
		}
	}

	if effect.Phase == imageagent.SlotEffectV3ArtifactStaged || effect.Phase == imageagent.SlotEffectV3PublicationClaimed {
		publicationFinalizationCtx := postProviderContext()
		owner, ownerErr := a.publicationOwner(publicationFinalizationCtx)
		if ownerErr != nil {
			return v3Result, fmt.Errorf("derive publication owner: %w", ownerErr)
		}
		finalManifest, manifestErr := expectedFinalManifestV3(executionInput, effect.StagingManifest)
		if manifestErr != nil {
			return v3Result, manifestErr
		}
		publicationFingerprint, fingerprintErr := imageagent.FinalManifestFingerprint(finalManifest)
		if fingerprintErr != nil {
			return v3Result, fingerprintErr
		}
		var publication imageagent.PublicationClaim
		var acquired bool
		var claimErr error
		effect, publication, acquired, claimErr = a.slotEffectsV3.ClaimSlotPublicationV3(publicationFinalizationCtx, imageagent.PublicationClaimRequest{
			Reservation: reservation, Owner: owner, LeaseDuration: a.publicationLeaseDuration,
			PublicationFingerprint: publicationFingerprint, FinalManifest: finalManifest,
		})
		if claimErr != nil {
			reconciled, getErr := a.slotEffectsV3.GetSlotExternalEffectV3(publicationFinalizationCtx, reservation.Identity)
			if getErr != nil {
				return v3Result, fmt.Errorf("claim slot publication: %w", persistedSlotEffectV3RepositoryError(getErr))
			}
			if reconciled.Phase == imageagent.SlotEffectV3PublicationComplete {
				return reconciled.Published, nil
			}
			if reconciled.Phase != imageagent.SlotEffectV3PublicationClaimed || reconciled.Publication.Owner != owner ||
				reconciled.PublicationFingerprint != publicationFingerprint || !reflect.DeepEqual(reconciled.FinalManifest, finalManifest) {
				return v3Result, fmt.Errorf("claim slot publication: %w", claimErr)
			}
			effect, publication, acquired = reconciled, reconciled.Publication, true
		}
		if effect.Phase == imageagent.SlotEffectV3PublicationComplete {
			return effect.Published, nil
		}
		if !acquired {
			return v3Result, a.publicationRecoveryError("slot publication is owned by another activity attempt", publication, imageagent.ErrRevisionConflict)
		}
		if _, err := a.renewPublicationV3(publicationFinalizationCtx, reservation.Identity, publication); err != nil {
			return v3Result, a.publicationRecoveryError("renew slot publication lease", publication, err)
		}
		actualFinal, finalizeErr := a.artifactStore.FinalizeWithProgress(publicationFinalizationCtx, effect.StagingManifest, func(progressCtx context.Context, _ int) error {
			renewed, renewErr := a.renewPublicationV3(progressCtx, reservation.Identity, publication)
			if renewErr == nil {
				publication = renewed
			}
			return renewErr
		})
		if finalizeErr != nil {
			if errors.Is(finalizeErr, objectstore.ErrArtifactUnavailable) || errors.Is(finalizeErr, objectstore.ErrObjectConflict) {
				return v3Result, a.blockSlotEffectV3(publicationFinalizationCtx, reservation, imageagent.SlotEffectV3PublicationUnknown, slotPublicationOutcomeUnknownCode, publication)
			}
			return v3Result, a.publicationRecoveryError("finalize slot artifacts after publication claim", publication, finalizeErr)
		}
		if !reflect.DeepEqual(actualFinal, effect.FinalManifest) {
			return v3Result, a.blockSlotEffectV3(publicationFinalizationCtx, reservation, imageagent.SlotEffectV3PublicationUnknown, slotPublicationOutcomeUnknownCode, publication)
		}
		renewedPublication, renewErr := a.renewPublicationV3(publicationFinalizationCtx, reservation.Identity, publication)
		if renewErr != nil {
			return v3Result, a.publicationRecoveryError("renew slot publication lease after finalize", publication, renewErr)
		}
		publication = renewedPublication
		result, buildErr := a.stagedSlotExecutor.BuildSlotResult(publicationFinalizationCtx, executionInput, imageagent.PublishedSlotOutput{SlotID: input.Slot.ID, Attempt: input.Attempt, Assets: actualFinal.Assets})
		if buildErr != nil {
			return v3Result, fmt.Errorf("build durable slot result: %w", buildErr)
		}
		published, publishedErr := imageagent.NewSlotEffectV3PublishedResult(result)
		if publishedErr != nil {
			return v3Result, fmt.Errorf("normalize durable slot result: %w", publishedErr)
		}
		resultFingerprint, resultFingerprintErr := imageagent.SlotEffectV3PublishedResultFingerprint(published)
		if resultFingerprintErr != nil {
			return v3Result, resultFingerprintErr
		}
		effect, err = a.slotEffectsV3.CompleteSlotPublicationV3(publicationFinalizationCtx, imageagent.PublicationCompletion{
			Reservation: reservation, Owner: publication.Owner, Fence: publication.Fence,
			PublicationFingerprint: publicationFingerprint, ResultFingerprint: resultFingerprint, Published: published,
		})
		if err != nil {
			reconciled, getErr := a.slotEffectsV3.GetSlotExternalEffectV3(publicationFinalizationCtx, reservation.Identity)
			if getErr != nil {
				return v3Result, fmt.Errorf("reconcile publication completion: %w", persistedSlotEffectV3RepositoryError(getErr))
			}
			if reconciled.Phase != imageagent.SlotEffectV3PublicationComplete || reconciled.ResultFingerprint != resultFingerprint || !reflect.DeepEqual(reconciled.Published, published) {
				return v3Result, imageagent.ErrRevisionConflict
			}
			effect = reconciled
		}
	}

	if err := validatePersistedSlotEffectV3(effect); err != nil {
		return v3Result, err
	}
	switch effect.Phase {
	case imageagent.SlotEffectV3PublicationComplete:
		return effect.Published, nil
	case imageagent.SlotEffectV3ProviderUnknown:
		return v3Result, blockedSlotEffectV3Error(slotProviderOutcomeUnknownCode)
	case imageagent.SlotEffectV3StagingUnknown:
		return v3Result, blockedSlotEffectV3Error(slotStagingOutcomeUnknownCode)
	case imageagent.SlotEffectV3PublicationUnknown:
		return v3Result, blockedSlotEffectV3Error(slotPublicationOutcomeUnknownCode)
	case imageagent.SlotEffectV3ReviewRequired:
		return v3Result, blockedSlotEffectV3Error(imageagent.SlotReviewRequiredCode)
	case imageagent.SlotEffectV3ReviewTransportRequired:
		return v3Result, blockedSlotEffectV3Error(imageagent.SlotReviewTransportRequiredCode)
	default:
		return v3Result, sdktemporal.NewNonRetryableApplicationError(
			fmt.Sprintf("unsupported persisted slot effect phase %q", effect.Phase), slotEffectPhaseInvalidCode, nil,
		)
	}
}
