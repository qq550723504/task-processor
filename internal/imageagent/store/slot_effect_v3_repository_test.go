package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"task-processor/internal/imageagent"
)

func TestSlotEffectV3RepositoryContract(t *testing.T) {
	tests := []struct {
		name string
		new  func(*testing.T) imageagent.SlotExternalEffectV3Repository
	}{
		{name: "memory", new: func(t *testing.T) imageagent.SlotExternalEffectV3Repository {
			repository := NewMemoryRepository()
			initializeSlotEffectRun(t, repository, "run-slot-effect-v3-memory")
			return repository.(imageagent.SlotExternalEffectV3Repository)
		}},
		{name: "gorm", new: func(t *testing.T) imageagent.SlotExternalEffectV3Repository {
			repository := NewGormRepository(newConcurrentSQLite(t))
			initializeSlotEffectRun(t, repository, "run-slot-effect-v3-gorm")
			return repository.(imageagent.SlotExternalEffectV3Repository)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effects := tt.new(t)
			reservation := v3Reservation(tt.name)

			attempt, claimed, err := effects.ReserveSlotProviderV3(context.Background(), reservation)
			require.NoError(t, err)
			require.True(t, claimed)
			require.Equal(t, imageagent.SlotEffectV3ProviderClaimed, attempt.Phase)

			manifest := v3StagingManifest()
			attempt, err = effects.PrepareSlotStagingV3(context.Background(), reservation, manifest)
			require.NoError(t, err)
			require.Equal(t, imageagent.SlotEffectV3StagingPrepared, attempt.Phase)
			require.NotEmpty(t, attempt.StagingManifestFingerprint)

			attempt, err = effects.CommitSlotStagedV3(context.Background(), reservation, attempt.StagingManifestFingerprint)
			require.NoError(t, err)
			require.Equal(t, imageagent.SlotEffectV3ArtifactStaged, attempt.Phase)

			claimRequest := imageagent.PublicationClaimRequest{
				Reservation:            reservation,
				Owner:                  "worker-a",
				LeaseDuration:          time.Minute,
				PublicationFingerprint: "publication-fingerprint-1",
				FinalManifest:          imageagent.FinalManifest{Assets: manifest.Assets},
			}
			attempt, claim, claimed, err := effects.ClaimSlotPublicationV3(context.Background(), claimRequest)
			require.NoError(t, err)
			require.True(t, claimed)
			require.Equal(t, imageagent.SlotEffectV3PublicationClaimed, attempt.Phase)
			require.EqualValues(t, 1, claim.Fence)

			completion := imageagent.PublicationCompletion{
				Reservation:            reservation,
				Owner:                  claimRequest.Owner,
				Fence:                  claim.Fence,
				PublicationFingerprint: claimRequest.PublicationFingerprint,
				ResultFingerprint:      "result-fingerprint-1",
				Published: imageagent.SlotExecutionResult{SlotID: reservation.Identity.SlotID, Attempt: reservation.Identity.Attempt,
					Candidates: []imageagent.AssetCandidate{{AssetID: "candidate-1", SourceAssetID: "source-1", DurableAsset: imageagent.DurableAssetIdentity{ObjectKey: "image-agent/final/tenant-a/run/asset.png", SHA256: v3SHA256}}},
				},
			}
			attempt, err = effects.CompleteSlotPublicationV3(context.Background(), completion)
			require.NoError(t, err)
			require.Equal(t, imageagent.SlotEffectV3PublicationComplete, attempt.Phase)
			require.Equal(t, completion.Published, attempt.Published)

			replayed, err := effects.CompleteSlotPublicationV3(context.Background(), completion)
			require.NoError(t, err)
			require.Equal(t, attempt, replayed)
		})
	}
}

func TestSlotEffectV3RejectsLocalPathsAndUnknownMetadata(t *testing.T) {
	repository := NewMemoryRepository()
	initializeSlotEffectRun(t, repository, "run-slot-effect-v3-invalid-artifact")
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	reservation := v3Reservation("invalid-artifact")
	_, _, err := effects.ReserveSlotProviderV3(context.Background(), reservation)
	require.NoError(t, err)

	local := v3StagingManifest()
	local.Assets[0].ObjectKey = `C:\\worker\\generated.png`
	_, err = effects.PrepareSlotStagingV3(context.Background(), reservation, local)
	require.ErrorIs(t, err, imageagent.ErrValidation)

	relativeWindowsPath := v3StagingManifest()
	relativeWindowsPath.Assets[0].ObjectKey = `worker\generated.png`
	_, err = effects.PrepareSlotStagingV3(context.Background(), reservation, relativeWindowsPath)
	require.ErrorIs(t, err, imageagent.ErrValidation)

	metadata := v3StagingManifest()
	metadata.ProviderMetadata = map[string]string{"authorization": "secret"}
	_, err = effects.PrepareSlotStagingV3(context.Background(), reservation, metadata)
	require.ErrorIs(t, err, imageagent.ErrValidation)
}

func TestSlotEffectV3RejectsManifestReplacement(t *testing.T) {
	repository := NewMemoryRepository()
	initializeSlotEffectRun(t, repository, "run-slot-effect-v3-manifest")
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	reservation := v3Reservation("manifest")
	_, _, err := effects.ReserveSlotProviderV3(context.Background(), reservation)
	require.NoError(t, err)
	_, err = effects.PrepareSlotStagingV3(context.Background(), reservation, v3StagingManifest())
	require.NoError(t, err)

	replacement := v3StagingManifest()
	replacement.Assets[0].ObjectKey = "image-agent/staging/tenant-a/run/replaced.png"
	_, err = effects.PrepareSlotStagingV3(context.Background(), reservation, replacement)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
}

func TestSlotEffectV3RejectsIllegalPhaseTransitions(t *testing.T) {
	repository := NewMemoryRepository()
	initializeSlotEffectRun(t, repository, "run-slot-effect-v3-phases")
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	reservation := v3Reservation("phases")
	_, _, err := effects.ReserveSlotProviderV3(context.Background(), reservation)
	require.NoError(t, err)

	_, err = effects.CommitSlotStagedV3(context.Background(), reservation, "not-prepared")
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
	_, _, _, err = effects.ClaimSlotPublicationV3(context.Background(), imageagent.PublicationClaimRequest{Reservation: reservation, Owner: "worker-a", LeaseDuration: time.Minute, PublicationFingerprint: "fp", FinalManifest: imageagent.FinalManifest{Assets: v3StagingManifest().Assets}})
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
	_, err = effects.BlockSlotEffectV3(context.Background(), imageagent.SlotEffectV3BlockTransition{Reservation: reservation, Phase: imageagent.SlotEffectV3StagingUnknown, Code: "slot_staging_outcome_unknown"})
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
}

func TestSlotEffectV3StalePublicationFenceCannotCommit(t *testing.T) {
	clock := time.Date(2026, time.August, 27, 12, 0, 0, 0, time.UTC)
	repository := newMemoryRepositoryWithClock(func() time.Time { return clock })
	initializeSlotEffectRun(t, repository, "run-slot-effect-v3-stale-fence")
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	reservation := v3Reservation("stale-fence")
	stageV3Attempt(t, effects, reservation)

	request := imageagent.PublicationClaimRequest{Reservation: reservation, Owner: "worker-a", LeaseDuration: time.Minute, PublicationFingerprint: "publication-fingerprint-1", FinalManifest: imageagent.FinalManifest{Assets: v3StagingManifest().Assets}}
	_, first, claimed, err := effects.ClaimSlotPublicationV3(context.Background(), request)
	require.NoError(t, err)
	require.True(t, claimed)
	clock = clock.Add(2 * time.Minute)
	request.Owner = "worker-b"
	_, second, claimed, err := effects.ClaimSlotPublicationV3(context.Background(), request)
	require.NoError(t, err)
	require.True(t, claimed)
	require.Greater(t, second.Fence, first.Fence)

	_, err = effects.CompleteSlotPublicationV3(context.Background(), imageagent.PublicationCompletion{Reservation: reservation, Owner: "worker-a", Fence: first.Fence, PublicationFingerprint: request.PublicationFingerprint, ResultFingerprint: "result-fingerprint-1", Published: v3PublishedResult(reservation)})
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
}

func TestSlotEffectV3IsolatesTenantAndOwner(t *testing.T) {
	repository := NewMemoryRepository()
	initializeSlotEffectRun(t, repository, "run-slot-effect-v3-isolation")
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	reservation := v3Reservation("isolation")
	_, _, err := effects.ReserveSlotProviderV3(context.Background(), reservation)
	require.NoError(t, err)

	otherOwner := reservation.Identity
	otherOwner.OwnerUserID = "user-b"
	_, err = effects.GetSlotExternalEffectV3(context.Background(), otherOwner)
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
	otherTenant := reservation.Identity
	otherTenant.TenantID = "tenant-b"
	_, err = effects.GetSlotExternalEffectV3(context.Background(), otherTenant)
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
}

func TestGormSlotEffectV3ConcurrentClaimsAndFencing(t *testing.T) {
	db := newConcurrentSQLite(t)
	repository := NewGormRepository(db)
	scope := initializeSlotEffectRun(t, repository, "run-slot-effect-v3-concurrent")
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	reservation := imageagent.SlotEffectV3Reservation{Identity: imageagent.SlotExternalEffectIdentity{RunScope: scope, PlanRevision: 1, SlotID: "slot-1", Attempt: 1}, IdempotencyKey: "slot-key-v3-concurrent", InputFingerprint: "input-fingerprint-v3-concurrent"}

	const callers = 8
	start := make(chan struct{})
	type providerOutcome struct {
		claimed bool
		err     error
	}
	providerClaims := make(chan providerOutcome, callers)
	var workers sync.WaitGroup
	for range callers {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			_, claimed, err := effects.ReserveSlotProviderV3(context.Background(), reservation)
			providerClaims <- providerOutcome{claimed: claimed, err: err}
		}()
	}
	close(start)
	workers.Wait()
	close(providerClaims)
	owners := 0
	for outcome := range providerClaims {
		require.NoError(t, outcome.err)
		if outcome.claimed {
			owners++
		}
	}
	require.Equal(t, 1, owners)

	stageV3Attempt(t, effects, reservation)
	type publicationOutcome struct {
		claim imageagent.PublicationClaim
		err   error
	}
	publicationClaims := make(chan publicationOutcome, callers)
	start = make(chan struct{})
	for worker := range callers {
		workers.Add(1)
		go func(owner string) {
			defer workers.Done()
			<-start
			_, claim, _, err := effects.ClaimSlotPublicationV3(context.Background(), imageagent.PublicationClaimRequest{Reservation: reservation, Owner: owner, LeaseDuration: time.Minute, PublicationFingerprint: "publication-fingerprint-concurrent", FinalManifest: imageagent.FinalManifest{Assets: v3StagingManifest().Assets}})
			publicationClaims <- publicationOutcome{claim: claim, err: err}
		}(string(rune('a' + worker)))
	}
	close(start)
	workers.Wait()
	close(publicationClaims)
	var winner imageagent.PublicationClaim
	for outcome := range publicationClaims {
		require.NoError(t, outcome.err)
		claim := outcome.claim
		if winner.Owner == "" {
			winner = claim
		}
		require.Equal(t, winner.Owner, claim.Owner)
		require.Equal(t, winner.Fence, claim.Fence)
	}
	require.EqualValues(t, 1, winner.Fence)

	require.NoError(t, slotEffectV3IdentityWhere(db.Model(&slotExternalEffectV3Record{}), reservation.Identity).Update("publication_lease_expires_at", time.Now().UTC().Add(-time.Hour)).Error)
	successorRequest := imageagent.PublicationClaimRequest{Reservation: reservation, Owner: "worker-successor", LeaseDuration: time.Minute, PublicationFingerprint: "publication-fingerprint-concurrent", FinalManifest: imageagent.FinalManifest{Assets: v3StagingManifest().Assets}}
	_, successor, claimed, err := effects.ClaimSlotPublicationV3(context.Background(), successorRequest)
	require.NoError(t, err)
	require.True(t, claimed)
	require.Greater(t, successor.Fence, winner.Fence)

	_, err = effects.CompleteSlotPublicationV3(context.Background(), imageagent.PublicationCompletion{Reservation: reservation, Owner: winner.Owner, Fence: winner.Fence, PublicationFingerprint: successorRequest.PublicationFingerprint, ResultFingerprint: "result-fingerprint-concurrent", Published: v3PublishedResult(reservation)})
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
	completion := imageagent.PublicationCompletion{Reservation: reservation, Owner: successor.Owner, Fence: successor.Fence, PublicationFingerprint: successorRequest.PublicationFingerprint, ResultFingerprint: "result-fingerprint-concurrent", Published: v3PublishedResult(reservation)}
	completed, err := effects.CompleteSlotPublicationV3(context.Background(), completion)
	require.NoError(t, err)
	replayed, err := effects.CompleteSlotPublicationV3(context.Background(), completion)
	require.NoError(t, err)
	require.Equal(t, completed, replayed)
}

func TestDatabaseNowReadsSQLiteCurrentTimestamp(t *testing.T) {
	db := newConcurrentSQLite(t)
	var now time.Time
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		var err error
		now, err = databaseNow(context.Background(), tx)
		return err
	}))
	require.False(t, now.IsZero())
}

const v3SHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func v3Reservation(suffix string) imageagent.SlotEffectV3Reservation {
	return imageagent.SlotEffectV3Reservation{Identity: imageagent.SlotExternalEffectIdentity{RunScope: imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-1", RunID: "run-slot-effect-v3-" + suffix}, PlanRevision: 1, SlotID: "slot-1", Attempt: 1}, IdempotencyKey: "slot-key-v3-" + suffix, InputFingerprint: "input-fingerprint-v3-" + suffix}
}

func v3StagingManifest() imageagent.StagingManifest {
	return imageagent.StagingManifest{Assets: []imageagent.StagedAssetRef{{ObjectKey: "image-agent/staging/tenant-a/run/asset.png", SHA256: v3SHA256, SizeBytes: 42, ContentType: "image/png", Width: 1200, Height: 1200, SourceAssetID: "source-1", Operations: []string{"resize"}, ProviderReceiptID: "receipt-1"}}}
}

func v3PublishedResult(reservation imageagent.SlotEffectV3Reservation) imageagent.SlotExecutionResult {
	return imageagent.SlotExecutionResult{SlotID: reservation.Identity.SlotID, Attempt: reservation.Identity.Attempt, Candidates: []imageagent.AssetCandidate{{AssetID: "candidate-1", SourceAssetID: "source-1", DurableAsset: imageagent.DurableAssetIdentity{ObjectKey: "image-agent/final/tenant-a/run/asset.png", SHA256: v3SHA256}}}}
}

func stageV3Attempt(t *testing.T, effects imageagent.SlotExternalEffectV3Repository, reservation imageagent.SlotEffectV3Reservation) imageagent.SlotEffectV3Attempt {
	t.Helper()
	_, _, err := effects.ReserveSlotProviderV3(context.Background(), reservation)
	require.NoError(t, err)
	attempt, err := effects.PrepareSlotStagingV3(context.Background(), reservation, v3StagingManifest())
	require.NoError(t, err)
	attempt, err = effects.CommitSlotStagedV3(context.Background(), reservation, attempt.StagingManifestFingerprint)
	require.NoError(t, err)
	return attempt
}
