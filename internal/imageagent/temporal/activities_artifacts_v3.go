package temporal

import (
	"context"
	"errors"
	"fmt"
	sdkactivity "go.temporal.io/sdk/activity"
	sdktemporal "go.temporal.io/sdk/temporal"
	"os"
	"path/filepath"
	"strings"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/objectstore"
	"task-processor/internal/pkg/imagex"
	"task-processor/internal/shared/resilience"
)

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
			imageagent.SlotEffectV3ProviderUnknown, imageagent.SlotEffectV3StagingUnknown, imageagent.SlotEffectV3PublicationUnknown, imageagent.SlotEffectV3ReviewRequired, imageagent.SlotEffectV3ReviewTransportRequired,
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
		TargetPlatform: input.TargetPlatform, ImagePolicyContext: clonePolicyContext(input.ImagePolicyContext),
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
		data := append([]byte(nil), generatedAsset.Bytes...)
		if data == nil {
			localPath := strings.TrimSpace(generatedAsset.Metadata["local_path"])
			if localPath == "" && !strings.Contains(generatedAsset.URL, "://") {
				localPath = strings.TrimSpace(generatedAsset.URL)
			}
			if localPath == "" {
				return objectstore.PreparedSlotArtifacts{}, imageagent.ErrValidation
			}
			var err error
			data, err = os.ReadFile(localPath)
			if err != nil {
				return objectstore.PreparedSlotArtifacts{}, fmt.Errorf("read generated artifact %d: %w", index, err)
			}
		}
		info, err := imagex.Inspect(data)
		if err != nil {
			return objectstore.PreparedSlotArtifacts{}, fmt.Errorf("inspect generated artifact %d: %w", index, err)
		}
		contentType := map[string]string{"jpeg": "image/jpeg", "png": "image/png", "webp": "image/webp"}[info.Format]
		if contentType == "" {
			return objectstore.PreparedSlotArtifacts{}, imageagent.ErrValidation
		}
		if generatedAsset.ContentType != "" && generatedAsset.ContentType != contentType ||
			generatedAsset.Width > 0 && generatedAsset.Width != info.Width || generatedAsset.Height > 0 && generatedAsset.Height != info.Height {
			return objectstore.PreparedSlotArtifacts{}, imageagent.ErrValidation
		}
		assets[index] = objectstore.ArtifactInput{
			Bytes: data, ContentType: contentType, Width: info.Width, Height: info.Height,
			SourceAssetID: generated.SourceAssetID, Operations: generatedAsset.Operations,
			ProviderReceiptID: generatedAsset.ProviderReceiptID,
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
		asset.Bytes = nil
		cleanupGeneratedSlotLocalAsset(asset)
	}
}

func cleanupGeneratedSlotLocalAsset(asset *imageagent.GeneratedAsset) {
	if asset == nil || asset.Metadata == nil {
		return
	}
	localPath := strings.TrimSpace(asset.Metadata["local_path"])
	if localPath == "" {
		return
	}
	publishedPath := strings.TrimSpace(asset.Metadata["published_path"])
	if publishedPath != "" && filepath.Clean(localPath) == filepath.Clean(publishedPath) {
		asset.Metadata["temp_file_cleaned"] = "skipped_same_as_published"
		return
	}
	if err := os.Remove(localPath); err != nil && !os.IsNotExist(err) {
		asset.Metadata["temp_file_cleaned"] = "false"
		asset.Metadata["temp_file_cleanup_error"] = err.Error()
		return
	}
	asset.Metadata["temp_file_cleaned"] = "true"
	asset.Metadata["temp_local_path"] = localPath
	delete(asset.Metadata, "local_path")
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
