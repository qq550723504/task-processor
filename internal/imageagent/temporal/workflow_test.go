package temporal

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	historypb "go.temporal.io/api/history/v1"
	taskqueuepb "go.temporal.io/api/taskqueue/v1"
	sdkactivity "go.temporal.io/sdk/activity"
	sdkclient "go.temporal.io/sdk/client"
	sdkconverter "go.temporal.io/sdk/converter"
	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/types/known/durationpb"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/store"
)

const sevenSlotResultDigest = "727974ab58eba204b1e83cd3dc97c064713351e944ae1896e6e9d36e2f9eaedd"

func TestManualWorkflowExecutesEverySlotIndependently(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
			Return(successfulSlotResult(slot.ID, 1), nil).
			Once()
	}
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).Return(nil).Once()
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalApproveResults, validApproval("approve-final-1"))
	}, time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCompleted, result.Status)
	require.Len(t, result.CompletedSlotIDs, 7)
	env.AssertExpectations(t)
}

func TestManualWorkflowBlocksOnlyFailedSlot(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		if slot.ID == "scene-2" {
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
				Return(imageagent.SlotExecutionResult{}, sdktemporal.NewNonRetryableApplicationError("provider rejected slot", "slot_rejected", nil)).
				Once()
			continue
		}
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
			Return(successfulSlotResult(slot.ID, 1), nil).
			Once()
	}

	env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusBlocked, result.Status)
	require.NotNil(t, result.Block)
	require.Equal(t, "scene-2", result.Block.SlotID)
	require.Len(t, result.CompletedSlotIDs, 6)
	env.AssertExpectations(t)
}

func TestManualWorkflowWaitsForFinalApprovalBeforePublishing(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
			Return(successfulSlotResult(slot.ID, 1), nil).
			Once()
	}
	publishedCalls := 0
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { publishedCalls++ }).
		Return(nil).
		Once()
	env.RegisterDelayedCallback(func() {
		require.Equal(t, 0, publishedCalls)
		env.SignalWorkflow(signalApproveResults, validApproval("approve-final-1"))
	}, time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 1, publishedCalls)
	env.AssertExpectations(t)
}

func TestManualWorkflowDuplicateApprovalPublishesOnceWithStableKey(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
			Return(successfulSlotResult(slot.ID, 1), nil).
			Once()
	}
	env.OnActivity(activityPublishApproved, mock.Anything, mock.MatchedBy(func(in PublishApprovedActivityInput) bool {
		return in.IdempotencyKey == "image-agent:run-1:plan:1:publication" &&
			requireCandidateIDs(in.CandidateAssetIDs, plan)
	})).Return(nil).Once()
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalApproveResults, validApproval("approve-final-1"))
		env.SignalWorkflow(signalApproveResults, validApproval("approve-final-1"))
	}, time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

	require.NoError(t, env.GetWorkflowError())
	env.AssertExpectations(t)
}

func TestManualWorkflowIgnoresApprovalForStaleRevision(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	published := 0
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).Run(func(mock.Arguments) { published++ }).Return(nil).Once()
	env.RegisterDelayedCallback(func() {
		stale := validApproval("approve-stale")
		stale.PlanRevision = 2
		env.SignalWorkflow(signalApproveResults, stale)
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		require.Zero(t, published)
		env.SignalWorkflow(signalApproveResults, validApproval("approve-final-1"))
	}, 2*time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 1, published)
}

func TestManualWorkflowRejectsEarlyApprovalEvenWhenLaterResent(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	published := 0
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).Run(func(mock.Arguments) { published++ }).Return(nil).Once()
	early := validApproval("approve-too-early")
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalApproveResults, early)
	}, 0)
	env.RegisterDelayedCallback(func() {
		require.Zero(t, published)
		env.SignalWorkflow(signalApproveResults, early)
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		require.Zero(t, published)
		env.SignalWorkflow(signalApproveResults, validApproval("approve-after-review"))
	}, 2*time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 1, published)
}

func TestManualWorkflowRejectsApprovalQueuedWhileBlocked(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	input := manualWorkflowInput(plan)
	input.WaitForCommands = true
	for _, slot := range plan.Slots {
		slot := slot
		if slot.ID == "scene-2" {
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
				Return(imageagent.SlotExecutionResult{}, sdktemporal.NewNonRetryableApplicationError("failed", "slot_rejected", nil)).Once()
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 2)).Return(successfulSlotResult(slot.ID, 2), nil).Once()
			continue
		}
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	published := 0
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).Run(func(mock.Arguments) { published++ }).Return(nil).Once()
	blockedApproval := validApproval("approve-while-blocked")
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalApproveResults, blockedApproval)
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalRetrySlot, RetrySlotSignal{RunID: "run-1", PlanRevision: 1, SlotID: "scene-2", ActorID: "user-a", ActionID: "retry-after-blocked-approval"})
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		require.Zero(t, published)
		env.SignalWorkflow(signalApproveResults, blockedApproval)
	}, 3*time.Second)
	env.RegisterDelayedCallback(func() {
		require.Zero(t, published)
		env.SignalWorkflow(signalApproveResults, validApproval("approve-after-review"))
	}, 4*time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 1, published)
	env.AssertExpectations(t)
}

func TestManualWorkflowRejectsApprovalWithWrongActorOrDigest(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*ApproveResultsSignal)
	}{
		{name: "wrong actor", mutate: func(signal *ApproveResultsSignal) { signal.ActorID = "attacker" }},
		{name: "missing digest", mutate: func(signal *ApproveResultsSignal) { signal.ResultDigest = "" }},
		{name: "mismatched digest", mutate: func(signal *ApproveResultsSignal) { signal.ResultDigest = "wrong-digest" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			env := newWorkflowEnv(t)
			plan := sevenSlotPlan()
			for _, slot := range plan.Slots {
				slot := slot
				env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).Return(successfulSlotResult(slot.ID, 1), nil).Once()
			}
			published := 0
			env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).Run(func(mock.Arguments) { published++ }).Return(nil).Once()
			env.RegisterDelayedCallback(func() {
				invalid := validApproval("approve-invalid")
				test.mutate(&invalid)
				env.SignalWorkflow(signalApproveResults, invalid)
			}, time.Second)
			env.RegisterDelayedCallback(func() {
				require.Zero(t, published)
				env.SignalWorkflow(signalApproveResults, validApproval("approve-valid"))
			}, 2*time.Second)

			env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

			require.NoError(t, env.GetWorkflowError())
			require.Equal(t, 1, published)
		})
	}
}

func TestManualWorkflowExposesFinalResultDigestWhileAwaitingApproval(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).Return(nil).Once()
	env.RegisterDelayedCallback(func() {
		encoded, err := env.QueryWorkflow(QueryWorkflowProjection)
		require.NoError(t, err)
		var projection WorkflowResult
		require.NoError(t, encoded.Get(&projection))
		require.Equal(t, imageagent.RunStatusAwaitingFinalApproval, projection.Status)
		require.Equal(t, sevenSlotResultDigest, projection.ResultDigest)
		env.SignalWorkflow(signalApproveResults, ApproveResultsSignal{
			RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "approve-query-result", ResultDigest: projection.ResultDigest,
		})
	}, time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

	require.NoError(t, env.GetWorkflowError())
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, sevenSlotResultDigest, result.ResultDigest)
}

func TestManualWorkflowDuplicateRetrySignalCreatesOneNewAttempt(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		if slot.ID == "scene-2" {
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
				Return(imageagent.SlotExecutionResult{}, sdktemporal.NewNonRetryableApplicationError("failed", "slot_rejected", nil)).Once()
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 2)).
				Return(successfulSlotResult(slot.ID, 2), nil).Once()
			continue
		}
		expectation := env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).Return(successfulSlotResult(slot.ID, 1), nil).Once()
		if slot.ID == "slot-1" {
			expectation.After(time.Minute)
		}
	}
	retry := RetrySlotSignal{RunID: "run-1", PlanRevision: 1, SlotID: "scene-2", ActorID: "user-a", ActionID: "retry-scene-2-1"}
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalRetrySlot, retry)
		env.SignalWorkflow(signalRetrySlot, retry)
	}, time.Second)
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(signalApproveResults, validApproval("approve-final-1")) }, 2*time.Minute)
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).Return(nil).Once()

	env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

	require.NoError(t, env.GetWorkflowError())
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCompleted, result.Status)
	env.AssertExpectations(t)
}

func TestManualWorkflowRemainsDurableWhileBlockedForLaterRetrySignal(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	input := manualWorkflowInput(plan)
	input.WaitForCommands = true
	for _, slot := range plan.Slots {
		slot := slot
		if slot.ID == "scene-2" {
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
				Return(imageagent.SlotExecutionResult{}, sdktemporal.NewNonRetryableApplicationError("failed", "slot_rejected", nil)).Once()
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 2)).
				Return(successfulSlotResult(slot.ID, 2), nil).Once()
			continue
		}
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalRetrySlot, RetrySlotSignal{RunID: "run-1", PlanRevision: 1, SlotID: "scene-2", ActorID: "user-a", ActionID: "retry-after-block"})
	}, time.Second)
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(signalApproveResults, validApproval("approve-final-1")) }, 2*time.Second)
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).Return(nil).Once()

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCompleted, result.Status)
	env.AssertExpectations(t)
}

func TestManualWorkflowRejectsRetrySignalFromDifferentActor(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		if slot.ID == "scene-2" {
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
				Return(imageagent.SlotExecutionResult{}, sdktemporal.NewNonRetryableApplicationError("failed", "slot_rejected", nil)).Once()
			continue
		}
		expectation := env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).Return(successfulSlotResult(slot.ID, 1), nil).Once()
		if slot.ID == "slot-1" {
			expectation.After(time.Minute)
		}
	}
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalRetrySlot, RetrySlotSignal{
			RunID: "run-1", PlanRevision: 1, SlotID: "scene-2", ActorID: "attacker", ActionID: "spoofed-retry",
		})
	}, time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

	require.NoError(t, env.GetWorkflowError())
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusBlocked, result.Status)
	require.Equal(t, "scene-2", result.Block.SlotID)
	env.AssertExpectations(t)
}

func TestManualWorkflowRejectsCancelSignalFromDifferentActor(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).Return(nil).Once()
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalCancel, CancelSignal{RunID: "run-1", PlanRevision: 1, ActorID: "attacker", ActionID: "spoofed-cancel"})
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalApproveResults, validApproval("approve-after-spoofed-cancel"))
	}, 2*time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

	require.NoError(t, env.GetWorkflowError())
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCompleted, result.Status)
	env.AssertExpectations(t)
}

func TestManualWorkflowDrainsQueuedDuplicateCancelBeforeStartingChildren(t *testing.T) {
	env := newWorkflowEnv(t)
	input := manualWorkflowInput(sevenSlotPlan())
	started := 0
	cancelledWrites := 0
	env.SetOnChildWorkflowStartedListener(func(*workflow.Info, workflow.Context, sdkconverter.EncodedValues) { started++ })
	env.OnActivity(activityPersistRunState, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		if activityInputFromArgs[PersistRunStateActivityInput](t, args).Status == imageagent.RunStatusCancelled {
			cancelledWrites++
		}
	}).Return(nil)
	cancel := CancelSignal{RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "cancel-before-start"}
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalCancel, cancel)
		env.SignalWorkflow(signalCancel, cancel)
	}, 0)

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCancelled, result.Status)
	require.Zero(t, started)
	require.Equal(t, 1, cancelledWrites)
	env.AssertExpectations(t)
}

func TestManualWorkflowCancellationStartsNoThirdChildAndKeepsCompletedSibling(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	input := manualWorkflowInput(plan)
	input.MaxConcurrentSlots = 1
	var mu sync.Mutex
	started := []string{}
	persisted := []string{}
	env.SetOnChildWorkflowStartedListener(func(_ *workflow.Info, _ workflow.Context, args sdkconverter.EncodedValues) {
		var childInput SlotWorkflowInput
		require.NoError(t, args.Get(&childInput))
		mu.Lock()
		started = append(started, childInput.Slot.ID)
		mu.Unlock()
	})
	env.OnActivity(activityExecuteSlot, mock.Anything, mock.Anything).
		After(time.Minute).
		Return(imageagent.SlotExecutionResult{}, nil)
	env.OnActivity(activityPersistSlotResult, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			in := activityInputFromArgs[PersistSlotResultActivityInput](t, args)
			mu.Lock()
			persisted = append(persisted, in.Result.Execution.SlotID)
			mu.Unlock()
		}).
		Return(nil)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalCancel, CancelSignal{RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "cancel-1"})
	}, 61*time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCancelled, result.Status)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{"slot-1", "slot-2"}, started)
	require.Equal(t, []string{"slot-1"}, persisted)
}

func TestManualWorkflowRejectsNonManualModesBeforeActivities(t *testing.T) {
	for _, mode := range []imageagent.RunMode{imageagent.RunModeAssisted, imageagent.RunModeAutomatic} {
		t.Run(string(mode), func(t *testing.T) {
			env := newWorkflowEnv(t)
			input := manualWorkflowInput(sevenSlotPlan())
			input.Mode = mode
			env.ExecuteWorkflow(ImageAgentWorkflow, input)
			require.ErrorContains(t, env.GetWorkflowError(), "mode must be manual")
		})
	}
}

func TestSlotWorkflowUsesTemporalTechnicalRetryWithoutChangingSemanticAttempt(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		if slot.ID == "slot-1" {
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).Return(imageagent.SlotExecutionResult{}, errors.New("temporary provider outage")).Once()
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).Return(successfulSlotResult(slot.ID, 1), nil).Once()
			continue
		}
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).Return(nil).Once()
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(signalApproveResults, validApproval("approve-final-1")) }, 2*time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

	require.NoError(t, env.GetWorkflowError())
	env.AssertExpectations(t)
}

func TestImageSlotWorkflowFailsClosedForMismatchedOrEmptyExecutorResult(t *testing.T) {
	for _, test := range []struct {
		name   string
		result imageagent.SlotExecutionResult
	}{
		{name: "wrong slot", result: successfulSlotResult("different-slot", 1)},
		{name: "wrong attempt", result: successfulSlotResult("slot-1", 2)},
		{name: "empty candidates", result: imageagent.SlotExecutionResult{SlotID: "slot-1", Attempt: 1}},
		{name: "whitespace candidate ID", result: imageagent.SlotExecutionResult{SlotID: "slot-1", Attempt: 1, Candidates: []imageagent.AssetCandidate{{AssetID: " \t "}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			env := newWorkflowEnv(t)
			slot := sevenSlotPlan().Slots[0]
			env.OnActivity(activityExecuteSlot, mock.Anything, mock.Anything).Return(test.result, nil).Once()

			env.ExecuteWorkflow(ImageSlotWorkflow, SlotWorkflowInput{RunID: "run-1", Identity: imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}, PlanRevision: 1, Slot: slot, Attempt: 1})

			require.NoError(t, env.GetWorkflowError())
			var result SlotWorkflowResult
			require.NoError(t, env.GetWorkflowResult(&result))
			require.Equal(t, imageagent.SlotStatusBlocked, result.Status)
			require.Equal(t, "invalid_slot_result", result.ErrorCode)
			require.Equal(t, "slot-1", result.Execution.SlotID)
			require.Equal(t, 1, result.Execution.Attempt)
		})
	}
}

func TestManualWorkflowTrimsCandidateIDsForDigestAndPublication(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		result := successfulSlotResult(slot.ID, 1)
		result.Candidates[0].AssetID = "  " + result.Candidates[0].AssetID + "\t"
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).Return(result, nil).Once()
	}
	env.OnActivity(activityPublishApproved, mock.Anything, mock.MatchedBy(func(in PublishApprovedActivityInput) bool {
		return requireCandidateIDs(in.CandidateAssetIDs, plan)
	})).Return(nil).Once()
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalApproveResults, validApproval("approve-trimmed-results"))
	}, time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

	require.NoError(t, env.GetWorkflowError())
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, sevenSlotResultDigest, result.ResultDigest)
	env.AssertExpectations(t)
}

func TestManualWorkflowBoundsConcurrentSlotChildren(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	input := manualWorkflowInput(plan)
	input.MaxConcurrentSlots = 2
	active, maximum := 0, 0
	env.SetOnChildWorkflowStartedListener(func(_ *workflow.Info, _ workflow.Context, _ sdkconverter.EncodedValues) {
		active++
		if active > maximum {
			maximum = active
		}
	})
	env.SetOnChildWorkflowCompletedListener(func(_ *workflow.Info, _ sdkconverter.EncodedValue, _ error) { active-- })
	for _, slot := range plan.Slots {
		slot := slot
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).After(time.Minute).Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).Return(nil).Once()
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(signalApproveResults, validApproval("approve-final-1")) }, 5*time.Minute)

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 2, maximum)
}

func TestActivitiesRestoreCapturedIdentityForExecutorAndPublisher(t *testing.T) {
	repository := store.NewMemoryRepository()
	executor := &identityCheckingExecutor{t: t}
	publisher := &identityCheckingPublisher{t: t}
	activities, err := NewActivities(ActivityDependencies{Repository: repository, SlotExecutor: executor, Publisher: publisher})
	require.NoError(t, err)
	identity := imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}
	slot := sevenSlotPlan().Slots[0]

	result, err := activities.ExecuteSlot(context.Background(), ExecuteSlotActivityInput{
		RunID: "run-1", Identity: identity, PlanRevision: 1, Slot: slot, Attempt: 1, IdempotencyKey: "slot-key-slot-1:attempt:1",
	})
	require.NoError(t, err)
	require.Equal(t, "slot-1", result.SlotID)
	require.NoError(t, activities.PublishApproved(context.Background(), PublishApprovedActivityInput{
		RunID: "run-1", Identity: identity, PlanRevision: 1, CandidateAssetIDs: []string{"candidate-slot-1"}, IdempotencyKey: "image-agent:run-1:plan:1:publication",
	}))
	require.Equal(t, 1, executor.calls)
	require.Equal(t, 1, publisher.calls)
}

func TestActivitiesPersistTerminalSlotResultIdempotently(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:image-agent-activity-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, store.AutoMigrate(db))
	repository := store.NewGormRepository(db)
	identity := imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}
	plan := sevenSlotPlan()
	run := &imageagent.Run{ID: "run-1", TenantID: identity.TenantID, UserID: identity.UserID, Mode: imageagent.RunModeManual, IdempotencyKey: "run-key-1", Status: imageagent.RunStatusExecuting, ActivePlanRevision: 0, Version: 1}
	require.NoError(t, repository.CreateRun(context.Background(), run))
	require.NoError(t, repository.AppendPlan(context.Background(), imageagent.RunScope{TenantID: identity.TenantID, RunID: run.ID}, 0, plan))
	activities, err := NewActivities(ActivityDependencies{Repository: repository, SlotExecutor: &identityCheckingExecutor{t: t}, Publisher: &identityCheckingPublisher{t: t}})
	require.NoError(t, err)
	input := PersistSlotResultActivityInput{
		RunID: "run-1", Identity: identity, PlanRevision: 1, AttemptKey: "slot-key-slot-1:attempt:1",
		Result: SlotWorkflowResult{Execution: successfulSlotResult("slot-1", 1), Status: imageagent.SlotStatusAccepted},
	}

	require.NoError(t, activities.PersistSlotResult(context.Background(), input))
	require.NoError(t, activities.PersistSlotResult(context.Background(), input))
	var attemptCount, resultCount, eventCount int64
	require.NoError(t, db.Table("image_agent_attempts").Where("tenant_id = ? AND run_id = ? AND slot_id = ? AND attempt = ?", "tenant-a", "run-1", "slot-1", 1).Count(&attemptCount).Error)
	require.NoError(t, db.Table("image_agent_slots").Where("tenant_id = ? AND run_id = ? AND plan_revision = ? AND id = ? AND attempt = ? AND status = ?", "tenant-a", "run-1", 1, "slot-1", 1, string(imageagent.SlotStatusAccepted)).Count(&resultCount).Error)
	require.NoError(t, db.Table("image_agent_events").Where("tenant_id = ? AND run_id = ? AND type = ?", "tenant-a", "run-1", slotResultPersistedEventType).Count(&eventCount).Error)
	require.EqualValues(t, 1, attemptCount)
	require.EqualValues(t, 1, resultCount)
	require.EqualValues(t, 1, eventCount)
}

func TestNewActivitiesRejectsMissingDependencies(t *testing.T) {
	_, err := NewActivities(ActivityDependencies{})
	require.ErrorContains(t, err, "repository")
	_, err = NewActivities(ActivityDependencies{Repository: store.NewMemoryRepository()})
	require.ErrorContains(t, err, "slot executor")
}

func TestServiceCapturesVerifiedIdentityAndRejectsNonManualStarts(t *testing.T) {
	repository := store.NewMemoryRepository()
	workflows := &recordingDomainWorkflowClient{}
	service, err := imageagent.NewService(repository, workflows)
	require.NoError(t, err)
	input := imageagent.StartRunInput{RunID: "run-1", Mode: imageagent.RunModeManual, IdempotencyKey: "run-key-1", Plan: sevenSlotPlan()}

	require.ErrorContains(t, service.Start(context.Background(), input), "verified")
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "tenant-a", UserID: "user-a"})
	nonManual := input
	nonManual.RunID = "run-assisted"
	nonManual.IdempotencyKey = "run-assisted-key"
	nonManual.Mode = imageagent.RunModeAssisted
	require.ErrorContains(t, service.Start(ctx, nonManual), "must be manual")

	require.NoError(t, service.Start(ctx, input))
	require.NoError(t, service.Start(ctx, input))
	require.Len(t, workflows.starts, 2)
	require.Equal(t, imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}, workflows.starts[0].Identity)
	require.Equal(t, "user-a", workflows.starts[0].Plan.CreatedBy)
	run, err := repository.GetRun(context.Background(), imageagent.RunScope{TenantID: "tenant-a", RunID: "run-1"})
	require.NoError(t, err)
	require.Equal(t, "user-a", run.UserID)
	require.Equal(t, imageagent.RunModeManual, run.Mode)
	require.NoError(t, service.ApproveResults(ctx, "run-1", 1, "  "+sevenSlotResultDigest+" ", "approve-service-1"))
	require.Len(t, workflows.approvals, 1)
	require.Equal(t, sevenSlotResultDigest, workflows.approvals[0].ResultDigest)
}

func TestTemporalClientUsesStableWorkflowAndRevisionBoundSignals(t *testing.T) {
	raw := &recordingSDKClient{}
	client := NewClient(raw)
	start := imageagent.WorkflowStart{
		Run:  imageagent.Run{ID: "run-1", TenantID: "tenant-a", UserID: "user-a", Mode: imageagent.RunModeManual},
		Plan: sevenSlotPlan(), Identity: imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}, MaxConcurrentSlots: 3,
	}

	require.NoError(t, client.StartManual(context.Background(), start))
	require.Equal(t, "image-agent:tenant-a:run-1", raw.startOptions.ID)
	require.Equal(t, TaskQueueName(), raw.startOptions.TaskQueue)
	require.Equal(t, workflowNameImageAgent, raw.workflowName)
	require.Equal(t, imageagent.RunModeManual, raw.workflowInput.Mode)
	require.True(t, raw.workflowInput.WaitForCommands)

	require.NoError(t, client.RetrySlot(context.Background(), imageagent.RetrySlotCommand{RunID: "run-1", PlanRevision: 1, SlotID: "scene-2", ActorID: "user-a", ActionID: "retry-1", Identity: start.Identity}))
	require.Equal(t, "image-agent:tenant-a:run-1", raw.signalWorkflowID)
	require.Equal(t, signalRetrySlot, raw.signalName)
	require.Equal(t, RetrySlotSignal{RunID: "run-1", PlanRevision: 1, SlotID: "scene-2", ActorID: "user-a", ActionID: "retry-1"}, raw.signalArg)
	require.NoError(t, client.ApproveResults(context.Background(), imageagent.ApproveResultsCommand{RunID: "run-1", PlanRevision: 1, ResultDigest: sevenSlotResultDigest, ActorID: "user-a", ActionID: "approve-1", Identity: start.Identity}))
	require.Equal(t, signalApproveResults, raw.signalName)
	require.Equal(t, ApproveResultsSignal{RunID: "run-1", PlanRevision: 1, ResultDigest: sevenSlotResultDigest, ActorID: "user-a", ActionID: "approve-1"}, raw.signalArg)
	require.NoError(t, client.Cancel(context.Background(), imageagent.CancelRunCommand{RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "cancel-1", Identity: start.Identity}))
	require.Equal(t, signalCancel, raw.signalName)
	require.ErrorContains(t, client.ApproveResults(context.Background(), imageagent.ApproveResultsCommand{RunID: "run-1", PlanRevision: 1, ResultDigest: sevenSlotResultDigest, ActorID: "attacker", ActionID: "spoofed", Identity: start.Identity}), "actor")
	require.ErrorContains(t, client.ApproveResults(context.Background(), imageagent.ApproveResultsCommand{RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "missing-digest", Identity: start.Identity}), "digest")
}

func TestRegisterWorkerRegistersParentChildAndActivities(t *testing.T) {
	activities, err := NewActivities(ActivityDependencies{Repository: store.NewMemoryRepository(), SlotExecutor: &identityCheckingExecutor{t: t}, Publisher: &identityCheckingPublisher{t: t}})
	require.NoError(t, err)
	registrar := &recordingWorkerRegistrar{}

	require.NoError(t, RegisterWorker(registrar, activities))

	require.Equal(t, []string{workflowNameImageAgent, workflowNameImageSlot}, registrar.workflows)
	require.Equal(t, []string{activityExecuteSlot, activityPersistSlotResult, activityPersistRunState, activityPublishApproved}, registrar.activities)
}

func TestImageAgentWorkflowReplaysAfterPersistStateWorkflowTaskRestart(t *testing.T) {
	input := manualWorkflowInput(sevenSlotPlan())
	workflowPayloads, err := sdkconverter.GetDefaultDataConverter().ToPayloads(input)
	require.NoError(t, err)
	persistInput := PersistRunStateActivityInput{
		RunID: "run-1", Identity: input.Identity, PlanRevision: 1,
		Status: imageagent.RunStatusExecuting, CurrentNode: "execute_slots",
	}
	activityPayloads, err := sdkconverter.GetDefaultDataConverter().ToPayloads(persistInput)
	require.NoError(t, err)
	history := &historypb.History{Events: []*historypb.HistoryEvent{
		{
			EventId: 1, EventType: enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED,
			Attributes: &historypb.HistoryEvent_WorkflowExecutionStartedEventAttributes{WorkflowExecutionStartedEventAttributes: &historypb.WorkflowExecutionStartedEventAttributes{
				WorkflowType: &commonpb.WorkflowType{Name: workflowNameImageAgent},
				TaskQueue:    &taskqueuepb.TaskQueue{Name: "image-agent-replay"}, Input: workflowPayloads,
			}},
		},
		{
			EventId: 2, EventType: enumspb.EVENT_TYPE_WORKFLOW_TASK_SCHEDULED,
			Attributes: &historypb.HistoryEvent_WorkflowTaskScheduledEventAttributes{WorkflowTaskScheduledEventAttributes: &historypb.WorkflowTaskScheduledEventAttributes{}},
		},
		{EventId: 3, EventType: enumspb.EVENT_TYPE_WORKFLOW_TASK_STARTED, Attributes: &historypb.HistoryEvent_WorkflowTaskStartedEventAttributes{WorkflowTaskStartedEventAttributes: &historypb.WorkflowTaskStartedEventAttributes{ScheduledEventId: 2}}},
		{
			EventId: 4, EventType: enumspb.EVENT_TYPE_WORKFLOW_TASK_COMPLETED,
			Attributes: &historypb.HistoryEvent_WorkflowTaskCompletedEventAttributes{WorkflowTaskCompletedEventAttributes: &historypb.WorkflowTaskCompletedEventAttributes{ScheduledEventId: 2, StartedEventId: 3}},
		},
		{
			EventId: 5, EventType: enumspb.EVENT_TYPE_ACTIVITY_TASK_SCHEDULED,
			Attributes: &historypb.HistoryEvent_ActivityTaskScheduledEventAttributes{ActivityTaskScheduledEventAttributes: &historypb.ActivityTaskScheduledEventAttributes{
				ActivityId: "5", ActivityType: &commonpb.ActivityType{Name: activityPersistRunState},
				TaskQueue: &taskqueuepb.TaskQueue{Name: "image-agent-replay"}, Input: activityPayloads,
				WorkflowTaskCompletedEventId: 4, StartToCloseTimeout: durationpb.New(time.Minute),
				RetryPolicy: &commonpb.RetryPolicy{InitialInterval: durationpb.New(time.Second), BackoffCoefficient: 2, MaximumInterval: durationpb.New(10 * time.Second), MaximumAttempts: 5},
			}},
		},
		{EventId: 6, EventType: enumspb.EVENT_TYPE_ACTIVITY_TASK_STARTED, Attributes: &historypb.HistoryEvent_ActivityTaskStartedEventAttributes{ActivityTaskStartedEventAttributes: &historypb.ActivityTaskStartedEventAttributes{ScheduledEventId: 5}}},
		{EventId: 7, EventType: enumspb.EVENT_TYPE_ACTIVITY_TASK_COMPLETED, Attributes: &historypb.HistoryEvent_ActivityTaskCompletedEventAttributes{ActivityTaskCompletedEventAttributes: &historypb.ActivityTaskCompletedEventAttributes{ScheduledEventId: 5, StartedEventId: 6}}},
		{
			EventId: 8, EventType: enumspb.EVENT_TYPE_WORKFLOW_TASK_SCHEDULED,
			Attributes: &historypb.HistoryEvent_WorkflowTaskScheduledEventAttributes{WorkflowTaskScheduledEventAttributes: &historypb.WorkflowTaskScheduledEventAttributes{}},
		},
		{EventId: 9, EventType: enumspb.EVENT_TYPE_WORKFLOW_TASK_STARTED, Attributes: &historypb.HistoryEvent_WorkflowTaskStartedEventAttributes{WorkflowTaskStartedEventAttributes: &historypb.WorkflowTaskStartedEventAttributes{ScheduledEventId: 8}}},
	}}
	replayer := worker.NewWorkflowReplayer()
	replayer.RegisterWorkflowWithOptions(ImageAgentWorkflow, workflow.RegisterOptions{Name: workflowNameImageAgent})

	require.NoError(t, replayer.ReplayWorkflowHistory(nil, history))
}

func newWorkflowEnv(t *testing.T) *testsuite.TestWorkflowEnvironment {
	t.Helper()
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetWorkerOptions(worker.Options{DeadlockDetectionTimeout: 5 * time.Second})
	env.RegisterWorkflow(ImageSlotWorkflow)
	env.RegisterActivityWithOptions(
		func(context.Context, ExecuteSlotActivityInput) (imageagent.SlotExecutionResult, error) {
			return imageagent.SlotExecutionResult{}, nil
		},
		sdkactivity.RegisterOptions{Name: activityExecuteSlot},
	)
	env.RegisterActivityWithOptions(
		func(context.Context, PersistSlotResultActivityInput) error { return nil },
		sdkactivity.RegisterOptions{Name: activityPersistSlotResult},
	)
	env.RegisterActivityWithOptions(
		func(context.Context, PersistRunStateActivityInput) error { return nil },
		sdkactivity.RegisterOptions{Name: activityPersistRunState},
	)
	env.RegisterActivityWithOptions(
		func(context.Context, PublishApprovedActivityInput) error { return nil },
		sdkactivity.RegisterOptions{Name: activityPublishApproved},
	)
	return env
}

func manualWorkflowInput(plan imageagent.Plan) WorkflowInput {
	return WorkflowInput{
		RunID:              "run-1",
		Mode:               imageagent.RunModeManual,
		Identity:           imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"},
		Plan:               plan,
		MaxConcurrentSlots: 3,
	}
}

func validApproval(actionID string) ApproveResultsSignal {
	return ApproveResultsSignal{RunID: "run-1", PlanRevision: 1, ResultDigest: sevenSlotResultDigest, ActorID: "user-a", ActionID: actionID}
}

func executeInputForSlot(slotID string, attempt int) interface{} {
	return mock.MatchedBy(func(in ExecuteSlotActivityInput) bool {
		return in.RunID == "run-1" && in.Identity.TenantID == "tenant-a" && in.Identity.UserID == "user-a" &&
			in.PlanRevision == 1 && in.Slot.ID == slotID && in.Attempt == attempt && in.IdempotencyKey == fmt.Sprintf("slot-key-%s:attempt:%d", slotID, attempt)
	})
}

func successfulSlotResult(slotID string, attempt int) imageagent.SlotExecutionResult {
	return imageagent.SlotExecutionResult{
		SlotID:  slotID,
		Attempt: attempt,
		Candidates: []imageagent.AssetCandidate{{
			AssetID: "candidate-" + slotID,
			URL:     "asset://" + slotID,
		}},
	}
}

func requireCandidateIDs(got []string, plan imageagent.Plan) bool {
	want := make([]string, len(plan.Slots))
	for index, slot := range plan.Slots {
		want[index] = "candidate-" + slot.ID
	}
	return fmt.Sprint(got) == fmt.Sprint(want)
}

func activityInputFromArgs[T any](t *testing.T, args mock.Arguments) T {
	t.Helper()
	for _, arg := range args {
		if input, ok := arg.(T); ok {
			return input
		}
	}
	t.Fatalf("activity input %T not found in %#v", *new(T), []interface{}(args))
	return *new(T)
}

type identityCheckingExecutor struct {
	t     *testing.T
	calls int
}

func (e *identityCheckingExecutor) ExecuteSlot(ctx context.Context, input imageagent.SlotExecutionInput) (imageagent.SlotExecutionResult, error) {
	e.calls++
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	require.True(e.t, ok)
	require.Equal(e.t, input.TenantID, identity.TenantID)
	require.Equal(e.t, input.UserID, identity.UserID)
	return successfulSlotResult(input.Slot.ID, input.Attempt), nil
}

type identityCheckingPublisher struct {
	t     *testing.T
	calls int
}

func (p *identityCheckingPublisher) PublishApproved(ctx context.Context, input imageagent.PublishApprovedInput) error {
	p.calls++
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	require.True(p.t, ok)
	require.Equal(p.t, input.TenantID, identity.TenantID)
	require.Equal(p.t, "user-a", identity.UserID)
	return nil
}

type recordingDomainWorkflowClient struct {
	starts    []imageagent.WorkflowStart
	approvals []imageagent.ApproveResultsCommand
}

func (c *recordingDomainWorkflowClient) StartManual(_ context.Context, input imageagent.WorkflowStart) error {
	c.starts = append(c.starts, input)
	return nil
}
func (*recordingDomainWorkflowClient) RetrySlot(context.Context, imageagent.RetrySlotCommand) error {
	return nil
}
func (c *recordingDomainWorkflowClient) ApproveResults(_ context.Context, command imageagent.ApproveResultsCommand) error {
	c.approvals = append(c.approvals, command)
	return nil
}
func (*recordingDomainWorkflowClient) Cancel(context.Context, imageagent.CancelRunCommand) error {
	return nil
}

type recordingSDKClient struct {
	startOptions     sdkclient.StartWorkflowOptions
	workflowName     string
	workflowInput    WorkflowInput
	signalWorkflowID string
	signalName       string
	signalArg        interface{}
}

func (c *recordingSDKClient) ExecuteWorkflow(_ context.Context, options sdkclient.StartWorkflowOptions, workflow interface{}, args ...interface{}) (sdkclient.WorkflowRun, error) {
	c.startOptions = options
	c.workflowName = workflow.(string)
	c.workflowInput = args[0].(WorkflowInput)
	return nil, nil
}

func (c *recordingSDKClient) SignalWorkflow(_ context.Context, workflowID, _ string, signalName string, arg interface{}) error {
	c.signalWorkflowID = workflowID
	c.signalName = signalName
	c.signalArg = arg
	return nil
}

type recordingWorkerRegistrar struct {
	workflows  []string
	activities []string
}

func (r *recordingWorkerRegistrar) RegisterWorkflowWithOptions(_ interface{}, options workflow.RegisterOptions) {
	r.workflows = append(r.workflows, options.Name)
}

func (r *recordingWorkerRegistrar) RegisterActivityWithOptions(_ interface{}, options sdkactivity.RegisterOptions) {
	r.activities = append(r.activities, options.Name)
}

func sevenSlotPlan() imageagent.Plan {
	slots := make([]imageagent.Slot, 0, 7)
	for i, role := range []imageagent.SlotRole{
		imageagent.SlotRoleMain,
		imageagent.SlotRoleScene,
		imageagent.SlotRoleScene,
		imageagent.SlotRoleScene,
		imageagent.SlotRoleDetail,
		imageagent.SlotRoleSellingPoint,
		imageagent.SlotRoleSize,
	} {
		id := fmt.Sprintf("slot-%d", i+1)
		if i == 2 {
			id = "scene-2"
		}
		slots = append(slots, imageagent.Slot{
			ID:             id,
			Role:           role,
			SourceAssetIDs: []string{"source-1"},
			IdempotencyKey: "slot-key-" + id,
			Status:         imageagent.SlotStatusPending,
		})
	}
	return imageagent.Plan{
		Revision:       1,
		IdempotencyKey: "plan-key-1",
		SourceAssetIDs: []string{"source-1"},
		Slots:          slots,
		CreatedBy:      "user-a",
	}
}
