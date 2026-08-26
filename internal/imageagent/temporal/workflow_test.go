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
	"go.temporal.io/api/serviceerror"
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

func TestManualWorkflowReplacesBlockedPlanAtExactRevision(t *testing.T) {
	env := newWorkflowEnv(t)
	initial := sevenSlotPlan()
	replacement := sevenSlotPlan()
	replacement.Revision = 2
	replacement.ParentRevision = 1
	replacement.IdempotencyKey = "plan-key-2"

	for _, slot := range initial.Slots {
		slot := slot
		if slot.ID == "scene-2" {
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlotRevision(slot.ID, 1, 1)).
				Return(imageagent.SlotExecutionResult{}, sdktemporal.NewNonRetryableApplicationError("provider rejected slot", "slot_rejected", nil)).
				Once()
		} else {
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlotRevision(slot.ID, 1, 1)).
				Return(successfulSlotResult(slot.ID, 1), nil).
				Once()
		}
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlotRevision(slot.ID, 1, 2)).
			Return(successfulSlotResult(slot.ID, 1), nil).
			Once()
	}
	env.OnActivity(activityPersistPlanRevision, mock.Anything, mock.MatchedBy(func(input PersistPlanRevisionActivityInput) bool {
		return input.RunID == "run-1" && input.ExpectedRevision == 1 && input.Plan.Revision == 2 && input.Identity.TenantID == "tenant-a"
	})).Return(nil).Once()
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).Return(nil).Once()
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalReplacePlan, ReplacePlanSignal{
			RunID: "run-1", ExpectedRevision: 1, Plan: replacement,
			ActorID: "user-a", ActionID: "replace-1",
		})
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		encoded, err := env.QueryWorkflow(QueryWorkflowProjection)
		require.NoError(t, err)
		var projection WorkflowResult
		require.NoError(t, encoded.Get(&projection))
		require.EqualValues(t, 2, projection.Plan.Revision)
		require.Equal(t, imageagent.RunStatusAwaitingFinalApproval, projection.Status)
		env.SignalWorkflow(signalApproveResults, ApproveResultsSignal{
			RunID: "run-1", PlanRevision: 2, ResultDigest: projection.ResultDigest,
			ActorID: "user-a", ActionID: "approve-replacement",
		})
	}, 2*time.Second)

	input := manualWorkflowInput(initial)
	input.WaitForCommands = true
	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCompleted, result.Status)
	require.EqualValues(t, 2, result.Plan.Revision)
	env.AssertExpectations(t)
}

func TestManualWorkflowReplaceUpdatesRejectLostCommandWindowAndAckAfterPersistence(t *testing.T) {
	env := newWorkflowEnv(t)
	initial := sevenSlotPlan()
	replacement := sevenSlotPlan()
	replacement.Revision = 2
	replacement.ParentRevision = 1
	replacement.IdempotencyKey = "plan-key-2"
	for _, slot := range initial.Slots {
		slot := slot
		if slot.ID == "scene-2" {
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlotRevision(slot.ID, 1, 1)).
				Return(imageagent.SlotExecutionResult{}, sdktemporal.NewNonRetryableApplicationError("provider rejected slot", "slot_rejected", nil)).Once()
		} else {
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlotRevision(slot.ID, 1, 1)).
				Return(successfulSlotResult(slot.ID, 1), nil).Once()
		}
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlotRevision(slot.ID, 1, 2)).
			Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	persisted := false
	env.OnActivity(activityPersistPlanRevision, mock.Anything, mock.Anything).
		After(time.Minute).
		Run(func(mock.Arguments) { persisted = true }).
		Return(nil).
		Once()
	firstCompleted := false
	var firstRejected, firstErr, secondRejected error
	replacementUpdate := ReplacePlanSignal{
		RunID: "run-1", ExpectedRevision: 1, Plan: replacement, ActorID: "user-a", ActionID: "replace-update-1",
	}
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalReplacePlan, "replace-request-1", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { firstRejected = err },
			OnAccept: func() {},
			OnComplete: func(_ interface{}, err error) {
				firstCompleted = true
				firstErr = err
				require.True(t, persisted)
				encoded, queryErr := env.QueryWorkflow(QueryWorkflowProjection)
				require.NoError(t, queryErr)
				var projection WorkflowResult
				require.NoError(t, encoded.Get(&projection))
				require.EqualValues(t, 2, projection.Plan.Revision)
			},
		}, replacementUpdate)
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		second := replacementUpdate
		second.ActionID = "replace-update-2"
		env.UpdateWorkflow(signalReplacePlan, "replace-request-2", &testsuite.TestUpdateCallback{
			OnReject:   func(err error) { secondRejected = err },
			OnAccept:   func() { require.Fail(t, "second replace must not be accepted during transition") },
			OnComplete: func(interface{}, error) {},
		}, second)
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalApproveResults, ApproveResultsSignal{
			RunID: "run-1", PlanRevision: 2, ResultDigest: sevenSlotResultDigest, ActorID: "user-a", ActionID: "approve-replacement-update",
		})
	}, 3*time.Minute)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalCancel, CancelSignal{RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "red-cleanup"})
	}, 4*time.Minute)
	input := manualWorkflowInput(initial)
	input.WaitForCommands = true

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	require.Nil(t, firstRejected)
	require.True(t, firstCompleted)
	require.NoError(t, firstErr)
	require.Error(t, secondRejected)
	var applicationError *sdktemporal.ApplicationError
	require.ErrorAs(t, secondRejected, &applicationError)
	require.Equal(t, updateErrorCommandBlocked, applicationError.Type())
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCompleted, result.Status)
	require.EqualValues(t, 2, result.Plan.Revision)
}

func TestManualWorkflowReplaceUpdateResumesAfterRunTransitionFailure(t *testing.T) {
	env := newWorkflowEnv(t)
	initial := sevenSlotPlan()
	replacement := sevenSlotPlan()
	replacement.Revision = 2
	replacement.ParentRevision = 1
	replacement.IdempotencyKey = "plan-key-2"
	for _, slot := range initial.Slots {
		slot := slot
		if slot.ID == "scene-2" {
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlotRevision(slot.ID, 1, 1)).
				Return(imageagent.SlotExecutionResult{}, sdktemporal.NewNonRetryableApplicationError("provider rejected slot", "slot_rejected", nil)).Once()
		} else {
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlotRevision(slot.ID, 1, 1)).
				Return(successfulSlotResult(slot.ID, 1), nil).Once()
		}
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlotRevision(slot.ID, 1, 2)).
			Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	planPersistCalls := 0
	logicalPlanWrites := 0
	durablePlanRevision := int64(1)
	durableStatus := imageagent.RunStatusBlocked
	env.OnActivity(activityPersistPlanRevision, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) {
			planPersistCalls++
			if logicalPlanWrites == 0 {
				logicalPlanWrites++
				durablePlanRevision = 2
			}
		}).Return(nil).Times(3)
	transition := mock.MatchedBy(func(input PersistRunStateActivityInput) bool {
		return input.PlanRevision == 2 && input.Status == imageagent.RunStatusExecuting
	})
	transitionCalls := 0
	env.OnActivity(activityPersistRunState, mock.Anything, transition).
		Run(func(mock.Arguments) { transitionCalls++ }).
		Return(sdktemporal.NewNonRetryableApplicationError("transition write failed after plan persisted", "terminal_test_failure", nil)).Once()
	env.OnActivity(activityPersistRunState, mock.Anything, transition).
		Run(func(mock.Arguments) { transitionCalls++ }).
		Return(sdktemporal.NewNonRetryableApplicationError("transition write still unavailable during signal resume", "terminal_test_failure", nil)).Once()
	env.OnActivity(activityPersistRunState, mock.Anything, transition).
		Run(func(mock.Arguments) {
			transitionCalls++
			durableStatus = imageagent.RunStatusExecuting
		}).Return(nil).Once()
	env.OnActivity(activityPersistRunState, mock.Anything, transition).
		Return(sdktemporal.NewNonRetryableApplicationError("redundant parent executing write must not run", "unexpected_duplicate_transition", nil)).Maybe()
	env.OnActivity(activityPersistRunState, mock.Anything, mock.Anything).Return(nil)
	command := ReplacePlanSignal{RunID: "run-1", ExpectedRevision: 1, Plan: replacement, ActorID: "user-a", ActionID: "replace-resume"}
	var firstErr, resumedErr, conflictingErr, competingErr error
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalReplacePlan, "replace-resume-first", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { firstErr = err }, OnAccept: func() {},
			OnComplete: func(_ interface{}, errorValue error) {
				firstErr = errorValue
				encoded, queryErr := env.QueryWorkflow(QueryWorkflowProjection)
				require.NoError(t, queryErr)
				var projection WorkflowResult
				require.NoError(t, encoded.Get(&projection))
				require.EqualValues(t, 1, projection.Plan.Revision)
				require.Equal(t, imageagent.RunStatusBlocked, projection.Status)
			},
		}, command)
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		conflict := command
		conflict.Plan.IdempotencyKey = "conflicting-plan-key"
		env.UpdateWorkflow(signalReplacePlan, "replace-resume-conflict", &testsuite.TestUpdateCallback{
			OnReject:   func(err error) { conflictingErr = err },
			OnAccept:   func() { require.Fail(t, "conflicting pending replace must be rejected") },
			OnComplete: func(interface{}, error) {},
		}, conflict)
		competing := command
		competing.ActionID = "replace-competing"
		env.UpdateWorkflow(signalReplacePlan, "replace-resume-competing", &testsuite.TestUpdateCallback{
			OnReject:   func(err error) { competingErr = err },
			OnAccept:   func() { require.Fail(t, "a new command must not bypass a pending saga") },
			OnComplete: func(interface{}, error) {},
		}, competing)
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalReplacePlan, command)
		conflictingReuse := command
		conflictingReuse.Plan.IdempotencyKey = "signal-conflicting-plan-key"
		env.SignalWorkflow(signalReplacePlan, conflictingReuse)
		competing := command
		competing.ActionID = "replace-signal-competing"
		env.SignalWorkflow(signalReplacePlan, competing)
	}, 2500*time.Millisecond)
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalReplacePlan, "replace-resume-second", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { resumedErr = err }, OnAccept: func() {},
			OnComplete: func(_ interface{}, errorValue error) {
				resumedErr = errorValue
				encoded, queryErr := env.QueryWorkflow(QueryWorkflowProjection)
				require.NoError(t, queryErr)
				var projection WorkflowResult
				require.NoError(t, encoded.Get(&projection))
				require.EqualValues(t, durablePlanRevision, projection.Plan.Revision)
				require.Equal(t, durableStatus, projection.Status)
			},
		}, command)
	}, 3*time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalApproveResults, ApproveResultsSignal{
			RunID: "run-1", PlanRevision: 2, ResultDigest: sevenSlotResultDigest,
			ActorID: "user-a", ActionID: "approve-replaced-plan",
		})
	}, 4*time.Second)
	input := manualWorkflowInput(initial)
	input.WaitForCommands = true

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	require.Error(t, firstErr)
	require.NoError(t, resumedErr)
	require.Error(t, conflictingErr)
	require.Error(t, competingErr)
	require.Equal(t, 3, planPersistCalls)
	require.Equal(t, 1, logicalPlanWrites)
	require.Equal(t, 3, transitionCalls)
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.EqualValues(t, 2, result.Plan.Revision)
	require.Equal(t, imageagent.RunStatusCompleted, result.Status)
	env.AssertExpectations(t)
}

func TestManualWorkflowRetryUpdatesRejectLostCommandWindowAndAckAfterProjectionPersistence(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		if slot.ID == "scene-2" {
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
				Return(imageagent.SlotExecutionResult{}, sdktemporal.NewNonRetryableApplicationError("provider rejected slot", "slot_rejected", nil)).Once()
			continue
		}
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
			Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot("scene-2", 2)).
		After(time.Minute).
		Return(successfulSlotResult("scene-2", 2), nil).
		Once()
	retryPersisted := false
	env.OnActivity(activityPersistSlotResult, mock.Anything, mock.MatchedBy(func(input PersistSlotResultActivityInput) bool {
		return input.Result.Execution.SlotID == "scene-2" && input.Result.Execution.Attempt == 2
	})).Run(func(mock.Arguments) { retryPersisted = true }).Return(nil).Once()
	env.OnActivity(activityPersistSlotResult, mock.Anything, mock.Anything).Return(nil)
	approvalStatePersisted := false
	env.OnActivity(activityPersistRunState, mock.Anything, mock.MatchedBy(func(input PersistRunStateActivityInput) bool {
		return input.Status == imageagent.RunStatusAwaitingFinalApproval
	})).Run(func(mock.Arguments) { approvalStatePersisted = true }).Return(nil).Once()
	env.OnActivity(activityPersistRunState, mock.Anything, mock.Anything).Return(nil)
	firstCompleted := false
	var firstRejected, firstErr, secondRejected error
	retryUpdate := RetrySlotSignal{
		RunID: "run-1", PlanRevision: 1, SlotID: "scene-2", ActorID: "user-a", ActionID: "retry-update-1",
	}
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalRetrySlot, "retry-request-1", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { firstRejected = err },
			OnAccept: func() {},
			OnComplete: func(_ interface{}, err error) {
				firstCompleted = true
				firstErr = err
				require.True(t, retryPersisted)
				require.True(t, approvalStatePersisted)
				encoded, queryErr := env.QueryWorkflow(QueryWorkflowProjection)
				require.NoError(t, queryErr)
				var projection WorkflowResult
				require.NoError(t, encoded.Get(&projection))
				require.Equal(t, imageagent.RunStatusAwaitingFinalApproval, projection.Status)
				require.Equal(t, sevenSlotResultDigest, projection.ResultDigest)
			},
		}, retryUpdate)
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		second := retryUpdate
		second.ActionID = "retry-update-2"
		env.UpdateWorkflow(signalRetrySlot, "retry-request-2", &testsuite.TestUpdateCallback{
			OnReject:   func(err error) { secondRejected = err },
			OnAccept:   func() { require.Fail(t, "second retry must not be accepted during transition") },
			OnComplete: func(interface{}, error) {},
		}, second)
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalApproveResults, validApproval("approve-after-retry-update"))
	}, 3*time.Minute)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalCancel, CancelSignal{RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "retry-red-cleanup"})
	}, 4*time.Minute)
	input := manualWorkflowInput(plan)
	input.WaitForCommands = true

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	require.Nil(t, firstRejected)
	require.True(t, firstCompleted)
	require.NoError(t, firstErr)
	require.Error(t, secondRejected)
	var applicationError *sdktemporal.ApplicationError
	require.ErrorAs(t, secondRejected, &applicationError)
	require.Equal(t, updateErrorCommandBlocked, applicationError.Type())
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCompleted, result.Status)
}

func TestManualWorkflowRetryUpdateResumesWithoutRerunningProviderAfterPersistenceFailure(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		if slot.ID == "scene-2" {
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
				Return(imageagent.SlotExecutionResult{}, sdktemporal.NewNonRetryableApplicationError("provider rejected slot", "slot_rejected", nil)).Once()
			continue
		}
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
			Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	providerRetryCalls := 0
	env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot("scene-2", 2)).
		Run(func(mock.Arguments) { providerRetryCalls++ }).
		Return(successfulSlotResult("scene-2", 2), nil).Once()
	retryPersist := mock.MatchedBy(func(input PersistSlotResultActivityInput) bool {
		return input.Result.Execution.SlotID == "scene-2" && input.Result.Execution.Attempt == 2
	})
	logicalSlotWrites := 0
	durableSlotStatus := imageagent.SlotStatusBlocked
	durableStatus := imageagent.RunStatusBlocked
	env.OnActivity(activityPersistSlotResult, mock.Anything, retryPersist).
		Run(func(mock.Arguments) {
			logicalSlotWrites = 1
			durableSlotStatus = imageagent.SlotStatusAccepted
		}).
		Return(sdktemporal.NewNonRetryableApplicationError("slot write reported failure", "terminal_test_failure", nil)).Once()
	env.OnActivity(activityPersistSlotResult, mock.Anything, retryPersist).Return(nil).Once()
	env.OnActivity(activityPersistSlotResult, mock.Anything, mock.Anything).Return(nil)
	approvalTransition := mock.MatchedBy(func(input PersistRunStateActivityInput) bool {
		return input.Status == imageagent.RunStatusAwaitingFinalApproval
	})
	approvalTransitionCalls := 0
	env.OnActivity(activityPersistRunState, mock.Anything, approvalTransition).
		Run(func(mock.Arguments) { approvalTransitionCalls++ }).
		Return(sdktemporal.NewNonRetryableApplicationError("approval transition write failed after slot persisted", "terminal_test_failure", nil)).Once()
	env.OnActivity(activityPersistRunState, mock.Anything, approvalTransition).
		Run(func(mock.Arguments) {
			approvalTransitionCalls++
			durableStatus = imageagent.RunStatusAwaitingFinalApproval
		}).Return(nil).Once()
	env.OnActivity(activityPersistRunState, mock.Anything, approvalTransition).
		Return(sdktemporal.NewNonRetryableApplicationError("redundant parent approval write must not run", "unexpected_duplicate_transition", nil)).Maybe()
	env.OnActivity(activityPersistRunState, mock.Anything, mock.Anything).Return(nil)
	command := RetrySlotSignal{RunID: "run-1", PlanRevision: 1, SlotID: "scene-2", ActorID: "user-a", ActionID: "retry-resume"}
	var firstErr, resumedErr, conflictingErr error
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalRetrySlot, "retry-resume-first", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { firstErr = err }, OnAccept: func() {},
			OnComplete: func(_ interface{}, errorValue error) {
				firstErr = errorValue
				encoded, queryErr := env.QueryWorkflow(QueryWorkflowProjection)
				require.NoError(t, queryErr)
				var projection WorkflowResult
				require.NoError(t, encoded.Get(&projection))
				require.Equal(t, imageagent.RunStatusBlocked, projection.Status)
				require.Equal(t, imageagent.SlotStatusBlocked, projection.Slots[2].Slot.Status)
			},
		}, command)
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		conflict := command
		conflict.SlotID = "slot-1"
		env.UpdateWorkflow(signalRetrySlot, "retry-resume-conflict", &testsuite.TestUpdateCallback{
			OnReject:   func(err error) { conflictingErr = err },
			OnAccept:   func() { require.Fail(t, "conflicting pending retry must be rejected") },
			OnComplete: func(interface{}, error) {},
		}, conflict)
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalRetrySlot, command)
		conflictingReuse := command
		conflictingReuse.SlotID = "slot-1"
		env.SignalWorkflow(signalRetrySlot, conflictingReuse)
		competing := command
		competing.ActionID = "retry-signal-competing"
		env.SignalWorkflow(signalRetrySlot, competing)
	}, 2500*time.Millisecond)
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalRetrySlot, "retry-resume-after-signal", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { resumedErr = err }, OnAccept: func() {},
			OnComplete: func(_ interface{}, errorValue error) {
				resumedErr = errorValue
				encoded, queryErr := env.QueryWorkflow(QueryWorkflowProjection)
				require.NoError(t, queryErr)
				var projection WorkflowResult
				require.NoError(t, encoded.Get(&projection))
				require.Equal(t, durableStatus, projection.Status)
				require.Equal(t, durableSlotStatus, projection.Slots[2].Slot.Status)
			},
		}, command)
	}, 3*time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalApproveResults, validApproval("approve-after-retry-resume"))
	}, 4*time.Second)
	input := manualWorkflowInput(plan)
	input.WaitForCommands = true

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	require.Error(t, firstErr)
	require.NoError(t, resumedErr)
	require.Error(t, conflictingErr)
	require.Equal(t, 1, providerRetryCalls)
	require.Equal(t, 1, logicalSlotWrites)
	require.Equal(t, 2, approvalTransitionCalls)
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCompleted, result.Status)
	env.AssertExpectations(t)
}

func TestManualWorkflowUpdateActionIDIsIdempotentAndRejectsConflictingReuse(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		if slot.ID == "scene-2" {
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
				Return(imageagent.SlotExecutionResult{}, sdktemporal.NewNonRetryableApplicationError("provider rejected slot", "slot_rejected", nil)).Once()
			continue
		}
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
			Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot("scene-2", 2)).
		Return(imageagent.SlotExecutionResult{}, sdktemporal.NewNonRetryableApplicationError("provider rejected retry", "slot_rejected", nil)).Once()
	retry := RetrySlotSignal{RunID: "run-1", PlanRevision: 1, SlotID: "scene-2", ActorID: "user-a", ActionID: "retry-idempotent"}
	var firstAck, duplicateAck CommandAcknowledgement
	var firstErr, duplicateErr, conflictingRejected error
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalRetrySlot, "retry-idempotent-request-1", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { firstErr = err },
			OnAccept: func() {},
			OnComplete: func(result interface{}, err error) {
				firstErr = err
				firstAck = result.(CommandAcknowledgement)
			},
		}, retry)
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalRetrySlot, "retry-idempotent-request-2", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { duplicateErr = err },
			OnAccept: func() {},
			OnComplete: func(result interface{}, err error) {
				duplicateErr = err
				duplicateAck = result.(CommandAcknowledgement)
			},
		}, retry)
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		conflict := retry
		conflict.SlotID = "slot-1"
		env.UpdateWorkflow(signalRetrySlot, "retry-idempotent-request-conflict", &testsuite.TestUpdateCallback{
			OnReject:   func(err error) { conflictingRejected = err },
			OnAccept:   func() { require.Fail(t, "conflicting action-ID reuse must be rejected") },
			OnComplete: func(interface{}, error) {},
		}, conflict)
	}, 3*time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalCancel, CancelSignal{RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "idempotency-cleanup"})
	}, 4*time.Second)
	input := manualWorkflowInput(plan)
	input.WaitForCommands = true

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	require.NoError(t, firstErr)
	require.NoError(t, duplicateErr)
	require.Equal(t, firstAck, duplicateAck)
	require.Equal(t, "retry-idempotent", firstAck.ActionID)
	require.Error(t, conflictingRejected)
	var applicationError *sdktemporal.ApplicationError
	require.ErrorAs(t, conflictingRejected, &applicationError)
	require.Equal(t, updateErrorCommandBlocked, applicationError.Type())
	env.AssertExpectations(t)
}

func TestWorkflowUpdateValidatorsBindRunRevisionActorActionStateAndDigest(t *testing.T) {
	plan := sevenSlotPlan()
	input := manualWorkflowInput(plan)
	results := make([]SlotWorkflowResult, len(plan.Slots))
	projection := WorkflowResult{
		Status:       imageagent.RunStatusAwaitingFinalApproval,
		Plan:         plan,
		Slots:        slotProjections(plan, results),
		ResultDigest: sevenSlotResultDigest,
	}
	state := &workflowUpdateState{
		input: &input, projection: &projection, results: &results,
		actions: make(map[string]*workflowUpdateRecord),
	}

	assertType := func(t *testing.T, err error, expected string) {
		t.Helper()
		require.Error(t, err)
		var applicationError *sdktemporal.ApplicationError
		require.ErrorAs(t, err, &applicationError)
		require.Equal(t, expected, applicationError.Type())
	}
	newApproval := func(actionID string) ApproveResultsSignal {
		return ApproveResultsSignal{
			RunID: "run-1", PlanRevision: 1, ResultDigest: sevenSlotResultDigest,
			ActorID: "user-a", ActionID: actionID,
		}
	}

	t.Run("run", func(t *testing.T) {
		signal := newApproval("approve-wrong-run")
		signal.RunID = "run-other"
		assertType(t, state.validateApproveResults(signal), updateErrorCommandBlocked)
	})
	t.Run("revision", func(t *testing.T) {
		signal := newApproval("approve-stale")
		signal.PlanRevision = 2
		assertType(t, state.validateApproveResults(signal), updateErrorRevisionConflict)
	})
	t.Run("actor", func(t *testing.T) {
		signal := newApproval("approve-wrong-actor")
		signal.ActorID = "attacker"
		assertType(t, state.validateApproveResults(signal), updateErrorCommandBlocked)
	})
	t.Run("action", func(t *testing.T) {
		signal := newApproval(" ")
		assertType(t, state.validateApproveResults(signal), updateErrorCommandBlocked)
	})
	t.Run("state", func(t *testing.T) {
		signal := newApproval("approve-wrong-state")
		projection.Status = imageagent.RunStatusExecuting
		assertType(t, state.validateApproveResults(signal), updateErrorCommandBlocked)
		projection.Status = imageagent.RunStatusAwaitingFinalApproval
	})
	t.Run("digest whitespace", func(t *testing.T) {
		signal := newApproval("approve-spaced-digest")
		signal.ResultDigest = " " + sevenSlotResultDigest
		assertType(t, state.validateApproveResults(signal), updateErrorCommandBlocked)
	})
	t.Run("digest mismatch", func(t *testing.T) {
		signal := newApproval("approve-wrong-digest")
		signal.ResultDigest = "wrong"
		assertType(t, state.validateApproveResults(signal), updateErrorCommandBlocked)
	})
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

func TestManualWorkflowApprovalUpdateRequiresCanonicalDigestAndAcksAfterPublication(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
			Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	published := false
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).
		After(time.Minute).
		Run(func(mock.Arguments) { published = true }).
		Return(nil).
		Once()
	completedPersisted := false
	env.OnActivity(activityPersistRunState, mock.Anything, mock.MatchedBy(func(input PersistRunStateActivityInput) bool {
		return input.Status == imageagent.RunStatusCompleted
	})).Run(func(mock.Arguments) { completedPersisted = true }).Return(nil).Once()
	env.OnActivity(activityPersistRunState, mock.Anything, mock.Anything).Return(nil)
	var nonCanonicalRejected, validRejected, validErr error
	validCompleted := false
	env.RegisterDelayedCallback(func() {
		nonCanonical := validApproval("approve-update-space")
		nonCanonical.ResultDigest = " " + nonCanonical.ResultDigest
		env.UpdateWorkflow(signalApproveResults, "approve-space-request", &testsuite.TestUpdateCallback{
			OnReject:   func(err error) { nonCanonicalRejected = err },
			OnAccept:   func() { require.Fail(t, "non-canonical digest must not be accepted") },
			OnComplete: func(interface{}, error) {},
		}, nonCanonical)
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalApproveResults, "approve-request", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { validRejected = err },
			OnAccept: func() {},
			OnComplete: func(_ interface{}, err error) {
				validCompleted = true
				validErr = err
				require.True(t, published)
				require.True(t, completedPersisted)
				encoded, queryErr := env.QueryWorkflow(QueryWorkflowProjection)
				require.NoError(t, queryErr)
				var projection WorkflowResult
				require.NoError(t, encoded.Get(&projection))
				require.Equal(t, imageagent.RunStatusCompleted, projection.Status)
			},
		}, validApproval("approve-update-1"))
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalApproveResults, validApproval("approval-red-cleanup"))
	}, 4*time.Minute)

	env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

	require.NoError(t, env.GetWorkflowError())
	require.Error(t, nonCanonicalRejected)
	require.Nil(t, validRejected)
	require.True(t, validCompleted)
	require.NoError(t, validErr)
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCompleted, result.Status)
}

func TestManualWorkflowApprovalUpdateResumesAfterCompletedStateFailureWithoutRepublishing(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
			Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	publishCalls := 0
	logicalPublications := 0
	env.OnActivity(activityPublishApproved, mock.Anything, mock.MatchedBy(func(input PublishApprovedActivityInput) bool {
		return input.IdempotencyKey == "image-agent:run-1:plan:1:publication"
	})).Run(func(mock.Arguments) {
		publishCalls++
		logicalPublications = 1
	}).Return(nil).Once()
	completed := mock.MatchedBy(func(input PersistRunStateActivityInput) bool {
		return input.Status == imageagent.RunStatusCompleted
	})
	durableStatus := imageagent.RunStatusAwaitingFinalApproval
	env.OnActivity(activityPersistRunState, mock.Anything, completed).
		Return(sdktemporal.NewNonRetryableApplicationError("completed state write failed after publication", "terminal_test_failure", nil)).Once()
	env.OnActivity(activityPersistRunState, mock.Anything, completed).
		Return(sdktemporal.NewNonRetryableApplicationError("completed state write still unavailable during signal resume", "terminal_test_failure", nil)).Once()
	env.OnActivity(activityPersistRunState, mock.Anything, completed).
		Run(func(mock.Arguments) { durableStatus = imageagent.RunStatusCompleted }).Return(nil).Once()
	env.OnActivity(activityPersistRunState, mock.Anything, mock.Anything).Return(nil)
	command := validApproval("approve-resume")
	var firstErr, resumedErr, conflictingErr error
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalApproveResults, "approve-resume-first", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { firstErr = err }, OnAccept: func() {},
			OnComplete: func(_ interface{}, errorValue error) {
				firstErr = errorValue
				encoded, queryErr := env.QueryWorkflow(QueryWorkflowProjection)
				require.NoError(t, queryErr)
				var projection WorkflowResult
				require.NoError(t, encoded.Get(&projection))
				require.Equal(t, imageagent.RunStatusAwaitingFinalApproval, projection.Status)
			},
		}, command)
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		conflict := command
		conflict.ResultDigest = "different-digest"
		env.UpdateWorkflow(signalApproveResults, "approve-resume-conflict", &testsuite.TestUpdateCallback{
			OnReject:   func(err error) { conflictingErr = err },
			OnAccept:   func() { require.Fail(t, "conflicting pending approval must be rejected") },
			OnComplete: func(interface{}, error) {},
		}, conflict)
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalApproveResults, command)
		conflictingReuse := command
		conflictingReuse.ResultDigest = "different-digest"
		env.SignalWorkflow(signalApproveResults, conflictingReuse)
		competing := command
		competing.ActionID = "approve-signal-competing"
		env.SignalWorkflow(signalApproveResults, competing)
	}, 2500*time.Millisecond)
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalApproveResults, "approve-resume-second", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { resumedErr = err }, OnAccept: func() {},
			OnComplete: func(_ interface{}, errorValue error) {
				resumedErr = errorValue
				encoded, queryErr := env.QueryWorkflow(QueryWorkflowProjection)
				require.NoError(t, queryErr)
				var projection WorkflowResult
				require.NoError(t, encoded.Get(&projection))
				require.Equal(t, durableStatus, projection.Status)
			},
		}, command)
	}, 3*time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

	require.NoError(t, env.GetWorkflowError())
	require.Error(t, firstErr)
	require.NoError(t, resumedErr)
	require.Error(t, conflictingErr)
	require.Equal(t, 1, publishCalls)
	require.Equal(t, 1, logicalPublications)
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCompleted, result.Status)
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
	freshSent := false
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).Run(func(mock.Arguments) { published++ }).Return(nil).Once()
	stale := validApproval("approve-stale")
	stale.PlanRevision = 2
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalApproveResults, stale)
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalApproveResults, validApproval("approve-stale"))
	}, 1500*time.Millisecond)
	env.RegisterDelayedCallback(func() {
		require.Zero(t, published)
		freshSent = true
		env.SignalWorkflow(signalApproveResults, validApproval("approve-final-1"))
	}, 2*time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

	require.NoError(t, env.GetWorkflowError())
	require.True(t, freshSent, "a stale legacy approval action ID must remain consumed after its revision is corrected")
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
	freshSent := false
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
		freshSent = true
		env.SignalWorkflow(signalApproveResults, validApproval("approve-after-review"))
	}, 2*time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

	require.NoError(t, env.GetWorkflowError())
	require.True(t, freshSent, "the rejected legacy action ID must not complete the workflow before the fresh approval")
	require.Equal(t, 1, published)
}

func TestManualWorkflowRejectedLegacySignalDoesNotTombstoneWorkflowUpdate(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).Return(nil).Once()
	command := validApproval("approval-shared-with-update")
	var rejected, completedErr error
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(signalApproveResults, command) }, 0)
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalApproveResults, "approval-after-rejected-signal", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { rejected = err },
			OnAccept: func() {},
			OnComplete: func(_ interface{}, err error) {
				completedErr = err
			},
		}, command)
	}, time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

	require.NoError(t, env.GetWorkflowError())
	require.NoError(t, rejected)
	require.NoError(t, completedErr)
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCompleted, result.Status)
	env.AssertExpectations(t)
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
	freshSent := false
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
		freshSent = true
		env.SignalWorkflow(signalApproveResults, validApproval("approve-after-review"))
	}, 4*time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	require.True(t, freshSent, "the blocked legacy action ID must not complete the workflow before the fresh approval")
	require.Equal(t, 1, published)
	env.AssertExpectations(t)
}

func TestManualWorkflowConsumesRejectedReplaceSignalActionID(t *testing.T) {
	env := newWorkflowEnv(t)
	initial := sevenSlotPlan()
	replacement := sevenSlotPlan()
	replacement.Revision = 2
	replacement.ParentRevision = 1
	replacement.IdempotencyKey = "plan-key-2"
	for _, slot := range initial.Slots {
		slot := slot
		if slot.ID == "scene-2" {
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlotRevision(slot.ID, 1, 1)).
				Return(imageagent.SlotExecutionResult{}, sdktemporal.NewNonRetryableApplicationError("failed", "slot_rejected", nil)).Once()
		} else {
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlotRevision(slot.ID, 1, 1)).
				Return(successfulSlotResult(slot.ID, 1), nil).Once()
		}
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlotRevision(slot.ID, 1, 2)).
			Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	planWrites := 0
	env.OnActivity(activityPersistPlanRevision, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { planWrites++ }).Return(nil).Once()
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).Return(nil).Once()
	rejected := ReplacePlanSignal{
		RunID: "run-1", ExpectedRevision: 1, Plan: replacement, ActorID: "user-a", ActionID: "replace-too-early",
	}
	oldActionApplied := false
	freshSent := false
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(signalReplacePlan, rejected) }, 0)
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(signalReplacePlan, rejected) }, time.Second)
	env.RegisterDelayedCallback(func() {
		encoded, err := env.QueryWorkflow(QueryWorkflowProjection)
		require.NoError(t, err)
		var projection WorkflowResult
		require.NoError(t, encoded.Get(&projection))
		oldActionApplied = projection.Plan.Revision == 2
		fresh := rejected
		fresh.ActionID = "replace-after-block"
		freshSent = true
		env.SignalWorkflow(signalReplacePlan, fresh)
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		encoded, err := env.QueryWorkflow(QueryWorkflowProjection)
		require.NoError(t, err)
		var projection WorkflowResult
		require.NoError(t, encoded.Get(&projection))
		env.SignalWorkflow(signalApproveResults, ApproveResultsSignal{
			RunID: "run-1", PlanRevision: 2, ResultDigest: projection.ResultDigest,
			ActorID: "user-a", ActionID: "approve-replacement-after-tombstone",
		})
	}, 3*time.Second)
	input := manualWorkflowInput(initial)
	input.WaitForCommands = true

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	require.True(t, freshSent)
	require.False(t, oldActionApplied, "a replacement Signal rejected before blocked must remain consumed")
	require.Equal(t, 1, planWrites)
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCompleted, result.Status)
	require.EqualValues(t, 2, result.Plan.Revision)
	env.AssertExpectations(t)
}

func TestManualWorkflowConsumesRejectedRetrySignalActionID(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		if slot.ID == "scene-2" {
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
				Return(imageagent.SlotExecutionResult{}, sdktemporal.NewNonRetryableApplicationError("failed", "slot_rejected", nil)).Once()
			continue
		}
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	providerRetryCalls := 0
	env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot("scene-2", 2)).
		Run(func(mock.Arguments) { providerRetryCalls++ }).Return(successfulSlotResult("scene-2", 2), nil).Once()
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).Return(nil).Once()
	rejected := RetrySlotSignal{
		RunID: "run-1", PlanRevision: 1, SlotID: "scene-2", ActorID: "attacker", ActionID: "retry-rejected-actor",
	}
	oldActionApplied := false
	freshSent := false
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(signalRetrySlot, rejected) }, time.Second)
	env.RegisterDelayedCallback(func() {
		corrected := rejected
		corrected.ActorID = "user-a"
		env.SignalWorkflow(signalRetrySlot, corrected)
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		oldActionApplied = providerRetryCalls != 0
		fresh := rejected
		fresh.ActorID = "user-a"
		fresh.ActionID = "retry-fresh-action"
		freshSent = true
		env.SignalWorkflow(signalRetrySlot, fresh)
	}, 3*time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalApproveResults, validApproval("approve-after-retry-tombstone"))
	}, 4*time.Second)
	input := manualWorkflowInput(plan)
	input.WaitForCommands = true

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	require.True(t, freshSent)
	require.False(t, oldActionApplied, "a rejected retry Signal action ID must not become valid after its actor changes")
	require.Equal(t, 1, providerRetryCalls)
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCompleted, result.Status)
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
			freshSent := false
			env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).Run(func(mock.Arguments) { published++ }).Return(nil).Once()
			invalid := validApproval("approve-invalid")
			test.mutate(&invalid)
			env.RegisterDelayedCallback(func() {
				env.SignalWorkflow(signalApproveResults, invalid)
			}, time.Second)
			env.RegisterDelayedCallback(func() {
				env.SignalWorkflow(signalApproveResults, validApproval("approve-invalid"))
			}, 1500*time.Millisecond)
			env.RegisterDelayedCallback(func() {
				require.Zero(t, published)
				freshSent = true
				env.SignalWorkflow(signalApproveResults, validApproval("approve-valid"))
			}, 2*time.Second)

			env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

			require.NoError(t, env.GetWorkflowError())
			require.True(t, freshSent, "a rejected approval action ID must remain consumed after its fields are corrected")
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
		require.Equal(t, plan, projection.Plan)
		require.Len(t, projection.Slots, len(plan.Slots))
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

func TestManualWorkflowConsumesRejectedCancelSignalActionID(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	input := manualWorkflowInput(plan)
	input.MaxConcurrentSlots = 1
	env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot("slot-1", 1)).
		After(10*time.Minute).Return(successfulSlotResult("slot-1", 1), nil).Once()
	rejected := CancelSignal{RunID: "run-1", PlanRevision: 1, ActorID: "attacker", ActionID: "cancel-rejected-actor"}
	freshSent := false
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(signalCancel, rejected) }, 0)
	env.RegisterDelayedCallback(func() {
		corrected := rejected
		corrected.ActorID = "user-a"
		env.SignalWorkflow(signalCancel, corrected)
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		fresh := rejected
		fresh.ActorID = "user-a"
		fresh.ActionID = "cancel-fresh-action"
		freshSent = true
		env.SignalWorkflow(signalCancel, fresh)
	}, 2*time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	require.True(t, freshSent, "the rejected cancel action ID must not cancel before the fresh action arrives")
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCancelled, result.Status)
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
	require.Equal(t, input.Plan, result.Plan)
	require.Empty(t, result.CompletedSlotIDs)
	require.Zero(t, started)
	require.Equal(t, 1, cancelledWrites)
	env.AssertExpectations(t)
}

func TestManualWorkflowSerializesCancelAfterInFlightParentTransition(t *testing.T) {
	env := newWorkflowEnv(t)
	input := manualWorkflowInput(sevenSlotPlan())
	input.MaxConcurrentSlots = 1
	started := 0
	var mu sync.Mutex
	durableStatus := imageagent.RunStatusPlanning
	persisted := []imageagent.RunStatus{}
	env.SetOnChildWorkflowStartedListener(func(*workflow.Info, workflow.Context, sdkconverter.EncodedValues) { started++ })
	env.OnActivity(activityPersistRunState, mock.Anything, mock.MatchedBy(func(input PersistRunStateActivityInput) bool {
		return input.Status == imageagent.RunStatusExecuting
	})).After(time.Minute).Run(func(mock.Arguments) {
		mu.Lock()
		defer mu.Unlock()
		durableStatus = imageagent.RunStatusExecuting
		persisted = append(persisted, imageagent.RunStatusExecuting)
	}).Return(nil).Once()
	env.OnActivity(activityPersistRunState, mock.Anything, mock.MatchedBy(func(input PersistRunStateActivityInput) bool {
		return input.Status == imageagent.RunStatusCancelled
	})).Run(func(mock.Arguments) {
		mu.Lock()
		defer mu.Unlock()
		durableStatus = imageagent.RunStatusCancelled
		persisted = append(persisted, imageagent.RunStatusCancelled)
	}).Return(nil).Once()
	cancel := CancelSignal{RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "cancel-during-executing-persist"}
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(signalCancel, cancel) }, 0)

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCancelled, result.Status)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, imageagent.RunStatusCancelled, durableStatus)
	require.Equal(t, []imageagent.RunStatus{imageagent.RunStatusExecuting, imageagent.RunStatusCancelled}, persisted)
	require.Zero(t, started)
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
	for _, slotID := range []string{"slot-1", "slot-2"} {
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slotID, 1)).
			After(time.Minute).
			Return(successfulSlotResult(slotID, 1), nil).
			Once()
	}
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
	require.Equal(t, plan, result.Plan)
	require.Equal(t, []string{"slot-1"}, result.CompletedSlotIDs)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{"slot-1", "slot-2"}, started)
	require.Equal(t, []string{"slot-1"}, persisted)
	encoded, err := env.QueryWorkflow(QueryWorkflowProjection)
	require.NoError(t, err)
	var projection WorkflowResult
	require.NoError(t, encoded.Get(&projection))
	require.Equal(t, imageagent.RunStatusCancelled, projection.Status)
	require.Equal(t, plan, projection.Plan)
	require.Equal(t, []string{"slot-1"}, projection.CompletedSlotIDs)
}

func TestManualWorkflowCancelUpdateAcksAfterCancelledProjectionPersistence(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	input := manualWorkflowInput(plan)
	input.MaxConcurrentSlots = 1
	env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot("slot-1", 1)).
		After(10*time.Minute).
		Return(successfulSlotResult("slot-1", 1), nil).
		Once()
	cancelledPersisted := false
	env.OnActivity(activityPersistRunState, mock.Anything, mock.MatchedBy(func(input PersistRunStateActivityInput) bool {
		return input.Status == imageagent.RunStatusCancelled
	})).Run(func(mock.Arguments) { cancelledPersisted = true }).Return(nil).Once()
	env.OnActivity(activityPersistRunState, mock.Anything, mock.Anything).Return(nil)
	var rejected, completedErr error
	completed := false
	cancelUpdate := CancelSignal{RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "cancel-update-1"}
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalCancel, "cancel-request-1", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { rejected = err },
			OnAccept: func() {},
			OnComplete: func(_ interface{}, err error) {
				completed = true
				completedErr = err
				require.True(t, cancelledPersisted)
				encoded, queryErr := env.QueryWorkflow(QueryWorkflowProjection)
				require.NoError(t, queryErr)
				var projection WorkflowResult
				require.NoError(t, encoded.Get(&projection))
				require.Equal(t, imageagent.RunStatusCancelled, projection.Status)
				require.Equal(t, plan, projection.Plan)
				require.Empty(t, imageagent.AllowedActions(imageagent.Run{Mode: imageagent.RunModeManual, Status: projection.Status}))
			},
		}, cancelUpdate)
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalCancel, CancelSignal{RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "cancel-red-cleanup"})
	}, 2*time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	require.Nil(t, rejected)
	require.True(t, completed)
	require.NoError(t, completedErr)
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCancelled, result.Status)
	require.Equal(t, plan, result.Plan)
}

func TestManualWorkflowCancelUpdateResumesAfterTerminalPersistenceFailure(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	input := manualWorkflowInput(plan)
	input.MaxConcurrentSlots = len(plan.Slots)
	var mu sync.Mutex
	childCompletions := 0
	env.SetOnChildWorkflowCompletedListener(func(_ *workflow.Info, _ sdkconverter.EncodedValue, _ error) {
		mu.Lock()
		defer mu.Unlock()
		childCompletions++
	})
	for _, slot := range plan.Slots {
		slot := slot
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
			After(time.Second).
			Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	cancelled := mock.MatchedBy(func(input PersistRunStateActivityInput) bool {
		return input.Status == imageagent.RunStatusCancelled
	})
	cancelPersistCalls := 0
	logicalCancelledWrites := 0
	durableStatus := imageagent.RunStatusExecuting
	forbiddenTransitions := 0
	env.OnActivity(activityPersistRunState, mock.Anything, cancelled).
		Run(func(mock.Arguments) {
			mu.Lock()
			defer mu.Unlock()
			cancelPersistCalls++
			logicalCancelledWrites = 1
			durableStatus = imageagent.RunStatusCancelled
		}).
		Return(sdktemporal.NewNonRetryableApplicationError("cancel write reported failure", "terminal_test_failure", nil)).Once()
	env.OnActivity(activityPersistRunState, mock.Anything, cancelled).
		Run(func(mock.Arguments) {
			mu.Lock()
			defer mu.Unlock()
			cancelPersistCalls++
			durableStatus = imageagent.RunStatusCancelled
		}).Return(nil).Once()
	env.OnActivity(activityPersistRunState, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			persist := activityInputFromArgs[PersistRunStateActivityInput](t, args)
			mu.Lock()
			defer mu.Unlock()
			if persist.Status == imageagent.RunStatusBlocked || persist.Status == imageagent.RunStatusAwaitingFinalApproval {
				forbiddenTransitions++
			}
			durableStatus = persist.Status
		}).Return(nil)
	command := CancelSignal{RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "cancel-resume"}
	var firstErr, resumedErr, conflictingErr error
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalCancel, "cancel-resume-first", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { firstErr = err }, OnAccept: func() {},
			OnComplete: func(_ interface{}, errorValue error) {
				firstErr = errorValue
				encoded, queryErr := env.QueryWorkflow(QueryWorkflowProjection)
				require.NoError(t, queryErr)
				var projection WorkflowResult
				require.NoError(t, encoded.Get(&projection))
				require.NotEqual(t, imageagent.RunStatusCancelled, projection.Status)
			},
		}, command)
	}, 100*time.Millisecond)
	env.RegisterDelayedCallback(func() {
		require.Error(t, firstErr)
		mu.Lock()
		completedChildren := childCompletions
		persistedStatus := durableStatus
		blockedTransitions := forbiddenTransitions
		mu.Unlock()
		require.Equal(t, len(plan.Slots), completedChildren, "all children must finish before the exact terminal retry")
		require.Equal(t, imageagent.RunStatusCancelled, persistedStatus)
		require.Zero(t, blockedTransitions, "parent must not overwrite ambiguous durable cancellation")
		encoded, queryErr := env.QueryWorkflow(QueryWorkflowProjection)
		require.NoError(t, queryErr)
		var projection WorkflowResult
		require.NoError(t, encoded.Get(&projection))
		require.NotEqual(t, imageagent.RunStatusCancelled, projection.Status, "ambiguous failure must not commit the public projection")
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		conflict := command
		conflict.PlanRevision = 2
		env.UpdateWorkflow(signalCancel, "cancel-resume-conflict", &testsuite.TestUpdateCallback{
			OnReject:   func(err error) { conflictingErr = err },
			OnAccept:   func() { require.Fail(t, "conflicting pending cancel must be rejected") },
			OnComplete: func(interface{}, error) {},
		}, conflict)
	}, 2100*time.Millisecond)
	env.RegisterDelayedCallback(func() {
		conflictingReuse := command
		conflictingReuse.PlanRevision = 2
		env.SignalWorkflow(signalCancel, conflictingReuse)
		competing := command
		competing.ActionID = "cancel-signal-competing"
		env.SignalWorkflow(signalCancel, competing)
	}, 2200*time.Millisecond)
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalCancel, "cancel-resume-second", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { resumedErr = err }, OnAccept: func() {},
			OnComplete: func(_ interface{}, errorValue error) {
				resumedErr = errorValue
				mu.Lock()
				persistedStatus := durableStatus
				mu.Unlock()
				encoded, queryErr := env.QueryWorkflow(QueryWorkflowProjection)
				require.NoError(t, queryErr)
				var projection WorkflowResult
				require.NoError(t, encoded.Get(&projection))
				require.Equal(t, persistedStatus, projection.Status)
			},
		}, command)
	}, 3*time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	require.Error(t, firstErr)
	require.NoError(t, resumedErr)
	require.Error(t, conflictingErr)
	mu.Lock()
	require.Equal(t, 2, cancelPersistCalls)
	require.Equal(t, 1, logicalCancelledWrites)
	require.Zero(t, forbiddenTransitions)
	require.Equal(t, len(plan.Slots), childCompletions)
	require.Equal(t, imageagent.RunStatusCancelled, durableStatus)
	mu.Unlock()
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCancelled, result.Status)
	require.Equal(t, plan, result.Plan)
	require.Empty(t, result.CompletedSlotIDs, "a slot result rejected by the terminal fence must not enter the projection")
	env.AssertExpectations(t)
}

func TestWorkflowEffectOwnerFencesEveryTerminalRunStatusBeforeExecution(t *testing.T) {
	for _, status := range []imageagent.RunStatus{
		imageagent.RunStatusCompleted,
		imageagent.RunStatusFailed,
		imageagent.RunStatusCancelled,
	} {
		t.Run(string(status), func(t *testing.T) {
			env := newWorkflowEnv(t)
			terminalCalls := 0
			forbiddenCalls := 0
			env.OnActivity(activityPersistRunState, mock.Anything, mock.MatchedBy(func(input PersistRunStateActivityInput) bool {
				return input.Status == status
			})).Run(func(mock.Arguments) { terminalCalls++ }).
				After(time.Second).
				Return(sdktemporal.NewNonRetryableApplicationError("durable terminal write returned an ambiguous error", "terminal_test_failure", nil)).Once()
			env.OnActivity(activityPersistRunState, mock.Anything, mock.MatchedBy(func(input PersistRunStateActivityInput) bool {
				return input.Status == status
			})).Run(func(mock.Arguments) { terminalCalls++ }).Return(nil).Once()
			env.OnActivity(activityPersistRunState, mock.Anything, mock.Anything).
				Run(func(mock.Arguments) { forbiddenCalls++ }).Return(nil)

			env.ExecuteWorkflow(workflowEffectOwnerFenceWorkflow, status)

			require.NoError(t, env.GetWorkflowError())
			var result workflowEffectOwnerFenceResult
			require.NoError(t, env.GetWorkflowResult(&result))
			require.ErrorContains(t, errors.New(result.FirstError), "ambiguous error")
			require.ErrorContains(t, errors.New(result.NonTerminalError), "terminal effect fence")
			require.ErrorContains(t, errors.New(result.DifferentActionError), "terminal effect fence")
			require.ErrorContains(t, errors.New(result.DifferentTerminalError), "terminal effect fence")
			require.Empty(t, result.ExactRetryError)
			require.ErrorContains(t, errors.New(result.AfterSuccessError), "terminal effect fence")
			require.Equal(t, 2, terminalCalls)
			require.Zero(t, forbiddenCalls)
			env.AssertExpectations(t)
		})
	}
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

func TestManualWorkflowQueryRecoversPersistedPartialSlotProgress(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	input := manualWorkflowInput(plan)
	input.MaxConcurrentSlots = 1
	env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot("slot-1", 1)).
		After(time.Minute).
		Return(successfulSlotResult("slot-1", 1), nil).
		Once()
	env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot("slot-2", 1)).
		After(10*time.Minute).
		Return(successfulSlotResult("slot-2", 1), nil).
		Once()
	env.RegisterDelayedCallback(func() {
		encoded, err := env.QueryWorkflow(QueryWorkflowProjection)
		require.NoError(t, err)
		var projection WorkflowResult
		require.NoError(t, encoded.Get(&projection))
		require.Equal(t, imageagent.RunStatusExecuting, projection.Status)
		require.Equal(t, []string{"slot-1"}, projection.CompletedSlotIDs)
		require.Equal(t, imageagent.SlotStatusAccepted, projection.Slots[0].Slot.Status)
		require.Equal(t, imageagent.SlotStatusPending, projection.Slots[1].Slot.Status)
	}, 61*time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalCancel, CancelSignal{
			RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "cancel-after-query",
		})
	}, 62*time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCancelled, result.Status)
	require.Equal(t, []string{"slot-1"}, result.CompletedSlotIDs)
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
	require.NoError(t, repository.UpdateRun(context.Background(), imageagent.RunScope{TenantID: "tenant-a", RunID: "run-1"}, run.Version, imageagent.RunMutation{
		Status: imageagent.RunStatusAwaitingFinalApproval, CurrentNode: "approve_results", ActivePlanRevision: 1,
	}))
	workflows.projection = imageagent.WorkflowProjection{Status: imageagent.RunStatusAwaitingFinalApproval, Plan: workflows.starts[0].Plan, ResultDigest: sevenSlotResultDigest}
	require.ErrorIs(t, service.ApproveResults(ctx, "run-1", 1, "  "+sevenSlotResultDigest+" ", "approve-service-space"), imageagent.ErrCommandBlocked)
	require.Empty(t, workflows.approvals)
	require.NoError(t, service.ApproveResults(ctx, "run-1", 1, sevenSlotResultDigest, "approve-service-1"))
	require.Len(t, workflows.approvals, 1)
	require.Equal(t, sevenSlotResultDigest, workflows.approvals[0].ResultDigest)
}

func TestTemporalClientUsesStableWorkflowAndProjectionQuery(t *testing.T) {
	raw := &recordingSDKClient{queryValue: imageagent.WorkflowProjection{Status: imageagent.RunStatusBlocked, Plan: sevenSlotPlan()}}
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

	projection, err := client.GetProjection(context.Background(), imageagent.RunScope{TenantID: "tenant-a", RunID: "run-1"}, start.Identity)
	require.NoError(t, err)
	require.Equal(t, raw.queryValue, projection)
	require.Equal(t, "image-agent:tenant-a:run-1", raw.queryWorkflowID)
	require.Equal(t, QueryWorkflowProjection, raw.queryType)

	require.ErrorContains(t, client.ApproveResults(context.Background(), imageagent.ApproveResultsCommand{RunID: "run-1", PlanRevision: 1, ResultDigest: sevenSlotResultDigest, ActorID: "attacker", ActionID: "spoofed", Identity: start.Identity}), "actor")
	require.ErrorContains(t, client.ApproveResults(context.Background(), imageagent.ApproveResultsCommand{RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "missing-digest", Identity: start.Identity}), "digest")
	require.ErrorContains(t, client.ApproveResults(context.Background(), imageagent.ApproveResultsCommand{RunID: "run-1", PlanRevision: 1, ResultDigest: " " + sevenSlotResultDigest, ActorID: "user-a", ActionID: "spaced-digest", Identity: start.Identity}), "digest")
}

func TestTemporalClientCommandsWaitForCompletedWorkflowUpdates(t *testing.T) {
	raw := &recordingSDKClient{}
	client := NewClient(raw)
	identity := imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}
	replacement := sevenSlotPlan()
	replacement.Revision = 2
	replacement.ParentRevision = 1
	replacement.IdempotencyKey = "plan-key-2"

	require.NoError(t, client.ReplacePlan(context.Background(), imageagent.ReplacePlanCommand{
		RunID: "run-1", ExpectedRevision: 1, Plan: replacement, ActorID: "user-a", ActionID: "replace-1", Identity: identity,
	}))
	require.NoError(t, client.RetrySlot(context.Background(), imageagent.RetrySlotCommand{
		RunID: "run-1", PlanRevision: 2, SlotID: "scene-2", ActorID: "user-a", ActionID: "retry-1", Identity: identity,
	}))
	require.NoError(t, client.ApproveResults(context.Background(), imageagent.ApproveResultsCommand{
		RunID: "run-1", PlanRevision: 2, ResultDigest: sevenSlotResultDigest, ActorID: "user-a", ActionID: "approve-1", Identity: identity,
	}))
	require.NoError(t, client.Cancel(context.Background(), imageagent.CancelRunCommand{
		RunID: "run-1", PlanRevision: 2, ActorID: "user-a", ActionID: "cancel-1", Identity: identity,
	}))

	require.Len(t, raw.updateOptions, 4)
	require.Equal(t, []string{"replace_plan", "retry_slot", "approve_results", "cancel"}, []string{
		raw.updateOptions[0].UpdateName, raw.updateOptions[1].UpdateName, raw.updateOptions[2].UpdateName, raw.updateOptions[3].UpdateName,
	})
	for _, options := range raw.updateOptions {
		require.Equal(t, "image-agent:tenant-a:run-1", options.WorkflowID)
		require.Equal(t, sdkclient.WorkflowUpdateStageCompleted, options.WaitForStage)
		require.NotEmpty(t, options.UpdateID)
		require.Len(t, options.Args, 1)
	}
	require.Equal(t, 4, raw.updateGetCalls)
	require.Empty(t, raw.signalName)
}

func TestTemporalClientMapsWorkflowUpdateErrorsToApplicationContracts(t *testing.T) {
	identity := imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}
	command := imageagent.CancelRunCommand{RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "cancel-1", Identity: identity}
	for _, tt := range []struct {
		name      string
		directErr error
		resultErr error
		want      error
	}{
		{name: "missing run", directErr: serviceerror.NewNotFound("missing"), want: imageagent.ErrRunNotFound},
		{name: "closed workflow", directErr: serviceerror.NewFailedPrecondition("workflow completed"), want: imageagent.ErrCommandBlocked},
		{name: "revision", resultErr: sdktemporal.NewNonRetryableApplicationError("stale", "imageagent_revision_conflict", nil), want: imageagent.ErrRevisionConflict},
		{name: "blocked", resultErr: sdktemporal.NewNonRetryableApplicationError("blocked", "imageagent_command_blocked", nil), want: imageagent.ErrCommandBlocked},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw := &recordingSDKClient{updateErr: tt.directErr, updateResultErr: tt.resultErr}
			err := NewClient(raw).Cancel(context.Background(), command)
			require.ErrorIs(t, err, tt.want)
		})
	}
}

func TestTemporalProjectionContractRoundTripsIntoApplicationProjection(t *testing.T) {
	workflowProjection := WorkflowResult{
		Status: imageagent.RunStatusAwaitingFinalApproval, Plan: sevenSlotPlan(),
		Slots:            []imageagent.SlotProjection{{Slot: sevenSlotPlan().Slots[0], Attempt: 1}},
		CompletedSlotIDs: []string{"slot-1"}, ResultDigest: sevenSlotResultDigest,
	}
	payloads, err := sdkconverter.GetDefaultDataConverter().ToPayloads(workflowProjection)
	require.NoError(t, err)
	var applicationProjection imageagent.WorkflowProjection
	require.NoError(t, sdkconverter.GetDefaultDataConverter().FromPayloads(payloads, &applicationProjection))
	require.Equal(t, workflowProjection.Status, applicationProjection.Status)
	require.Equal(t, workflowProjection.Plan, applicationProjection.Plan)
	require.Equal(t, workflowProjection.Slots, applicationProjection.Slots)
	require.Equal(t, workflowProjection.CompletedSlotIDs, applicationProjection.CompletedSlotIDs)
	require.Equal(t, workflowProjection.ResultDigest, applicationProjection.ResultDigest)
}

func TestTemporalPayloadEncodingPreservesLegacyDefaultGoKeys(t *testing.T) {
	input := manualWorkflowInput(sevenSlotPlan())
	payloads, err := sdkconverter.GetDefaultDataConverter().ToPayloads(input)
	require.NoError(t, err)
	require.Len(t, payloads.Payloads, 1)
	encoded := string(payloads.Payloads[0].Data)
	require.Contains(t, encoded, `"Plan":{"Revision":1`)
	require.Contains(t, encoded, `"Slots":[{"ID":"slot-1"`)
	require.NotContains(t, encoded, `"revision":1`)

	legacy := []byte(`{"RunID":"run-legacy","Mode":"manual","Identity":{"TenantID":"tenant-a","UserID":"user-a"},"Plan":{"Revision":1,"IdempotencyKey":"plan-key-1","SourceAssetIDs":["source-1"],"Slots":[{"ID":"slot-1","Role":"scene","SourceAssetIDs":["source-1"],"IdempotencyKey":"slot-key-1","Status":"pending"}],"CreatedBy":"user-a"},"MaxConcurrentSlots":1,"WaitForCommands":true}`)
	legacyPayloads := &commonpb.Payloads{Payloads: []*commonpb.Payload{{
		Metadata: map[string][]byte{"encoding": []byte("json/plain")},
		Data:     legacy,
	}}}
	var decoded WorkflowInput
	require.NoError(t, sdkconverter.GetDefaultDataConverter().FromPayloads(legacyPayloads, &decoded))
	require.Equal(t, "run-legacy", decoded.RunID)
	require.EqualValues(t, 1, decoded.Plan.Revision)
	require.Equal(t, "slot-1", decoded.Plan.Slots[0].ID)
}

func TestRegisterWorkerRegistersParentChildAndActivities(t *testing.T) {
	activities, err := NewActivities(ActivityDependencies{Repository: store.NewMemoryRepository(), SlotExecutor: &identityCheckingExecutor{t: t}, Publisher: &identityCheckingPublisher{t: t}})
	require.NoError(t, err)
	registrar := &recordingWorkerRegistrar{}

	require.NoError(t, RegisterWorker(registrar, activities))

	require.Equal(t, []string{workflowNameImageAgent, workflowNameImageSlot}, registrar.workflows)
	require.Equal(t, []string{activityExecuteSlot, activityPersistSlotResult, activityPersistRunState, activityPersistPlanRevision, activityPublishApproved}, registrar.activities)
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

type workflowEffectOwnerFenceResult struct {
	FirstError             string
	NonTerminalError       string
	DifferentActionError   string
	DifferentTerminalError string
	ExactRetryError        string
	AfterSuccessError      string
}

func workflowEffectOwnerFenceWorkflow(ctx workflow.Context, status imageagent.RunStatus) (workflowEffectOwnerFenceResult, error) {
	ctx = imageAgentActivityContext(ctx)
	owner := newWorkflowEffectOwner(ctx)
	input := manualWorkflowInput(sevenSlotPlan())
	node := string(status)
	firstDone := workflow.NewBufferedChannel(ctx, 1)
	workflow.Go(ctx, func(effectCtx workflow.Context) {
		firstDone.Send(effectCtx, workflowTestErrorString(
			owner.persistTerminalRunState(effectCtx, input, status, node, nil, "action-a"),
		))
	})
	if err := workflow.Sleep(ctx, 100*time.Millisecond); err != nil {
		return workflowEffectOwnerFenceResult{}, err
	}
	nonTerminalErr := owner.persistRunState(ctx, input, imageagent.RunStatusBlocked, "retry_slot", nil)
	var firstErr string
	firstDone.Receive(ctx, &firstErr)
	differentActionErr := owner.persistTerminalRunState(ctx, input, status, node, nil, "action-b")
	differentStatus := imageagent.RunStatusCancelled
	if status == imageagent.RunStatusCancelled {
		differentStatus = imageagent.RunStatusCompleted
	}
	differentTerminalErr := owner.persistTerminalRunState(ctx, input, differentStatus, string(differentStatus), nil, "action-a")
	exactRetryErr := owner.persistTerminalRunState(ctx, input, status, node, nil, "action-a")
	afterSuccessErr := owner.persistRunState(ctx, input, imageagent.RunStatusAwaitingFinalApproval, "approve_results", nil)
	return workflowEffectOwnerFenceResult{
		FirstError:             firstErr,
		NonTerminalError:       workflowTestErrorString(nonTerminalErr),
		DifferentActionError:   workflowTestErrorString(differentActionErr),
		DifferentTerminalError: workflowTestErrorString(differentTerminalErr),
		ExactRetryError:        workflowTestErrorString(exactRetryErr),
		AfterSuccessError:      workflowTestErrorString(afterSuccessErr),
	}, nil
}

func workflowTestErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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
		func(context.Context, PersistPlanRevisionActivityInput) error { return nil },
		sdkactivity.RegisterOptions{Name: activityPersistPlanRevision},
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
	return executeInputForSlotRevision(slotID, attempt, 1)
}

func executeInputForSlotRevision(slotID string, attempt int, revision int64) interface{} {
	return mock.MatchedBy(func(in ExecuteSlotActivityInput) bool {
		return in.RunID == "run-1" && in.Identity.TenantID == "tenant-a" && in.Identity.UserID == "user-a" &&
			in.PlanRevision == revision && in.Slot.ID == slotID && in.Attempt == attempt && in.IdempotencyKey == fmt.Sprintf("slot-key-%s:attempt:%d", slotID, attempt)
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
	starts     []imageagent.WorkflowStart
	approvals  []imageagent.ApproveResultsCommand
	projection imageagent.WorkflowProjection
}

func (c *recordingDomainWorkflowClient) StartManual(_ context.Context, input imageagent.WorkflowStart) error {
	c.starts = append(c.starts, input)
	return nil
}
func (c *recordingDomainWorkflowClient) GetProjection(context.Context, imageagent.RunScope, imageagent.ExecutionIdentity) (imageagent.WorkflowProjection, error) {
	return c.projection, nil
}
func (*recordingDomainWorkflowClient) ReplacePlan(context.Context, imageagent.ReplacePlanCommand) error {
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
	queryWorkflowID  string
	queryType        string
	queryValue       imageagent.WorkflowProjection
	updateOptions    []sdkclient.UpdateWorkflowOptions
	updateGetCalls   int
	updateErr        error
	updateResultErr  error
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

func (c *recordingSDKClient) QueryWorkflow(_ context.Context, workflowID, _ string, queryType string, _ ...interface{}) (sdkconverter.EncodedValue, error) {
	c.queryWorkflowID = workflowID
	c.queryType = queryType
	return projectionEncodedValue{value: c.queryValue}, nil
}

func (c *recordingSDKClient) UpdateWorkflow(_ context.Context, options sdkclient.UpdateWorkflowOptions) (sdkclient.WorkflowUpdateHandle, error) {
	c.updateOptions = append(c.updateOptions, options)
	if c.updateErr != nil {
		return nil, c.updateErr
	}
	return &recordingWorkflowUpdateHandle{client: c, workflowID: options.WorkflowID, updateID: options.UpdateID}, nil
}

type recordingWorkflowUpdateHandle struct {
	client     *recordingSDKClient
	workflowID string
	updateID   string
}

func (h *recordingWorkflowUpdateHandle) WorkflowID() string { return h.workflowID }
func (*recordingWorkflowUpdateHandle) RunID() string        { return "" }
func (h *recordingWorkflowUpdateHandle) UpdateID() string   { return h.updateID }
func (h *recordingWorkflowUpdateHandle) Get(context.Context, interface{}) error {
	h.client.updateGetCalls++
	return h.client.updateResultErr
}

type projectionEncodedValue struct{ value imageagent.WorkflowProjection }

func (projectionEncodedValue) HasValue() bool { return true }

func (v projectionEncodedValue) Get(target interface{}) error {
	projection, ok := target.(*imageagent.WorkflowProjection)
	if !ok {
		return fmt.Errorf("unexpected projection query target %T", target)
	}
	*projection = v.value
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
