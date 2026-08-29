package store

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/effectpolicy"
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

			published := v3PublishedResult(reservation)
			completion := imageagent.PublicationCompletion{
				Reservation:            reservation,
				Owner:                  claimRequest.Owner,
				Fence:                  claim.Fence,
				PublicationFingerprint: claimRequest.PublicationFingerprint,
				ResultFingerprint:      mustV3ResultFingerprint(t, published),
				Published:              published,
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

func TestSlotEffectV3StagingRepositoryConformance(t *testing.T) {
	factories := []struct {
		name string
		new  func(*testing.T) imageagent.Repository
	}{
		{name: "memory", new: func(*testing.T) imageagent.Repository { return NewMemoryRepository() }},
		{name: "gorm", new: func(t *testing.T) imageagent.Repository { return NewGormRepository(newConcurrentSQLite(t)) }},
	}
	var baseline stagingConformanceTrace
	for index, factory := range factories {
		t.Run(factory.name, func(t *testing.T) {
			trace := runSlotEffectV3StagingConformance(t, factory.new(t))
			if index == 0 {
				baseline = trace
				return
			}
			require.Equal(t, baseline, trace)
		})
	}
}

func TestSlotEffectV3PublicationRepositoryConformance(t *testing.T) {
	var baseline publicationConformanceTrace
	for index, factory := range newV3ReviewFixtures() {
		t.Run(factory.name, func(t *testing.T) {
			trace := runSlotEffectV3PublicationConformance(t, factory.new(t))
			if index == 0 {
				baseline = trace
				return
			}
			require.Equal(t, baseline, trace)
		})
	}
}

type publicationConformanceTrace struct {
	FirstPhase              imageagent.SlotEffectV3Phase
	FirstOwner              string
	FirstFence              int64
	FirstAcquired           bool
	NormalizedManifestSHA   string
	ReplayAcquired          bool
	ReplayFence             int64
	ManifestConflict        bool
	SuccessorOwner          string
	SuccessorFence          int64
	SuccessorAcquired       bool
	StaleRenewalConflict    bool
	StaleCompletionConflict bool
	RenewedFence            int64
	CompletedPhase          imageagent.SlotEffectV3Phase
	CompletedFingerprint    string
	CompletedPublished      imageagent.SlotEffectV3PublishedResult
	CompletionRepeat        bool
	PostCompletionAcquired  bool
	PostCompletionPhase     imageagent.SlotEffectV3Phase
}

func runSlotEffectV3PublicationConformance(t *testing.T, fixture v3ReviewFixture) publicationConformanceTrace {
	t.Helper()
	ctx := context.Background()
	stageV3Attempt(t, fixture.effects, fixture.reservation)
	request := v3PublicationRequest(fixture.reservation, "worker-a")
	request.FinalManifest.Assets[0].SHA256 = strings.ToUpper(request.FinalManifest.Assets[0].SHA256)
	firstAttempt, first, firstAcquired, err := fixture.effects.ClaimSlotPublicationV3(ctx, request)
	require.NoError(t, err)
	require.True(t, firstAcquired)
	require.Equal(t, imageagent.SlotEffectV3PublicationClaimed, firstAttempt.Phase)
	require.Equal(t, "worker-a", first.Owner)
	require.EqualValues(t, 1, first.Fence)
	require.False(t, first.LeaseExpiresAt.IsZero())
	require.Equal(t, v3SHA256, firstAttempt.FinalManifest.Assets[0].SHA256)
	require.Equal(t, request.PublicationFingerprint, firstAttempt.PublicationFingerprint)
	replayAttempt, replay, replayAcquired, err := fixture.effects.ClaimSlotPublicationV3(ctx, request)
	require.NoError(t, err)
	require.False(t, replayAcquired)
	require.Equal(t, first, replay)
	require.Equal(t, firstAttempt, replayAttempt)

	conflict := request
	conflict.FinalManifest.Assets = append([]imageagent.PublishedAssetRef(nil), request.FinalManifest.Assets...)
	conflict.FinalManifest.Assets[0].ObjectKey = "image-agent/final/tenant-a/run/conflict.png"
	_, _, _, conflictErr := fixture.effects.ClaimSlotPublicationV3(ctx, conflict)
	require.ErrorIs(t, conflictErr, imageagent.ErrRevisionConflict)

	fixture.expireLease(t, fixture.reservation.Identity)
	request.Owner = "worker-b"
	successorAttempt, successor, successorAcquired, err := fixture.effects.ClaimSlotPublicationV3(ctx, request)
	require.NoError(t, err)
	require.True(t, successorAcquired)
	require.Equal(t, imageagent.SlotEffectV3PublicationClaimed, successorAttempt.Phase)
	require.Equal(t, "worker-b", successor.Owner)
	require.EqualValues(t, 2, successor.Fence)
	_, staleRenewalErr := fixture.effects.RenewSlotPublicationV3(ctx, imageagent.PublicationLeaseRenewal{Identity: fixture.reservation.Identity, Owner: first.Owner, Fence: first.Fence, LeaseDuration: time.Minute})
	require.ErrorIs(t, staleRenewalErr, imageagent.ErrRevisionConflict)
	published := v3PublishedResult(fixture.reservation)
	published.Candidates[0].DurableAsset.SHA256 = strings.ToUpper(published.Candidates[0].DurableAsset.SHA256)
	fingerprintInput := cloneV3PublishedResultForTest(published)
	resultFingerprint := mustV3ResultFingerprint(t, fingerprintInput)
	completion := imageagent.PublicationCompletion{Reservation: fixture.reservation, Owner: successor.Owner, Fence: successor.Fence, PublicationFingerprint: request.PublicationFingerprint, ResultFingerprint: resultFingerprint, Published: published}
	originalCompletion := completion
	originalCompletion.Published = cloneV3PublishedResultForTest(completion.Published)
	staleCompletion := completion
	staleCompletion.Owner = first.Owner
	staleCompletion.Fence = first.Fence
	_, staleCompletionErr := fixture.effects.CompleteSlotPublicationV3(ctx, staleCompletion)
	require.ErrorIs(t, staleCompletionErr, imageagent.ErrRevisionConflict)
	require.Equal(t, strings.ToUpper(v3SHA256), completion.Published.Candidates[0].DurableAsset.SHA256)
	renewed, err := fixture.effects.RenewSlotPublicationV3(ctx, imageagent.PublicationLeaseRenewal{Identity: fixture.reservation.Identity, Owner: successor.Owner, Fence: successor.Fence, LeaseDuration: time.Minute})
	require.NoError(t, err)
	require.Equal(t, successor.Owner, renewed.Owner)
	require.Equal(t, successor.Fence, renewed.Fence)
	require.False(t, renewed.LeaseExpiresAt.Before(successor.LeaseExpiresAt))
	completed, err := fixture.effects.CompleteSlotPublicationV3(ctx, completion)
	require.NoError(t, err)
	require.Equal(t, originalCompletion, completion)
	require.Equal(t, strings.ToUpper(v3SHA256), completion.Published.Candidates[0].DurableAsset.SHA256)
	expectedPublished := cloneV3PublishedResultForTest(published)
	expectedPublished, err = imageagent.NormalizeSlotEffectV3PublishedResult(expectedPublished)
	require.NoError(t, err)
	require.Equal(t, imageagent.SlotEffectV3PublicationComplete, completed.Phase)
	require.Equal(t, resultFingerprint, completed.ResultFingerprint)
	require.Equal(t, expectedPublished, completed.Published)
	repeated, err := fixture.effects.CompleteSlotPublicationV3(ctx, completion)
	require.NoError(t, err)
	require.Equal(t, completed, repeated)
	postComplete, postClaim, postAcquired, err := fixture.effects.ClaimSlotPublicationV3(ctx, request)
	require.NoError(t, err)
	require.False(t, postAcquired)
	require.Equal(t, completed, postComplete)
	require.Equal(t, successor, postClaim)

	return publicationConformanceTrace{
		FirstPhase: firstAttempt.Phase, FirstOwner: first.Owner, FirstFence: first.Fence, FirstAcquired: firstAcquired,
		NormalizedManifestSHA: firstAttempt.FinalManifest.Assets[0].SHA256, ReplayAcquired: replayAcquired, ReplayFence: replay.Fence,
		ManifestConflict: errors.Is(conflictErr, imageagent.ErrRevisionConflict), SuccessorOwner: successor.Owner, SuccessorFence: successor.Fence, SuccessorAcquired: successorAcquired,
		StaleRenewalConflict: errors.Is(staleRenewalErr, imageagent.ErrRevisionConflict), StaleCompletionConflict: errors.Is(staleCompletionErr, imageagent.ErrRevisionConflict),
		RenewedFence: renewed.Fence, CompletedPhase: completed.Phase, CompletedFingerprint: completed.ResultFingerprint, CompletedPublished: completed.Published,
		CompletionRepeat: reflect.DeepEqual(completed, repeated), PostCompletionAcquired: postAcquired, PostCompletionPhase: postComplete.Phase,
	}
}

func TestSlotEffectV3StagingInputValidationPrecedesStoredStateAcrossAdapters(t *testing.T) {
	factories := []struct {
		name string
		new  func(*testing.T) imageagent.Repository
	}{
		{name: "memory", new: func(*testing.T) imageagent.Repository { return NewMemoryRepository() }},
		{name: "gorm", new: func(t *testing.T) imageagent.Repository { return NewGormRepository(newConcurrentSQLite(t)) }},
	}
	for _, factory := range factories {
		t.Run(factory.name+"/missing_effect", func(t *testing.T) {
			repository := factory.new(t)
			reservation := v3Reservation("staging-input-missing-" + factory.name)
			initializeSlotEffectRun(t, repository, reservation.Identity.RunID)
			_, err := repository.(imageagent.SlotExternalEffectV3Repository).PrepareSlotStagingV3(context.Background(), reservation, invalidV3StagingManifest())
			require.ErrorIs(t, err, imageagent.ErrValidation)
		})

		t.Run(factory.name+"/invalid_or_corrupt_persisted_state", func(t *testing.T) {
			repository := factory.new(t)
			reservation := v3Reservation("staging-input-corrupt-" + factory.name)
			initializeSlotEffectRun(t, repository, reservation.Identity.RunID)
			effects := repository.(imageagent.SlotExternalEffectV3Repository)
			_, claimed, err := effects.ReserveSlotProviderV3(context.Background(), reservation)
			require.NoError(t, err)
			require.True(t, claimed)
			switch typed := repository.(type) {
			case *memoryRepository:
				typed.mu.Lock()
				attempt := typed.slotEffectsV3[slotEffectKey(reservation.Identity)]
				attempt.Phase = "invalid_persisted_phase"
				typed.slotEffectsV3[slotEffectKey(reservation.Identity)] = attempt
				typed.mu.Unlock()
			case *gormRepository:
				require.NoError(t, slotEffectV3IdentityWhere(typed.db.Model(&slotExternalEffectV3Record{}), reservation.Identity).Update("staging_manifest_json", []byte(`{`)).Error)
			default:
				t.Fatalf("unsupported repository type %T", repository)
			}
			_, err = effects.PrepareSlotStagingV3(context.Background(), reservation, invalidV3StagingManifest())
			require.ErrorIs(t, err, imageagent.ErrValidation)
		})
	}
}

func invalidV3StagingManifest() imageagent.StagingManifest {
	manifest := v3StagingManifest()
	manifest.Assets[0].ObjectKey = `C:\\worker\\generated.png`
	return manifest
}

type stagingConformanceTrace struct {
	Prepared             imageagent.SlotEffectV3Attempt
	PreparedRepeat       imageagent.SlotEffectV3Attempt
	ConflictingPrepare   bool
	WrongCommit          bool
	Committed            imageagent.SlotEffectV3Attempt
	CommittedRepeat      imageagent.SlotEffectV3Attempt
	CommitBeforePrepared bool
}

func runSlotEffectV3StagingConformance(t *testing.T, repository imageagent.Repository) stagingConformanceTrace {
	t.Helper()
	reservation := v3Reservation("staging-conformance")
	initializeSlotEffectRun(t, repository, reservation.Identity.RunID)
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	_, claimed, err := effects.ReserveSlotProviderV3(context.Background(), reservation)
	require.NoError(t, err)
	require.True(t, claimed)

	manifest := v3StagingManifest()
	manifest.Assets[0].SHA256 = strings.ToUpper(manifest.Assets[0].SHA256)
	prepared, err := effects.PrepareSlotStagingV3(context.Background(), reservation, manifest)
	require.NoError(t, err)
	preparedRepeat, err := effects.PrepareSlotStagingV3(context.Background(), reservation, manifest)
	require.NoError(t, err)
	conflicting := manifest
	conflicting.Assets = append([]imageagent.StagedAssetRef(nil), manifest.Assets...)
	conflicting.Assets[0].ObjectKey = "image-agent/staging/tenant-a/run/replacement.png"
	_, conflictingErr := effects.PrepareSlotStagingV3(context.Background(), reservation, conflicting)
	_, wrongCommitErr := effects.CommitSlotStagedV3(context.Background(), reservation, "wrong-fingerprint")
	committed, err := effects.CommitSlotStagedV3(context.Background(), reservation, prepared.StagingManifestFingerprint)
	require.NoError(t, err)
	committedRepeat, err := effects.CommitSlotStagedV3(context.Background(), reservation, prepared.StagingManifestFingerprint)
	require.NoError(t, err)

	beforePreparedReservation := v3Reservation("staging-before-prepared")
	initializeSlotEffectRun(t, repository, beforePreparedReservation.Identity.RunID)
	_, claimed, err = effects.ReserveSlotProviderV3(context.Background(), beforePreparedReservation)
	require.NoError(t, err)
	require.True(t, claimed)
	_, beforePreparedErr := effects.CommitSlotStagedV3(context.Background(), beforePreparedReservation, prepared.StagingManifestFingerprint)
	require.Equal(t, imageagent.SlotEffectV3StagingPrepared, prepared.Phase)
	require.Equal(t, strings.ToLower(manifest.Assets[0].SHA256), prepared.StagingManifest.Assets[0].SHA256)
	require.Equal(t, prepared, preparedRepeat)
	require.ErrorIs(t, conflictingErr, imageagent.ErrRevisionConflict)
	require.ErrorIs(t, wrongCommitErr, imageagent.ErrRevisionConflict)
	require.Equal(t, imageagent.SlotEffectV3ArtifactStaged, committed.Phase)
	require.Equal(t, committed, committedRepeat)
	require.ErrorIs(t, beforePreparedErr, imageagent.ErrRevisionConflict)

	return stagingConformanceTrace{
		Prepared:             normalizeProviderBudgetAttempt(prepared),
		PreparedRepeat:       normalizeProviderBudgetAttempt(preparedRepeat),
		ConflictingPrepare:   errors.Is(conflictingErr, imageagent.ErrRevisionConflict),
		WrongCommit:          errors.Is(wrongCommitErr, imageagent.ErrRevisionConflict),
		Committed:            normalizeProviderBudgetAttempt(committed),
		CommittedRepeat:      normalizeProviderBudgetAttempt(committedRepeat),
		CommitBeforePrepared: errors.Is(beforePreparedErr, imageagent.ErrRevisionConflict),
	}
}

func TestSlotEffectV3ProviderBudgetRepositoryConformance(t *testing.T) {
	factories := []struct {
		name string
		new  func(*testing.T) imageagent.Repository
	}{
		{name: "memory", new: func(*testing.T) imageagent.Repository { return NewMemoryRepository() }},
		{name: "gorm", new: func(t *testing.T) imageagent.Repository { return NewGormRepository(newConcurrentSQLite(t)) }},
	}
	var baseline providerBudgetConformanceTrace
	for index, factory := range factories {
		t.Run(factory.name, func(t *testing.T) {
			repository := factory.new(t)
			scope, policy := initializeProviderBudgetConformanceRun(t, repository)
			trace := exerciseProviderBudgetConformance(t, repository, budgetedV3Reservation(scope, policy, 1, "quote-conformance"))
			if index == 0 {
				baseline = trace
				return
			}
			require.Equal(t, baseline, trace)
		})
	}
}

func TestSlotEffectV3RecordFromProviderDecisionUsesAttemptState(t *testing.T) {
	reservation := v3Reservation("decision-record")
	reservation.Policy = imageagent.BudgetPolicy{Images: imageagent.Limit{Enabled: true, Value: 3}}
	reservation.Quote = imageagent.SlotUsageQuote{
		Maximum:        imageagent.UsageVector{Images: 1},
		Operations:     []imageagent.SlotUsageOperation{{Name: "generate", Fingerprint: "operation-record", MaximumOutputs: 1, Maximum: imageagent.UsageVector{Images: 1}}},
		PricingVersion: "pricing-record", Fingerprint: "quote-record",
	}
	attempt := imageagent.SlotEffectV3Attempt{
		Identity: reservation.Identity, IdempotencyKey: reservation.IdempotencyKey, InputFingerprint: reservation.InputFingerprint,
		Phase: imageagent.SlotEffectV3ProviderNotDispatched, BudgetStatus: imageagent.SlotBudgetReleased,
		Policy: reservation.Policy, Quote: reservation.Quote,
	}
	claimedAt := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)

	record := slotEffectV3RecordFromProviderDecision(effectpolicy.ProviderReservationDecision{
		AccountingDecision: effectpolicy.AccountingDecision{EffectDecision: effectpolicy.EffectDecision{Attempt: attempt, Changed: true}},
		Acquired:           true,
	}, claimedAt)

	require.Equal(t, string(attempt.Phase), record.Phase)
	require.Equal(t, string(attempt.BudgetStatus), record.BudgetStatus)
	require.Equal(t, attempt.Quote.Fingerprint, record.UsageQuoteFingerprint)
	require.Equal(t, attempt.Quote.PricingVersion, record.PricingVersion)
	require.Equal(t, claimedAt, record.ProviderClaimedAt)
	var persistedPolicy imageagent.BudgetPolicy
	require.NoError(t, json.Unmarshal(record.BudgetPolicyJSON, &persistedPolicy))
	require.Equal(t, attempt.Policy, persistedPolicy)
	var persistedQuote imageagent.SlotUsageQuote
	require.NoError(t, json.Unmarshal(record.UsageQuoteJSON, &persistedQuote))
	require.Equal(t, attempt.Quote, persistedQuote)
}

type providerBudgetConformanceTrace struct {
	ReservedAttempt          imageagent.SlotEffectV3Attempt
	ReservedAcquired         bool
	ReservedAccounting       providerBudgetAccounting
	RepeatedAttempt          imageagent.SlotEffectV3Attempt
	RepeatedAcquired         bool
	RepeatedAccounting       providerBudgetAccounting
	ConflictIsRevision       bool
	NotDispatchedAttempt     imageagent.SlotEffectV3Attempt
	NotDispatchedAccounting  providerBudgetAccounting
	NotDispatchedRepeat      imageagent.SlotEffectV3Attempt
	RedispatchedAttempt      imageagent.SlotEffectV3Attempt
	RedispatchedAcquired     bool
	RedispatchedAccounting   providerBudgetAccounting
	ExceededIsBudgetExceeded bool
	SettledAttempt           imageagent.SlotEffectV3Attempt
	SettledAccounting        providerBudgetAccounting
	SettledRepeat            imageagent.SlotEffectV3Attempt
	SettlementConflict       bool
	ReleasedAttempt          imageagent.SlotEffectV3Attempt
	ReleasedRepeat           imageagent.SlotEffectV3Attempt
	ReleasedAccounting       providerBudgetAccounting
	UnknownAttempt           imageagent.SlotEffectV3Attempt
	UnknownRepeat            imageagent.SlotEffectV3Attempt
	UnknownAccounting        providerBudgetAccounting
}

type providerBudgetAccounting struct {
	Policy    imageagent.BudgetPolicy
	Committed imageagent.UsageVector
	Reserved  imageagent.UsageVector
	Elapsed   time.Duration
}

func initializeProviderBudgetConformanceRun(t *testing.T, repository imageagent.Repository) (imageagent.RunScope, imageagent.BudgetPolicy) {
	t.Helper()
	run := manualRun("run-provider-budget-conformance", "tenant-a")
	run.BusinessTaskID = "task-run-provider-budget-conformance"
	run.Budget = imageagent.Budget{MaxImages: 3, EnabledLimits: imageagent.BudgetLimitImages}
	run.StartedAt = time.Date(2099, time.January, 1, 0, 0, 0, 0, time.UTC)
	policy, err := run.Budget.Policy()
	require.NoError(t, err)
	plan := planRevision(1)
	scope := imageagent.ScopeForRun(*run)
	_, err = repository.InitializeRun(context.Background(), imageagent.ProjectionInitialization{
		Scope: scope, Run: *run, Plan: plan,
		Catalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{
			{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/source.png"},
			{ID: "style-1", Type: imageagent.AuthorizedAssetStyle, URL: "https://style.example/style.png"},
		}},
		Snapshot: imageagent.RunProjection{Run: *run, Plan: plan}, CommitID: "start:provider-budget-conformance", EventType: "run.initialized", EventPayload: []byte(`{}`),
	})
	require.NoError(t, err)
	return scope, policy
}

func exerciseProviderBudgetConformance(t *testing.T, repository imageagent.Repository, reservation imageagent.SlotEffectV3Reservation) providerBudgetConformanceTrace {
	t.Helper()
	ctx := context.Background()
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	var trace providerBudgetConformanceTrace
	var err error
	var attempt imageagent.SlotEffectV3Attempt

	attempt, trace.ReservedAcquired, err = effects.ReserveSlotProviderV3(ctx, reservation)
	require.NoError(t, err)
	require.True(t, trace.ReservedAcquired)
	trace.ReservedAttempt = normalizeProviderBudgetAttempt(attempt)
	trace.ReservedAccounting = readProviderBudgetAccounting(t, repository, reservation.Identity.RunScope)

	attempt, trace.RepeatedAcquired, err = effects.ReserveSlotProviderV3(ctx, reservation)
	require.NoError(t, err)
	require.False(t, trace.RepeatedAcquired)
	trace.RepeatedAttempt = normalizeProviderBudgetAttempt(attempt)
	trace.RepeatedAccounting = readProviderBudgetAccounting(t, repository, reservation.Identity.RunScope)

	conflict := reservation
	conflict.InputFingerprint = "conflicting-input"
	_, _, err = effects.ReserveSlotProviderV3(ctx, conflict)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
	trace.ConflictIsRevision = errors.Is(err, imageagent.ErrRevisionConflict)

	attempt, err = effects.RecordSlotProviderNotDispatchedV3(ctx, reservation)
	require.NoError(t, err)
	trace.NotDispatchedAttempt = normalizeProviderBudgetAttempt(attempt)
	trace.NotDispatchedAccounting = readProviderBudgetAccounting(t, repository, reservation.Identity.RunScope)
	attempt, err = effects.RecordSlotProviderNotDispatchedV3(ctx, reservation)
	require.NoError(t, err)
	trace.NotDispatchedRepeat = normalizeProviderBudgetAttempt(attempt)

	attempt, trace.RedispatchedAcquired, err = effects.ReserveSlotProviderV3(ctx, reservation)
	require.NoError(t, err)
	require.True(t, trace.RedispatchedAcquired)
	trace.RedispatchedAttempt = normalizeProviderBudgetAttempt(attempt)
	trace.RedispatchedAccounting = readProviderBudgetAccounting(t, repository, reservation.Identity.RunScope)

	denied := budgetedV3Reservation(reservation.Identity.RunScope, reservation.Policy, 2, "quote-denied-conformance")
	denied.Quote.Maximum.Images = 3
	denied.Quote.Operations[0].Maximum.Images = 3
	_, _, err = effects.ReserveSlotProviderV3(ctx, denied)
	require.ErrorIs(t, err, imageagent.ErrBudgetExceeded)
	trace.ExceededIsBudgetExceeded = errors.Is(err, imageagent.ErrBudgetExceeded)

	receipt := imageagent.SlotUsageReceipt{Actual: imageagent.UsageVector{Images: 1, AgentSteps: 1}, ProviderRequestIDs: []string{"request-conformance"}, CostBasis: imageagent.UsageCostReservedUpperBound}
	attempt, err = effects.SettleSlotProviderV3(ctx, reservation, receipt)
	require.NoError(t, err)
	trace.SettledAttempt = normalizeProviderBudgetAttempt(attempt)
	trace.SettledAccounting = readProviderBudgetAccounting(t, repository, reservation.Identity.RunScope)
	attempt, err = effects.SettleSlotProviderV3(ctx, reservation, receipt)
	require.NoError(t, err)
	trace.SettledRepeat = normalizeProviderBudgetAttempt(attempt)
	conflictingReceipt := receipt
	conflictingReceipt.ProviderRequestIDs = []string{"other-request"}
	_, err = effects.SettleSlotProviderV3(ctx, reservation, conflictingReceipt)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
	trace.SettlementConflict = errors.Is(err, imageagent.ErrRevisionConflict)

	releaseReservation := budgetedV3Reservation(reservation.Identity.RunScope, reservation.Policy, 2, "quote-release-conformance")
	_, acquired, err := effects.ReserveSlotProviderV3(ctx, releaseReservation)
	require.NoError(t, err)
	require.True(t, acquired)
	attempt, err = effects.ReleaseSlotProviderBudgetV3(ctx, releaseReservation)
	require.NoError(t, err)
	trace.ReleasedAttempt = normalizeProviderBudgetAttempt(attempt)
	attempt, err = effects.ReleaseSlotProviderBudgetV3(ctx, releaseReservation)
	require.NoError(t, err)
	trace.ReleasedRepeat = normalizeProviderBudgetAttempt(attempt)
	trace.ReleasedAccounting = readProviderBudgetAccounting(t, repository, reservation.Identity.RunScope)

	unknownReservation := budgetedV3Reservation(reservation.Identity.RunScope, reservation.Policy, 3, "quote-unknown-conformance")
	_, acquired, err = effects.ReserveSlotProviderV3(ctx, unknownReservation)
	require.NoError(t, err)
	require.True(t, acquired)
	attempt, err = effects.MarkSlotProviderBudgetUnknownV3(ctx, unknownReservation)
	require.NoError(t, err)
	trace.UnknownAttempt = normalizeProviderBudgetAttempt(attempt)
	attempt, err = effects.MarkSlotProviderBudgetUnknownV3(ctx, unknownReservation)
	require.NoError(t, err)
	trace.UnknownRepeat = normalizeProviderBudgetAttempt(attempt)
	trace.UnknownAccounting = readProviderBudgetAccounting(t, repository, reservation.Identity.RunScope)
	return trace
}

func normalizeProviderBudgetAttempt(attempt imageagent.SlotEffectV3Attempt) imageagent.SlotEffectV3Attempt {
	if len(attempt.StagingManifest.Assets) == 0 {
		attempt.StagingManifest.Assets = nil
	}
	if len(attempt.FinalManifest.Assets) == 0 {
		attempt.FinalManifest.Assets = nil
	}
	return attempt
}

func readProviderBudgetAccounting(t *testing.T, repository imageagent.Repository, scope imageagent.RunScope) providerBudgetAccounting {
	t.Helper()
	switch typed := repository.(type) {
	case *memoryRepository:
		typed.mu.Lock()
		defer typed.mu.Unlock()
		run := typed.runs[scopeKey(scope)]
		policy, err := run.Budget.Policy()
		require.NoError(t, err)
		committed, err := imageagent.UsageVectorFromBudgetUsage(run.Usage)
		require.NoError(t, err)
		return providerBudgetAccounting{Policy: policy, Committed: committed, Reserved: typed.reservedUsage[scopeKey(scope)], Elapsed: run.Usage.Elapsed}
	case *gormRepository:
		var row runRecord
		require.NoError(t, runScopeWhere(typed.db, scope).First(&row).Error)
		run, err := recordToRun(row)
		require.NoError(t, err)
		policy, err := run.Budget.Policy()
		require.NoError(t, err)
		committed, err := imageagent.UsageVectorFromBudgetUsage(run.Usage)
		require.NoError(t, err)
		reserved, err := decodeReservedUsage(row.ReservedUsageJSON)
		require.NoError(t, err)
		return providerBudgetAccounting{Policy: policy, Committed: committed, Reserved: reserved, Elapsed: run.Usage.Elapsed}
	default:
		t.Fatalf("unsupported repository type %T", repository)
		return providerBudgetAccounting{}
	}
}

func TestRecoveryBlockedEffectRestoresItsPriorPublicationPhaseForExplicitRedrive(t *testing.T) {
	for _, fixtureFactory := range newV3ReviewFixtures() {
		t.Run(fixtureFactory.name, func(t *testing.T) {
			fixture := fixtureFactory.new(t)
			ctx := context.Background()
			_, _, err := fixture.effects.ReserveSlotProviderV3(ctx, fixture.reservation)
			require.NoError(t, err)
			prepared, err := fixture.effects.PrepareSlotStagingV3(ctx, fixture.reservation, v3StagingManifest())
			require.NoError(t, err)
			_, err = fixture.effects.CommitSlotStagedV3(ctx, fixture.reservation, prepared.StagingManifestFingerprint)
			require.NoError(t, err)
			_, claim, claimed, err := fixture.effects.ClaimSlotPublicationV3(ctx, imageagent.PublicationClaimRequest{
				Reservation: fixture.reservation, Owner: "worker-a", LeaseDuration: time.Minute,
				PublicationFingerprint: "publication-fingerprint-redrive", FinalManifest: v3FinalManifest(),
			})
			require.NoError(t, err)
			require.True(t, claimed)

			blocked, err := fixture.effects.BlockSlotEffectV3(ctx, imageagent.SlotEffectV3BlockTransition{
				Reservation: fixture.reservation, Phase: imageagent.SlotEffectV3RecoveryBlocked, Code: imageagent.SlotRecoveryBlockedCode,
			})
			require.NoError(t, err)
			require.Equal(t, imageagent.SlotEffectV3PublicationClaimed, blocked.RecoveryPhase)

			restorer, ok := fixture.effects.(imageagent.RecoveryBlockedSlotEffectV3Repository)
			require.True(t, ok)
			restored, err := restorer.RestoreRecoveryBlockedEffectV3(ctx, fixture.reservation)
			require.NoError(t, err)
			require.Equal(t, imageagent.SlotEffectV3PublicationClaimed, restored.Phase)
			require.Empty(t, restored.BlockedCode)
			require.Empty(t, restored.RecoveryPhase)
			require.Equal(t, claim.Fence, restored.Publication.Fence)

			_, err = restorer.RestoreRecoveryBlockedEffectV3(ctx, fixture.reservation)
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
		})
	}
}

func TestSlotEffectV3RepositoryPreservesNilAndEmptyOperationsAcrossReplay(t *testing.T) {
	for _, factory := range newV3ReviewFixtures() {
		for _, operations := range []struct {
			name  string
			value []string
		}{
			{name: "nil", value: nil},
			{name: "empty", value: []string{}},
		} {
			t.Run(factory.name+"/"+operations.name, func(t *testing.T) {
				fixture := factory.new(t)
				ctx := context.Background()
				_, _, err := fixture.effects.ReserveSlotProviderV3(ctx, fixture.reservation)
				require.NoError(t, err)

				staging := v3StagingManifest()
				staging.Assets[0].Operations = operations.value
				stagingFingerprint, err := imageagent.StagingManifestFingerprint(staging)
				require.NoError(t, err)
				prepared, err := fixture.effects.PrepareSlotStagingV3(ctx, fixture.reservation, staging)
				require.NoError(t, err)
				require.Equal(t, stagingFingerprint, prepared.StagingManifestFingerprint)
				require.Equal(t, operations.value == nil, prepared.StagingManifest.Assets[0].Operations == nil)

				staged, err := fixture.effects.CommitSlotStagedV3(ctx, fixture.reservation, stagingFingerprint)
				require.NoError(t, err)
				require.Equal(t, operations.value == nil, staged.StagingManifest.Assets[0].Operations == nil)

				final := v3FinalManifest()
				final.Assets[0].Operations = operations.value
				publicationFingerprint, err := imageagent.FinalManifestFingerprint(final)
				require.NoError(t, err)
				request := imageagent.PublicationClaimRequest{
					Reservation: fixture.reservation, Owner: "worker-a", LeaseDuration: time.Minute,
					PublicationFingerprint: publicationFingerprint, FinalManifest: final,
				}
				claimedAttempt, firstClaim, claimed, err := fixture.effects.ClaimSlotPublicationV3(ctx, request)
				require.NoError(t, err)
				require.True(t, claimed)
				require.Equal(t, operations.value == nil, claimedAttempt.FinalManifest.Assets[0].Operations == nil)

				replayedAttempt, replayedClaim, claimed, err := fixture.effects.ClaimSlotPublicationV3(ctx, request)
				require.NoError(t, err)
				require.False(t, claimed)
				require.Equal(t, firstClaim, replayedClaim)
				require.Equal(t, operations.value == nil, replayedAttempt.FinalManifest.Assets[0].Operations == nil)

				got, err := fixture.effects.GetSlotExternalEffectV3(ctx, fixture.reservation.Identity)
				require.NoError(t, err)
				require.Equal(t, operations.value == nil, got.StagingManifest.Assets[0].Operations == nil)
				require.Equal(t, operations.value == nil, got.FinalManifest.Assets[0].Operations == nil)
				gotStagingFingerprint, err := imageagent.StagingManifestFingerprint(got.StagingManifest)
				require.NoError(t, err)
				require.Equal(t, stagingFingerprint, gotStagingFingerprint)
				gotFinalFingerprint, err := imageagent.FinalManifestFingerprint(got.FinalManifest)
				require.NoError(t, err)
				require.Equal(t, publicationFingerprint, gotFinalFingerprint)

				stagingJSON, err := json.Marshal(got.StagingManifest)
				require.NoError(t, err)
				finalJSON, err := json.Marshal(got.FinalManifest)
				require.NoError(t, err)
				if operations.value == nil {
					require.Contains(t, string(stagingJSON), `"operations":null`)
					require.Contains(t, string(finalJSON), `"operations":null`)
				} else {
					require.Contains(t, string(stagingJSON), `"operations":[]`)
					require.Contains(t, string(finalJSON), `"operations":[]`)
				}
			})
		}
	}
}

func TestSlotEffectV3CompletionRequiresOrderedBijectionAcrossAdapters(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*imageagent.SlotEffectV3PublishedResult, *string)
	}{
		{name: "missing", mutate: func(result *imageagent.SlotEffectV3PublishedResult, _ *string) {
			result.Candidates = result.Candidates[:1]
		}},
		{name: "duplicate ref", mutate: func(result *imageagent.SlotEffectV3PublishedResult, _ *string) {
			result.Candidates[1].DurableAsset = result.Candidates[0].DurableAsset
			result.Candidates[1].SourceAssetID = result.Candidates[0].SourceAssetID
		}},
		{name: "reordered", mutate: func(result *imageagent.SlotEffectV3PublishedResult, _ *string) {
			result.Candidates[0], result.Candidates[1] = result.Candidates[1], result.Candidates[0]
		}},
		{name: "key", mutate: func(result *imageagent.SlotEffectV3PublishedResult, _ *string) {
			result.Candidates[1].DurableAsset.ObjectKey = "image-agent/final/tenant-a/run/other.png"
		}},
		{name: "hash", mutate: func(result *imageagent.SlotEffectV3PublishedResult, _ *string) {
			result.Candidates[1].DurableAsset.SHA256 = strings.Repeat("c", 64)
		}},
		{name: "false lineage", mutate: func(result *imageagent.SlotEffectV3PublishedResult, _ *string) {
			result.Candidates[1].SourceAssetID = "source-false"
		}},
		{name: "fingerprint", mutate: func(_ *imageagent.SlotEffectV3PublishedResult, fingerprint *string) {
			*fingerprint = "wrong-fingerprint"
		}},
	}
	for _, factory := range newV3ReviewFixtures() {
		for _, mutation := range mutations {
			t.Run(factory.name+"/"+mutation.name, func(t *testing.T) {
				fixture := factory.new(t)
				ctx := context.Background()
				stageV3Attempt(t, fixture.effects, fixture.reservation)
				final := v3TwoAssetFinalManifest()
				publicationFingerprint, err := imageagent.FinalManifestFingerprint(final)
				require.NoError(t, err)
				request := imageagent.PublicationClaimRequest{Reservation: fixture.reservation, Owner: "worker-a", LeaseDuration: time.Minute, PublicationFingerprint: publicationFingerprint, FinalManifest: final}
				_, claim, claimed, err := fixture.effects.ClaimSlotPublicationV3(ctx, request)
				require.NoError(t, err)
				require.True(t, claimed)
				published := v3TwoAssetPublishedResult(fixture.reservation)
				fingerprint, err := imageagent.SlotEffectV3PublishedResultFingerprint(published)
				require.NoError(t, err)
				mutation.mutate(&published, &fingerprint)
				_, err = fixture.effects.CompleteSlotPublicationV3(ctx, imageagent.PublicationCompletion{Reservation: fixture.reservation, Owner: claim.Owner, Fence: claim.Fence, PublicationFingerprint: publicationFingerprint, ResultFingerprint: fingerprint, Published: published})
				require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
			})
		}
	}
}

func TestSlotEffectV3RepositoryRejectsMismatchedBlockedPolicyOnWrite(t *testing.T) {
	for _, factory := range newV3ReviewFixtures() {
		t.Run(factory.name, func(t *testing.T) {
			fixture := factory.new(t)
			_, _, err := fixture.effects.ReserveSlotProviderV3(context.Background(), fixture.reservation)
			require.NoError(t, err)
			_, err = fixture.effects.BlockSlotEffectV3(context.Background(), imageagent.SlotEffectV3BlockTransition{Reservation: fixture.reservation, Phase: imageagent.SlotEffectV3ProviderUnknown, Code: imageagent.SlotPublicationOutcomeUnknownCode})
			require.ErrorIs(t, err, imageagent.ErrInvalidPersistedPolicy)
		})
	}
}

func TestSlotEffectV3RepositoryRejectsMismatchedBlockedPolicyOnRead(t *testing.T) {
	t.Run("memory", func(t *testing.T) {
		reservation := v3Reservation("invalid-policy-read-memory")
		repository := NewMemoryRepository()
		initializeSlotEffectRun(t, repository, reservation.Identity.RunID)
		memory := repository.(*memoryRepository)
		memory.mu.Lock()
		memory.slotEffectsV3[slotEffectKey(reservation.Identity)] = imageagent.SlotEffectV3Attempt{Identity: reservation.Identity, IdempotencyKey: reservation.IdempotencyKey, InputFingerprint: reservation.InputFingerprint, Phase: imageagent.SlotEffectV3PublicationUnknown, BlockedCode: imageagent.SlotProviderOutcomeUnknownCode}
		memory.mu.Unlock()
		_, err := repository.(imageagent.SlotExternalEffectV3Repository).GetSlotExternalEffectV3(context.Background(), reservation.Identity)
		require.ErrorIs(t, err, imageagent.ErrInvalidPersistedPolicy)
	})
	t.Run("gorm", func(t *testing.T) {
		reservation := v3Reservation("invalid-policy-read-gorm")
		db := newConcurrentSQLite(t)
		repository := NewGormRepository(db)
		initializeSlotEffectRun(t, repository, reservation.Identity.RunID)
		row := slotEffectV3RecordFixtureFromReservation(reservation, time.Now().UTC())
		row.Phase = string(imageagent.SlotEffectV3PublicationUnknown)
		row.BlockedCode = imageagent.SlotProviderOutcomeUnknownCode
		require.NoError(t, db.Create(&row).Error)
		_, err := repository.(imageagent.SlotExternalEffectV3Repository).GetSlotExternalEffectV3(context.Background(), reservation.Identity)
		require.ErrorIs(t, err, imageagent.ErrInvalidPersistedPolicy)
	})
}

func TestGormSlotEffectV3CanFailClosedCorruptBudgetPayload(t *testing.T) {
	for _, tc := range []struct {
		name  string
		field string
	}{
		{name: "policy", field: "budget_policy_json"},
		{name: "quote", field: "usage_quote_json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reservation := v3Reservation("corrupt-" + tc.name)
			db := newConcurrentSQLite(t)
			repository := NewGormRepository(db)
			effects := repository.(imageagent.SlotExternalEffectV3Repository)
			initializeSlotEffectRun(t, repository, reservation.Identity.RunID)
			row := slotEffectV3RecordFixtureFromReservation(reservation, time.Now().UTC())
			require.NoError(t, db.Create(&row).Error)
			require.NoError(t, db.Model(&slotExternalEffectV3Record{}).
				Where("tenant_id = ? AND owner_user_id = ? AND run_id = ? AND plan_revision = ? AND slot_id = ? AND attempt = ?",
					reservation.Identity.TenantID, reservation.Identity.OwnerUserID, reservation.Identity.RunID,
					reservation.Identity.PlanRevision, reservation.Identity.SlotID, reservation.Identity.Attempt).
				Update(tc.field, []byte("{corrupt-json")).Error)

			_, err := effects.GetSlotExternalEffectV3(context.Background(), reservation.Identity)
			require.ErrorIs(t, err, imageagent.ErrCorruptPersistedEffect)

			corruptor, ok := repository.(interface {
				BlockCorruptSlotEffectV3(context.Context, imageagent.SlotExternalEffectIdentity) (imageagent.SlotEffectV3Attempt, error)
			})
			require.True(t, ok, "repository must expose an atomic fail-closed corruption transition")
			blocked, err := corruptor.BlockCorruptSlotEffectV3(context.Background(), reservation.Identity)
			require.NoError(t, err)
			require.Equal(t, reservation.Identity, blocked.Identity)
			require.Equal(t, reservation.Identity.Attempt, blocked.Identity.Attempt)
			require.Equal(t, imageagent.SlotEffectV3RecoveryBlocked, blocked.Phase)
			require.Equal(t, imageagent.SlotRecoveryBlockedCode, blocked.BlockedCode)
			require.NotEmpty(t, blocked.CorruptionMarker)

			replayed, err := effects.GetSlotExternalEffectV3(context.Background(), reservation.Identity)
			require.NoError(t, err)
			require.Equal(t, blocked, replayed)
			again, err := corruptor.BlockCorruptSlotEffectV3(context.Background(), reservation.Identity)
			require.NoError(t, err)
			require.Equal(t, blocked, again, "recovery of the same corrupt identity must be idempotent")
		})
	}
}

func TestMemoryReserveSlotProviderV3RejectsCorruptExistingAttempt(t *testing.T) {
	reservation := v3Reservation("invalid-policy-reserve-memory")
	repository := NewMemoryRepository()
	initializeSlotEffectRun(t, repository, reservation.Identity.RunID)
	memory := repository.(*memoryRepository)
	memory.mu.Lock()
	memory.slotEffectsV3[slotEffectKey(reservation.Identity)] = imageagent.SlotEffectV3Attempt{
		Identity: reservation.Identity, IdempotencyKey: reservation.IdempotencyKey, InputFingerprint: reservation.InputFingerprint,
		Phase: imageagent.SlotEffectV3PublicationUnknown, BlockedCode: imageagent.SlotProviderOutcomeUnknownCode,
	}
	memory.mu.Unlock()

	_, _, err := repository.(imageagent.SlotExternalEffectV3Repository).ReserveSlotProviderV3(context.Background(), reservation)
	require.ErrorIs(t, err, imageagent.ErrInvalidPersistedPolicy)
}

func TestSlotEffectV3RejectsLocalPathsAndUnknownMetadata(t *testing.T) {
	for _, factory := range newV3ReviewFixtures() {
		t.Run(factory.name, func(t *testing.T) {
			fixture := factory.new(t)
			ctx := context.Background()
			_, _, err := fixture.effects.ReserveSlotProviderV3(ctx, fixture.reservation)
			require.NoError(t, err)

			local := v3StagingManifest()
			local.Assets[0].ObjectKey = `C:\\worker\\generated.png`
			_, err = fixture.effects.PrepareSlotStagingV3(ctx, fixture.reservation, local)
			require.ErrorIs(t, err, imageagent.ErrValidation)

			relativeWindowsPath := v3StagingManifest()
			relativeWindowsPath.Assets[0].ObjectKey = `worker\generated.png`
			_, err = fixture.effects.PrepareSlotStagingV3(ctx, fixture.reservation, relativeWindowsPath)
			require.ErrorIs(t, err, imageagent.ErrValidation)

			whitespaceKey := v3StagingManifest()
			whitespaceKey.Assets[0].ObjectKey = " image-agent/staging/tenant-a/run/asset.png"
			_, err = fixture.effects.PrepareSlotStagingV3(ctx, fixture.reservation, whitespaceKey)
			require.ErrorIs(t, err, imageagent.ErrValidation)

			metadata := v3StagingManifest()
			metadata.ProviderMetadata = map[string]string{"authorization": "secret"}
			_, err = fixture.effects.PrepareSlotStagingV3(ctx, fixture.reservation, metadata)
			require.ErrorIs(t, err, imageagent.ErrValidation)
		})
	}
}

func TestSlotEffectV3RejectsManifestReplacement(t *testing.T) {
	for _, factory := range newV3ReviewFixtures() {
		t.Run(factory.name, func(t *testing.T) {
			fixture := factory.new(t)
			ctx := context.Background()
			_, _, err := fixture.effects.ReserveSlotProviderV3(ctx, fixture.reservation)
			require.NoError(t, err)
			_, err = fixture.effects.PrepareSlotStagingV3(ctx, fixture.reservation, v3StagingManifest())
			require.NoError(t, err)

			replacement := v3StagingManifest()
			replacement.Assets[0].ObjectKey = "image-agent/staging/tenant-a/run/replaced.png"
			_, err = fixture.effects.PrepareSlotStagingV3(ctx, fixture.reservation, replacement)
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
		})
	}
}

func TestSlotEffectV3HistoricalNilOperationManifestReplaysAcrossAdapters(t *testing.T) {
	for _, factory := range []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "memory", run: testMemoryHistoricalNilOperationReplay},
		{name: "gorm", run: testGormHistoricalNilOperationReplay},
	} {
		t.Run(factory.name, factory.run)
	}
}

const (
	historicalNilOperationStagingJSON = `{"assets":[{"object_key":"image-agent/staging/tenant-a/run/asset.png","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","size_bytes":42,"content_type":"image/png","width":1200,"height":1200,"source_asset_id":"source-1","operations":null,"provider_receipt_id":"receipt-1"}]}`
	historicalNilOperationStagingFP   = "d567351ea12b329121705ff34e42e5e9b4c9c660eda7fee964a04a43bd4de3b7"
	historicalNilOperationFinalJSON   = `{"assets":[{"object_key":"image-agent/final/tenant-a/run/asset.png","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","size_bytes":42,"content_type":"image/png","width":1200,"height":1200,"source_asset_id":"source-1","operations":null,"provider_receipt_id":"receipt-1"}]}`
)

func testMemoryHistoricalNilOperationReplay(t *testing.T) {
	t.Run("staging", func(t *testing.T) {
		repository := NewMemoryRepository()
		reservation := v3Reservation("historical-nil-memory-staging")
		initializeSlotEffectRun(t, repository, reservation.Identity.RunID)
		memory := repository.(*memoryRepository)
		memory.mu.Lock()
		memory.slotEffectsV3[slotEffectKey(reservation.Identity)] = imageagent.SlotEffectV3Attempt{Identity: reservation.Identity, IdempotencyKey: reservation.IdempotencyKey, InputFingerprint: reservation.InputFingerprint, Phase: imageagent.SlotEffectV3StagingPrepared, StagingManifest: v3NilOperationStagingManifest(), StagingManifestFingerprint: historicalNilOperationStagingFP}
		memory.mu.Unlock()

		attempt, err := repository.(imageagent.SlotExternalEffectV3Repository).PrepareSlotStagingV3(context.Background(), reservation, v3NilOperationStagingManifest())
		require.NoError(t, err)
		require.Equal(t, historicalNilOperationStagingFP, attempt.StagingManifestFingerprint)
		require.Nil(t, attempt.StagingManifest.Assets[0].Operations)
	})

	t.Run("final", func(t *testing.T) {
		repository := NewMemoryRepository()
		reservation := v3Reservation("historical-nil-memory-final")
		initializeSlotEffectRun(t, repository, reservation.Identity.RunID)
		request := v3NilOperationPublicationRequest(reservation)
		memory := repository.(*memoryRepository)
		memory.mu.Lock()
		memory.slotEffectsV3[slotEffectKey(reservation.Identity)] = historicalNilOperationPublicationAttempt(reservation, request)
		memory.mu.Unlock()

		attempt, claim, claimed, err := repository.(imageagent.SlotExternalEffectV3Repository).ClaimSlotPublicationV3(context.Background(), request)
		require.NoError(t, err)
		require.False(t, claimed)
		require.Equal(t, int64(1), claim.Fence)
		require.Nil(t, attempt.FinalManifest.Assets[0].Operations)
	})
}

func testGormHistoricalNilOperationReplay(t *testing.T) {
	t.Run("staging", func(t *testing.T) {
		db := newConcurrentSQLite(t)
		repository := NewGormRepository(db)
		reservation := v3Reservation("historical-nil-gorm-staging")
		initializeSlotEffectRun(t, repository, reservation.Identity.RunID)
		row := slotEffectV3RecordFixtureFromReservation(reservation, time.Now().UTC())
		row.Phase = string(imageagent.SlotEffectV3StagingPrepared)
		row.StagingManifestJSON = []byte(historicalNilOperationStagingJSON)
		row.StagingManifestFingerprint = historicalNilOperationStagingFP
		require.NoError(t, db.Create(&row).Error)

		attempt, err := repository.(imageagent.SlotExternalEffectV3Repository).PrepareSlotStagingV3(context.Background(), reservation, v3NilOperationStagingManifest())
		require.NoError(t, err)
		require.Equal(t, historicalNilOperationStagingFP, attempt.StagingManifestFingerprint)
		require.Nil(t, attempt.StagingManifest.Assets[0].Operations)
	})

	t.Run("final", func(t *testing.T) {
		db := newConcurrentSQLite(t)
		repository := NewGormRepository(db)
		reservation := v3Reservation("historical-nil-gorm-final")
		initializeSlotEffectRun(t, repository, reservation.Identity.RunID)
		request := v3NilOperationPublicationRequest(reservation)
		row := slotEffectV3RecordFixtureFromReservation(reservation, time.Now().UTC())
		row.Phase = string(imageagent.SlotEffectV3PublicationClaimed)
		row.PublicationOwner = request.Owner
		row.PublicationFence = 1
		row.PublicationFingerprint = request.PublicationFingerprint
		lease := time.Now().UTC().Add(time.Hour)
		row.PublicationLeaseExpiresAt = &lease
		row.FinalManifestJSON = []byte(historicalNilOperationFinalJSON)
		require.NoError(t, db.Create(&row).Error)

		attempt, claim, claimed, err := repository.(imageagent.SlotExternalEffectV3Repository).ClaimSlotPublicationV3(context.Background(), request)
		require.NoError(t, err)
		require.False(t, claimed)
		require.Equal(t, int64(1), claim.Fence)
		require.Nil(t, attempt.FinalManifest.Assets[0].Operations)
	})
}

func v3NilOperationStagingManifest() imageagent.StagingManifest {
	return imageagent.StagingManifest{Assets: []imageagent.StagedAssetRef{{ObjectKey: "image-agent/staging/tenant-a/run/asset.png", SHA256: v3SHA256, SizeBytes: 42, ContentType: "image/png", Width: 1200, Height: 1200, SourceAssetID: "source-1", ProviderReceiptID: "receipt-1"}}}
}

func v3NilOperationFinalManifest() imageagent.FinalManifest {
	return imageagent.FinalManifest{Assets: []imageagent.PublishedAssetRef{{ObjectKey: "image-agent/final/tenant-a/run/asset.png", SHA256: v3SHA256, SizeBytes: 42, ContentType: "image/png", Width: 1200, Height: 1200, SourceAssetID: "source-1", ProviderReceiptID: "receipt-1"}}}
}

func v3NilOperationPublicationRequest(reservation imageagent.SlotEffectV3Reservation) imageagent.PublicationClaimRequest {
	return imageagent.PublicationClaimRequest{Reservation: reservation, Owner: "worker-a", LeaseDuration: time.Hour, PublicationFingerprint: "historical-nil-publication", FinalManifest: v3NilOperationFinalManifest()}
}

func historicalNilOperationPublicationAttempt(reservation imageagent.SlotEffectV3Reservation, request imageagent.PublicationClaimRequest) imageagent.SlotEffectV3Attempt {
	return imageagent.SlotEffectV3Attempt{Identity: reservation.Identity, IdempotencyKey: reservation.IdempotencyKey, InputFingerprint: reservation.InputFingerprint, Phase: imageagent.SlotEffectV3PublicationClaimed, Publication: imageagent.PublicationClaim{Owner: request.Owner, Fence: 1, LeaseExpiresAt: time.Now().UTC().Add(time.Hour)}, PublicationFingerprint: request.PublicationFingerprint, FinalManifest: v3NilOperationFinalManifest()}
}

func TestSlotEffectV3RejectsIllegalPhaseTransitions(t *testing.T) {
	for _, factory := range newV3ReviewFixtures() {
		t.Run(factory.name, func(t *testing.T) {
			fixture := factory.new(t)
			ctx := context.Background()
			_, _, err := fixture.effects.ReserveSlotProviderV3(ctx, fixture.reservation)
			require.NoError(t, err)

			_, err = fixture.effects.CommitSlotStagedV3(ctx, fixture.reservation, "not-prepared")
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
			_, _, _, err = fixture.effects.ClaimSlotPublicationV3(ctx, imageagent.PublicationClaimRequest{Reservation: fixture.reservation, Owner: "worker-a", LeaseDuration: time.Minute, PublicationFingerprint: "fp", FinalManifest: v3FinalManifest()})
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
			_, err = fixture.effects.BlockSlotEffectV3(ctx, imageagent.SlotEffectV3BlockTransition{Reservation: fixture.reservation, Phase: imageagent.SlotEffectV3StagingUnknown, Code: "slot_staging_outcome_unknown"})
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
		})
	}
}

func TestSlotEffectV3StalePublicationFenceCannotCommit(t *testing.T) {
	for _, factory := range newV3ReviewFixtures() {
		t.Run(factory.name, func(t *testing.T) {
			fixture := factory.new(t)
			ctx := context.Background()
			stageV3Attempt(t, fixture.effects, fixture.reservation)

			request := v3PublicationRequest(fixture.reservation, "worker-a")
			_, first, claimed, err := fixture.effects.ClaimSlotPublicationV3(ctx, request)
			require.NoError(t, err)
			require.True(t, claimed)
			fixture.expireLease(t, fixture.reservation.Identity)
			request.Owner = "worker-b"
			_, second, claimed, err := fixture.effects.ClaimSlotPublicationV3(ctx, request)
			require.NoError(t, err)
			require.True(t, claimed)
			require.Greater(t, second.Fence, first.Fence)

			_, err = fixture.effects.CompleteSlotPublicationV3(ctx, imageagent.PublicationCompletion{Reservation: fixture.reservation, Owner: "worker-a", Fence: first.Fence, PublicationFingerprint: request.PublicationFingerprint, ResultFingerprint: "result-fingerprint-1", Published: v3PublishedResult(fixture.reservation)})
			require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
		})
	}
}

func TestSlotEffectV3IsolatesTenantAndOwner(t *testing.T) {
	for _, factory := range newV3ReviewFixtures() {
		t.Run(factory.name, func(t *testing.T) {
			fixture := factory.new(t)
			ctx := context.Background()
			stageV3Attempt(t, fixture.effects, fixture.reservation)
			request := v3PublicationRequest(fixture.reservation, "worker-a")
			_, claim, claimed, err := fixture.effects.ClaimSlotPublicationV3(ctx, request)
			require.NoError(t, err)
			require.True(t, claimed)

			operations := []struct {
				name   string
				invoke func(imageagent.SlotEffectV3Reservation) error
			}{
				{name: "reserve_provider", invoke: func(reservation imageagent.SlotEffectV3Reservation) error {
					_, _, err := fixture.effects.ReserveSlotProviderV3(ctx, reservation)
					return err
				}},
				{name: "prepare_staging", invoke: func(reservation imageagent.SlotEffectV3Reservation) error {
					_, err := fixture.effects.PrepareSlotStagingV3(ctx, reservation, v3StagingManifest())
					return err
				}},
				{name: "commit_staged", invoke: func(reservation imageagent.SlotEffectV3Reservation) error {
					_, err := fixture.effects.CommitSlotStagedV3(ctx, reservation, "staging-fingerprint")
					return err
				}},
				{name: "claim_publication", invoke: func(reservation imageagent.SlotEffectV3Reservation) error {
					_, _, _, err := fixture.effects.ClaimSlotPublicationV3(ctx, v3PublicationRequest(reservation, "worker-b"))
					return err
				}},
				{name: "renew_publication", invoke: func(reservation imageagent.SlotEffectV3Reservation) error {
					_, err := fixture.effects.RenewSlotPublicationV3(ctx, imageagent.PublicationLeaseRenewal{Identity: reservation.Identity, Owner: claim.Owner, Fence: claim.Fence, LeaseDuration: time.Minute})
					return err
				}},
				{name: "complete_publication", invoke: func(reservation imageagent.SlotEffectV3Reservation) error {
					_, err := fixture.effects.CompleteSlotPublicationV3(ctx, imageagent.PublicationCompletion{Reservation: reservation, Owner: claim.Owner, Fence: claim.Fence, PublicationFingerprint: request.PublicationFingerprint, ResultFingerprint: "result-fingerprint-isolation", Published: v3PublishedResult(reservation)})
					return err
				}},
				{name: "block_effect", invoke: func(reservation imageagent.SlotEffectV3Reservation) error {
					_, err := fixture.effects.BlockSlotEffectV3(ctx, imageagent.SlotEffectV3BlockTransition{Reservation: reservation, Phase: imageagent.SlotEffectV3PublicationUnknown, Code: "slot_publication_outcome_unknown", Owner: claim.Owner, Fence: claim.Fence})
					return err
				}},
				{name: "get_effect", invoke: func(reservation imageagent.SlotEffectV3Reservation) error {
					_, err := fixture.effects.GetSlotExternalEffectV3(ctx, reservation.Identity)
					return err
				}},
			}
			for _, operation := range operations {
				for _, mismatch := range v3MismatchedReservations(fixture.reservation) {
					t.Run(operation.name+"/"+mismatch.name, func(t *testing.T) {
						before, err := fixture.effects.GetSlotExternalEffectV3(ctx, fixture.reservation.Identity)
						require.NoError(t, err)
						err = operation.invoke(mismatch.reservation)
						require.ErrorIs(t, err, imageagent.ErrRunNotFound)
						_, err = fixture.effects.GetSlotExternalEffectV3(ctx, mismatch.reservation.Identity)
						require.ErrorIs(t, err, imageagent.ErrRunNotFound)
						after, err := fixture.effects.GetSlotExternalEffectV3(ctx, fixture.reservation.Identity)
						require.NoError(t, err)
						require.Equal(t, before, after)
					})
				}
			}
		})
	}
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
	published := v3PublishedResult(reservation)
	completion := imageagent.PublicationCompletion{Reservation: reservation, Owner: successor.Owner, Fence: successor.Fence, PublicationFingerprint: successorRequest.PublicationFingerprint, ResultFingerprint: mustV3ResultFingerprint(t, published), Published: published}
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
			attempt, err := fixture.effects.CompleteSlotPublicationV3(ctx, imageagent.PublicationCompletion{Reservation: fixture.reservation, Owner: claim.Owner, Fence: claim.Fence, PublicationFingerprint: request.PublicationFingerprint, ResultFingerprint: mustV3ResultFingerprint(t, published), Published: published})
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

type v3MismatchedReservation struct {
	name        string
	reservation imageagent.SlotEffectV3Reservation
}

func v3MismatchedReservations(reservation imageagent.SlotEffectV3Reservation) []v3MismatchedReservation {
	otherOwner := reservation
	otherOwner.Identity.OwnerUserID = "user-b"
	otherTenant := reservation
	otherTenant.Identity.TenantID = "tenant-b"
	return []v3MismatchedReservation{
		{name: "owner", reservation: otherOwner},
		{name: "tenant", reservation: otherTenant},
	}
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

func slotEffectV3RecordFixtureFromReservation(reservation imageagent.SlotEffectV3Reservation, claimedAt time.Time) slotExternalEffectV3Record {
	status := imageagent.SlotBudgetStatus("")
	if reservation.Quote.Fingerprint != "" {
		status = imageagent.SlotBudgetReserved
	}
	attempt := imageagent.SlotEffectV3Attempt{
		Identity: reservation.Identity, IdempotencyKey: reservation.IdempotencyKey, InputFingerprint: reservation.InputFingerprint,
		Phase: imageagent.SlotEffectV3ProviderClaimed, BudgetStatus: status, Policy: reservation.Policy, Quote: reservation.Quote,
	}
	return slotEffectV3RecordFromProviderDecision(effectpolicy.ProviderReservationDecision{
		AccountingDecision: effectpolicy.AccountingDecision{EffectDecision: effectpolicy.EffectDecision{Attempt: attempt, Changed: true}},
		Acquired:           true,
	}, claimedAt)
}

func v3StagingManifest() imageagent.StagingManifest {
	return imageagent.StagingManifest{Assets: []imageagent.StagedAssetRef{{ObjectKey: "image-agent/staging/tenant-a/run/asset.png", SHA256: v3SHA256, SizeBytes: 42, ContentType: "image/png", Width: 1200, Height: 1200, SourceAssetID: "source-1", Operations: []string{"resize"}, ProviderReceiptID: "receipt-1"}}}
}

func v3FinalManifest() imageagent.FinalManifest {
	return imageagent.FinalManifest{Assets: []imageagent.PublishedAssetRef{{ObjectKey: "image-agent/final/tenant-a/run/asset.png", SHA256: v3SHA256, SizeBytes: 42, ContentType: "image/png", Width: 1200, Height: 1200, SourceAssetID: "source-1", Operations: []string{"resize"}, ProviderReceiptID: "receipt-1"}}}
}

func v3PublicationRequest(reservation imageagent.SlotEffectV3Reservation, owner string) imageagent.PublicationClaimRequest {
	return imageagent.PublicationClaimRequest{Reservation: reservation, Owner: owner, LeaseDuration: time.Minute, PublicationFingerprint: "publication-fingerprint-1", FinalManifest: v3FinalManifest()}
}

func v3PublishedResult(reservation imageagent.SlotEffectV3Reservation) imageagent.SlotEffectV3PublishedResult {
	return imageagent.SlotEffectV3PublishedResult{SlotID: reservation.Identity.SlotID, Attempt: reservation.Identity.Attempt, Candidates: []imageagent.SlotEffectV3AssetCandidate{{AssetID: "candidate-1", SourceAssetID: "source-1", DurableAsset: imageagent.DurableAssetIdentity{ObjectKey: "image-agent/final/tenant-a/run/asset.png", SHA256: v3SHA256}}}}
}

func cloneV3PublishedResultForTest(result imageagent.SlotEffectV3PublishedResult) imageagent.SlotEffectV3PublishedResult {
	result.Candidates = append([]imageagent.SlotEffectV3AssetCandidate(nil), result.Candidates...)
	return result
}

func mustV3ResultFingerprint(t *testing.T, result imageagent.SlotEffectV3PublishedResult) string {
	t.Helper()
	fingerprint, err := imageagent.SlotEffectV3PublishedResultFingerprint(result)
	require.NoError(t, err)
	return fingerprint
}

func v3TwoAssetFinalManifest() imageagent.FinalManifest {
	return imageagent.FinalManifest{Assets: []imageagent.PublishedAssetRef{
		{ObjectKey: "image-agent/final/tenant-a/run/asset-a.png", SHA256: v3SHA256, SizeBytes: 42, ContentType: "image/png", Width: 1200, Height: 1200, SourceAssetID: "source-1", Operations: []string{"resize"}},
		{ObjectKey: "image-agent/final/tenant-a/run/asset-b.png", SHA256: strings.Repeat("b", 64), SizeBytes: 84, ContentType: "image/png", Width: 800, Height: 800, SourceAssetID: "source-2", Operations: []string{"resize"}},
	}}
}

func v3TwoAssetPublishedResult(reservation imageagent.SlotEffectV3Reservation) imageagent.SlotEffectV3PublishedResult {
	final := v3TwoAssetFinalManifest()
	return imageagent.SlotEffectV3PublishedResult{SlotID: reservation.Identity.SlotID, Attempt: reservation.Identity.Attempt, Candidates: []imageagent.SlotEffectV3AssetCandidate{
		{AssetID: "candidate-a", SourceAssetID: final.Assets[0].SourceAssetID, DurableAsset: imageagent.DurableAssetIdentity{ObjectKey: final.Assets[0].ObjectKey, SHA256: final.Assets[0].SHA256}},
		{AssetID: "candidate-b", SourceAssetID: final.Assets[1].SourceAssetID, DurableAsset: imageagent.DurableAssetIdentity{ObjectKey: final.Assets[1].ObjectKey, SHA256: final.Assets[1].SHA256}},
	}}
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
