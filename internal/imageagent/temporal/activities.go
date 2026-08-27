package temporal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"task-processor/internal/pkg/imagex"
)

const slotResultPersistedEventType = "slot.result.persisted"

type ActivityDependencies struct {
	Repository               imageagent.Repository
	SlotEffects              imageagent.SlotExternalEffectRepository
	SlotExecutor             imageagent.RecoverableSlotExecutor
	Publisher                imageagent.ApprovedAssetPublisher
	SlotEffectsV3            imageagent.SlotExternalEffectV3Repository
	StagedSlotExecutor       imageagent.StagedSlotExecutor
	ArtifactStore            DurableArtifactStore
	PublicationOwner         func(context.Context) (string, error)
	PublicationLeaseDuration time.Duration
}

type Activities struct {
	repository               imageagent.Repository
	slotEffects              imageagent.SlotExternalEffectRepository
	slotExecutor             imageagent.RecoverableSlotExecutor
	publisher                imageagent.ApprovedAssetPublisher
	slotEffectsV3            imageagent.SlotExternalEffectV3Repository
	stagedSlotExecutor       imageagent.StagedSlotExecutor
	artifactStore            DurableArtifactStore
	publicationOwner         func(context.Context) (string, error)
	publicationLeaseDuration time.Duration
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
		if dependencies.PublicationOwner == nil {
			dependencies.PublicationOwner = temporalPublicationOwner
		}
		if dependencies.PublicationLeaseDuration <= 0 {
			dependencies.PublicationLeaseDuration = 2 * time.Minute
		}
	}
	return &Activities{
		repository: dependencies.Repository, slotEffects: dependencies.SlotEffects, slotExecutor: dependencies.SlotExecutor, publisher: dependencies.Publisher,
		slotEffectsV3: dependencies.SlotEffectsV3, stagedSlotExecutor: dependencies.StagedSlotExecutor, artifactStore: dependencies.ArtifactStore,
		publicationOwner: dependencies.PublicationOwner, publicationLeaseDuration: dependencies.PublicationLeaseDuration,
	}, nil
}

const slotProviderOutcomeUnknownErrorType = "imageagent_slot_provider_outcome_unknown"

const (
	slotProviderOutcomeUnknownCode    = "slot_provider_outcome_unknown"
	slotStagingOutcomeUnknownCode     = "slot_staging_outcome_unknown"
	slotPublicationOutcomeUnknownCode = "slot_publication_outcome_unknown"
	slotEffectPhaseInvalidCode        = "imageagent_slot_effect_phase_invalid"
	invalidMainCandidateCountCode     = "invalid_main_candidate_count"
)

func (a *Activities) ExecuteSlot(ctx context.Context, input ExecuteSlotActivityInput) (imageagent.SlotExecutionResult, error) {
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return imageagent.SlotExecutionResult{}, err
	}
	executionInput := imageagent.SlotExecutionInput{
		RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID,
		PlanRevision: input.PlanRevision, Slot: input.Slot, Attempt: input.Attempt,
		IdempotencyKey: input.IdempotencyKey,
		AssetCatalog:   input.AssetCatalog,
	}
	reservation := imageagent.SlotExternalEffectReservation{
		Identity:       imageagent.SlotExternalEffectIdentity{RunScope: imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID}, PlanRevision: input.PlanRevision, SlotID: input.Slot.ID, Attempt: input.Attempt},
		IdempotencyKey: input.IdempotencyKey, InputFingerprint: imageagent.SlotExecutionFingerprint(executionInput),
	}
	effect, claimed, err := a.slotEffects.ReserveSlotExternalEffect(ctx, reservation)
	if err != nil {
		return imageagent.SlotExecutionResult{}, err
	}
	if effect.Phase == imageagent.SlotExternalEffectProviderStarted {
		if !claimed {
			return imageagent.SlotExecutionResult{}, sdktemporal.NewNonRetryableApplicationError("slot provider outcome is unknown; start a new manual attempt", slotProviderOutcomeUnknownErrorType, nil)
		}
		generated, generateErr := a.slotExecutor.GenerateSlot(ctx, executionInput)
		if generateErr != nil {
			return imageagent.SlotExecutionResult{}, sdktemporal.NewNonRetryableApplicationError("slot provider generation failed", "imageagent_slot_provider_failed", generateErr)
		}
		effect, err = a.slotEffects.StoreSlotGeneratedOutput(ctx, reservation, generated)
		if err != nil {
			return imageagent.SlotExecutionResult{}, fmt.Errorf("persist generated slot output: %w", err)
		}
	}
	if effect.Phase == imageagent.SlotExternalEffectGeneratedComplete {
		published, publishErr := a.slotExecutor.PublishSlot(ctx, executionInput, effect.Generated)
		if publishErr != nil {
			return imageagent.SlotExecutionResult{}, fmt.Errorf("publish generated slot output: %w", publishErr)
		}
		effect, err = a.slotEffects.CompleteSlotPublication(ctx, reservation, published)
		if err != nil {
			return imageagent.SlotExecutionResult{}, fmt.Errorf("persist slot publication completion: %w", err)
		}
	}
	if effect.Phase != imageagent.SlotExternalEffectPublicationComplete {
		return imageagent.SlotExecutionResult{}, imageagent.ErrRevisionConflict
	}
	return effect.Published, nil
}

func (a *Activities) ExecuteSlotV3(ctx context.Context, input ExecuteSlotV3ActivityInput) (v3Result imageagent.SlotEffectV3PublishedResult, err error) {
	if a.slotEffectsV3 == nil || a.stagedSlotExecutor == nil || a.artifactStore == nil || a.publicationOwner == nil {
		return v3Result, fmt.Errorf("image agent v3 activity dependencies are incomplete")
	}
	ctx, err = restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return v3Result, err
	}
	executionInput := slotExecutionInputV3(input)
	reservation := slotEffectReservationV3(executionInput)
	effect, claimed, err := a.slotEffectsV3.ReserveSlotProviderV3(ctx, reservation)
	if err != nil {
		return v3Result, err
	}

	var prepared objectstore.PreparedSlotArtifacts
	if effect.Phase == imageagent.SlotEffectV3ProviderClaimed {
		if !claimed {
			return v3Result, a.blockSlotEffectV3(ctx, reservation, imageagent.SlotEffectV3ProviderUnknown, slotProviderOutcomeUnknownCode, imageagent.PublicationClaim{})
		}
		generated, generateErr := a.stagedSlotExecutor.GenerateSlot(ctx, executionInput)
		if generateErr != nil {
			return v3Result, a.blockSlotEffectV3(ctx, reservation, imageagent.SlotEffectV3ProviderUnknown, slotProviderOutcomeUnknownCode, imageagent.PublicationClaim{})
		}
		prepared, err = prepareGeneratedSlotArtifacts(executionInput, generated, a.artifactStore)
		if err != nil {
			return v3Result, a.blockSlotEffectV3(ctx, reservation, imageagent.SlotEffectV3ProviderUnknown, slotProviderOutcomeUnknownCode, imageagent.PublicationClaim{})
		}
		effect, err = a.slotEffectsV3.PrepareSlotStagingV3(ctx, reservation, prepared.Manifest)
		if err != nil {
			effect, err = a.slotEffectsV3.GetSlotExternalEffectV3(ctx, reservation.Identity)
			if err != nil {
				return v3Result, fmt.Errorf("reconcile persisted staging manifest: %w", err)
			}
			if effect.Phase == imageagent.SlotEffectV3ProviderClaimed {
				return v3Result, a.blockSlotEffectV3(ctx, reservation, imageagent.SlotEffectV3ProviderUnknown, slotProviderOutcomeUnknownCode, imageagent.PublicationClaim{})
			}
		}
	}

	if effect.Phase == imageagent.SlotEffectV3StagingPrepared {
		if len(prepared.Manifest.Assets) == 0 {
			prepared = objectstore.PreparedSlotArtifacts{Manifest: effect.StagingManifest}
		}
		if err := a.artifactStore.EnsureStaged(ctx, prepared); err != nil {
			if errors.Is(err, objectstore.ErrArtifactUnavailable) || errors.Is(err, objectstore.ErrObjectConflict) {
				return v3Result, a.blockSlotEffectV3(ctx, reservation, imageagent.SlotEffectV3StagingUnknown, slotStagingOutcomeUnknownCode, imageagent.PublicationClaim{})
			}
			return v3Result, fmt.Errorf("ensure staged artifacts: %w", err)
		}
		effect, err = a.slotEffectsV3.CommitSlotStagedV3(ctx, reservation, effect.StagingManifestFingerprint)
		if err != nil {
			effect, err = a.slotEffectsV3.GetSlotExternalEffectV3(ctx, reservation.Identity)
			if err != nil {
				return v3Result, fmt.Errorf("reconcile staged commit: %w", err)
			}
			if effect.Phase == imageagent.SlotEffectV3StagingPrepared {
				return v3Result, fmt.Errorf("commit staged artifacts: %w", imageagent.ErrRevisionConflict)
			}
		}
	}

	if effect.Phase == imageagent.SlotEffectV3ArtifactStaged || effect.Phase == imageagent.SlotEffectV3PublicationClaimed {
		owner, ownerErr := a.publicationOwner(ctx)
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
		effect, publication, acquired, claimErr = a.slotEffectsV3.ClaimSlotPublicationV3(ctx, imageagent.PublicationClaimRequest{
			Reservation: reservation, Owner: owner, LeaseDuration: a.publicationLeaseDuration,
			PublicationFingerprint: publicationFingerprint, FinalManifest: finalManifest,
		})
		if claimErr != nil {
			reconciled, getErr := a.slotEffectsV3.GetSlotExternalEffectV3(ctx, reservation.Identity)
			if getErr != nil {
				return v3Result, fmt.Errorf("claim slot publication: %w", claimErr)
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
			return v3Result, fmt.Errorf("slot publication is owned by another activity attempt: %w", imageagent.ErrRevisionConflict)
		}
		if _, err := a.renewPublicationV3(ctx, reservation.Identity, publication); err != nil {
			return v3Result, err
		}
		actualFinal, finalizeErr := a.artifactStore.Finalize(ctx, effect.StagingManifest)
		if finalizeErr != nil {
			if errors.Is(finalizeErr, objectstore.ErrArtifactUnavailable) || errors.Is(finalizeErr, objectstore.ErrObjectConflict) {
				return v3Result, a.blockSlotEffectV3(ctx, reservation, imageagent.SlotEffectV3PublicationUnknown, slotPublicationOutcomeUnknownCode, publication)
			}
			return v3Result, fmt.Errorf("finalize slot artifacts: %w", finalizeErr)
		}
		if !reflect.DeepEqual(actualFinal, effect.FinalManifest) {
			return v3Result, a.blockSlotEffectV3(ctx, reservation, imageagent.SlotEffectV3PublicationUnknown, slotPublicationOutcomeUnknownCode, publication)
		}
		publication, err = a.renewPublicationV3(ctx, reservation.Identity, publication)
		if err != nil {
			return v3Result, err
		}
		result, buildErr := a.stagedSlotExecutor.BuildSlotResult(ctx, executionInput, imageagent.PublishedSlotOutput{SlotID: input.Slot.ID, Attempt: input.Attempt, Assets: actualFinal.Assets})
		if buildErr != nil {
			return v3Result, fmt.Errorf("build durable slot result: %w", buildErr)
		}
		published, publishedErr := imageagent.NewSlotEffectV3PublishedResult(result)
		if publishedErr != nil {
			return v3Result, fmt.Errorf("normalize durable slot result: %w", publishedErr)
		}
		resultFingerprint, resultFingerprintErr := slotEffectV3ResultFingerprint(published)
		if resultFingerprintErr != nil {
			return v3Result, resultFingerprintErr
		}
		effect, err = a.slotEffectsV3.CompleteSlotPublicationV3(ctx, imageagent.PublicationCompletion{
			Reservation: reservation, Owner: publication.Owner, Fence: publication.Fence,
			PublicationFingerprint: publicationFingerprint, ResultFingerprint: resultFingerprint, Published: published,
		})
		if err != nil {
			reconciled, getErr := a.slotEffectsV3.GetSlotExternalEffectV3(ctx, reservation.Identity)
			if getErr != nil {
				return v3Result, fmt.Errorf("reconcile publication completion: %w", getErr)
			}
			if reconciled.Phase != imageagent.SlotEffectV3PublicationComplete || reconciled.ResultFingerprint != resultFingerprint || !reflect.DeepEqual(reconciled.Published, published) {
				return v3Result, imageagent.ErrRevisionConflict
			}
			effect = reconciled
		}
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

func slotExecutionInputV3(input ExecuteSlotV3ActivityInput) imageagent.SlotExecutionInput {
	return imageagent.SlotExecutionInput{
		RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID,
		PlanRevision: input.PlanRevision, Slot: input.Slot, Attempt: input.Attempt,
		IdempotencyKey: input.IdempotencyKey, AssetCatalog: input.AssetCatalog,
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
		return imageagent.PublicationClaim{}, fmt.Errorf("renew slot publication claim: %w", err)
	}
	return claim, nil
}

func (a *Activities) blockSlotEffectV3(ctx context.Context, reservation imageagent.SlotEffectV3Reservation, phase imageagent.SlotEffectV3Phase, code string, publication imageagent.PublicationClaim) error {
	_, err := a.slotEffectsV3.BlockSlotEffectV3(ctx, imageagent.SlotEffectV3BlockTransition{
		Reservation: reservation, Phase: phase, Code: code, Owner: publication.Owner, Fence: publication.Fence,
	})
	if err != nil {
		return fmt.Errorf("persist blocked slot effect: %w", err)
	}
	return blockedSlotEffectV3Error(code)
}

func blockedSlotEffectV3Error(code string) error {
	return sdktemporal.NewNonRetryableApplicationError("slot external effect outcome requires a new user attempt or operator reconciliation", code, nil)
}

func slotEffectV3ResultFingerprint(result imageagent.SlotEffectV3PublishedResult) (string, error) {
	normalized, err := imageagent.NormalizeSlotEffectV3PublishedResult(result)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func temporalPublicationOwner(ctx context.Context) (string, error) {
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
	scope := imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID}
	current, err := a.repository.GetProjection(ctx, scope)
	if err != nil {
		return fmt.Errorf("load image agent projection: %w", err)
	}
	slotIndex := slotProjectionIndex(current.Slots, result.Execution.SlotID)
	if slotIndex < 0 {
		return imageagent.ErrRevisionConflict
	}
	if result.Status == imageagent.SlotStatusAccepted && current.Slots[slotIndex].Slot.Role == imageagent.SlotRoleMain && len(result.Execution.Candidates) != 1 {
		result.Status = imageagent.SlotStatusBlocked
		result.ErrorCode = invalidMainCandidateCountCode
		result.Execution.Candidates = nil
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
		if strings.TrimSpace(candidate.URL) == "" {
			durable, validateErr := imageagent.NormalizeDurableAssetIdentity(candidate.DurableAsset)
			if validateErr != nil {
				return fmt.Errorf("candidate %q has invalid durable asset identity: %w", id, validateErr)
			}
			candidate.DurableAsset = durable
		} else {
			validatedURL, validateErr := imageagent.ValidateSafeImageURL(candidate.URL)
			if validateErr != nil {
				return fmt.Errorf("candidate %q has unsafe URL: %w", id, validateErr)
			}
			candidate.URL = validatedURL
		}
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
	if slotProjectionAlreadyPersisted(current.Slots, result.Execution.SlotID, result.Execution.Attempt, result.Status, candidates, result.ErrorCode) {
		return nil
	}
	storedResult := imageagent.SlotResult{
		SlotID: result.Execution.SlotID, Attempt: result.Execution.Attempt,
		Status: result.Status, CandidateAssetIDs: candidateIDs, ErrorCode: result.ErrorCode,
	}
	updated := current
	for index := range updated.Slots {
		if updated.Slots[index].Slot.ID != result.Execution.SlotID {
			continue
		}
		updated.Slots[index] = imageagent.SlotProjection{Slot: updated.Slots[index].Slot, Attempt: result.Execution.Attempt, Candidates: candidates, ErrorCode: result.ErrorCode}
		updated.Slots[index].Slot.Status = result.Status
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
		reflect.DeepEqual(current.PendingCommand, input.Projection.PendingCommand) {
		return nil
	}
	updated := current
	updated.Run.Status = input.Projection.Status
	updated.Run.CurrentNode = input.CurrentNode
	updated.Run.Block = cloneTemporalBlock(input.Projection.Block)
	updated.Run.Version++
	updated.Plan = input.Projection.Plan
	updated.Slots = append([]imageagent.SlotProjection(nil), input.Projection.Slots...)
	updated.ResultDigest = input.Projection.ResultDigest
	updated.PendingCommand = clonePendingReceipt(input.Projection.PendingCommand)
	updated.CommandIngress = input.Projection.CommandIngress
	_, err = a.repository.CommitProjection(ctx, imageagent.ProjectionCommit{Scope: scope, CommitID: input.CommitID, ExpectedProjectionVersion: current.ProjectionVersion, Snapshot: updated, EventType: "run.updated", EventPayload: json.RawMessage(`{}`), ExpectedRunVersion: current.Run.Version, RunMutation: &imageagent.RunMutation{Status: updated.Run.Status, CurrentNode: input.CurrentNode, ActivePlanRevision: input.PlanRevision, Block: updated.Run.Block}})
	if err != nil {
		return fmt.Errorf("commit image agent run projection: %w", err)
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
	if registrar == nil {
		return fmt.Errorf("temporal activity registrar is required")
	}
	if activities == nil {
		return fmt.Errorf("image agent activities are required")
	}
	registrar.RegisterActivityWithOptions(activities.LegacyExecuteSlot, sdkactivity.RegisterOptions{Name: activityExecuteSlotLegacy})
	registrar.RegisterActivityWithOptions(activities.LegacyPersistSlotResult, sdkactivity.RegisterOptions{Name: activityPersistSlotResultLegacy})
	registrar.RegisterActivityWithOptions(activities.LegacyPersistRunState, sdkactivity.RegisterOptions{Name: activityPersistRunStateLegacy})
	registrar.RegisterActivityWithOptions(activities.LegacyPersistPlanRevision, sdkactivity.RegisterOptions{Name: activityPersistPlanRevisionLegacy})
	registrar.RegisterActivityWithOptions(activities.LegacyPersistPendingCommand, sdkactivity.RegisterOptions{Name: activityPersistPendingCommandLegacy})
	registrar.RegisterActivityWithOptions(activities.LegacyPublishApproved, sdkactivity.RegisterOptions{Name: activityPublishApprovedLegacy})
	registrar.RegisterActivityWithOptions(activities.ExecuteSlot, sdkactivity.RegisterOptions{Name: activityExecuteSlot})
	registrar.RegisterActivityWithOptions(activities.PersistSlotResult, sdkactivity.RegisterOptions{Name: activityPersistSlotResult})
	registrar.RegisterActivityWithOptions(activities.PersistRunState, sdkactivity.RegisterOptions{Name: activityPersistRunState})
	registrar.RegisterActivityWithOptions(activities.PersistPlanRevision, sdkactivity.RegisterOptions{Name: activityPersistPlanRevision})
	registrar.RegisterActivityWithOptions(activities.PersistPendingCommand, sdkactivity.RegisterOptions{Name: activityPersistPendingCommand})
	registrar.RegisterActivityWithOptions(activities.PublishApproved, sdkactivity.RegisterOptions{Name: activityPublishApproved})
	return nil
}

func restoreActivityIdentity(ctx context.Context, identity imageagent.ExecutionIdentity) (context.Context, error) {
	identity.TenantID = strings.TrimSpace(identity.TenantID)
	identity.UserID = strings.TrimSpace(identity.UserID)
	if identity.TenantID == "" || identity.UserID == "" {
		return nil, fmt.Errorf("captured image agent tenant and user identity are required")
	}
	return authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{TenantID: identity.TenantID, UserID: identity.UserID}), nil
}
