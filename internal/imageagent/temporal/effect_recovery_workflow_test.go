package temporal

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

func TestEffectRecoveryWorkflowReconcilesParentProjectionAfterPublication(t *testing.T) {
	repository, inputs := initializedBlockedEffectRecoveryProjection(t, "run-v3-recovery-parent-published", 2)
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	seedV3ArtifactStaged(t, effects, inputs[0], v3StagingManifest(inputs[0], tinyPNGBytes(t)))
	executor := &recordingStagedExecutor{}
	activities := newV3Activities(t, repository, effects, executor, &recordingArtifactStore{})
	env := newEffectRecoveryWorkflowEnv(t, activities)

	env.ExecuteWorkflow(ImageAgentEffectRecoveryWorkflow, effectRecoveryWorkflowInput(inputs[0]))

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	_, err := activities.ReconcileEffectRecoveryV3(context.Background(), effectRecoveryWorkflowInput(inputs[0]))
	require.NoError(t, err, "replaying publication reconciliation must return the already-applied projection")
	projection, err := repository.GetProjection(context.Background(), imageagent.RunScope{
		TenantID: "tenant-a", OwnerUserID: "user-a", RunID: inputs[0].RunID,
	})
	require.NoError(t, err)
	require.Equal(t, imageagent.RunStatusBlocked, projection.Run.Status)
	require.Equal(t, []imageagent.RecoverableEffect{{
		SlotID: inputs[1].Slot.ID, Attempt: inputs[1].Attempt, Code: recoveryRequestedBlockCode,
	}}, projection.RecoverableEffects, "publication must remove only the matching recovery owner")
	require.NotNil(t, projection.Run.Block)
	require.Equal(t, inputs[1].Slot.ID, projection.Run.Block.SlotID)
	require.Equal(t, recoveryRequestedBlockCode, projection.Run.Block.Code)
	require.Equal(t, imageagent.SlotStatusAccepted, projection.Slots[0].Slot.Status)
	require.Equal(t, inputs[0].Attempt, projection.Slots[0].Attempt)
	require.Empty(t, projection.Slots[0].ErrorCode)
	require.Len(t, projection.Slots[0].Candidates, 1)
	require.NotEmpty(t, projection.Slots[0].Candidates[0].DurableAsset.ObjectKey)
	require.Equal(t, imageagent.SlotStatusBlocked, projection.Slots[1].Slot.Status)
	require.Equal(t, recoveryRequestedBlockCode, projection.Slots[1].ErrorCode)
	require.Zero(t, executor.GenerateCalls(), "parent reconciliation must not redispatch the provider")
}

func TestEffectRecoveryWorkflowReconcilesUnknownPhaseWithoutClearingOwner(t *testing.T) {
	for _, test := range []struct {
		name  string
		phase imageagent.SlotEffectV3Phase
		code  string
	}{
		{name: "provider unknown", phase: imageagent.SlotEffectV3ProviderUnknown, code: imageagent.SlotProviderOutcomeUnknownCode},
		{name: "recovery blocked", phase: imageagent.SlotEffectV3RecoveryBlocked, code: imageagent.SlotRecoveryBlockedCode},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository, inputs := initializedBlockedEffectRecoveryProjection(t, "run-v3-recovery-parent-"+strings.ReplaceAll(test.name, " ", "-"), 1)
			effects := repository.(imageagent.SlotExternalEffectV3Repository)
			reservation := slotEffectReservationV3(slotExecutionInputV3(inputs[0]))
			_, claimed, err := effects.ReserveSlotProviderV3(context.Background(), reservation)
			require.NoError(t, err)
			require.True(t, claimed)
			_, err = effects.BlockSlotEffectV3(context.Background(), imageagent.SlotEffectV3BlockTransition{
				Reservation: reservation, Phase: test.phase, Code: test.code,
			})
			require.NoError(t, err)
			executor := &recordingStagedExecutor{}
			activities := newV3Activities(t, repository, effects, executor, &recordingArtifactStore{})
			env := newEffectRecoveryWorkflowEnv(t, activities)

			env.ExecuteWorkflow(ImageAgentEffectRecoveryWorkflow, effectRecoveryWorkflowInput(inputs[0]))

			require.True(t, env.IsWorkflowCompleted())
			require.NoError(t, env.GetWorkflowError())
			_, err = activities.ReconcileEffectRecoveryV3(context.Background(), effectRecoveryWorkflowInput(inputs[0]))
			require.NoError(t, err, "replaying blocked reconciliation must return the already-applied projection")
			projection, err := repository.GetProjection(context.Background(), imageagent.RunScope{
				TenantID: "tenant-a", OwnerUserID: "user-a", RunID: inputs[0].RunID,
			})
			require.NoError(t, err)
			require.Equal(t, imageagent.RunStatusBlocked, projection.Run.Status)
			require.Equal(t, []imageagent.RecoverableEffect{{
				SlotID: inputs[0].Slot.ID, Attempt: inputs[0].Attempt, Code: test.code,
			}}, projection.RecoverableEffects)
			require.NotNil(t, projection.Run.Block)
			require.Equal(t, inputs[0].Slot.ID, projection.Run.Block.SlotID)
			require.Equal(t, test.code, projection.Run.Block.Code)
			require.Equal(t, imageagent.SlotStatusBlocked, projection.Slots[0].Slot.Status)
			require.Equal(t, test.code, projection.Slots[0].ErrorCode)
			require.Zero(t, executor.GenerateCalls(), "unknown recovery reconciliation must not call the provider")
		})
	}
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
	env := newEffectRecoveryWorkflowEnvWithoutParentProjection(t, activities)

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
	env := newEffectRecoveryWorkflowEnvWithoutParentProjection(t, activities)

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
	env := newEffectRecoveryWorkflowEnvWithoutParentProjection(t, activities)

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
	env := newEffectRecoveryWorkflowEnvWithoutParentProjection(t, activities)

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
			env := newEffectRecoveryWorkflowEnvWithoutParentProjection(t, activities)

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
	env := newEffectRecoveryWorkflowEnvWithoutParentProjection(t, activities)

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
		func(context.Context, EffectRecoveryWorkflowInput) (EffectRecoveryResult, error) {
			return EffectRecoveryResult{}, errors.New("parent reconciliation must not run")
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
	return newEffectRecoveryWorkflowEnvWithActivities(t, activities.RecoverEffectV3, activities.PersistRecoveryBlockedEffectV3, activities.ReconcileEffectRecoveryV3)
}

func newEffectRecoveryWorkflowEnvWithoutParentProjection(t *testing.T, activities *Activities) *testsuite.TestWorkflowEnvironment {
	t.Helper()
	return newEffectRecoveryWorkflowEnvWithActivities(t, activities.RecoverEffectV3, activities.PersistRecoveryBlockedEffectV3, activities.RecoverEffectV3)
}

func newEffectRecoveryWorkflowEnvWithActivities(t *testing.T, recover func(context.Context, EffectRecoveryWorkflowInput) (EffectRecoveryResult, error), persist func(context.Context, EffectRecoveryWorkflowInput) (EffectRecoveryResult, error), reconcile func(context.Context, EffectRecoveryWorkflowInput) (EffectRecoveryResult, error)) *testsuite.TestWorkflowEnvironment {
	t.Helper()
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(ImageAgentEffectRecoveryWorkflow)
	env.RegisterActivityWithOptions(recover, sdkactivity.RegisterOptions{Name: activityRecoverEffectV3})
	env.RegisterActivityWithOptions(persist, sdkactivity.RegisterOptions{Name: activityPersistRecoveryBlockedV3})
	env.RegisterActivityWithOptions(reconcile, sdkactivity.RegisterOptions{Name: activityReconcileEffectRecoveryV3})
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

func initializedBlockedEffectRecoveryProjection(t *testing.T, runID string, slotCount int) (imageagent.Repository, []ExecuteSlotV3ActivityInput) {
	t.Helper()
	require.Positive(t, slotCount)
	repository := store.NewMemoryRepository()
	slots := make([]imageagent.Slot, 0, slotCount)
	for index := 0; index < slotCount; index++ {
		role := imageagent.SlotRoleScene
		if index == 0 {
			role = imageagent.SlotRoleMain
		}
		slots = append(slots, imageagent.Slot{
			ID: fmt.Sprintf("slot-%d", index+1), Role: role,
			SourceAssetIDs: []string{"source-1"}, IdempotencyKey: fmt.Sprintf("slot-key-%d", index+1),
		})
	}
	run := imageagent.Run{
		ID: runID, BusinessTaskID: "task-" + runID, TenantID: "tenant-a", UserID: "user-a",
		Mode: imageagent.RunModeManual, IdempotencyKey: "run-key-" + runID, Status: imageagent.RunStatusPlanning,
		CurrentNode: "plan", Version: 1, ActivePlanRevision: 1, MaxConcurrentSlots: 1,
	}
	plan := imageagent.Plan{
		Revision: 1, IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1"},
		CreatedBy: "user-a", Slots: slots,
	}
	catalog, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{
		ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/source.png",
	}}})
	require.NoError(t, err)
	_, err = repository.InitializeRun(context.Background(), imageagent.ProjectionInitialization{
		Scope: imageagent.ScopeForRun(run), Run: run, Plan: plan, Catalog: catalog,
		Snapshot: imageagent.RunProjection{Run: run, Plan: plan}, CommitID: "start:" + runID,
		EventType: "run.initialized", EventPayload: []byte(`{}`),
	})
	require.NoError(t, err)
	executor := &recordingStagedExecutor{}
	activities := newV3Activities(t, repository, repository.(imageagent.SlotExternalEffectV3Repository), executor, &recordingArtifactStore{})
	identity := imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}
	inputs := make([]ExecuteSlotV3ActivityInput, 0, slotCount)
	owners := make([]imageagent.RecoverableEffect, 0, slotCount)
	for _, slot := range slots {
		input := ExecuteSlotV3ActivityInput{
			RunID: runID, Identity: identity, PlanRevision: plan.Revision, Slot: slot, Attempt: 1,
			IdempotencyKey: slotAttemptKey(plan.Revision, slot, 1), AssetCatalog: catalog,
		}
		require.NoError(t, activities.PersistSlotResultV3(context.Background(), PersistSlotResultV3ActivityInput{
			RunID: runID, Identity: identity, PlanRevision: plan.Revision,
			Result: SlotWorkflowV3Result{
				Published: imageagent.SlotEffectV3PublishedResult{SlotID: slot.ID, Attempt: 1},
				Status:    imageagent.SlotStatusBlocked, ErrorCode: recoveryRequestedBlockCode,
			},
			AttemptKey: input.IdempotencyKey,
		}))
		inputs = append(inputs, input)
		owners = append(owners, imageagent.RecoverableEffect{SlotID: slot.ID, Attempt: 1, Code: recoveryRequestedBlockCode})
	}
	current, err := repository.GetProjection(context.Background(), imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-a", RunID: runID})
	require.NoError(t, err)
	require.NoError(t, activities.PersistRunState(context.Background(), PersistRunStateActivityInput{
		RunID: runID, Identity: identity, PlanRevision: plan.Revision,
		Projection: WorkflowResult{
			Status: imageagent.RunStatusBlocked,
			Block:  &imageagent.Block{Code: recoveryRequestedBlockCode, Message: recoveryRequestedBlockCode, SlotID: slots[0].ID},
			Plan:   plan, Slots: current.Slots, RecoverableEffects: owners,
		},
		CurrentNode: "retry_slot", CommitID: "test:recovery-handoff:" + runID,
	}))
	return repository, inputs
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
