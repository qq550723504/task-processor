package temporal

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	sdkactivity "go.temporal.io/sdk/activity"
	sdkconverter "go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
	"task-processor/internal/imageagent"
)

func TestImageSetTemporalStartRequiresOriginalAdmissionAndPreservesItsDeadline(t *testing.T) {
	_, repo, input := imageSetPersistenceFixture(t)
	current, err := repo.GetProjection(context.Background(), imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID})
	require.NoError(t, err)
	raw := &recordingSDKClient{}
	start := imageagent.WorkflowStart{Run: current.Run, Plan: current.Plan, Identity: input.Identity, AssetCatalog: current.AssetCatalog, MaxConcurrentSlots: 1}
	require.NoError(t, NewOrganizationClient(raw).StartManual(context.Background(), start))
	require.Equal(t, current.Run.ImageAdmission.AdmittedAt, raw.workflowInput.StartedAt)
	require.Equal(t, current.Run.ImageAdmission.Deadline, raw.workflowInput.DeadlineAt)
	require.Nil(t, raw.workflowInput.ImagePolicyContext)
	for _, kind := range []string{"missing_admission", "changed_time", "changed_budget", "changed_member"} {
		t.Run(kind, func(t *testing.T) {
			bad := start
			switch kind {
			case "missing_admission":
				bad.Run.ImageAdmission = nil
			case "changed_time":
				bad.Run.StartedAt = bad.Run.StartedAt.Add(1)
			case "changed_budget":
				bad.Run.Budget.MaxImages++
			case "changed_member":
				bad.Run.MemberID = "other"
			}
			rejected := &recordingSDKClient{}
			require.Error(t, NewOrganizationClient(rejected).StartManual(context.Background(), bad))
			require.Empty(t, rejected.workflowName)
		})
	}
}

func TestSetExecutionRejectsProjectionWithNoWholeRunAdmission(t *testing.T) {
	a, repo, input := imageSetPersistenceFixture(t)
	current, err := repo.GetProjection(context.Background(), imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID})
	require.NoError(t, err)
	current.Run.ImageAdmission = nil
	a.repository = setPlanRepository{projection: current}
	require.ErrorIs(t, a.validatePersistedImageSetExecution(context.Background(), slotExecutionInputV3(input)), imageagent.ErrRevisionConflict)
}

func TestLateOriginalImageSetWorkflowClosesExpiredSlotsWithoutDispatch(t *testing.T) {
	for _, mode := range []string{"known_unstarted", "original_claimed"} {
		t.Run(mode, func(t *testing.T) {
			a, repo, input := imageSetPersistenceFixture(t)
			ctx := context.Background()
			scope := imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID}
			current, err := repo.GetProjection(ctx, scope)
			require.NoError(t, err)
			reservation := slotEffectReservationV3(slotExecutionInputV3(input))
			var originalEffect imageagent.SlotEffectV3Attempt
			if mode == "original_claimed" {
				var won bool
				originalEffect, won, err = repo.(imageagent.SlotExternalEffectV3Repository).ReserveSlotProviderV3(ctx, reservation)
				require.NoError(t, err)
				require.True(t, won)
			}
			raw := &recordingSDKClient{}
			require.NoError(t, NewOrganizationClient(raw).StartManual(ctx, imageagent.WorkflowStart{Run: current.Run, Plan: current.Plan, Identity: input.Identity, AssetCatalog: current.AssetCatalog, MaxConcurrentSlots: 1}))
			var suite testsuite.WorkflowTestSuite
			env := suite.NewTestWorkflowEnvironment()
			env.SetTestTimeout(10 * time.Second)
			env.SetStartWorkflowOptions(raw.startOptions)
			env.SetStartTime(raw.workflowInput.DeadlineAt.Add(time.Second))
			children := 0
			env.SetOnChildWorkflowStartedListener(func(*workflow.Info, workflow.Context, sdkconverter.EncodedValues) { children++ })
			env.RegisterWorkflowWithOptions(ImageAgentWorkflow, workflow.RegisterOptions{Name: organizationWorkflowName})
			env.RegisterActivityWithOptions(a.PersistImageSetSlotResult, sdkactivity.RegisterOptions{Name: activityPersistImageSetSlotResult})
			env.RegisterActivityWithOptions(a.PersistRunState, sdkactivity.RegisterOptions{Name: activityPersistRunState})
			env.RegisterActivityWithOptions(a.PersistPendingCommand, sdkactivity.RegisterOptions{Name: activityPersistPendingCommand})
			env.RegisterActivityWithOptions(a.PersistWorkflowFailureV2, sdkactivity.RegisterOptions{Name: activityPersistWorkflowFailureV2})
			var observed imageagent.RunProjection
			var readErr error
			env.RegisterDelayedCallback(func() {
				observed, readErr = repo.GetProjection(ctx, scope)
				if mode == "known_unstarted" {
					env.SignalWorkflow(signalCancel, CancelSignal{RunID: input.RunID, PlanRevision: 1, ActorID: input.Identity.UserID, ActionID: "8d272ce6-0833-4f67-9479-d18708a8cb59"})
				}
			}, time.Second)
			env.ExecuteWorkflow(organizationWorkflowName, raw.workflowInput)
			require.NoError(t, env.GetWorkflowError())
			require.NoError(t, readErr)
			require.Equal(t, imageagent.RunStatusBlocked, observed.Run.Status)
			require.Equal(t, current.Run.ImageAdmission, observed.Run.ImageAdmission)
			require.Len(t, observed.Slots, 1)
			require.Equal(t, imageagent.SlotStatusBlocked, observed.Slots[0].Slot.Status)
			require.Empty(t, observed.Slots[0].Candidates)
			require.Zero(t, children)
			require.Zero(t, a.stagedSlotExecutor.(*recordingStagedExecutor).GenerateCalls())
			_, err = repo.(imageagent.GenerationFactRepository).ReadGenerationFact(ctx, slotEffectReservationV3(slotExecutionInputV3(input)).Identity)
			require.ErrorIs(t, err, imageagent.ErrRunNotFound, "expiration cannot reserve points or dispatch a new generation intent")
			stored, err := repo.GetProjection(ctx, scope)
			require.NoError(t, err)
			if mode == "known_unstarted" {
				require.Equal(t, imageagent.BudgetElapsedCode, observed.Run.Block.Code)
				require.Equal(t, imageagent.BudgetElapsedCode, observed.Slots[0].ErrorCode)
				require.Equal(t, &imageagent.ImageSlotClosure{Kind: "not_dispatched"}, observed.Slots[0].Closure)
				require.Zero(t, observed.Slots[0].Attempt)
				require.Empty(t, observed.RecoverableEffects)
				closedDigest, err := imageagent.ImageSetClosedEffectsDigest(observed.Plan, observed.Slots, observed.RecoverableEffects)
				require.NoError(t, err)
				require.NotEmpty(t, closedDigest)
				require.Equal(t, imageagent.RunStatusCancelled, stored.Run.Status, "the original workflow remains controllable")
			} else {
				require.Equal(t, imageagent.SlotProviderOutcomeUnknownCode, observed.Run.Block.Code)
				require.Equal(t, imageagent.SlotProviderOutcomeUnknownCode, observed.Slots[0].ErrorCode)
				require.Nil(t, observed.Slots[0].Closure)
				require.Equal(t, 1, observed.Slots[0].Attempt)
				require.Equal(t, []imageagent.RecoverableEffect{{SlotID: input.Slot.ID, Attempt: 1, Code: imageagent.SlotProviderOutcomeUnknownCode}}, observed.RecoverableEffects)
				_, err = imageagent.ImageSetClosedEffectsDigest(observed.Plan, observed.Slots, observed.RecoverableEffects)
				require.Error(t, err)
				retained, err := repo.(imageagent.SlotExternalEffectV3Repository).GetSlotExternalEffectV3(ctx, reservation.Identity)
				require.NoError(t, err)
				require.Equal(t, originalEffect, retained, "the original claimed/UNKNOWN effect cannot be rewritten as not dispatched")
			}
		})
	}
}
