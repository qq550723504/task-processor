package temporal

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	sdkactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/store"
)

func TestEffectRecoveryWorkflowUsesDeterministicIDAndAttachesDuplicateStart(t *testing.T) {
	_, input := initializedSlotEffectV3Activity(t, "run-v3-recovery-id")

	duplicateKey := input.RunID + ":" + input.Slot.ID
	first := EffectRecoveryWorkflowID(input.Identity, input.PlanRevision, duplicateKey, input.Attempt)
	second := EffectRecoveryWorkflowID(input.Identity, input.PlanRevision, duplicateKey, input.Attempt)
	nextAttempt := EffectRecoveryWorkflowID(input.Identity, input.PlanRevision, duplicateKey, input.Attempt+1)

	require.Equal(t, "image-agent-effect-recovery:tenant-a:user-a:run-v3-recovery-id:1:slot-1:1", first)
	require.Equal(t, first, second, "duplicate starts must target the same deterministic recovery workflow ID")
	require.NotEqual(t, first, nextAttempt)
}

func TestEffectRecoveryWorkflowReusesPersistedBudgetAuthorizationWithoutQuoteOrGenerate(t *testing.T) {
	repository, input, policy := initializedBudgetedV3Activity(t, "run-v3-recovery-budgeted", 1)
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	persistedQuote := budgetActivityQuote("quote-before-recovery")
	reservation := slotEffectReservationV3(slotExecutionInputV3(input))
	reservation.Policy = policy
	reservation.Quote = persistedQuote
	_, claimed, err := effects.ReserveSlotProviderV3(context.Background(), reservation)
	require.NoError(t, err)
	require.True(t, claimed)
	_, err = effects.SettleSlotProviderV3(context.Background(), reservation, imageagent.SlotUsageReceipt{
		Actual: persistedQuote.Maximum, CostBasis: imageagent.UsageCostReservedUpperBound,
	})
	require.NoError(t, err)
	manifest := v3StagingManifest(input, tinyPNGBytes(t))
	prepared, err := effects.PrepareSlotStagingV3(context.Background(), reservation, manifest)
	require.NoError(t, err)
	_, err = effects.CommitSlotStagedV3(context.Background(), reservation, prepared.StagingManifestFingerprint)
	require.NoError(t, err)
	executor := &budgetedRecordingExecutor{
		recordingStagedExecutor: &recordingStagedExecutor{},
		quote:                   budgetActivityQuote("quote-after-recovery"),
	}
	activities := newBudgetV3Activities(t, repository, executor, &recordingArtifactStore{})
	env := newEffectRecoveryWorkflowEnv(t, activities)

	env.ExecuteWorkflow(ImageAgentEffectRecoveryWorkflow, effectRecoveryWorkflowInput(input))

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result EffectRecoveryResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, EffectRecoveryOutcomePublished, result.Outcome)
	require.Equal(t, imageagent.SlotEffectV3PublicationComplete, result.EffectPhase)
	require.Zero(t, executor.QuoteCalls(), "recovery must reuse the persisted quote and must not ask for a new one")
	require.Zero(t, executor.GenerateCalls(), "recovery must not dispatch the provider for a persisted staged/publication effect")
	stored, err := effects.GetSlotExternalEffectV3(context.Background(), reservation.Identity)
	require.NoError(t, err)
	require.Equal(t, imageagent.SlotEffectV3PublicationComplete, stored.Phase)
	require.Equal(t, persistedQuote.Fingerprint, stored.Quote.Fingerprint)
}

func TestEffectRecoveryWorkflowNeverRedispatchesReleasedBudgetedProviderClaim(t *testing.T) {
	repository, input, policy := initializedBudgetedV3Activity(t, "run-v3-recovery-budget-released", 1)
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	persistedQuote := budgetActivityQuote("quote-before-recovery")
	reservation := slotEffectReservationV3(slotExecutionInputV3(input))
	reservation.Policy = policy
	reservation.Quote = persistedQuote
	_, claimed, err := effects.ReserveSlotProviderV3(context.Background(), reservation)
	require.NoError(t, err)
	require.True(t, claimed)
	_, err = effects.ReleaseSlotProviderBudgetV3(context.Background(), reservation)
	require.NoError(t, err)
	executor := &budgetedRecordingExecutor{
		recordingStagedExecutor: &recordingStagedExecutor{},
		quote:                   persistedQuote,
	}
	activities := newBudgetV3Activities(t, repository, executor, &recordingArtifactStore{})
	env := newEffectRecoveryWorkflowEnv(t, activities)

	env.ExecuteWorkflow(ImageAgentEffectRecoveryWorkflow, effectRecoveryWorkflowInput(input))

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result EffectRecoveryResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, EffectRecoveryOutcomeRecoveryBlocked, result.Outcome)
	require.Equal(t, imageagent.SlotEffectV3RecoveryBlocked, result.EffectPhase)
	require.Zero(t, executor.QuoteCalls(), "recovery must not replace the persisted authorization")
	require.Zero(t, executor.GenerateCalls(), "recovery must never redispatch a provider claim")
	stored, err := effects.GetSlotExternalEffectV3(context.Background(), reservation.Identity)
	require.NoError(t, err)
	require.Equal(t, imageagent.SlotEffectV3RecoveryBlocked, stored.Phase)
	require.Equal(t, imageagent.SlotRecoveryBlockedCode, stored.BlockedCode)
}

func TestEffectRecoveryWorkflowPersistsRecoveryBlockedForMissingEffectWithoutProviderCall(t *testing.T) {
	repository, input := initializedSlotEffectV3Activity(t, "run-v3-recovery-missing-effect")
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	executor := &recordingStagedExecutor{}
	activities := newV3Activities(t, repository, effects, executor, &recordingArtifactStore{})
	env := newEffectRecoveryWorkflowEnv(t, activities)

	env.ExecuteWorkflow(ImageAgentEffectRecoveryWorkflow, effectRecoveryWorkflowInput(input))

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result EffectRecoveryResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, EffectRecoveryOutcomeRecoveryBlocked, result.Outcome)
	require.Equal(t, imageagent.SlotEffectV3RecoveryBlocked, result.EffectPhase)
	require.Zero(t, executor.GenerateCalls())
	stored, err := effects.GetSlotExternalEffectV3(context.Background(), slotEffectReservationV3(slotExecutionInputV3(input)).Identity)
	require.NoError(t, err)
	require.Equal(t, imageagent.SlotEffectV3RecoveryBlocked, stored.Phase)
	require.Equal(t, imageagent.SlotRecoveryBlockedCode, stored.BlockedCode)
}

func TestEffectRecoveryWorkflowScopesToExactEffectIdentityWithoutProviderCall(t *testing.T) {
	repository, firstAttempt := initializedSlotEffectV3Activity(t, "run-v3-recovery-exact-identity")
	secondAttempt := firstAttempt
	secondAttempt.Attempt = 2
	secondAttempt.IdempotencyKey = slotAttemptKey(secondAttempt.PlanRevision, secondAttempt.Slot, secondAttempt.Attempt)

	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	firstReservation := slotEffectReservationV3(slotExecutionInputV3(firstAttempt))
	_, claimed, err := effects.ReserveSlotProviderV3(context.Background(), firstReservation)
	require.NoError(t, err)
	require.True(t, claimed)
	_, err = effects.BlockSlotEffectV3(context.Background(), imageagent.SlotEffectV3BlockTransition{
		Reservation: firstReservation,
		Phase:       imageagent.SlotEffectV3ProviderUnknown,
		Code:        imageagent.SlotProviderOutcomeUnknownCode,
	})
	require.NoError(t, err)

	secondManifest := v3StagingManifest(secondAttempt, tinyPNGBytes(t))
	seedV3ArtifactStaged(t, effects, secondAttempt, secondManifest)
	executor := &recordingStagedExecutor{}
	artifacts := &recordingArtifactStore{}
	activities := newV3Activities(t, repository, effects, executor, artifacts)
	env := newEffectRecoveryWorkflowEnv(t, activities)

	env.ExecuteWorkflow(ImageAgentEffectRecoveryWorkflow, effectRecoveryWorkflowInput(secondAttempt))

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result EffectRecoveryResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, EffectRecoveryOutcomePublished, result.Outcome)
	require.Equal(t, imageagent.SlotEffectV3PublicationComplete, result.EffectPhase)
	require.Zero(t, executor.GenerateCalls(), "recovery must not regenerate any attempt while reconciling a persisted effect")
	require.Equal(t, 1, executor.BuildCalls())
	require.Equal(t, 1, artifacts.FinalizeCalls())

	firstStored, err := effects.GetSlotExternalEffectV3(context.Background(), firstReservation.Identity)
	require.NoError(t, err)
	require.Equal(t, 1, firstStored.Identity.Attempt)
	require.Equal(t, imageagent.SlotEffectV3ProviderUnknown, firstStored.Phase)
	require.Equal(t, imageagent.SlotProviderOutcomeUnknownCode, firstStored.BlockedCode)

	secondReservation := slotEffectReservationV3(slotExecutionInputV3(secondAttempt))
	secondStored, err := effects.GetSlotExternalEffectV3(context.Background(), secondReservation.Identity)
	require.NoError(t, err)
	require.Equal(t, 2, secondStored.Identity.Attempt)
	require.Equal(t, imageagent.SlotEffectV3PublicationComplete, secondStored.Phase)
	require.Equal(t, 2, secondStored.Published.Attempt)
}

func TestEffectRecoveryWorkflowReturnsPersistedTerminalAndUnknownPhasesWithoutProviderCall(t *testing.T) {
	tests := []struct {
		name        string
		slug        string
		setup       func(*testing.T, imageagent.SlotExternalEffectV3Repository, imageagent.Repository, ExecuteSlotV3ActivityInput)
		wantOutcome EffectRecoveryOutcome
		wantPhase   imageagent.SlotEffectV3Phase
		wantCode    string
	}{
		{
			name: "publication complete",
			slug: "publication-complete",
			setup: func(t *testing.T, effects imageagent.SlotExternalEffectV3Repository, repository imageagent.Repository, input ExecuteSlotV3ActivityInput) {
				t.Helper()
				seedV3ArtifactStaged(t, effects, input, v3StagingManifest(input, tinyPNGBytes(t)))
				_, err := newV3Activities(t, repository, effects, &recordingStagedExecutor{}, &recordingArtifactStore{}).ExecuteSlotV3(context.Background(), input)
				require.NoError(t, err)
			},
			wantOutcome: EffectRecoveryOutcomePublished,
			wantPhase:   imageagent.SlotEffectV3PublicationComplete,
		},
		{
			name: "provider unknown",
			slug: "provider-unknown",
			setup: func(t *testing.T, effects imageagent.SlotExternalEffectV3Repository, _ imageagent.Repository, input ExecuteSlotV3ActivityInput) {
				t.Helper()
				reservation := slotEffectReservationV3(slotExecutionInputV3(input))
				_, claimed, err := effects.ReserveSlotProviderV3(context.Background(), reservation)
				require.NoError(t, err)
				require.True(t, claimed)
				_, err = effects.BlockSlotEffectV3(context.Background(), imageagent.SlotEffectV3BlockTransition{
					Reservation: reservation,
					Phase:       imageagent.SlotEffectV3ProviderUnknown,
					Code:        imageagent.SlotProviderOutcomeUnknownCode,
				})
				require.NoError(t, err)
			},
			wantOutcome: EffectRecoveryOutcomeProviderUnknown,
			wantPhase:   imageagent.SlotEffectV3ProviderUnknown,
			wantCode:    imageagent.SlotProviderOutcomeUnknownCode,
		},
		{
			name: "staging unknown",
			slug: "staging-unknown",
			setup: func(t *testing.T, effects imageagent.SlotExternalEffectV3Repository, _ imageagent.Repository, input ExecuteSlotV3ActivityInput) {
				t.Helper()
				reservation := slotEffectReservationV3(slotExecutionInputV3(input))
				seedV3StagingPrepared(t, effects, input, v3StagingManifest(input, tinyPNGBytes(t)))
				_, err := effects.BlockSlotEffectV3(context.Background(), imageagent.SlotEffectV3BlockTransition{
					Reservation: reservation,
					Phase:       imageagent.SlotEffectV3StagingUnknown,
					Code:        imageagent.SlotStagingOutcomeUnknownCode,
				})
				require.NoError(t, err)
			},
			wantOutcome: EffectRecoveryOutcomeStagingUnknown,
			wantPhase:   imageagent.SlotEffectV3StagingUnknown,
			wantCode:    imageagent.SlotStagingOutcomeUnknownCode,
		},
		{
			name: "publication unknown",
			slug: "publication-unknown",
			setup: func(t *testing.T, effects imageagent.SlotExternalEffectV3Repository, _ imageagent.Repository, input ExecuteSlotV3ActivityInput) {
				t.Helper()
				execution := slotExecutionInputV3(input)
				reservation := slotEffectReservationV3(slotExecutionInputV3(input))
				manifest := v3StagingManifest(input, tinyPNGBytes(t))
				seedV3ArtifactStaged(t, effects, input, manifest)
				finalManifest, err := expectedFinalManifestV3(execution, manifest)
				require.NoError(t, err)
				publicationFingerprint, err := imageagent.FinalManifestFingerprint(finalManifest)
				require.NoError(t, err)
				stored, claim, claimed, err := effects.ClaimSlotPublicationV3(context.Background(), imageagent.PublicationClaimRequest{
					Reservation:            reservation,
					Owner:                  "workflow-run/activity/1",
					LeaseDuration:          time.Minute,
					PublicationFingerprint: publicationFingerprint,
					FinalManifest:          finalManifest,
				})
				require.NoError(t, err)
				require.True(t, claimed)
				_, err = effects.BlockSlotEffectV3(context.Background(), imageagent.SlotEffectV3BlockTransition{
					Reservation: reservation,
					Phase:       imageagent.SlotEffectV3PublicationUnknown,
					Code:        imageagent.SlotPublicationOutcomeUnknownCode,
					Owner:       claim.Owner,
					Fence:       claim.Fence,
				})
				require.NoError(t, err)
				require.Equal(t, imageagent.SlotEffectV3PublicationClaimed, stored.Phase)
			},
			wantOutcome: EffectRecoveryOutcomePublicationUnknown,
			wantPhase:   imageagent.SlotEffectV3PublicationUnknown,
			wantCode:    imageagent.SlotPublicationOutcomeUnknownCode,
		},
		{
			name: "recovery blocked",
			slug: "recovery-blocked",
			setup: func(t *testing.T, effects imageagent.SlotExternalEffectV3Repository, _ imageagent.Repository, input ExecuteSlotV3ActivityInput) {
				t.Helper()
				reservation := slotEffectReservationV3(slotExecutionInputV3(input))
				_, claimed, err := effects.ReserveSlotProviderV3(context.Background(), reservation)
				require.NoError(t, err)
				require.True(t, claimed)
				_, err = effects.BlockSlotEffectV3(context.Background(), imageagent.SlotEffectV3BlockTransition{
					Reservation: reservation,
					Phase:       imageagent.SlotEffectV3RecoveryBlocked,
					Code:        imageagent.SlotRecoveryBlockedCode,
				})
				require.NoError(t, err)
			},
			wantOutcome: EffectRecoveryOutcomeRecoveryBlocked,
			wantPhase:   imageagent.SlotEffectV3RecoveryBlocked,
			wantCode:    imageagent.SlotRecoveryBlockedCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository, input := initializedSlotEffectV3Activity(t, "run-v3-recovery-persisted-"+tt.slug)
			effects := repository.(imageagent.SlotExternalEffectV3Repository)
			tt.setup(t, effects, repository, input)
			executor := &recordingStagedExecutor{}
			artifacts := &recordingArtifactStore{}
			activities := newV3Activities(t, repository, effects, executor, artifacts)
			env := newEffectRecoveryWorkflowEnv(t, activities)

			env.ExecuteWorkflow(ImageAgentEffectRecoveryWorkflow, effectRecoveryWorkflowInput(input))

			require.True(t, env.IsWorkflowCompleted())
			require.NoError(t, env.GetWorkflowError())
			var result EffectRecoveryResult
			require.NoError(t, env.GetWorkflowResult(&result))
			require.Equal(t, tt.wantOutcome, result.Outcome)
			require.Equal(t, tt.wantPhase, result.EffectPhase)
			require.Equal(t, tt.wantCode, result.BlockedCode)
			require.Zero(t, executor.GenerateCalls(), "recovery must not invoke the provider for persisted terminal or unknown phases")
			require.Zero(t, executor.BuildCalls(), "persisted recovery phases should not rebuild slot output")
			require.Zero(t, artifacts.FinalizeCalls(), "persisted recovery phases should not finalize staged artifacts again")

			stored, err := effects.GetSlotExternalEffectV3(context.Background(), slotEffectReservationV3(slotExecutionInputV3(input)).Identity)
			require.NoError(t, err)
			require.Equal(t, tt.wantPhase, stored.Phase)
			require.Equal(t, tt.wantCode, stored.BlockedCode)
		})
	}
}

func TestEffectRecoveryWorkflowPersistsRecoveryBlockedAfterBoundedExhaustion(t *testing.T) {
	var clockMu sync.Mutex
	clock := time.Date(2026, time.August, 29, 1, 0, 0, 0, time.UTC)
	repository := store.NewMemoryRepositoryWithClock(func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		return clock
	})
	input := initializeEffectRecoveryV3Activity(t, repository, "run-v3-recovery-blocked")
	baseEffects := repository.(imageagent.SlotExternalEffectV3Repository)
	reservation := slotEffectReservationV3(slotExecutionInputV3(input))
	manifest := v3StagingManifest(input, tinyPNGBytes(t))
	seedV3ArtifactStaged(t, baseEffects, input, manifest)
	effects := &publicationRenewalFailingV3Repository{
		SlotExternalEffectV3Repository: baseEffects,
		renewErr:                       errors.New("publication lease renewal unavailable"),
		onRenew: func() {
			clockMu.Lock()
			clock = clock.Add(2 * time.Second)
			clockMu.Unlock()
		},
	}
	activities, err := NewActivities(ActivityDependencies{
		Repository: repository, SlotEffects: repository.(imageagent.SlotExternalEffectRepository), SlotExecutor: &recordingStagedExecutor{},
		SlotEffectsV3: effects, StagedSlotExecutor: &recordingStagedExecutor{}, ArtifactStore: &recordingArtifactStore{},
		Publisher: &identityCheckingPublisher{t: t}, PublisherV3: &identityCheckingPublisher{t: t},
		PublicationOwner:         func(context.Context) (string, error) { return "workflow-run/activity/1", nil },
		PublicationLeaseDuration: 10 * time.Millisecond,
	})
	require.NoError(t, err)
	env := newEffectRecoveryWorkflowEnv(t, activities)

	env.ExecuteWorkflow(ImageAgentEffectRecoveryWorkflow, effectRecoveryWorkflowInput(input))

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result EffectRecoveryResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, EffectRecoveryOutcomeRecoveryBlocked, result.Outcome)
	require.Equal(t, imageagent.SlotRecoveryBlockedCode, result.BlockedCode)
	require.Equal(t, imageagent.SlotEffectV3RecoveryBlocked, result.EffectPhase)
	require.Equal(t, 3, effects.RenewCalls(), "publication recovery must stop after bounded workflow retries")
	stored, err := baseEffects.GetSlotExternalEffectV3(context.Background(), reservation.Identity)
	require.NoError(t, err)
	require.Equal(t, imageagent.SlotEffectV3RecoveryBlocked, stored.Phase)
	require.Equal(t, imageagent.SlotRecoveryBlockedCode, stored.BlockedCode)
}

func TestEffectRecoveryWorkflowDoesNotReportRecoveryBlockedWhenPersistenceFails(t *testing.T) {
	_, input := initializedSlotEffectV3Activity(t, "run-v3-recovery-persist-fails")
	var recoverAttempts, persistAttempts int
	env := newEffectRecoveryWorkflowEnvWithActivities(t,
		func(context.Context, EffectRecoveryWorkflowInput) (EffectRecoveryResult, error) {
			recoverAttempts++
			return EffectRecoveryResult{}, errors.New("durable repository unavailable")
		},
		func(context.Context, EffectRecoveryWorkflowInput) (EffectRecoveryResult, error) {
			persistAttempts++
			return EffectRecoveryResult{}, errors.New("durable recovery block unavailable")
		},
	)

	env.ExecuteWorkflow(ImageAgentEffectRecoveryWorkflow, effectRecoveryWorkflowInput(input))

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError(), "the workflow must not claim a durable recovery result when the persistence activity exhausted")
	require.Equal(t, 3, recoverAttempts)
	require.Equal(t, 3, persistAttempts)
}

func newEffectRecoveryWorkflowEnv(t *testing.T, activities *Activities) *testsuite.TestWorkflowEnvironment {
	t.Helper()
	return newEffectRecoveryWorkflowEnvWithActivities(t, activities.RecoverEffectV3, activities.PersistRecoveryBlockedEffectV3)
}

func newEffectRecoveryWorkflowEnvWithActivities(t *testing.T, recover func(context.Context, EffectRecoveryWorkflowInput) (EffectRecoveryResult, error), persist func(context.Context, EffectRecoveryWorkflowInput) (EffectRecoveryResult, error)) *testsuite.TestWorkflowEnvironment {
	t.Helper()
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(ImageAgentEffectRecoveryWorkflow)
	env.RegisterActivityWithOptions(recover, sdkactivity.RegisterOptions{Name: activityRecoverEffectV3})
	env.RegisterActivityWithOptions(persist, sdkactivity.RegisterOptions{Name: activityPersistRecoveryBlockedV3})
	return env
}

func effectRecoveryWorkflowInput(input ExecuteSlotV3ActivityInput) EffectRecoveryWorkflowInput {
	return EffectRecoveryWorkflowInput{
		RunID:        input.RunID,
		Identity:     input.Identity,
		PlanRevision: input.PlanRevision,
		Slot:         input.Slot,
		Attempt:      input.Attempt,
		AssetCatalog: input.AssetCatalog,
	}
}

func initializeEffectRecoveryV3Activity(t *testing.T, repository imageagent.Repository, runID string) ExecuteSlotV3ActivityInput {
	t.Helper()
	run := imageagent.Run{ID: runID, BusinessTaskID: "task-" + runID, TenantID: "tenant-a", UserID: "user-a", Mode: imageagent.RunModeManual, IdempotencyKey: "run-key-" + runID, Status: imageagent.RunStatusPlanning, CurrentNode: "plan", Version: 1, ActivePlanRevision: 1, MaxConcurrentSlots: 1}
	plan := imageagent.Plan{Revision: 1, IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1"}, CreatedBy: "user-a", Slots: []imageagent.Slot{{ID: "slot-1", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-key-1"}}}
	catalog, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/source.png"}}})
	require.NoError(t, err)
	_, err = repository.InitializeRun(context.Background(), imageagent.ProjectionInitialization{Scope: imageagent.ScopeForRun(run), Run: run, Plan: plan, Catalog: catalog, Snapshot: imageagent.RunProjection{Run: run, Plan: plan}, CommitID: "start:" + runID, EventType: "run.initialized", EventPayload: []byte(`{}`)})
	require.NoError(t, err)
	return ExecuteSlotV3ActivityInput{RunID: runID, Identity: imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}, PlanRevision: 1, Slot: plan.Slots[0], Attempt: 1, IdempotencyKey: "slot-key-1:plan:1:attempt:1", AssetCatalog: catalog}
}

func seedV3PublicationClaimed(t *testing.T, effects imageagent.SlotExternalEffectV3Repository, input ExecuteSlotV3ActivityInput, manifest imageagent.StagingManifest, owner string) {
	t.Helper()
	seedV3ArtifactStaged(t, effects, input, manifest)
	execution := slotExecutionInputV3(input)
	reservation := slotEffectReservationV3(execution)
	finalManifest, err := expectedFinalManifestV3(execution, manifest)
	require.NoError(t, err)
	publicationFingerprint, err := imageagent.FinalManifestFingerprint(finalManifest)
	require.NoError(t, err)
	_, _, claimed, err := effects.ClaimSlotPublicationV3(context.Background(), imageagent.PublicationClaimRequest{
		Reservation:            reservation,
		Owner:                  owner,
		LeaseDuration:          time.Minute,
		PublicationFingerprint: publicationFingerprint,
		FinalManifest:          finalManifest,
	})
	require.NoError(t, err)
	require.True(t, claimed)
}

type publicationRenewalFailingV3Repository struct {
	imageagent.SlotExternalEffectV3Repository
	mu       sync.Mutex
	renewErr error
	onRenew  func()
	calls    int
}

func (r *publicationRenewalFailingV3Repository) RenewSlotPublicationV3(context.Context, imageagent.PublicationLeaseRenewal) (imageagent.PublicationClaim, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.onRenew != nil {
		r.onRenew()
	}
	return imageagent.PublicationClaim{}, r.renewErr
}

func (r *publicationRenewalFailingV3Repository) RenewCalls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}
