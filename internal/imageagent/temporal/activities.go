package temporal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	sdkactivity "go.temporal.io/sdk/activity"
	sdktemporal "go.temporal.io/sdk/temporal"

	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/objectstore"
	"task-processor/internal/infra/resilience"
	"task-processor/internal/pkg/imagex"
	"task-processor/internal/productimage"
	"task-processor/internal/shared/aiidentity"
)

const slotResultPersistedEventType = "slot.result.persisted"

var errPublicationOwnerRequiresActivity = errors.New("publication owner requires a Temporal activity context")

type RecoveryWorkflowStarter func(context.Context, EffectRecoveryWorkflowInput) error

type ActivityDependencies struct {
	Repository               imageagent.Repository
	SlotEffects              imageagent.SlotExternalEffectRepository
	SlotExecutor             imageagent.SlotExecutor
	Publisher                imageagent.ApprovedAssetPublisher
	PublisherV3              imageagent.ApprovedAssetPublisherV3
	SlotEffectsV3            imageagent.SlotExternalEffectV3Repository
	StagedSlotExecutor       imageagent.StagedSlotExecutor
	ArtifactStore            DurableArtifactStore
	PublicationOwner         func(context.Context) (string, error)
	PublicationLeaseDuration time.Duration
	RecoveryWorkflowStarter  RecoveryWorkflowStarter
}

type Activities struct {
	repository               imageagent.Repository
	slotEffects              imageagent.SlotExternalEffectRepository
	slotExecutor             imageagent.SlotExecutor
	publisher                imageagent.ApprovedAssetPublisher
	publisherV3              imageagent.ApprovedAssetPublisherV3
	slotEffectsV3            imageagent.SlotExternalEffectV3Repository
	stagedSlotExecutor       imageagent.StagedSlotExecutor
	artifactStore            DurableArtifactStore
	publicationOwner         func(context.Context) (string, error)
	publicationLeaseDuration time.Duration
	recoveryWorkflowStarter  RecoveryWorkflowStarter
}

func NewActivities(dependencies ActivityDependencies) (*Activities, error) {
	if dependencies.Repository == nil {
		return nil, fmt.Errorf("image agent repository is required")
	}
	if dependencies.SlotEffects == nil {
		if slotEffects, ok := dependencies.Repository.(imageagent.SlotExternalEffectRepository); ok {
			dependencies.SlotEffects = slotEffects
		}
	}
	if dependencies.SlotExecutor == nil {
		return nil, fmt.Errorf("image agent slot executor is required")
	}
	if dependencies.SlotEffects == nil {
		return nil, fmt.Errorf("image agent slot external effect repository is required")
	}
	if dependencies.Publisher == nil {
		return nil, fmt.Errorf("image agent approved asset publisher is required")
	}
	v3Requested := dependencies.SlotEffectsV3 != nil || dependencies.StagedSlotExecutor != nil || dependencies.ArtifactStore != nil
	if v3Requested {
		if dependencies.SlotEffectsV3 == nil {
			if slotEffects, ok := dependencies.Repository.(imageagent.SlotExternalEffectV3Repository); ok {
				dependencies.SlotEffectsV3 = slotEffects
			}
		}
		if dependencies.SlotEffectsV3 == nil {
			return nil, fmt.Errorf("image agent v3 slot external effect repository is required")
		}
		if dependencies.StagedSlotExecutor == nil {
			return nil, fmt.Errorf("image agent staged slot executor is required")
		}
		if dependencies.ArtifactStore == nil {
			return nil, fmt.Errorf("image agent durable artifact store is required")
		}
		if dependencies.PublisherV3 == nil {
			return nil, fmt.Errorf("image agent v3 approved asset publisher is required")
		}
		if dependencies.PublicationOwner == nil {
			dependencies.PublicationOwner = temporalPublicationOwner
		}
		if dependencies.PublicationLeaseDuration <= 0 {
			dependencies.PublicationLeaseDuration = 2 * time.Minute
		}
	}
	return &Activities{
		repository: dependencies.Repository, slotEffects: dependencies.SlotEffects, slotExecutor: dependencies.SlotExecutor, publisher: dependencies.Publisher, publisherV3: dependencies.PublisherV3,
		slotEffectsV3: dependencies.SlotEffectsV3, stagedSlotExecutor: dependencies.StagedSlotExecutor, artifactStore: dependencies.ArtifactStore,
		publicationOwner: dependencies.PublicationOwner, publicationLeaseDuration: dependencies.PublicationLeaseDuration,
		recoveryWorkflowStarter: dependencies.RecoveryWorkflowStarter,
	}, nil
}

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
	default:
		return v3Result, sdktemporal.NewNonRetryableApplicationError(
			fmt.Sprintf("unsupported persisted slot effect phase %q", effect.Phase), slotEffectPhaseInvalidCode, nil,
		)
	}
}

func (a *Activities) RecoverEffectV3(ctx context.Context, input EffectRecoveryWorkflowInput) (EffectRecoveryResult, error) {
	if a.slotEffectsV3 == nil || a.stagedSlotExecutor == nil || a.artifactStore == nil || a.publicationOwner == nil {
		return EffectRecoveryResult{}, fmt.Errorf("image agent v3 activity dependencies are incomplete")
	}
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return EffectRecoveryResult{}, err
	}
	executionInput := imageagent.SlotExecutionInput{
		RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID,
		PlanRevision: input.PlanRevision, Slot: input.Slot, Attempt: input.Attempt,
		IdempotencyKey: slotAttemptKey(input.PlanRevision, input.Slot, input.Attempt),
		AssetCatalog:   input.AssetCatalog,
		ProductContext: input.AssetCatalog.ProductContext,
	}
	reservation := slotEffectReservationV3(executionInput)
	effect, err := a.slotEffectsV3.GetSlotExternalEffectV3(ctx, reservation.Identity)
	if err != nil {
		if errors.Is(err, imageagent.ErrRunNotFound) {
			return a.persistMissingEffectRecoveryBlockedV3(ctx, input, reservation)
		}
		if result, handled, blockErr := a.failClosedCorruptEffectRecovery(ctx, input, err); handled {
			return result, blockErr
		}
		return EffectRecoveryResult{}, persistedSlotEffectV3RepositoryError(err)
	}
	if err := validatePersistedSlotEffectV3(effect); err != nil {
		return EffectRecoveryResult{}, err
	}
	reservation.Policy = effect.Policy
	reservation.Quote = effect.Quote
	if effect.Phase == imageagent.SlotEffectV3RecoveryBlocked && strings.TrimSpace(input.ActionID) != "" {
		restorer, ok := a.slotEffectsV3.(imageagent.RecoveryBlockedSlotEffectV3Repository)
		if !ok {
			return EffectRecoveryResult{}, persistedSlotEffectV3RepositoryError(imageagent.ErrRevisionConflict)
		}
		effect, err = restorer.RestoreRecoveryBlockedEffectV3(ctx, reservation)
		if err != nil {
			return EffectRecoveryResult{}, persistedSlotEffectV3RepositoryError(err)
		}
	}
	switch effect.Phase {
	case imageagent.SlotEffectV3PublicationComplete:
		return EffectRecoveryResult{
			Outcome: EffectRecoveryOutcomePublished, Published: effect.Published,
			EffectPhase: effect.Phase,
		}, nil
	case imageagent.SlotEffectV3ProviderUnknown:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeProviderUnknown, effect.Phase, slotProviderOutcomeUnknownCode), nil
	case imageagent.SlotEffectV3StagingUnknown:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeStagingUnknown, effect.Phase, slotStagingOutcomeUnknownCode), nil
	case imageagent.SlotEffectV3PublicationUnknown:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomePublicationUnknown, effect.Phase, slotPublicationOutcomeUnknownCode), nil
	case imageagent.SlotEffectV3ProviderNotDispatched:
		return a.blockEffectRecoveryV3(ctx, input, reservation)
	case imageagent.SlotEffectV3ProviderClaimed:
		if effect.Quote.Fingerprint != "" && effect.BudgetStatus == imageagent.SlotBudgetReleased {
			return a.blockEffectRecoveryV3(ctx, input, reservation)
		}
	case imageagent.SlotEffectV3RecoveryBlocked:
		return effectRecoveryBlockedResult(input), nil
	}
	budgetAuthorization := effect.Quote.Fingerprint != ""
	published, err := a.ExecuteSlotV3(ctx, ExecuteSlotV3ActivityInput{
		RunID: input.RunID, Identity: input.Identity, PlanRevision: input.PlanRevision,
		Slot: input.Slot, Attempt: input.Attempt,
		IdempotencyKey:             executionInput.IdempotencyKey,
		AssetCatalog:               input.AssetCatalog,
		ExternalEffectFinalization: true,
		BudgetAuthorization:        budgetAuthorization,
		BudgetPolicy:               effect.Policy,
	})
	if err == nil {
		return EffectRecoveryResult{
			Outcome: EffectRecoveryOutcomePublished, Published: published,
			EffectPhase: imageagent.SlotEffectV3PublicationComplete,
		}, nil
	}
	if result, mapped := effectRecoveryResultFromError(err); mapped {
		return result, nil
	}
	return EffectRecoveryResult{}, err
}

func (a *Activities) PersistRecoveryBlockedEffectV3(ctx context.Context, input EffectRecoveryWorkflowInput) (EffectRecoveryResult, error) {
	if a.slotEffectsV3 == nil {
		return EffectRecoveryResult{}, fmt.Errorf("image agent v3 activity dependencies are incomplete")
	}
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return EffectRecoveryResult{}, err
	}
	executionInput := imageagent.SlotExecutionInput{
		RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID,
		PlanRevision: input.PlanRevision, Slot: input.Slot, Attempt: input.Attempt,
		IdempotencyKey: slotAttemptKey(input.PlanRevision, input.Slot, input.Attempt),
		AssetCatalog:   input.AssetCatalog,
		ProductContext: input.AssetCatalog.ProductContext,
	}
	reservation := slotEffectReservationV3(executionInput)
	effect, err := a.slotEffectsV3.GetSlotExternalEffectV3(ctx, reservation.Identity)
	if err != nil {
		if errors.Is(err, imageagent.ErrRunNotFound) {
			return a.persistMissingEffectRecoveryBlockedV3(ctx, input, reservation)
		}
		if result, handled, blockErr := a.failClosedCorruptEffectRecovery(ctx, input, err); handled {
			return result, blockErr
		}
		return EffectRecoveryResult{}, persistedSlotEffectV3RepositoryError(err)
	}
	if err := validatePersistedSlotEffectV3(effect); err != nil {
		return EffectRecoveryResult{}, err
	}
	reservation.Policy = effect.Policy
	reservation.Quote = effect.Quote
	switch effect.Phase {
	case imageagent.SlotEffectV3PublicationComplete:
		return EffectRecoveryResult{Outcome: EffectRecoveryOutcomePublished, Published: effect.Published, EffectPhase: effect.Phase}, nil
	case imageagent.SlotEffectV3ProviderUnknown:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeProviderUnknown, effect.Phase, slotProviderOutcomeUnknownCode), nil
	case imageagent.SlotEffectV3StagingUnknown:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeStagingUnknown, effect.Phase, slotStagingOutcomeUnknownCode), nil
	case imageagent.SlotEffectV3PublicationUnknown:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomePublicationUnknown, effect.Phase, slotPublicationOutcomeUnknownCode), nil
	case imageagent.SlotEffectV3RecoveryBlocked:
		return effectRecoveryBlockedResult(input), nil
	}
	return a.blockEffectRecoveryV3(ctx, input, reservation)
}

func (a *Activities) StartEffectRecoveryV3(ctx context.Context, input EffectRecoveryWorkflowInput) error {
	if a.recoveryWorkflowStarter == nil {
		return fmt.Errorf("image agent effect recovery workflow starter is required")
	}
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return err
	}
	return a.recoveryWorkflowStarter(ctx, input)
}

func (a *Activities) ReconcileEffectRecoveryV3(ctx context.Context, input EffectRecoveryWorkflowInput) (EffectRecoveryResult, error) {
	if a.slotEffectsV3 == nil {
		return EffectRecoveryResult{}, fmt.Errorf("image agent v3 activity dependencies are incomplete")
	}
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return EffectRecoveryResult{}, err
	}
	execution := imageagent.SlotExecutionInput{
		RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID,
		PlanRevision: input.PlanRevision, Slot: input.Slot, Attempt: input.Attempt,
		IdempotencyKey: slotAttemptKey(input.PlanRevision, input.Slot, input.Attempt),
		AssetCatalog:   input.AssetCatalog, ProductContext: input.AssetCatalog.ProductContext,
	}
	effect, err := a.slotEffectsV3.GetSlotExternalEffectV3(ctx, slotEffectReservationV3(execution).Identity)
	if err != nil {
		if result, handled, blockErr := a.failClosedCorruptEffectRecovery(ctx, input, err); handled {
			return result, blockErr
		}
		return EffectRecoveryResult{}, persistedSlotEffectV3RepositoryError(err)
	}
	if err := validatePersistedSlotEffectV3(effect); err != nil {
		return EffectRecoveryResult{}, err
	}
	result, err := effectRecoveryResultFromDurableEffect(effect)
	if err != nil {
		return EffectRecoveryResult{}, err
	}
	if err := a.reconcileEffectRecoveryProjection(ctx, input, effect, result); err != nil {
		return EffectRecoveryResult{}, err
	}
	return result, nil
}

func (a *Activities) failClosedCorruptEffectRecovery(ctx context.Context, input EffectRecoveryWorkflowInput, cause error) (EffectRecoveryResult, bool, error) {
	if !errors.Is(cause, imageagent.ErrCorruptPersistedEffect) {
		return EffectRecoveryResult{}, false, nil
	}
	corruptor, ok := a.slotEffectsV3.(imageagent.CorruptSlotEffectV3Repository)
	if !ok {
		return EffectRecoveryResult{}, true, persistedSlotEffectV3RepositoryError(cause)
	}
	identity := imageagent.SlotExternalEffectIdentity{
		RunScope:     imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID},
		PlanRevision: input.PlanRevision, SlotID: input.Slot.ID, Attempt: input.Attempt,
	}
	blocked, err := corruptor.BlockCorruptSlotEffectV3(ctx, identity)
	if err != nil {
		return EffectRecoveryResult{}, true, persistedSlotEffectV3RepositoryError(err)
	}
	return EffectRecoveryResult{
		Outcome:     EffectRecoveryOutcomeRecoveryBlocked,
		Published:   imageagent.SlotEffectV3PublishedResult{SlotID: strings.TrimSpace(input.Slot.ID), Attempt: input.Attempt},
		EffectPhase: blocked.Phase, BlockedCode: blocked.BlockedCode,
	}, true, nil
}

func effectRecoveryResultFromDurableEffect(effect imageagent.SlotEffectV3Attempt) (EffectRecoveryResult, error) {
	switch effect.Phase {
	case imageagent.SlotEffectV3PublicationComplete:
		return EffectRecoveryResult{Outcome: EffectRecoveryOutcomePublished, Published: effect.Published, EffectPhase: effect.Phase}, nil
	case imageagent.SlotEffectV3ProviderUnknown:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeProviderUnknown, effect.Phase, effect.BlockedCode), nil
	case imageagent.SlotEffectV3StagingUnknown:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeStagingUnknown, effect.Phase, effect.BlockedCode), nil
	case imageagent.SlotEffectV3PublicationUnknown:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomePublicationUnknown, effect.Phase, effect.BlockedCode), nil
	case imageagent.SlotEffectV3RecoveryBlocked:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeRecoveryBlocked, effect.Phase, effect.BlockedCode), nil
	default:
		return EffectRecoveryResult{}, fmt.Errorf("effect recovery phase %q is not durable reconciliation evidence", effect.Phase)
	}
}

func (a *Activities) reconcileEffectRecoveryProjection(ctx context.Context, input EffectRecoveryWorkflowInput, effect imageagent.SlotEffectV3Attempt, result EffectRecoveryResult) error {
	scope := imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID}
	current, err := a.repository.GetProjection(ctx, scope)
	if err != nil {
		return fmt.Errorf("get image agent recovery parent projection: %w", err)
	}
	if current.Run.Status != imageagent.RunStatusBlocked || current.Run.ActivePlanRevision != input.PlanRevision || current.Plan.Revision != input.PlanRevision {
		return imageagent.ErrRevisionConflict
	}
	slotIndex := slotProjectionIndex(current.Slots, input.Slot.ID)
	if slotIndex < 0 || current.Slots[slotIndex].Attempt != input.Attempt {
		return imageagent.ErrRevisionConflict
	}
	owners, err := imageagent.NormalizeRecoverableEffects(current.RecoverableEffects)
	if err != nil {
		return err
	}
	ownerIndex := recoverableEffectIndex(owners, input.Slot.ID, input.Attempt)
	if ownerIndex < 0 {
		if effect.Phase == imageagent.SlotEffectV3PublicationComplete && recoveredSlotProjectionMatches(current.Slots[slotIndex], effect.Published) {
			return nil
		}
		return imageagent.ErrRevisionConflict
	}
	if effect.Phase != imageagent.SlotEffectV3PublicationComplete && recoveredBlockedProjectionMatches(current, slotIndex, ownerIndex, result.BlockedCode) {
		return nil
	}
	updated := current
	updated.Slots = append([]imageagent.SlotProjection(nil), current.Slots...)
	updated.RecoverableEffects = append([]imageagent.RecoverableEffect(nil), owners...)
	updated.Run.Block = cloneTemporalBlock(current.Run.Block)
	updated.Run.Version++
	if effect.Phase == imageagent.SlotEffectV3PublicationComplete {
		published, normalizeErr := imageagent.NormalizeSlotEffectV3PublishedResult(effect.Published)
		if normalizeErr != nil {
			return normalizeErr
		}
		candidates := make([]imageagent.AssetCandidate, 0, len(published.Candidates))
		for _, candidate := range published.Candidates {
			candidates = append(candidates, imageagent.AssetCandidate{
				AssetID: candidate.AssetID, SourceAssetID: candidate.SourceAssetID, DurableAsset: candidate.DurableAsset,
			})
		}
		updated.Slots[slotIndex] = imageagent.SlotProjection{
			Slot: updated.Slots[slotIndex].Slot, Attempt: published.Attempt, Candidates: candidates,
		}
		updated.Slots[slotIndex].Slot.Status = imageagent.SlotStatusAccepted
		updated.RecoverableEffects = append(updated.RecoverableEffects[:ownerIndex], updated.RecoverableEffects[ownerIndex+1:]...)
		if updated.Run.Block != nil && strings.TrimSpace(updated.Run.Block.SlotID) == strings.TrimSpace(input.Slot.ID) {
			updated.Run.Block = recoveryParentBlock(updated.RecoverableEffects)
		}
	} else {
		updated.RecoverableEffects[ownerIndex].Code = result.BlockedCode
		updated.Slots[slotIndex].ErrorCode = result.BlockedCode
		if updated.Run.Block != nil && strings.TrimSpace(updated.Run.Block.SlotID) == strings.TrimSpace(input.Slot.ID) {
			updated.Run.Block.Code = result.BlockedCode
			updated.Run.Block.Message = result.BlockedCode
		}
	}
	commitID, err := effectRecoveryReconciliationCommitID(input, effect.Phase)
	if err != nil {
		return err
	}
	candidateIDs := make([]string, 0, len(updated.Slots[slotIndex].Candidates))
	for _, candidate := range updated.Slots[slotIndex].Candidates {
		candidateIDs = append(candidateIDs, candidate.AssetID)
	}
	outcome := "blocked"
	if updated.Slots[slotIndex].Slot.Status == imageagent.SlotStatusAccepted {
		outcome = "accepted"
	}
	eventPayload, err := json.Marshal(struct {
		RunID        string                       `json:"run_id"`
		PlanRevision int64                        `json:"plan_revision"`
		SlotID       string                       `json:"slot_id"`
		Attempt      int                          `json:"attempt"`
		EffectPhase  imageagent.SlotEffectV3Phase `json:"effect_phase"`
	}{input.RunID, input.PlanRevision, input.Slot.ID, input.Attempt, effect.Phase})
	if err != nil {
		return err
	}
	_, err = a.repository.CommitProjection(ctx, imageagent.ProjectionCommit{
		Scope: scope, CommitID: commitID, ExpectedProjectionVersion: current.ProjectionVersion,
		Snapshot: updated, EventType: "effect.recovery.reconciled", EventPayload: eventPayload,
		ExpectedRunVersion: current.Run.Version,
		RunMutation: &imageagent.RunMutation{
			Status: updated.Run.Status, CurrentNode: updated.Run.CurrentNode,
			ActivePlanRevision: updated.Run.ActivePlanRevision, Block: updated.Run.Block,
		},
		SlotMutation: &imageagent.SlotProjectionMutation{
			PlanRevision: input.PlanRevision,
			Result: imageagent.SlotResult{
				SlotID: input.Slot.ID, Attempt: input.Attempt, Status: updated.Slots[slotIndex].Slot.Status,
				CandidateAssetIDs: candidateIDs, ErrorCode: updated.Slots[slotIndex].ErrorCode,
			},
			Projection: updated.Slots[slotIndex],
			Attempt: imageagent.StepAttempt{
				TenantID: scope.TenantID, OwnerUserID: scope.OwnerUserID, RunID: scope.RunID,
				PlanRevision: input.PlanRevision, SlotID: input.Slot.ID, Attempt: input.Attempt,
				Node:           updated.Run.CurrentNode,
				IdempotencyKey: fmt.Sprintf("%s:slot:%s:attempt:%d", commitID, input.Slot.ID, input.Attempt),
				Outcome:        outcome, ErrorCategory: updated.Slots[slotIndex].ErrorCode,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("commit image agent recovery parent projection: %w", err)
	}
	return nil
}

func effectRecoveryReconciliationCommitID(input EffectRecoveryWorkflowInput, phase imageagent.SlotEffectV3Phase) (string, error) {
	return updateFingerprint("effect_recovery_reconcile", struct {
		TenantID, OwnerUserID, RunID, SlotID string
		PlanRevision                         int64
		Attempt                              int
		Phase                                imageagent.SlotEffectV3Phase
	}{
		TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID,
		SlotID: input.Slot.ID, PlanRevision: input.PlanRevision, Attempt: input.Attempt, Phase: phase,
	})
}

func recoverableEffectIndex(effects []imageagent.RecoverableEffect, slotID string, attempt int) int {
	for index, effect := range effects {
		if effect.SlotID == strings.TrimSpace(slotID) && effect.Attempt == attempt {
			return index
		}
	}
	return -1
}

func recoveryParentBlock(effects []imageagent.RecoverableEffect) *imageagent.Block {
	if len(effects) == 0 {
		return nil
	}
	return &imageagent.Block{Code: effects[0].Code, Message: effects[0].Code, SlotID: effects[0].SlotID}
}

func recoveredSlotProjectionMatches(slot imageagent.SlotProjection, published imageagent.SlotEffectV3PublishedResult) bool {
	normalized, err := imageagent.NormalizeSlotEffectV3PublishedResult(published)
	if err != nil || slot.Slot.Status != imageagent.SlotStatusAccepted || slot.Attempt != normalized.Attempt || slot.ErrorCode != "" || len(slot.Candidates) != len(normalized.Candidates) {
		return false
	}
	for index, candidate := range normalized.Candidates {
		if slot.Candidates[index].AssetID != candidate.AssetID || slot.Candidates[index].SourceAssetID != candidate.SourceAssetID || slot.Candidates[index].DurableAsset != candidate.DurableAsset {
			return false
		}
	}
	return true
}

func recoveredBlockedProjectionMatches(projection imageagent.RunProjection, slotIndex, ownerIndex int, blockedCode string) bool {
	if projection.Slots[slotIndex].Slot.Status != imageagent.SlotStatusBlocked || projection.Slots[slotIndex].ErrorCode != blockedCode || projection.RecoverableEffects[ownerIndex].Code != blockedCode {
		return false
	}
	if projection.Run.Block == nil || strings.TrimSpace(projection.Run.Block.SlotID) != strings.TrimSpace(projection.Slots[slotIndex].Slot.ID) {
		return true
	}
	return projection.Run.Block.Code == blockedCode && projection.Run.Block.Message == blockedCode
}

func (a *Activities) persistMissingEffectRecoveryBlockedV3(ctx context.Context, input EffectRecoveryWorkflowInput, reservation imageagent.SlotEffectV3Reservation) (EffectRecoveryResult, error) {
	effect, _, err := a.slotEffectsV3.ReserveSlotProviderV3(ctx, reservation)
	if err != nil {
		return EffectRecoveryResult{}, persistedSlotEffectV3RepositoryError(err)
	}
	if err := validatePersistedSlotEffectV3(effect); err != nil {
		return EffectRecoveryResult{}, err
	}
	reservation.Policy = effect.Policy
	reservation.Quote = effect.Quote
	if effect.Phase == imageagent.SlotEffectV3RecoveryBlocked {
		return effectRecoveryBlockedResult(input), nil
	}
	return a.blockEffectRecoveryV3(ctx, input, reservation)
}

func (a *Activities) blockEffectRecoveryV3(ctx context.Context, input EffectRecoveryWorkflowInput, reservation imageagent.SlotEffectV3Reservation) (EffectRecoveryResult, error) {
	blocked, err := a.slotEffectsV3.BlockSlotEffectV3(ctx, imageagent.SlotEffectV3BlockTransition{
		Reservation: reservation,
		Phase:       imageagent.SlotEffectV3RecoveryBlocked,
		Code:        imageagent.SlotRecoveryBlockedCode,
	})
	if err != nil {
		return EffectRecoveryResult{}, persistedSlotEffectV3RepositoryError(err)
	}
	return EffectRecoveryResult{
		Outcome:     EffectRecoveryOutcomeRecoveryBlocked,
		Published:   imageagent.SlotEffectV3PublishedResult{SlotID: input.Slot.ID, Attempt: input.Attempt},
		EffectPhase: blocked.Phase,
		BlockedCode: blocked.BlockedCode,
	}, nil
}

func (a *Activities) publicationRecoveryError(message string, publication imageagent.PublicationClaim, cause error) error {
	delay := time.Until(publication.LeaseExpiresAt)
	if delay < 0 {
		delay = 0
	}
	delay += publicationLeaseRetrySafetyMargin
	maxDelay := a.publicationLeaseDuration + publicationLeaseRetrySafetyMargin
	if delay > maxDelay {
		delay = maxDelay
	}
	details := slotPublicationRecoveryDetails{
		RetryDelay: delay, LeaseExpiresAt: publication.LeaseExpiresAt,
		Owner: publication.Owner, Fence: publication.Fence,
	}
	return sdktemporal.NewApplicationErrorWithOptions(
		message,
		slotPublicationRecoveryErrorType,
		sdktemporal.ApplicationErrorOptions{
			NonRetryable: true, Cause: cause, Details: []interface{}{details}, NextRetryDelay: delay,
		},
	)
}

func effectRecoveryBlockedPhaseResult(outcome EffectRecoveryOutcome, phase imageagent.SlotEffectV3Phase, blockedCode string) EffectRecoveryResult {
	return EffectRecoveryResult{Outcome: outcome, EffectPhase: phase, BlockedCode: blockedCode}
}

func effectRecoveryResultFromError(err error) (EffectRecoveryResult, bool) {
	var applicationError *sdktemporal.ApplicationError
	if !errors.As(err, &applicationError) {
		return EffectRecoveryResult{}, false
	}
	switch applicationError.Type() {
	case slotProviderOutcomeUnknownCode:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeProviderUnknown, imageagent.SlotEffectV3ProviderUnknown, slotProviderOutcomeUnknownCode), true
	case slotStagingOutcomeUnknownCode:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomeStagingUnknown, imageagent.SlotEffectV3StagingUnknown, slotStagingOutcomeUnknownCode), true
	case slotPublicationOutcomeUnknownCode:
		return effectRecoveryBlockedPhaseResult(EffectRecoveryOutcomePublicationUnknown, imageagent.SlotEffectV3PublicationUnknown, slotPublicationOutcomeUnknownCode), true
	default:
		return EffectRecoveryResult{}, false
	}
}

func (a *Activities) preserveSlotRecoveryBundle(ctx context.Context, identity imageagent.SlotExternalEffectIdentity, prepared objectstore.PreparedSlotArtifacts) error {
	return resilience.Retry(ctx, resilience.RetryConfig{
		MaxAttempts: recoveryBundlePersistenceAttempts, InitialDelay: recoveryBundlePersistenceInitialDelay,
		MaxDelay: recoveryBundlePersistenceMaxDelay, Multiplier: 2, RandomizationFactor: 0,
		IsRetryable: func(err error) bool {
			return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) &&
				!errors.Is(err, objectstore.ErrArtifactUnavailable) && !errors.Is(err, objectstore.ErrObjectConflict)
		},
	}, func(retryCtx context.Context) error {
		return a.artifactStore.PreserveSlotArtifacts(retryCtx, identity, prepared)
	})
}

func validatePersistedSlotEffectV3(effect imageagent.SlotEffectV3Attempt) error {
	if err := imageagent.ValidateSlotEffectV3AttemptPolicy(effect); err != nil {
		code := slotEffectPolicyInvalidCode
		switch effect.Phase {
		case imageagent.SlotEffectV3ProviderClaimed, imageagent.SlotEffectV3StagingPrepared, imageagent.SlotEffectV3ArtifactStaged,
			imageagent.SlotEffectV3PublicationClaimed, imageagent.SlotEffectV3PublicationComplete,
			imageagent.SlotEffectV3ProviderUnknown, imageagent.SlotEffectV3StagingUnknown, imageagent.SlotEffectV3PublicationUnknown,
			imageagent.SlotEffectV3RecoveryBlocked:
		default:
			code = slotEffectPhaseInvalidCode
		}
		return sdktemporal.NewNonRetryableApplicationError("invalid persisted slot effect policy", code, err)
	}
	return nil
}

func persistedSlotEffectV3RepositoryError(err error) error {
	if errors.Is(err, imageagent.ErrInvalidPersistedPolicy) {
		return sdktemporal.NewNonRetryableApplicationError("invalid persisted slot effect policy", slotEffectPolicyInvalidCode, err)
	}
	return err
}

func slotExecutionInputV3(input ExecuteSlotV3ActivityInput) imageagent.SlotExecutionInput {
	return imageagent.SlotExecutionInput{
		RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID,
		PlanRevision: input.PlanRevision, Slot: input.Slot, Attempt: input.Attempt,
		IdempotencyKey: input.IdempotencyKey, AssetCatalog: input.AssetCatalog, ProductContext: input.AssetCatalog.ProductContext,
	}
}

func slotEffectReservationV3(input imageagent.SlotExecutionInput) imageagent.SlotEffectV3Reservation {
	return imageagent.SlotEffectV3Reservation{
		Identity: imageagent.SlotExternalEffectIdentity{
			RunScope:     imageagent.RunScope{TenantID: input.TenantID, OwnerUserID: input.UserID, RunID: input.RunID},
			PlanRevision: input.PlanRevision, SlotID: input.Slot.ID, Attempt: input.Attempt,
		},
		IdempotencyKey: input.IdempotencyKey, InputFingerprint: imageagent.SlotExecutionFingerprint(input),
	}
}

func prepareGeneratedSlotArtifacts(input imageagent.SlotExecutionInput, generated imageagent.SlotGeneratedOutput, store DurableArtifactStore) (objectstore.PreparedSlotArtifacts, error) {
	if generated.SlotID != input.Slot.ID || generated.Attempt != input.Attempt || strings.TrimSpace(generated.SourceAssetID) == "" || len(generated.Assets) == 0 {
		return objectstore.PreparedSlotArtifacts{}, imageagent.ErrValidation
	}
	assets := make([]objectstore.ArtifactInput, len(generated.Assets))
	for index, generatedAsset := range generated.Assets {
		localPath := strings.TrimSpace(generatedAsset.Metadata["local_path"])
		if localPath == "" && !strings.Contains(generatedAsset.URL, "://") {
			localPath = strings.TrimSpace(generatedAsset.URL)
		}
		if localPath == "" {
			return objectstore.PreparedSlotArtifacts{}, imageagent.ErrValidation
		}
		data, err := os.ReadFile(localPath)
		if err != nil {
			return objectstore.PreparedSlotArtifacts{}, fmt.Errorf("read generated artifact %d: %w", index, err)
		}
		info, err := imagex.Inspect(data)
		if err != nil {
			return objectstore.PreparedSlotArtifacts{}, fmt.Errorf("inspect generated artifact %d: %w", index, err)
		}
		contentType := map[string]string{"jpeg": "image/jpeg", "png": "image/png", "webp": "image/webp"}[info.Format]
		if contentType == "" {
			return objectstore.PreparedSlotArtifacts{}, imageagent.ErrValidation
		}
		assets[index] = objectstore.ArtifactInput{
			Bytes: data, ContentType: contentType, Width: info.Width, Height: info.Height,
			SourceAssetID: generated.SourceAssetID, Operations: generatedAsset.Operations,
		}
	}
	return store.PrepareSlotArtifacts(objectstore.PrepareSlotArtifactsInput{Identity: slotEffectReservationV3(input).Identity, Assets: assets})
}

func cleanupGeneratedSlotTemporaryAssets(generated *imageagent.SlotGeneratedOutput) {
	if generated == nil {
		return
	}
	for index := range generated.Assets {
		asset := &generated.Assets[index]
		transient := productimage.ImageAsset{URL: asset.URL, Metadata: asset.Metadata}
		productimage.CleanupTemporaryAsset(&transient)
		asset.Metadata = transient.Metadata
	}
}

func expectedFinalManifestV3(input imageagent.SlotExecutionInput, staging imageagent.StagingManifest) (imageagent.FinalManifest, error) {
	staging, err := imageagent.NormalizeStagingManifest(staging)
	if err != nil {
		return imageagent.FinalManifest{}, err
	}
	assets := make([]imageagent.PublishedAssetRef, len(staging.Assets))
	for index, staged := range staging.Assets {
		if !strings.HasPrefix(staged.ObjectKey, "image-agent/staging/") {
			return imageagent.FinalManifest{}, imageagent.ErrValidation
		}
		asset := imageagent.PublishedAssetRef{
			ObjectKey: strings.Replace(staged.ObjectKey, "image-agent/staging/", imageagent.PublishedArtifactPrefix+"/", 1),
			SHA256:    staged.SHA256, SizeBytes: staged.SizeBytes, ContentType: staged.ContentType,
			Width: staged.Width, Height: staged.Height, SourceAssetID: staged.SourceAssetID,
			Operations: cloneStringSlicePreservingNil(staged.Operations), ProviderReceiptID: staged.ProviderReceiptID,
		}
		if err := imageagent.ValidatePublishedAssetRefForSlot(input, asset, index); err != nil {
			return imageagent.FinalManifest{}, err
		}
		assets[index] = asset
	}
	return imageagent.NormalizeFinalManifest(imageagent.FinalManifest{Assets: assets})
}

func cloneStringSlicePreservingNil(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}

func (a *Activities) renewPublicationV3(ctx context.Context, identity imageagent.SlotExternalEffectIdentity, publication imageagent.PublicationClaim) (imageagent.PublicationClaim, error) {
	claim, err := a.slotEffectsV3.RenewSlotPublicationV3(ctx, imageagent.PublicationLeaseRenewal{
		Identity: identity, Owner: publication.Owner, Fence: publication.Fence, LeaseDuration: a.publicationLeaseDuration,
	})
	if err != nil {
		return imageagent.PublicationClaim{}, fmt.Errorf("renew slot publication claim: %w", persistedSlotEffectV3RepositoryError(err))
	}
	return claim, nil
}

func (a *Activities) blockSlotEffectV3(ctx context.Context, reservation imageagent.SlotEffectV3Reservation, phase imageagent.SlotEffectV3Phase, code string, publication imageagent.PublicationClaim) error {
	_, err := a.slotEffectsV3.BlockSlotEffectV3(ctx, imageagent.SlotEffectV3BlockTransition{
		Reservation: reservation, Phase: phase, Code: code, Owner: publication.Owner, Fence: publication.Fence,
	})
	if err != nil {
		return fmt.Errorf("persist blocked slot effect: %w", persistedSlotEffectV3RepositoryError(err))
	}
	return blockedSlotEffectV3Error(code)
}

func blockedSlotEffectV3Error(code string) error {
	return sdktemporal.NewNonRetryableApplicationError("slot external effect outcome requires a new user attempt or operator reconciliation", code, nil)
}

func temporalPublicationOwner(ctx context.Context) (string, error) {
	if !sdkactivity.IsActivity(ctx) {
		return "", errPublicationOwnerRequiresActivity
	}
	return publicationOwnerFromActivityInfo(sdkactivity.GetInfo(ctx))
}

func publicationOwnerFromActivityInfo(info sdkactivity.Info) (string, error) {
	runID, activityID := strings.TrimSpace(info.WorkflowExecution.RunID), strings.TrimSpace(info.ActivityID)
	if runID == "" || activityID == "" || info.Attempt <= 0 {
		return "", fmt.Errorf("Temporal workflow run ID, activity ID, and positive attempt are required")
	}
	return fmt.Sprintf("%s/%s/%d", runID, activityID, info.Attempt), nil
}

func (a *Activities) PersistSlotResult(ctx context.Context, input PersistSlotResultActivityInput) error {
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return err
	}
	result := input.Result
	if strings.TrimSpace(result.Execution.SlotID) == "" || result.Execution.Attempt <= 0 {
		return fmt.Errorf("terminal slot result requires slot ID and positive attempt")
	}
	var candidateIDs []string
	var candidates []imageagent.AssetCandidate
	for _, candidate := range result.Execution.Candidates {
		id := strings.TrimSpace(candidate.AssetID)
		if id == "" {
			if result.Status == imageagent.SlotStatusAccepted {
				return fmt.Errorf("accepted slot result contains an empty candidate asset ID")
			}
			continue
		}
		validatedURL, validateErr := imageagent.ValidateSafeImageURL(candidate.URL)
		if validateErr != nil {
			return fmt.Errorf("candidate %q has unsafe URL: %w", id, validateErr)
		}
		candidate.URL = validatedURL
		candidateIDs = append(candidateIDs, id)
		candidates = append(candidates, candidate)
	}
	if result.Status == imageagent.SlotStatusAccepted && len(candidateIDs) == 0 {
		return fmt.Errorf("accepted slot result requires at least one candidate asset ID")
	}
	outcome := "accepted"
	if result.Status != imageagent.SlotStatusAccepted {
		outcome = "blocked"
	}
	attempt := imageagent.StepAttempt{
		TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID,
		PlanRevision: input.PlanRevision, SlotID: result.Execution.SlotID, Node: "execute_slot",
		IdempotencyKey: input.AttemptKey, Attempt: result.Execution.Attempt,
		Outcome: outcome, ErrorCategory: result.ErrorCode,
	}
	scope := imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID}
	current, err := a.repository.GetProjection(ctx, scope)
	if err != nil {
		return fmt.Errorf("load image agent projection: %w", err)
	}
	if slotProjectionAlreadyPersisted(current.Slots, result.Execution.SlotID, result.Execution.Attempt, result.Status, candidates, result.ErrorCode) {
		return nil
	}
	storedResult := imageagent.SlotResult{
		SlotID: result.Execution.SlotID, Attempt: result.Execution.Attempt,
		Status: result.Status, CandidateAssetIDs: candidateIDs, ErrorCode: result.ErrorCode,
	}
	updated := current
	found := false
	for index := range updated.Slots {
		if updated.Slots[index].Slot.ID != result.Execution.SlotID {
			continue
		}
		updated.Slots[index] = imageagent.SlotProjection{Slot: updated.Slots[index].Slot, Attempt: result.Execution.Attempt, Candidates: candidates, ErrorCode: result.ErrorCode}
		updated.Slots[index].Slot.Status = result.Status
		found = true
	}
	if !found {
		return imageagent.ErrRevisionConflict
	}
	eventPayload, err := json.Marshal(slotResultPersistedEventPayload{
		PlanRevision: input.PlanRevision, SlotID: result.Execution.SlotID,
		Attempt: result.Execution.Attempt, AttemptKey: input.AttemptKey,
		Status: result.Status, CandidateAssetIDs: candidateIDs, ErrorCode: result.ErrorCode,
	})
	if err != nil {
		return fmt.Errorf("encode terminal slot result event: %w", err)
	}
	_, err = a.repository.CommitProjection(ctx, imageagent.ProjectionCommit{Scope: scope, CommitID: "slot:" + input.AttemptKey, ExpectedProjectionVersion: current.ProjectionVersion, Snapshot: updated, EventType: slotResultPersistedEventType, EventPayload: eventPayload, SlotMutation: &imageagent.SlotProjectionMutation{PlanRevision: input.PlanRevision, Result: storedResult, Projection: updated.Slots[slotProjectionIndex(updated.Slots, result.Execution.SlotID)], Attempt: attempt}})
	if err != nil {
		return fmt.Errorf("commit terminal slot projection: %w", err)
	}
	return nil
}

// PersistSlotResultV3 persists only the additive v3 durable result contract.
// It is intentionally absent from RegisterActivities until Task 6 selects the
// final production wire.
func (a *Activities) PersistSlotResultV3(ctx context.Context, input PersistSlotResultV3ActivityInput) error {
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return err
	}
	result := input.Result
	if strings.TrimSpace(result.Published.SlotID) == "" || result.Published.Attempt <= 0 {
		return fmt.Errorf("terminal v3 slot result requires slot ID and positive attempt")
	}
	scope := imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID}
	current, err := a.repository.GetProjection(ctx, scope)
	if err != nil {
		return fmt.Errorf("load image agent projection: %w", err)
	}
	slotIndex := slotProjectionIndex(current.Slots, result.Published.SlotID)
	if slotIndex < 0 {
		return imageagent.ErrRevisionConflict
	}
	if result.Status == imageagent.SlotStatusAccepted && current.Slots[slotIndex].Slot.Role == imageagent.SlotRoleMain && len(result.Published.Candidates) != 1 {
		result.Status = imageagent.SlotStatusBlocked
		result.ErrorCode = invalidMainCandidateCountCode
		result.Published.Candidates = nil
	}
	var candidateIDs []string
	var candidates []imageagent.AssetCandidate
	if result.Status == imageagent.SlotStatusAccepted {
		normalized, normalizeErr := imageagent.NormalizeSlotEffectV3PublishedResult(result.Published)
		if normalizeErr != nil {
			return fmt.Errorf("normalize terminal v3 slot result: %w", normalizeErr)
		}
		result.Published = normalized
		for _, candidate := range normalized.Candidates {
			candidateIDs = append(candidateIDs, candidate.AssetID)
			candidates = append(candidates, imageagent.AssetCandidate{AssetID: candidate.AssetID, SourceAssetID: candidate.SourceAssetID, DurableAsset: candidate.DurableAsset})
		}
	}
	outcome := "accepted"
	if result.Status != imageagent.SlotStatusAccepted {
		outcome = "blocked"
	}
	attempt := imageagent.StepAttempt{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID, PlanRevision: input.PlanRevision, SlotID: result.Published.SlotID, Node: "execute_slot_v3", IdempotencyKey: input.AttemptKey, Attempt: result.Published.Attempt, Outcome: outcome, ErrorCategory: result.ErrorCode}
	if slotProjectionAlreadyPersisted(current.Slots, result.Published.SlotID, result.Published.Attempt, result.Status, candidates, result.ErrorCode) {
		return nil
	}
	storedResult := imageagent.SlotResult{SlotID: result.Published.SlotID, Attempt: result.Published.Attempt, Status: result.Status, CandidateAssetIDs: candidateIDs, ErrorCode: result.ErrorCode}
	updated := current
	updated.Slots[slotIndex] = imageagent.SlotProjection{Slot: updated.Slots[slotIndex].Slot, Attempt: result.Published.Attempt, Candidates: candidates, ErrorCode: result.ErrorCode}
	updated.Slots[slotIndex].Slot.Status = result.Status
	eventPayload, err := json.Marshal(slotResultPersistedEventPayload{PlanRevision: input.PlanRevision, SlotID: result.Published.SlotID, Attempt: result.Published.Attempt, AttemptKey: input.AttemptKey, Status: result.Status, CandidateAssetIDs: candidateIDs, ErrorCode: result.ErrorCode})
	if err != nil {
		return fmt.Errorf("encode terminal v3 slot result event: %w", err)
	}
	_, err = a.repository.CommitProjection(ctx, imageagent.ProjectionCommit{Scope: scope, CommitID: "slot-v3:" + input.AttemptKey, ExpectedProjectionVersion: current.ProjectionVersion, Snapshot: updated, EventType: slotResultPersistedEventType, EventPayload: eventPayload, SlotMutation: &imageagent.SlotProjectionMutation{PlanRevision: input.PlanRevision, Result: storedResult, Projection: updated.Slots[slotIndex], Attempt: attempt}})
	if err != nil {
		return fmt.Errorf("commit terminal v3 slot projection: %w", err)
	}
	return nil
}

func slotProjectionAlreadyPersisted(slots []imageagent.SlotProjection, slotID string, attempt int, status imageagent.SlotStatus, candidates []imageagent.AssetCandidate, errorCode string) bool {
	for _, slot := range slots {
		if slot.Slot.ID == slotID {
			return slot.Attempt == attempt && slot.Slot.Status == status && slot.ErrorCode == errorCode && reflect.DeepEqual(slot.Candidates, candidates)
		}
	}
	return false
}

type slotResultPersistedEventPayload struct {
	PlanRevision      int64                 `json:"plan_revision"`
	SlotID            string                `json:"slot_id"`
	Attempt           int                   `json:"attempt"`
	AttemptKey        string                `json:"attempt_key"`
	Status            imageagent.SlotStatus `json:"status"`
	CandidateAssetIDs []string              `json:"candidate_asset_ids"`
	ErrorCode         string                `json:"error_code,omitempty"`
}

func (a *Activities) PersistRunState(ctx context.Context, input PersistRunStateActivityInput) error {
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return err
	}
	scope := imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID}
	current, err := a.repository.GetProjection(ctx, scope)
	if err != nil {
		return fmt.Errorf("get image agent projection: %w", err)
	}
	if current.Run.ActivePlanRevision != input.PlanRevision {
		return imageagent.ErrRevisionConflict
	}
	if current.Run.Status == input.Projection.Status && current.Run.CurrentNode == input.CurrentNode &&
		reflect.DeepEqual(current.Run.Block, input.Projection.Block) && reflect.DeepEqual(current.Plan, input.Projection.Plan) &&
		reflect.DeepEqual(current.Slots, input.Projection.Slots) && current.ResultDigest == input.Projection.ResultDigest &&
		reflect.DeepEqual(current.PendingCommand, input.Projection.PendingCommand) &&
		reflect.DeepEqual(current.RecoverableEffects, input.Projection.RecoverableEffects) {
		return nil
	}
	updated := current
	updated.Run.Status = input.Projection.Status
	updated.Run.CurrentNode = input.CurrentNode
	updated.Run.Block = cloneTemporalBlock(input.Projection.Block)
	updated.Run.Version++
	updated.Plan = input.Projection.Plan
	if current.Run.Status == imageagent.RunStatusFailed && input.Projection.Status == imageagent.RunStatusExecuting && input.CurrentNode == "execute_slots" {
		// A failed execution may already have committed slot results. Preserve
		// those logical facts while the new Temporal execution replays their
		// stable external-effect identities.
		updated.Slots = append([]imageagent.SlotProjection(nil), current.Slots...)
	} else {
		updated.Slots = append([]imageagent.SlotProjection(nil), input.Projection.Slots...)
	}
	updated.ResultDigest = input.Projection.ResultDigest
	updated.PendingCommand = clonePendingReceipt(input.Projection.PendingCommand)
	updated.RecoverableEffects = append([]imageagent.RecoverableEffect(nil), input.Projection.RecoverableEffects...)
	updated.CommandIngress = input.Projection.CommandIngress
	slotMutations, err := recoverySlotCodeMutations(current.Slots, updated.Slots, scope, input.PlanRevision, input.CurrentNode, input.CommitID)
	if err != nil {
		return err
	}
	_, err = a.repository.CommitProjection(ctx, imageagent.ProjectionCommit{
		Scope: scope, CommitID: input.CommitID, ExpectedProjectionVersion: current.ProjectionVersion,
		Snapshot: updated, EventType: "run.updated", EventPayload: json.RawMessage(`{}`),
		ExpectedRunVersion: current.Run.Version,
		RunMutation:        &imageagent.RunMutation{Status: updated.Run.Status, CurrentNode: input.CurrentNode, ActivePlanRevision: input.PlanRevision, Block: updated.Run.Block},
		SlotMutations:      slotMutations,
	})
	if err != nil {
		return fmt.Errorf("commit image agent run projection: %w", err)
	}
	return nil
}

func recoverySlotCodeMutations(current, updated []imageagent.SlotProjection, scope imageagent.RunScope, planRevision int64, node, commitID string) ([]imageagent.SlotProjectionMutation, error) {
	if len(current) != len(updated) {
		return nil, imageagent.ErrRevisionConflict
	}
	mutations := make([]imageagent.SlotProjectionMutation, 0, len(updated))
	for index := range updated {
		before, after := current[index], updated[index]
		if reflect.DeepEqual(before, after) {
			continue
		}
		afterCode := after.ErrorCode
		before.ErrorCode, after.ErrorCode = "", ""
		if !reflect.DeepEqual(before, after) || after.Attempt <= 0 || after.Slot.Status != imageagent.SlotStatusBlocked || !imageagent.IsRecoverableEffectBlockCode(afterCode) {
			return nil, fmt.Errorf("%w: run-state persistence may only refresh recoverable slot codes", imageagent.ErrRevisionConflict)
		}
		candidateIDs := make([]string, 0, len(after.Candidates))
		for _, candidate := range after.Candidates {
			candidateIDs = append(candidateIDs, candidate.AssetID)
		}
		mutations = append(mutations, imageagent.SlotProjectionMutation{
			PlanRevision: planRevision,
			Result: imageagent.SlotResult{
				SlotID: after.Slot.ID, Attempt: after.Attempt, Status: after.Slot.Status,
				CandidateAssetIDs: candidateIDs, ErrorCode: afterCode,
			},
			Projection: updated[index],
			Attempt: imageagent.StepAttempt{
				TenantID: scope.TenantID, OwnerUserID: scope.OwnerUserID, RunID: scope.RunID, PlanRevision: planRevision,
				SlotID: after.Slot.ID, Attempt: after.Attempt, Node: node,
				IdempotencyKey: fmt.Sprintf("%s:slot:%s:attempt:%d", commitID, after.Slot.ID, after.Attempt),
				Outcome:        "blocked", ErrorCategory: afterCode,
			},
		})
	}
	return mutations, nil
}

func (a *Activities) PersistWorkflowFailure(ctx context.Context, input PersistWorkflowFailureActivityInput) error {
	return a.persistWorkflowFailure(ctx, input.RunID, input.Identity, input.FailureCode, input.FailureMessage, "workflow-failed")
}

func (a *Activities) PersistWorkflowFailureV2(ctx context.Context, input PersistWorkflowFailureV2ActivityInput) error {
	commitID := strings.TrimSpace(input.CommitID)
	if commitID == "" {
		return fmt.Errorf("workflow failure projection commit ID is required")
	}
	return a.persistWorkflowFailure(ctx, input.RunID, input.Identity, input.FailureCode, input.FailureMessage, commitID)
}

func (a *Activities) persistWorkflowFailure(ctx context.Context, runID string, identity imageagent.ExecutionIdentity, failureCode, failureMessage, commitID string) error {
	ctx, err := restoreActivityIdentity(ctx, identity)
	if err != nil {
		return err
	}
	if strings.TrimSpace(runID) == "" || failureCode != "workflow_failed" || strings.TrimSpace(failureMessage) == "" {
		return fmt.Errorf("persist workflow failure input is invalid")
	}
	scope := imageagent.RunScope{TenantID: identity.TenantID, OwnerUserID: identity.UserID, RunID: runID}
	current, err := a.repository.GetProjection(ctx, scope)
	if err != nil {
		return fmt.Errorf("get failed image agent projection: %w", err)
	}
	if isTerminalRunStatus(current.Run.Status) {
		return nil
	}
	block := &imageagent.Block{Code: failureCode, Message: failureMessage}
	updated := current
	updated.Run.Status = imageagent.RunStatusFailed
	updated.Run.CurrentNode = "workflow_failed"
	updated.Run.Block = block
	updated.Run.Version++
	eventPayload, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf("encode image agent workflow failure: %w", err)
	}
	_, err = a.repository.CommitProjection(ctx, imageagent.ProjectionCommit{
		Scope: scope, CommitID: commitID, ExpectedProjectionVersion: current.ProjectionVersion,
		Snapshot: updated, EventType: "run.failed", EventPayload: eventPayload, ExpectedRunVersion: current.Run.Version,
		RunMutation: &imageagent.RunMutation{Status: imageagent.RunStatusFailed, CurrentNode: "workflow_failed", ActivePlanRevision: current.Run.ActivePlanRevision, Block: block},
	})
	if err != nil {
		return fmt.Errorf("commit failed image agent projection: %w", err)
	}
	return nil
}

func (a *Activities) PersistPlanRevision(ctx context.Context, input PersistPlanRevisionActivityInput) error {
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return err
	}
	if input.Plan.ParentRevision != input.ExpectedRevision || input.Plan.Revision <= input.ExpectedRevision || strings.TrimSpace(input.Plan.CreatedBy) != input.Identity.UserID {
		return fmt.Errorf("replacement plan revision, parent, and actor are invalid")
	}
	if err := imageagent.ValidatePlan(input.Plan); err != nil {
		return fmt.Errorf("validate replacement plan: %w", err)
	}
	scope := imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID}
	current, err := a.repository.GetProjection(ctx, scope)
	if err != nil {
		return err
	}
	if current.Plan.Revision == input.Plan.Revision && current.Run.ActivePlanRevision == input.Plan.Revision &&
		current.Run.Status == imageagent.RunStatusExecuting && reflect.DeepEqual(current.Plan, input.Plan) {
		return nil
	}
	updated := current
	updated.Plan = input.Plan
	updated.Run.ActivePlanRevision = input.Plan.Revision
	updated.Run.Status = imageagent.RunStatusExecuting
	updated.Run.CurrentNode = "execute_slots"
	updated.Run.Block = nil
	updated.Run.Version++
	updated.ResultDigest = ""
	updated.PendingCommand = nil
	updated.Slots = make([]imageagent.SlotProjection, len(input.Plan.Slots))
	for index, slot := range input.Plan.Slots {
		updated.Slots[index] = imageagent.SlotProjection{Slot: slot}
	}
	_, err = a.repository.CommitProjection(ctx, imageagent.ProjectionCommit{Scope: scope, CommitID: "plan:" + input.Plan.IdempotencyKey, ExpectedProjectionVersion: current.ProjectionVersion, Snapshot: updated, EventType: "plan.replaced", EventPayload: json.RawMessage(`{}`), ExpectedRunVersion: current.Run.Version, RunMutation: &imageagent.RunMutation{Status: imageagent.RunStatusExecuting, CurrentNode: "execute_slots", ActivePlanRevision: input.Plan.Revision}, PlanMutation: &imageagent.PlanProjectionMutation{ExpectedActiveRevision: input.ExpectedRevision, Plan: input.Plan}})
	if err != nil {
		return fmt.Errorf("commit replacement plan projection: %w", err)
	}
	return nil
}

func slotProjectionIndex(slots []imageagent.SlotProjection, slotID string) int {
	for index := range slots {
		if slots[index].Slot.ID == slotID {
			return index
		}
	}
	return -1
}
func cloneTemporalBlock(block *imageagent.Block) *imageagent.Block {
	if block == nil {
		return nil
	}
	cloned := *block
	return &cloned
}

func (a *Activities) PersistPendingCommand(ctx context.Context, input PersistPendingCommandActivityInput) error {
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return err
	}
	if strings.TrimSpace(input.CommitID) == "" {
		return fmt.Errorf("pending command projection commit ID is required")
	}
	scope := imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID}
	current, err := a.repository.GetProjection(ctx, scope)
	if err != nil {
		return err
	}
	if reflect.DeepEqual(current.PendingCommand, input.Receipt) && reflect.DeepEqual(current.CommandIngress, input.CommandIngress) {
		return nil
	}
	updated := current
	updated.PendingCommand = clonePendingReceipt(input.Receipt)
	updated.CommandIngress = input.CommandIngress
	eventType := "command.receipt.updated"
	if input.CommandIngress.Exhausted && !current.CommandIngress.Exhausted {
		eventType = "command.ingress.exhausted"
	}
	_, err = a.repository.CommitProjection(ctx, imageagent.ProjectionCommit{Scope: scope, CommitID: input.CommitID, ExpectedProjectionVersion: current.ProjectionVersion, Snapshot: updated, EventType: eventType, EventPayload: json.RawMessage(`{}`)})
	return err
}

func clonePendingReceipt(receipt *imageagent.PendingCommandReceipt) *imageagent.PendingCommandReceipt {
	if receipt == nil {
		return nil
	}
	cloned := *receipt
	return &cloned
}

func (a *Activities) PublishApproved(ctx context.Context, input PublishApprovedActivityInput) error {
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return err
	}
	_, err = a.publisher.PublishApproved(ctx, imageagent.PublishApprovedInput{
		RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID,
		PlanRevision: input.PlanRevision, CandidateAssetIDs: append([]string(nil), input.CandidateAssetIDs...),
		IdempotencyKey: input.IdempotencyKey,
	})
	return err
}

// PublishApprovedV3 is additive and intentionally not registered here. Task 6
// owns selecting and registering imageagent.publish_approved.v3.
func (a *Activities) PublishApprovedV3(ctx context.Context, input PublishApprovedV3ActivityInput) error {
	if a.publisherV3 == nil {
		return fmt.Errorf("image agent v3 approved asset publisher is required")
	}
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return err
	}
	_, err = a.publisherV3.PublishApprovedV3(ctx, imageagent.PublishApprovedV3Input{
		RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID,
		PlanRevision: input.PlanRevision, CandidateAssetIDs: append([]string(nil), input.CandidateAssetIDs...),
		IdempotencyKey: input.IdempotencyKey,
	})
	return err
}

func legacyMigrationRequiredError() error {
	return sdktemporal.NewNonRetryableApplicationError(
		"legacy image agent workflow requires an explicit v2 migration or a new run",
		updateErrorLegacyMigrationRequired,
		nil,
	)
}

func (a *Activities) LegacyExecuteSlot(context.Context, LegacyExecuteSlotActivityInput) (imageagent.SlotExecutionResult, error) {
	return imageagent.SlotExecutionResult{}, legacyMigrationRequiredError()
}

func (a *Activities) LegacyPersistSlotResult(context.Context, LegacyPersistSlotResultActivityInput) error {
	return legacyMigrationRequiredError()
}

func (a *Activities) LegacyPersistRunState(context.Context, LegacyPersistRunStateActivityInput) error {
	return legacyMigrationRequiredError()
}

func (a *Activities) LegacyPersistPlanRevision(context.Context, LegacyPersistPlanRevisionActivityInput) error {
	return legacyMigrationRequiredError()
}

func (a *Activities) LegacyPersistPendingCommand(context.Context, LegacyPersistPendingCommandActivityInput) error {
	return legacyMigrationRequiredError()
}

func (a *Activities) LegacyPublishApproved(context.Context, LegacyPublishApprovedActivityInput) error {
	return legacyMigrationRequiredError()
}

type activityRegistrar interface {
	RegisterActivityWithOptions(interface{}, sdkactivity.RegisterOptions)
}

func RegisterActivities(registrar activityRegistrar, activities *Activities) error {
	return RegisterActivitiesForMode(registrar, activities, WorkerWireModeV2)
}

func RegisterActivitiesForMode(registrar activityRegistrar, activities *Activities, mode WorkerWireMode) error {
	if registrar == nil {
		return fmt.Errorf("temporal activity registrar is required")
	}
	if activities == nil {
		return fmt.Errorf("image agent activities are required")
	}
	if err := validateWorkerWireMode(mode); err != nil {
		return err
	}
	if mode == WorkerWireModeV2 {
		registrar.RegisterActivityWithOptions(activities.LegacyExecuteSlot, sdkactivity.RegisterOptions{Name: activityExecuteSlotLegacy})
		registrar.RegisterActivityWithOptions(activities.LegacyPersistSlotResult, sdkactivity.RegisterOptions{Name: activityPersistSlotResultLegacy})
		registrar.RegisterActivityWithOptions(activities.LegacyPersistRunState, sdkactivity.RegisterOptions{Name: activityPersistRunStateLegacy})
		registrar.RegisterActivityWithOptions(activities.LegacyPersistPlanRevision, sdkactivity.RegisterOptions{Name: activityPersistPlanRevisionLegacy})
		registrar.RegisterActivityWithOptions(activities.LegacyPersistPendingCommand, sdkactivity.RegisterOptions{Name: activityPersistPendingCommandLegacy})
		registrar.RegisterActivityWithOptions(activities.LegacyPublishApproved, sdkactivity.RegisterOptions{Name: activityPublishApprovedLegacy})
		registrar.RegisterActivityWithOptions(activities.ExecuteSlot, sdkactivity.RegisterOptions{Name: activityExecuteSlot})
		registrar.RegisterActivityWithOptions(activities.PersistSlotResult, sdkactivity.RegisterOptions{Name: activityPersistSlotResult})
	}
	registrar.RegisterActivityWithOptions(activities.PersistRunState, sdkactivity.RegisterOptions{Name: activityPersistRunState})
	registrar.RegisterActivityWithOptions(activities.PersistWorkflowFailure, sdkactivity.RegisterOptions{Name: activityPersistWorkflowFailure})
	registrar.RegisterActivityWithOptions(activities.PersistWorkflowFailureV2, sdkactivity.RegisterOptions{Name: activityPersistWorkflowFailureV2})
	registrar.RegisterActivityWithOptions(activities.PersistPlanRevision, sdkactivity.RegisterOptions{Name: activityPersistPlanRevision})
	registrar.RegisterActivityWithOptions(activities.PersistPendingCommand, sdkactivity.RegisterOptions{Name: activityPersistPendingCommand})
	if mode == WorkerWireModeV2 {
		registrar.RegisterActivityWithOptions(activities.PublishApproved, sdkactivity.RegisterOptions{Name: activityPublishApproved})
	} else {
		registrar.RegisterActivityWithOptions(activities.ExecuteSlotV3, sdkactivity.RegisterOptions{Name: activityExecuteSlotV3})
		registrar.RegisterActivityWithOptions(activities.StartEffectRecoveryV3, sdkactivity.RegisterOptions{Name: activityStartEffectRecoveryV3})
		registrar.RegisterActivityWithOptions(activities.RecoverEffectV3, sdkactivity.RegisterOptions{Name: activityRecoverEffectV3})
		registrar.RegisterActivityWithOptions(activities.PersistRecoveryBlockedEffectV3, sdkactivity.RegisterOptions{Name: activityPersistRecoveryBlockedV3})
		registrar.RegisterActivityWithOptions(activities.ReconcileEffectRecoveryV3, sdkactivity.RegisterOptions{Name: activityReconcileEffectRecoveryV3})
		registrar.RegisterActivityWithOptions(activities.PersistSlotResultV3, sdkactivity.RegisterOptions{Name: activityPersistSlotResultV3})
		registrar.RegisterActivityWithOptions(activities.PublishApprovedV3, sdkactivity.RegisterOptions{Name: activityPublishApprovedV3})
	}
	return nil
}

func restoreActivityIdentity(ctx context.Context, identity imageagent.ExecutionIdentity) (context.Context, error) {
	identity.TenantID = strings.TrimSpace(identity.TenantID)
	identity.UserID = strings.TrimSpace(identity.UserID)
	identity.BusinessTaskID = strings.TrimSpace(identity.BusinessTaskID)
	identity.TraceID = strings.TrimSpace(identity.TraceID)
	if identity.TenantID == "" || identity.UserID == "" {
		return nil, fmt.Errorf("captured image agent tenant and user identity are required")
	}
	ctx = authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{TenantID: identity.TenantID, UserID: identity.UserID})
	return aiidentity.WithIdentity(ctx, aiidentity.Identity{
		TenantID: identity.TenantID, UserID: identity.UserID, BusinessTaskID: identity.BusinessTaskID, TraceID: identity.TraceID,
	}), nil
}
