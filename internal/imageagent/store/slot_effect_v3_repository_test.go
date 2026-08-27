package store

import (
	"context"
	"encoding/json"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

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
				FinalManifest:          v3FinalManifest(),
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
				Published:              v3PublishedResult(reservation),
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

	whitespaceKey := v3StagingManifest()
	whitespaceKey.Assets[0].ObjectKey = " image-agent/staging/tenant-a/run/asset.png"
	_, err = effects.PrepareSlotStagingV3(context.Background(), reservation, whitespaceKey)
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
	_, _, _, err = effects.ClaimSlotPublicationV3(context.Background(), imageagent.PublicationClaimRequest{Reservation: reservation, Owner: "worker-a", LeaseDuration: time.Minute, PublicationFingerprint: "fp", FinalManifest: v3FinalManifest()})
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

	request := v3PublicationRequest(reservation, "worker-a")
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
		claim   imageagent.PublicationClaim
		claimed bool
		err     error
	}
	publicationClaims := make(chan publicationOutcome, callers)
	start = make(chan struct{})
	for worker := range callers {
		workers.Add(1)
		go func(owner string) {
			defer workers.Done()
			<-start
			_, claim, claimed, err := effects.ClaimSlotPublicationV3(context.Background(), v3PublicationRequest(reservation, owner))
			publicationClaims <- publicationOutcome{claim: claim, claimed: claimed, err: err}
		}(string(rune('a' + worker)))
	}
	close(start)
	workers.Wait()
	close(publicationClaims)
	var winner imageagent.PublicationClaim
	publicationOwners := 0
	claimedOwner := ""
	for outcome := range publicationClaims {
		require.NoError(t, outcome.err)
		claim := outcome.claim
		if outcome.claimed {
			publicationOwners++
			claimedOwner = claim.Owner
		}
		if winner.Owner == "" {
			winner = claim
		}
		require.Equal(t, winner.Owner, claim.Owner)
		require.Equal(t, winner.Fence, claim.Fence)
	}
	require.Equal(t, 1, publicationOwners)
	require.Equal(t, winner.Owner, claimedOwner)
	require.EqualValues(t, 1, winner.Fence)

	require.NoError(t, slotEffectV3IdentityWhere(db.Model(&slotExternalEffectV3Record{}), reservation.Identity).Update("publication_lease_expires_at", time.Now().UTC().Add(-time.Hour)).Error)
	successorRequest := v3PublicationRequest(reservation, "worker-successor")
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

func TestDatabaseNowUsesPostgresWallClock(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, WithoutReturning: true}), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT clock_timestamp()")).WillReturnRows(sqlmock.NewRows([]string{"clock_timestamp"}).AddRow("2026-08-27 12:00:00"))
	now, err := databaseNow(context.Background(), db)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, time.August, 27, 12, 0, 0, 0, time.UTC), now)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGormPostgresPublicationClaimLocksBeforeReadingWallClock(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, WithoutReturning: true}), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	reservation := v3Reservation("postgres-lock-order")
	claimedAt := time.Date(2026, time.August, 27, 12, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "image_agent_v3_slot_external_effects".*FOR UPDATE`).
		WithArgs(reservation.Identity.TenantID, reservation.Identity.OwnerUserID, reservation.Identity.RunID, reservation.Identity.PlanRevision, reservation.Identity.SlotID, reservation.Identity.Attempt, 1).
		WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "owner_user_id", "run_id", "plan_revision", "slot_id", "attempt", "idempotency_key", "input_fingerprint", "phase", "staging_manifest_json", "staging_manifest_fingerprint", "publication_owner", "publication_lease_expires_at", "publication_fence", "publication_fingerprint", "result_fingerprint", "final_manifest_json", "published_json", "blocked_code", "provider_claimed_at", "staging_prepared_at", "staged_at", "published_at", "created_at", "updated_at"}).AddRow(reservation.Identity.TenantID, reservation.Identity.OwnerUserID, reservation.Identity.RunID, reservation.Identity.PlanRevision, reservation.Identity.SlotID, reservation.Identity.Attempt, reservation.IdempotencyKey, reservation.InputFingerprint, imageagent.SlotEffectV3ArtifactStaged, nil, nil, "", nil, 0, "", "", nil, nil, "", claimedAt, nil, claimedAt, nil, claimedAt, claimedAt))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT clock_timestamp()")).WillReturnRows(sqlmock.NewRows([]string{"clock_timestamp"}).AddRow("2026-08-27 12:00:05"))
	mock.ExpectExec(`UPDATE "image_agent_v3_slot_external_effects"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	effects := NewGormRepository(db).(imageagent.SlotExternalEffectV3Repository)
	_, claim, claimed, err := effects.ClaimSlotPublicationV3(context.Background(), v3PublicationRequest(reservation, "worker-a"))
	require.NoError(t, err)
	require.True(t, claimed)
	require.Equal(t, time.Date(2026, time.August, 27, 12, 1, 5, 0, time.UTC), claim.LeaseExpiresAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSlotExecutionResultV2JSONRemainsFrozen(t *testing.T) {
	result := imageagent.SlotExecutionResult{
		SlotID:  "slot-1",
		Attempt: 1,
		Candidates: []imageagent.AssetCandidate{{
			AssetID:       "candidate-1",
			URL:           "https://cdn.example.test/candidate.png",
			SourceAssetID: "source-1",
			Metadata:      map[string]string{"legacy": "kept"},
			DurableAsset:  imageagent.DurableAssetIdentity{ObjectKey: "image-agent/final/tenant-a/run/asset.png", SHA256: v3SHA256},
		}},
	}
	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	require.JSONEq(t, `{"SlotID":"slot-1","Attempt":1,"Candidates":[{"AssetID":"candidate-1","URL":"https://cdn.example.test/candidate.png","SourceAssetID":"source-1","Metadata":{"legacy":"kept"}}]}`, string(encoded))
}

func TestSlotEffectV3PublishedResultRejectsLegacyCandidateFields(t *testing.T) {
	for _, test := range []struct {
		name      string
		candidate imageagent.AssetCandidate
	}{
		{name: "URL", candidate: imageagent.AssetCandidate{AssetID: "candidate-1", URL: "file:///tmp/generated.png", SourceAssetID: "source-1", DurableAsset: imageagent.DurableAssetIdentity{ObjectKey: "image-agent/final/tenant-a/run/asset.png", SHA256: v3SHA256}}},
		{name: "metadata", candidate: imageagent.AssetCandidate{AssetID: "candidate-1", SourceAssetID: "source-1", Metadata: map[string]string{"authorization": "secret"}, DurableAsset: imageagent.DurableAssetIdentity{ObjectKey: "image-agent/final/tenant-a/run/asset.png", SHA256: v3SHA256}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := imageagent.NewSlotEffectV3PublishedResult(imageagent.SlotExecutionResult{SlotID: "slot-1", Attempt: 1, Candidates: []imageagent.AssetCandidate{test.candidate}})
			require.ErrorIs(t, err, imageagent.ErrValidation)
		})
	}
}

func TestSlotEffectV3ReviewContract(t *testing.T) {
	for _, factory := range newV3ReviewFixtures() {
		t.Run(factory.name, func(t *testing.T) {
			fixture := factory.new(t)
			ctx := context.Background()
			reservation := fixture.reservation
			_, _, err := fixture.effects.ReserveSlotProviderV3(ctx, reservation)
			require.NoError(t, err)

			changed := v3StagingManifest()
			changed.Assets[0].ObjectKey = "image-agent/staging/tenant-a/run/replacement.png"
			_, err = fixture.effects.PrepareSlotStagingV3(ctx, reservation, v3StagingManifest())
			require.NoError(t, err)
			_, err = fixture.effects.PrepareSlotStagingV3(ctx, reservation, changed)
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
			_, err = fixture.effects.CommitSlotStagedV3(ctx, reservation, "wrong-fingerprint")
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict)

			attempt, err := fixture.effects.GetSlotExternalEffectV3(ctx, reservation.Identity)
			require.NoError(t, err)
			_, err = fixture.effects.CommitSlotStagedV3(ctx, reservation, attempt.StagingManifestFingerprint)
			require.NoError(t, err)
			request := v3PublicationRequest(reservation, "worker-a")
			_, first, claimed, err := fixture.effects.ClaimSlotPublicationV3(ctx, request)
			require.NoError(t, err)
			require.True(t, claimed)
			outsideFinalManifest := v3PublishedResult(reservation)
			outsideFinalManifest.Candidates[0].DurableAsset.ObjectKey = "image-agent/final/tenant-a/run/unclaimed.png"
			_, err = fixture.effects.CompleteSlotPublicationV3(ctx, imageagent.PublicationCompletion{Reservation: reservation, Owner: first.Owner, Fence: first.Fence, PublicationFingerprint: request.PublicationFingerprint, ResultFingerprint: "result-fingerprint-unclaimed", Published: outsideFinalManifest})
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict)

			fixture.expireLease(t, reservation.Identity)
			request.Owner = "worker-b"
			_, second, claimed, err := fixture.effects.ClaimSlotPublicationV3(ctx, request)
			require.NoError(t, err)
			require.True(t, claimed)
			require.Greater(t, second.Fence, first.Fence)

			_, err = fixture.effects.BlockSlotEffectV3(ctx, imageagent.SlotEffectV3BlockTransition{Reservation: reservation, Phase: imageagent.SlotEffectV3PublicationUnknown, Code: "slot_publication_outcome_unknown", Owner: first.Owner, Fence: first.Fence})
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
			_, err = fixture.effects.CompleteSlotPublicationV3(ctx, imageagent.PublicationCompletion{Reservation: reservation, Owner: first.Owner, Fence: first.Fence, PublicationFingerprint: request.PublicationFingerprint, ResultFingerprint: "result-fingerprint-1", Published: v3PublishedResult(reservation)})
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict)

			otherOwner := reservation.Identity
			otherOwner.OwnerUserID = "user-b"
			_, err = fixture.effects.GetSlotExternalEffectV3(ctx, otherOwner)
			require.ErrorIs(t, err, imageagent.ErrRunNotFound)
			otherTenant := reservation.Identity
			otherTenant.TenantID = "tenant-b"
			_, err = fixture.effects.GetSlotExternalEffectV3(ctx, otherTenant)
			require.ErrorIs(t, err, imageagent.ErrRunNotFound)
		})
	}
}

func TestSlotEffectV3RejectsScopedIdempotencyCollisionAcrossAdapters(t *testing.T) {
	for _, factory := range newV3ReviewFixtures() {
		t.Run(factory.name, func(t *testing.T) {
			fixture := factory.new(t)
			ctx := context.Background()
			_, _, err := fixture.effects.ReserveSlotProviderV3(ctx, fixture.reservation)
			require.NoError(t, err)
			collision := fixture.reservation
			collision.Identity.Attempt++
			_, _, err = fixture.effects.ReserveSlotProviderV3(ctx, collision)
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
		})
	}
}

func TestSlotEffectV3PublishedJSONIsAllowlistedAcrossAdapters(t *testing.T) {
	for _, factory := range newV3ReviewFixtures() {
		t.Run(factory.name, func(t *testing.T) {
			fixture := factory.new(t)
			ctx := context.Background()
			stageV3Attempt(t, fixture.effects, fixture.reservation)
			request := v3PublicationRequest(fixture.reservation, "worker-a")
			_, claim, claimed, err := fixture.effects.ClaimSlotPublicationV3(ctx, request)
			require.NoError(t, err)
			require.True(t, claimed)
			published := v3PublishedResult(fixture.reservation)
			published.Candidates[0].DurableAsset.SHA256 = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
			attempt, err := fixture.effects.CompleteSlotPublicationV3(ctx, imageagent.PublicationCompletion{Reservation: fixture.reservation, Owner: claim.Owner, Fence: claim.Fence, PublicationFingerprint: request.PublicationFingerprint, ResultFingerprint: "result-fingerprint-allowlisted", Published: published})
			require.NoError(t, err)
			require.Equal(t, v3SHA256, attempt.Published.Candidates[0].DurableAsset.SHA256)

			encoded, err := fixture.publishedJSON(fixture.reservation.Identity)
			require.NoError(t, err)
			require.NotContains(t, string(encoded), "URL")
			require.NotContains(t, string(encoded), "Metadata")
			require.NotContains(t, string(encoded), "authorization")
			require.Contains(t, string(encoded), "object_key")
			require.Contains(t, string(encoded), v3SHA256)
			require.NotContains(t, string(encoded), "AAAAAAAA")
		})
	}
}

type v3ReviewFixture struct {
	name          string
	effects       imageagent.SlotExternalEffectV3Repository
	reservation   imageagent.SlotEffectV3Reservation
	expireLease   func(*testing.T, imageagent.SlotExternalEffectIdentity)
	publishedJSON func(imageagent.SlotExternalEffectIdentity) ([]byte, error)
}

type v3ReviewFixtureFactory struct {
	name string
	new  func(*testing.T) v3ReviewFixture
}

func newV3ReviewFixtures() []v3ReviewFixtureFactory {
	return []v3ReviewFixtureFactory{
		{
			name: "memory",
			new: func(t *testing.T) v3ReviewFixture {
				clock := time.Date(2026, time.August, 27, 12, 0, 0, 0, time.UTC)
				repository := newMemoryRepositoryWithClock(func() time.Time { return clock })
				reservation := v3Reservation("review-memory")
				initializeSlotEffectRun(t, repository, reservation.Identity.RunID)
				memory := repository.(*memoryRepository)
				return v3ReviewFixture{
					name:        "memory",
					effects:     repository.(imageagent.SlotExternalEffectV3Repository),
					reservation: reservation,
					expireLease: func(t *testing.T, _ imageagent.SlotExternalEffectIdentity) {
						t.Helper()
						clock = clock.Add(2 * time.Minute)
					},
					publishedJSON: func(identity imageagent.SlotExternalEffectIdentity) ([]byte, error) {
						memory.mu.Lock()
						defer memory.mu.Unlock()
						record, ok := memory.slotEffectsV3[slotEffectKey(identity)]
						if !ok {
							return nil, imageagent.ErrRunNotFound
						}
						return json.Marshal(record.Published)
					},
				}
			},
		},
		{
			name: "gorm",
			new: func(t *testing.T) v3ReviewFixture {
				db := newConcurrentSQLite(t)
				repository := NewGormRepository(db)
				reservation := v3Reservation("review-gorm")
				initializeSlotEffectRun(t, repository, reservation.Identity.RunID)
				return v3ReviewFixture{
					name:        "gorm",
					effects:     repository.(imageagent.SlotExternalEffectV3Repository),
					reservation: reservation,
					expireLease: func(t *testing.T, identity imageagent.SlotExternalEffectIdentity) {
						t.Helper()
						require.NoError(t, slotEffectV3IdentityWhere(db.Model(&slotExternalEffectV3Record{}), identity).Update("publication_lease_expires_at", time.Now().UTC().Add(-time.Hour)).Error)
					},
					publishedJSON: func(identity imageagent.SlotExternalEffectIdentity) ([]byte, error) {
						var record slotExternalEffectV3Record
						err := slotEffectV3IdentityWhere(db, identity).First(&record).Error
						return record.PublishedJSON, err
					},
				}
			},
		},
	}
}

const v3SHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func v3Reservation(suffix string) imageagent.SlotEffectV3Reservation {
	return imageagent.SlotEffectV3Reservation{Identity: imageagent.SlotExternalEffectIdentity{RunScope: imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-1", RunID: "run-slot-effect-v3-" + suffix}, PlanRevision: 1, SlotID: "slot-1", Attempt: 1}, IdempotencyKey: "slot-key-v3-" + suffix, InputFingerprint: "input-fingerprint-v3-" + suffix}
}

func v3StagingManifest() imageagent.StagingManifest {
	return imageagent.StagingManifest{Assets: []imageagent.StagedAssetRef{{ObjectKey: "image-agent/staging/tenant-a/run/asset.png", SHA256: v3SHA256, SizeBytes: 42, ContentType: "image/png", Width: 1200, Height: 1200, SourceAssetID: "source-1", Operations: []string{"resize"}, ProviderReceiptID: "receipt-1"}}}
}

func v3FinalManifest() imageagent.FinalManifest {
	return imageagent.FinalManifest{Assets: []imageagent.StagedAssetRef{{ObjectKey: "image-agent/final/tenant-a/run/asset.png", SHA256: v3SHA256, SizeBytes: 42, ContentType: "image/png", Width: 1200, Height: 1200, SourceAssetID: "source-1", Operations: []string{"resize"}, ProviderReceiptID: "receipt-1"}}}
}

func v3PublicationRequest(reservation imageagent.SlotEffectV3Reservation, owner string) imageagent.PublicationClaimRequest {
	return imageagent.PublicationClaimRequest{Reservation: reservation, Owner: owner, LeaseDuration: time.Minute, PublicationFingerprint: "publication-fingerprint-1", FinalManifest: v3FinalManifest()}
}

func v3PublishedResult(reservation imageagent.SlotEffectV3Reservation) imageagent.SlotEffectV3PublishedResult {
	return imageagent.SlotEffectV3PublishedResult{SlotID: reservation.Identity.SlotID, Attempt: reservation.Identity.Attempt, Candidates: []imageagent.SlotEffectV3AssetCandidate{{AssetID: "candidate-1", SourceAssetID: "source-1", DurableAsset: imageagent.DurableAssetIdentity{ObjectKey: "image-agent/final/tenant-a/run/asset.png", SHA256: v3SHA256}}}}
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
