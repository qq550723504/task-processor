package temporal

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
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
			OnAccept:   func() {},
			OnComplete: func(_ interface{}, err error) { secondRejected = err },
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
		}).Return(sdktemporal.NewNonRetryableApplicationError("atomic plan projection write failed ambiguously", "projection_test_failure", nil)).Once()
	env.OnActivity(activityPersistPlanRevision, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) {
			planPersistCalls++
			logicalPlanWrites++
			durablePlanRevision = 2
			durableStatus = imageagent.RunStatusExecuting
		}).Return(nil).Once()
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
			OnAccept:   func() {},
			OnComplete: func(_ interface{}, err error) { conflictingErr = err },
		}, conflict)
		competing := command
		competing.ActionID = "replace-competing"
		env.UpdateWorkflow(signalReplacePlan, "replace-resume-competing", &testsuite.TestUpdateCallback{
			OnReject:   func(err error) { competingErr = err },
			OnAccept:   func() {},
			OnComplete: func(_ interface{}, err error) { competingErr = err },
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
		env.UpdateWorkflow(updateResumeCommand, "replace-resume-second", &testsuite.TestUpdateCallback{
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
		}, ResumeCommandInput{RunID: "run-1", ActorID: "user-a", ActionID: command.ActionID})
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
	require.Equal(t, 2, planPersistCalls)
	require.Equal(t, 1, logicalPlanWrites)
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
		return input.Projection.Status == imageagent.RunStatusAwaitingFinalApproval
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
			OnAccept:   func() {},
			OnComplete: func(_ interface{}, err error) { secondRejected = err },
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
		return input.Projection.Status == imageagent.RunStatusAwaitingFinalApproval
	})
	approvalTransitionCalls := 0
	env.OnActivity(activityPersistRunState, mock.Anything, approvalTransition).
		Run(func(mock.Arguments) {
			approvalTransitionCalls++
			durableStatus = imageagent.RunStatusAwaitingFinalApproval
		}).Return(nil).Once()
	env.OnActivity(activityPersistRunState, mock.Anything, approvalTransition).
		Return(sdktemporal.NewNonRetryableApplicationError("redundant parent approval write must not run", "unexpected_duplicate_transition", nil)).Maybe()
	env.OnActivity(activityPersistRunState, mock.Anything, mock.Anything).Return(nil)
	command := RetrySlotSignal{RunID: "run-1", PlanRevision: 1, SlotID: "scene-2", ActorID: "user-a", ActionID: "retry-resume"}
	var firstErr, resumedErr, conflictingErr, wrongActorErr, wrongActionErr error
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
				require.NotNil(t, projection.PendingCommand)
				require.Equal(t, "retry-resume", projection.PendingCommand.ActionID)
				require.Equal(t, signalRetrySlot, projection.PendingCommand.Kind)
				require.Equal(t, string(updatePhaseRetryPersistResult), projection.PendingCommand.Phase)
				require.Equal(t, "scene-2", projection.PendingCommand.SlotID)
				require.Equal(t, "persistence_failed", projection.PendingCommand.FailureCode)
				require.Equal(t, "persistence", projection.PendingCommand.FailureCategory)
				require.NotEmpty(t, projection.PendingCommand.FailureMessage)
				require.NotNil(t, projection.PendingCommand.LastFailedAt)
				require.Equal(t, 1, projection.PendingCommand.Attempt)
			},
		}, command)
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		conflict := command
		conflict.SlotID = "slot-1"
		env.UpdateWorkflow(signalRetrySlot, "retry-resume-conflict", &testsuite.TestUpdateCallback{
			OnReject:   func(err error) { conflictingErr = err },
			OnAccept:   func() {},
			OnComplete: func(_ interface{}, err error) { conflictingErr = err },
		}, conflict)
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(updateResumeCommand, "retry-resume-wrong-actor", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { wrongActorErr = err }, OnAccept: func() {}, OnComplete: func(_ interface{}, err error) { wrongActorErr = err },
		}, ResumeCommandInput{RunID: "run-1", ActorID: "attacker", ActionID: "retry-resume"})
		env.UpdateWorkflow(updateResumeCommand, "retry-resume-wrong-action", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { wrongActionErr = err }, OnAccept: func() {}, OnComplete: func(_ interface{}, err error) { wrongActionErr = err },
		}, ResumeCommandInput{RunID: "run-1", ActorID: "user-a", ActionID: "unknown-action"})
	}, 2200*time.Millisecond)
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
		env.UpdateWorkflow(updateResumeCommand, "retry-resume-new-client", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { resumedErr = err }, OnAccept: func() {},
			OnComplete: func(_ interface{}, errorValue error) {
				resumedErr = errorValue
				encoded, queryErr := env.QueryWorkflow(QueryWorkflowProjection)
				require.NoError(t, queryErr)
				var projection WorkflowResult
				require.NoError(t, encoded.Get(&projection))
				require.Equal(t, durableStatus, projection.Status)
				require.Equal(t, durableSlotStatus, projection.Slots[2].Slot.Status)
				require.Nil(t, projection.PendingCommand)
			},
		}, ResumeCommandInput{RunID: "run-1", ActorID: "user-a", ActionID: "retry-resume"})
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
	require.Error(t, wrongActorErr)
	require.Error(t, wrongActionErr)
	require.Equal(t, 1, providerRetryCalls)
	require.Equal(t, 1, logicalSlotWrites)
	require.Equal(t, 1, approvalTransitionCalls)
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCompleted, result.Status)
	env.AssertExpectations(t)
}

func TestManualWorkflowMixedCommandsAtOneTickReserveExactlyOnePendingOwner(t *testing.T) {
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
		After(time.Minute).
		Run(func(mock.Arguments) { providerRetryCalls++ }).
		Return(successfulSlotResult("scene-2", 2), nil).Once()
	planWrites := 0
	env.OnActivity(activityPersistPlanRevision, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { planWrites++ }).Return(nil).Maybe()
	published := 0
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { published++ }).Return(nil).Once()

	retry := RetrySlotSignal{RunID: "run-1", PlanRevision: 1, SlotID: "scene-2", ActorID: "user-a", ActionID: "retry-owner"}
	var retryErr, replaceErr, cancelErr error
	replacement := sevenSlotPlan()
	replacement.Revision = 2
	replacement.ParentRevision = 1
	replacement.IdempotencyKey = "plan-key-competing"
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalRetrySlot, "retry-owner-update", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { retryErr = err }, OnAccept: func() {}, OnComplete: func(_ interface{}, err error) { retryErr = err },
		}, retry)
		env.UpdateWorkflow(signalReplacePlan, "replace-competing-update", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { replaceErr = err }, OnAccept: func() {}, OnComplete: func(_ interface{}, err error) { replaceErr = err },
		}, ReplacePlanSignal{RunID: "run-1", ExpectedRevision: 1, Plan: replacement, ActorID: "user-a", ActionID: "replace-competing"})
		env.UpdateWorkflow(signalCancel, "cancel-competing-update", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { cancelErr = err }, OnAccept: func() {}, OnComplete: func(_ interface{}, err error) { cancelErr = err },
		}, CancelSignal{RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "cancel-competing"})
		env.SignalWorkflow(signalApproveResults, validApproval("approve-competing-signal"))
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalApproveResults, validApproval("approve-after-single-owner"))
	}, 3*time.Minute)
	input := manualWorkflowInput(plan)
	input.WaitForCommands = true

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	require.NoError(t, retryErr)
	require.Error(t, replaceErr)
	require.Error(t, cancelErr)
	require.Equal(t, 1, providerRetryCalls)
	require.Zero(t, planWrites)
	require.Equal(t, 1, published)
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCompleted, result.Status)
	require.EqualValues(t, 1, result.Plan.Revision)
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
			OnAccept:   func() {},
			OnComplete: func(_ interface{}, err error) { conflictingRejected = err },
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

func TestApprovalUpdateValidatorKeepsCompletionChecksInHandler(t *testing.T) {
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

	t.Run("global ledger fails closed at its documented capacity", func(t *testing.T) {
		full := &workflowUpdateState{actions: make(map[string]*workflowUpdateRecord, maxActionLedgerEntries)}
		for index := 0; index < maxActionLedgerEntries; index++ {
			full.actions[fmt.Sprintf("tombstone-%d", index)] = &workflowUpdateRecord{fingerprint: "rejected", ingressState: signalIngressRejected}
		}
		_, err := full.validateAction("one-too-many", "fingerprint")
		assertType(t, err, updateErrorCommandBlocked)
		require.ErrorContains(t, err, "capacity")
	})

	t.Run("run", func(t *testing.T) {
		signal := newApproval("approve-wrong-run")
		signal.RunID = "run-other"
		require.NoError(t, state.validateApproveResults(signal))
		assertType(t, validateCommandOwner(input, signal.RunID, signal.ActorID, signal.ActionID), updateErrorRunNotFound)
	})
	t.Run("revision", func(t *testing.T) {
		signal := newApproval("approve-stale")
		signal.PlanRevision = 2
		require.NoError(t, state.validateApproveResults(signal))
		assertType(t, state.validateApproveResultsBusiness(signal), updateErrorRevisionConflict)
	})
	t.Run("actor", func(t *testing.T) {
		signal := newApproval("approve-wrong-actor")
		signal.ActorID = "attacker"
		require.NoError(t, state.validateApproveResults(signal))
		assertType(t, validateCommandOwner(input, signal.RunID, signal.ActorID, signal.ActionID), updateErrorRunNotFound)
	})
	t.Run("action", func(t *testing.T) {
		signal := newApproval(" ")
		assertType(t, state.validateApproveResults(signal), updateErrorCommandBlocked)
	})
	t.Run("state", func(t *testing.T) {
		signal := newApproval("approve-wrong-state")
		projection.Status = imageagent.RunStatusExecuting
		require.NoError(t, state.validateApproveResults(signal))
		assertType(t, state.validateApproveResultsBusiness(signal), updateErrorCommandBlocked)
		projection.Status = imageagent.RunStatusAwaitingFinalApproval
	})
	t.Run("digest whitespace", func(t *testing.T) {
		signal := newApproval("approve-spaced-digest")
		signal.ResultDigest = " " + sevenSlotResultDigest
		require.NoError(t, state.validateApproveResults(signal))
		assertType(t, state.validateApproveResultsBusiness(signal), updateErrorCommandBlocked)
	})
	t.Run("digest mismatch", func(t *testing.T) {
		signal := newApproval("approve-wrong-digest")
		signal.ResultDigest = "wrong"
		require.NoError(t, state.validateApproveResults(signal))
		assertType(t, state.validateApproveResultsBusiness(signal), updateErrorCommandBlocked)
	})
}

func TestRetryUpdateValidatorRejectsBusinessConflictsBeforeAcceptance(t *testing.T) {
	plan := sevenSlotPlan()
	input := manualWorkflowInput(plan)
	results := make([]SlotWorkflowResult, len(plan.Slots))
	blockedIndex := slotIndex(plan, "scene-2")
	results[blockedIndex] = SlotWorkflowResult{
		Execution: imageagent.SlotExecutionResult{SlotID: "scene-2", Attempt: 1},
		Status:    imageagent.SlotStatusBlocked,
		ErrorCode: "slot_workflow_failed",
	}
	projection := WorkflowResult{
		Status: imageagent.RunStatusBlocked,
		Plan:   plan,
		Slots:  slotProjections(plan, results),
		Block:  &imageagent.Block{Code: "slot_workflow_failed", SlotID: "scene-2"},
	}
	state := &workflowUpdateState{
		input: &input, projection: &projection, results: &results,
		actions: make(map[string]*workflowUpdateRecord),
	}
	valid := RetrySlotSignal{RunID: input.RunID, PlanRevision: plan.Revision, SlotID: "scene-2", ActorID: input.Identity.UserID, ActionID: "retry-valid"}
	require.NoError(t, state.validateRetrySlot(valid))

	stale := valid
	stale.ActionID = "retry-stale"
	stale.PlanRevision++
	require.ErrorContains(t, state.validateRetrySlot(stale), "revision is stale")

	wrongSlot := valid
	wrongSlot.ActionID = "retry-wrong-slot"
	wrongSlot.SlotID = "slot-1"
	require.ErrorContains(t, state.validateRetrySlot(wrongSlot), "current blocked slot")

	projection.Block.Code = imageagent.SlotPublicationOutcomeUnknownCode
	disallowed := valid
	disallowed.ActionID = "retry-disallowed"
	require.ErrorContains(t, state.validateRetrySlot(disallowed), "not permitted")

	fingerprint, err := updateFingerprint(signalRetrySlot, valid)
	require.NoError(t, err)
	state.actions[valid.ActionID] = &workflowUpdateRecord{fingerprint: fingerprint, ingressState: signalIngressAccepted, completed: true}
	require.NoError(t, state.validateRetrySlot(valid), "an exact accepted retry remains idempotently replayable after the projection advances")
}

func TestSafeCommandFailureClassificationNeverExposesRawErrors(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		phase    workflowUpdatePhase
		code     string
		category string
		message  string
	}{
		{name: "provider", phase: updatePhaseRetryExecuteChild, code: "provider_unavailable", category: "provider", message: "图片生成服务暂时不可用"},
		{name: "persistence", phase: updatePhaseReplacePersistPlan, code: "persistence_failed", category: "persistence", message: "运行状态保存暂时失败"},
		{name: "publication", phase: updatePhaseApprovalPublish, code: "publication_failed", category: "publication", message: "结果发布暂时失败"},
		{name: "unknown", phase: workflowUpdatePhase("future.phase"), code: "technical_failure", category: "technical", message: "上次操作遇到技术问题，可以恢复后继续"},
	} {
		t.Run(test.name, func(t *testing.T) {
			code, category, message := safeCommandFailure(test.phase)
			require.Equal(t, test.code, code)
			require.Equal(t, test.category, category)
			require.Equal(t, test.message, message)
			require.NotContains(t, message, "secret-token")
		})
	}
}

func TestSummarizeV3ResultsPreservesPublicationUnknownBlockCode(t *testing.T) {
	plan := imageagent.Plan{Revision: 1, Slots: []imageagent.Slot{{ID: "scene-1", Role: imageagent.SlotRoleScene}}}
	projection := summarizeResultsV3(plan, []SlotWorkflowV3Result{{Published: imageagent.SlotEffectV3PublishedResult{SlotID: "scene-1", Attempt: 1}, Status: imageagent.SlotStatusBlocked, ErrorCode: imageagent.SlotPublicationOutcomeUnknownCode}})
	require.Equal(t, imageagent.SlotPublicationOutcomeUnknownCode, projection.Block.Code)
	require.Equal(t, []imageagent.Action{imageagent.ActionEditPlan, imageagent.ActionCancel}, imageagent.AllowedActions(imageagent.Run{Mode: imageagent.RunModeManual, Status: projection.Status, Block: projection.Block}))
}

func TestV3InitialProductionSummaryPreservesExactBlockCodesAndActions(t *testing.T) {
	for _, tc := range []struct {
		name         string
		activityType string
		wantCode     string
		wantActions  []imageagent.Action
	}{
		{name: "provider unknown", activityType: slotProviderOutcomeUnknownCode, wantCode: imageagent.SlotProviderOutcomeUnknownCode, wantActions: []imageagent.Action{imageagent.ActionCancel}},
		{name: "staging unknown", activityType: slotStagingOutcomeUnknownCode, wantCode: imageagent.SlotStagingOutcomeUnknownCode, wantActions: []imageagent.Action{imageagent.ActionEditPlan, imageagent.ActionRetrySlot, imageagent.ActionCancel}},
		{name: "publication unknown", activityType: slotPublicationOutcomeUnknownCode, wantCode: imageagent.SlotPublicationOutcomeUnknownCode, wantActions: []imageagent.Action{imageagent.ActionEditPlan, imageagent.ActionCancel}},
		{name: "phase invalid", activityType: slotEffectPhaseInvalidCode, wantCode: imageagent.SlotEffectPhaseInvalidCode, wantActions: []imageagent.Action{imageagent.ActionCancel}},
		{name: "policy invalid", activityType: slotEffectPolicyInvalidCode, wantCode: imageagent.SlotEffectPolicyInvalidCode, wantActions: []imageagent.Action{imageagent.ActionCancel}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newV3BlockWorkflowEnv(t, func(input ExecuteSlotV3ActivityInput) error {
				if input.Slot.ID == "scene-2" {
					return sdktemporal.NewNonRetryableApplicationError("blocked v3 slot", tc.activityType, nil)
				}
				return nil
			})
			var persistedBlock *imageagent.Block
			env.OnActivity(activityPersistRunState, mock.Anything, mock.MatchedBy(func(input PersistRunStateActivityInput) bool {
				if input.Projection.Status == imageagent.RunStatusBlocked {
					persistedBlock = input.Projection.Block
				}
				return true
			})).Return(nil)

			env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(sevenSlotPlan()))

			require.NoError(t, env.GetWorkflowError())
			var result WorkflowResult
			require.NoError(t, env.GetWorkflowResult(&result))
			require.NotNil(t, persistedBlock)
			require.Equal(t, tc.wantCode, persistedBlock.Code)
			require.Equal(t, tc.wantCode, result.Block.Code)
			require.Equal(t, tc.wantActions, imageagent.AllowedActions(imageagent.Run{Mode: imageagent.RunModeManual, Status: result.Status, Block: result.Block}))
		})
	}
}

func TestV3InitialChildFailurePersistsRecoverableBlockedSlotIdentity(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetWorkerOptions(worker.Options{DeadlockDetectionTimeout: 5 * time.Second})
	env.RegisterWorkflowWithOptions(
		func(workflow.Context, SlotWorkflowV3Input) (SlotWorkflowV3Result, error) {
			return SlotWorkflowV3Result{}, sdktemporal.NewNonRetryableApplicationError("child crashed", "child_test_failure", nil)
		},
		workflow.RegisterOptions{Name: "ImageSlotWorkflowV3"},
	)
	var persistedMu sync.Mutex
	var persisted SlotWorkflowV3Result
	env.RegisterActivityWithOptions(func(_ context.Context, input PersistSlotResultV3ActivityInput) error {
		if input.Result.Published.SlotID == "" || input.Result.Published.Attempt <= 0 {
			return sdktemporal.NewNonRetryableApplicationError("empty recoverable slot identity", "invalid_test_projection", nil)
		}
		persistedMu.Lock()
		persisted = input.Result
		persistedMu.Unlock()
		return nil
	}, sdkactivity.RegisterOptions{Name: activityPersistSlotResultV3})
	env.RegisterActivityWithOptions(func(context.Context, PersistRunStateActivityInput) error { return nil }, sdkactivity.RegisterOptions{Name: activityPersistRunState})
	env.RegisterActivityWithOptions(func(context.Context, PersistWorkflowFailureActivityInput) error { return nil }, sdkactivity.RegisterOptions{Name: activityPersistWorkflowFailure})
	env.RegisterActivityWithOptions(func(context.Context, PersistWorkflowFailureV2ActivityInput) error { return nil }, sdkactivity.RegisterOptions{Name: activityPersistWorkflowFailureV2})
	env.OnGetVersion(slotExecutionWireV3Patch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	env.OnGetVersion(approvalActionIDV3Patch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	env.OnGetVersion(approvalPublicationWireV3Patch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	env.OnGetVersion(resultDigestV3Patch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	env.OnGetVersion(approvalPublicationScopePatch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	env.OnGetVersion(activityWireV2Patch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	env.OnGetVersion(commandIngressPlanPolicyPatch, workflow.DefaultVersion, 1).Return(workflow.Version(1))

	plan := imageagent.Plan{
		Revision: 1, IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1"}, CreatedBy: "user-a",
		Slots: []imageagent.Slot{{ID: "main-1", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-key-main-1", Status: imageagent.SlotStatusPending}},
	}
	env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

	require.NoError(t, env.GetWorkflowError())
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusBlocked, result.Status)
	require.Equal(t, "main-1", result.Block.SlotID)
	persistedMu.Lock()
	require.Equal(t, "main-1", persisted.Published.SlotID)
	require.Equal(t, 1, persisted.Published.Attempt)
	require.Equal(t, imageagent.SlotStatusBlocked, persisted.Status)
	require.Equal(t, "slot_workflow_failed", persisted.ErrorCode)
	persistedMu.Unlock()
}

func TestV3ProviderUnknownDeniesDirectRetry(t *testing.T) {
	env := newV3BlockWorkflowEnv(t, func(input ExecuteSlotV3ActivityInput) error {
		if input.Slot.ID != "scene-2" {
			return nil
		}
		return sdktemporal.NewNonRetryableApplicationError("provider outcome unknown", slotProviderOutcomeUnknownCode, nil)
	})
	var blocksMu sync.Mutex
	var persistedBlockCodes []string
	env.OnActivity(activityPersistRunState, mock.Anything, mock.MatchedBy(func(input PersistRunStateActivityInput) bool {
		if input.Projection.Status == imageagent.RunStatusBlocked && input.Projection.Block != nil {
			blocksMu.Lock()
			persistedBlockCodes = append(persistedBlockCodes, input.Projection.Block.Code)
			blocksMu.Unlock()
		}
		return true
	})).Return(nil)

	var retryRejected error
	retry := RetrySlotSignal{RunID: "run-1", PlanRevision: 1, SlotID: "scene-2", ActorID: "user-a", ActionID: "retry-v3-publication-unknown"}
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalRetrySlot, "retry-v3-publication-unknown-1", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { retryRejected = err }, OnAccept: func() {},
			OnComplete: func(_ interface{}, err error) { retryRejected = err },
		}, retry)
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalCancel, CancelSignal{RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "cancel-v3-retry-test"})
	}, 2*time.Second)
	input := manualWorkflowInput(sevenSlotPlan())
	input.WaitForCommands = true

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	require.Error(t, retryRejected)
	var applicationError *sdktemporal.ApplicationError
	require.ErrorAs(t, retryRejected, &applicationError)
	require.Equal(t, updateErrorCommandBlocked, applicationError.Type())
	blocksMu.Lock()
	require.Contains(t, persistedBlockCodes, imageagent.SlotProviderOutcomeUnknownCode)
	require.NotContains(t, persistedBlockCodes, "slot_failed")
	blocksMu.Unlock()
}

func newV3BlockWorkflowEnv(t *testing.T, blocked func(ExecuteSlotV3ActivityInput) error) *testsuite.TestWorkflowEnvironment {
	t.Helper()
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetWorkerOptions(worker.Options{DeadlockDetectionTimeout: 5 * time.Second})
	env.OnGetVersion(slotExecutionWireV3Patch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	env.OnGetVersion(approvalActionIDV3Patch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	env.OnGetVersion(approvalPublicationWireV3Patch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	env.OnGetVersion(resultDigestV3Patch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	env.OnGetVersion(approvalPublicationScopePatch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	env.OnGetVersion(activityWireV2Patch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	env.OnGetVersion(commandIngressPlanPolicyPatch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	env.RegisterWorkflow(ImageSlotWorkflowV3)
	env.RegisterActivityWithOptions(func(_ context.Context, input ExecuteSlotV3ActivityInput) (imageagent.SlotEffectV3PublishedResult, error) {
		if err := blocked(input); err != nil {
			return imageagent.SlotEffectV3PublishedResult{}, err
		}
		return imageagent.SlotEffectV3PublishedResult{
			SlotID: input.Slot.ID, Attempt: input.Attempt,
			Candidates: []imageagent.SlotEffectV3AssetCandidate{{
				AssetID: "candidate-" + input.Slot.ID, SourceAssetID: "source-1",
				DurableAsset: imageagent.DurableAssetIdentity{ObjectKey: fmt.Sprintf("image-agent/public/tenant-a/fc95297aa4f56781f0decb7d4bf59b1447f09b3611039b80188b1c6beb03ee6a/run-1/1/%s/%d/0-%s.png", input.Slot.ID, input.Attempt, v3SHA256), SHA256: v3SHA256},
			}},
		}, nil
	}, sdkactivity.RegisterOptions{Name: activityExecuteSlotV3})
	env.RegisterActivityWithOptions(func(context.Context, PersistSlotResultV3ActivityInput) error { return nil }, sdkactivity.RegisterOptions{Name: activityPersistSlotResultV3})
	env.RegisterActivityWithOptions(func(context.Context, PersistRunStateActivityInput) error { return nil }, sdkactivity.RegisterOptions{Name: activityPersistRunState})
	env.RegisterActivityWithOptions(func(context.Context, PersistPendingCommandActivityInput) error { return nil }, sdkactivity.RegisterOptions{Name: activityPersistPendingCommand})
	return env
}

func TestInvalidPersistedV3PolicySurvivesProjectionRefreshAndRejectsDirectRetry(t *testing.T) {
	for _, tc := range []struct {
		name string
		code string
		want string
	}{
		{name: "phase", code: "slot_effect_phase_invalid", want: "slot_effect_phase_invalid"},
		{name: "policy", code: "slot_effect_policy_invalid", want: "slot_effect_policy_invalid"},
		{name: "unknown v3 policy", code: "slot_effect_future_policy_invalid", want: "slot_effect_policy_invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := imageagent.Plan{
				Revision: 1, IdempotencyKey: "plan-invalid-policy", SourceAssetIDs: []string{"source-1"}, CreatedBy: "user-a",
				Slots: []imageagent.Slot{{ID: "scene-1", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "scene-key"}},
			}
			result := SlotWorkflowV3Result{Published: imageagent.SlotEffectV3PublishedResult{SlotID: "scene-1", Attempt: 1}, Status: imageagent.SlotStatusBlocked, ErrorCode: tc.code}
			summarized := summarizeResultsV3(plan, []SlotWorkflowV3Result{result})
			require.Equal(t, tc.want, summarized.Block.Code)

			for _, repository := range []struct {
				name string
				new  func(t *testing.T) imageagent.Repository
			}{
				{name: "memory", new: func(*testing.T) imageagent.Repository { return store.NewMemoryRepository() }},
				{name: "gorm", new: func(t *testing.T) imageagent.Repository {
					db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
					require.NoError(t, err)
					require.NoError(t, store.AutoMigrate(db))
					return store.NewGormRepository(db)
				}},
			} {
				t.Run(repository.name, func(t *testing.T) {
					repo := repository.new(t)
					run := imageagent.Run{
						ID: "run-invalid-policy-" + tc.name + "-" + repository.name, BusinessTaskID: "task-invalid-policy",
						TenantID: "tenant-a", UserID: "user-a", Mode: imageagent.RunModeManual, IdempotencyKey: "run-key-" + tc.name + "-" + repository.name,
						Status: summarized.Status, ActivePlanRevision: plan.Revision, Version: 1, Block: summarized.Block,
					}
					snapshot := imageagent.RunProjection{Run: run, Plan: plan, Slots: summarized.Slots, Actions: imageagent.AllowedActions(run)}
					_, err := repo.InitializeRun(context.Background(), imageagent.ProjectionInitialization{
						Scope: imageagent.ScopeForRun(run), Run: run, Plan: plan,
						Catalog:  imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/source.png"}}},
						Snapshot: snapshot, CommitID: "start:" + run.IdempotencyKey, EventType: "run.initialized", EventPayload: []byte(`{}`),
					})
					require.NoError(t, err)
					refreshed, err := repo.GetProjection(context.Background(), imageagent.ScopeForRun(run))
					require.NoError(t, err)
					require.Equal(t, tc.want, refreshed.Run.Block.Code)
					require.Equal(t, []imageagent.Action{imageagent.ActionCancel}, refreshed.Actions)

					input := WorkflowInput{RunID: run.ID, Mode: run.Mode, Identity: imageagent.ExecutionIdentity{TenantID: run.TenantID, UserID: run.UserID}, Plan: refreshed.Plan}
					results := []SlotWorkflowResult{{Execution: imageagent.SlotExecutionResult{SlotID: "scene-1", Attempt: 1}, Status: imageagent.SlotStatusBlocked, ErrorCode: tc.want}}
					projection := WorkflowResult{Status: refreshed.Run.Status, Block: refreshed.Run.Block, Plan: refreshed.Plan, Slots: refreshed.Slots}
					state := workflowUpdateState{input: &input, projection: &projection, results: &results}
					err = state.validateRetrySlotBusiness(RetrySlotSignal{RunID: run.ID, PlanRevision: plan.Revision, SlotID: "scene-1"})
					require.Error(t, err)
				})
			}
		})
	}
}

func TestRetrySlotBusinessRejectsUnknownExternalEffectsButKeepsSafeRetryPolicies(t *testing.T) {
	for _, tc := range []struct {
		code    string
		wantErr bool
	}{
		{code: "slot_failed"},
		{code: imageagent.SlotProviderOutcomeUnknownCode, wantErr: true},
		{code: imageagent.SlotStagingOutcomeUnknownCode},
		{code: imageagent.SlotPublicationOutcomeUnknownCode, wantErr: true},
	} {
		t.Run(tc.code, func(t *testing.T) {
			input := manualWorkflowInput(sevenSlotPlan())
			results := make([]SlotWorkflowResult, len(input.Plan.Slots))
			results[1] = SlotWorkflowResult{Execution: imageagent.SlotExecutionResult{SlotID: input.Plan.Slots[1].ID, Attempt: 1}, Status: imageagent.SlotStatusBlocked, ErrorCode: tc.code}
			projection := WorkflowResult{Status: imageagent.RunStatusBlocked, Plan: input.Plan, Block: &imageagent.Block{Code: tc.code, SlotID: input.Plan.Slots[1].ID}, Slots: slotProjections(input.Plan, results)}
			state := workflowUpdateState{input: &input, projection: &projection, results: &results}
			err := state.validateRetrySlotBusiness(RetrySlotSignal{RunID: input.RunID, PlanRevision: input.Plan.Revision, SlotID: input.Plan.Slots[1].ID})
			if tc.wantErr {
				require.Error(t, err)
				var applicationError *sdktemporal.ApplicationError
				require.ErrorAs(t, err, &applicationError)
				require.Equal(t, updateErrorCommandBlocked, applicationError.Type())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

type commandIngressExhaustionResult struct {
	UnknownError   string
	PendingError   string
	CompletedError string
	ActionCount    int
	Projection     WorkflowResult
}

func commandIngressExhaustionWorkflow(ctx workflow.Context) (commandIngressExhaustionResult, error) {
	ctx = imageAgentActivityContext(ctx)
	input := manualWorkflowInput(sevenSlotPlan())
	projection := WorkflowResult{
		Status: imageagent.RunStatusBlocked, Plan: input.Plan,
		Block: &imageagent.Block{Code: "slot_failed", SlotID: "scene-2"},
		Slots: slotProjections(input.Plan, nil),
	}
	results := make([]SlotWorkflowResult, len(input.Plan.Slots))
	state := newWorkflowUpdateState(ctx, &input, &projection, &results, newWorkflowEffectOwner(ctx))
	pendingCommand := RetrySlotSignal{RunID: input.RunID, PlanRevision: 1, SlotID: "scene-2", ActorID: input.Identity.UserID, ActionID: "pending-at-cap"}
	pendingFingerprint, _ := updateFingerprint(signalRetrySlot, pendingCommand)
	state.pendingActionID = pendingCommand.ActionID
	state.actions[pendingCommand.ActionID] = &workflowUpdateRecord{
		fingerprint: pendingFingerprint, kind: signalRetrySlot, command: pendingCommand,
		phase: updatePhaseRetryPersistResult, ingressState: signalIngressAccepted,
		attempt: 2, readyAttempt: true, businessValidated: true,
	}
	completedCommand := CancelSignal{RunID: input.RunID, PlanRevision: 1, ActorID: input.Identity.UserID, ActionID: "completed-at-cap"}
	completedFingerprint, _ := updateFingerprint(signalCancel, completedCommand)
	state.actions[completedCommand.ActionID] = &workflowUpdateRecord{
		fingerprint: completedFingerprint, completed: true, ingressState: signalIngressAccepted,
		acknowledgement: CommandAcknowledgement{RunID: input.RunID, PlanRevision: 1, ActionID: completedCommand.ActionID, Status: imageagent.RunStatusBlocked},
	}
	for index := 0; len(state.actions) < maxActionLedgerEntries; index++ {
		state.actions[fmt.Sprintf("tombstone-%d", index)] = &workflowUpdateRecord{fingerprint: "rejected", ingressState: signalIngressRejected}
	}

	_, _, unknownErr := state.prepareAction(ctx, "overflow", "overflow-fingerprint", updatePhaseCancelPersist, signalCancel, CancelSignal{})
	_, _, pendingErr := state.prepareAction(ctx, pendingCommand.ActionID, pendingFingerprint, updatePhaseRetryExecuteChild, signalRetrySlot, pendingCommand)
	_, _, completedErr := state.prepareAction(ctx, completedCommand.ActionID, completedFingerprint, updatePhaseCancelPersist, signalCancel, completedCommand)
	return commandIngressExhaustionResult{
		UnknownError: workflowTestErrorString(unknownErr), PendingError: workflowTestErrorString(pendingErr),
		CompletedError: workflowTestErrorString(completedErr), ActionCount: len(state.actions), Projection: state.projectionSnapshot(),
	}, nil
}

func TestWorkflowLedgerExhaustionPersistsBlockerWithoutConsumingUnknownAction(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(commandIngressExhaustionWorkflow)
	env.RegisterActivityWithOptions(
		func(context.Context, PersistPendingCommandActivityInput) error { return nil },
		sdkactivity.RegisterOptions{Name: activityPersistPendingCommand},
	)
	var persisted PersistPendingCommandActivityInput
	env.OnActivity(activityPersistPendingCommand, mock.Anything, mock.MatchedBy(func(input PersistPendingCommandActivityInput) bool {
		persisted = input
		return true
	})).Return(nil).Once()

	env.ExecuteWorkflow(commandIngressExhaustionWorkflow)

	require.NoError(t, env.GetWorkflowError())
	var result commandIngressExhaustionResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Contains(t, result.UnknownError, "capacity")
	require.Empty(t, result.PendingError, "known pending action remains addressable at capacity")
	require.Empty(t, result.CompletedError, "known completed acknowledgement remains addressable at capacity")
	require.Equal(t, maxActionLedgerEntries, result.ActionCount, "the unknown 1025th action must not enter the ledger")
	require.True(t, result.Projection.CommandIngress.Exhausted)
	require.Equal(t, "command_capacity_exhausted", result.Projection.CommandIngress.Reason)
	require.NotNil(t, result.Projection.PendingCommand)
	require.Equal(t, "pending-at-cap", result.Projection.PendingCommand.ActionID)
	require.True(t, persisted.CommandIngress.Exhausted)
	require.Equal(t, "command-ingress:exhausted", persisted.CommitID)
	require.Equal(t, "pending-at-cap", persisted.Receipt.ActionID)
	env.AssertExpectations(t)
}

type cancelAtFullLedgerResult struct {
	Acknowledgement CommandAcknowledgement
	Error           string
	ActionCount     int
	Superseded      bool
	Cancelled       bool
	Ingress         imageagent.CommandIngress
}

func cancelAtFullLedgerWorkflow(ctx workflow.Context, withFailedPending bool) (cancelAtFullLedgerResult, error) {
	ctx = imageAgentActivityContext(ctx)
	input := manualWorkflowInput(sevenSlotPlan())
	projection := WorkflowResult{
		Status: imageagent.RunStatusBlocked, Plan: input.Plan,
		Block: &imageagent.Block{Code: "slot_failed", SlotID: "scene-2"},
		Slots: slotProjections(input.Plan, nil),
	}
	results := make([]SlotWorkflowResult, len(input.Plan.Slots))
	state := newWorkflowUpdateState(ctx, &input, &projection, &results, newWorkflowEffectOwner(ctx))
	failedActionID := ""
	if withFailedPending {
		failedActionID = "failed-at-cap"
		failedAt := workflow.Now(ctx).UTC()
		state.pendingActionID = failedActionID
		state.actions[failedActionID] = &workflowUpdateRecord{
			fingerprint: "failed-fingerprint", kind: signalRetrySlot,
			command: RetrySlotSignal{RunID: input.RunID, PlanRevision: 1, SlotID: "scene-2", ActorID: input.Identity.UserID, ActionID: failedActionID},
			phase:   updatePhaseRetryPersistResult, ingressState: signalIngressAccepted,
			attempt: 1, businessValidated: true, lastFailedAt: &failedAt,
		}
	}
	for index := 0; len(state.actions) < maxActionLedgerEntries; index++ {
		state.actions[fmt.Sprintf("tombstone-%d", index)] = &workflowUpdateRecord{fingerprint: "rejected", ingressState: signalIngressRejected}
	}
	signal := CancelSignal{RunID: input.RunID, PlanRevision: 1, ActorID: input.Identity.UserID, ActionID: "cancel-at-cap"}
	acknowledgement, err := state.handleCancel(ctx, signal)
	return cancelAtFullLedgerResult{
		Acknowledgement: acknowledgement, Error: workflowTestErrorString(err), ActionCount: len(state.actions),
		Superseded: failedActionID != "" && state.actions[failedActionID].ingressState == signalIngressSuperseded,
		Cancelled:  state.cancelCommitted && state.cancelRequested, Ingress: state.commandIngress(),
	}, nil
}

func TestCancellationCanTerminateRunAfterOrdinaryActionLedgerIsFull(t *testing.T) {
	for _, withFailedPending := range []bool{false, true} {
		t.Run(fmt.Sprintf("failed_pending_%t", withFailedPending), func(t *testing.T) {
			env := newWorkflowEnv(t)
			env.RegisterWorkflow(cancelAtFullLedgerWorkflow)
			env.ExecuteWorkflow(cancelAtFullLedgerWorkflow, withFailedPending)

			require.NoError(t, env.GetWorkflowError())
			var result cancelAtFullLedgerResult
			require.NoError(t, env.GetWorkflowResult(&result))
			require.Empty(t, result.Error)
			require.Equal(t, imageagent.RunStatusCancelled, result.Acknowledgement.Status)
			require.Equal(t, maxActionLedgerEntries+1, result.ActionCount)
			require.Equal(t, withFailedPending, result.Superseded)
			require.True(t, result.Cancelled)
			require.LessOrEqual(t, result.Ingress.Used, result.Ingress.Limit, "the terminal capacity exemption must not violate the public quota contract")
		})
	}
}

type rejectedUpdateTombstoneResult struct {
	RejectedActionRecorded bool
	RejectedActionState    signalIngressState
	FreshActionCompleted   bool
}

func rejectedUpdateTombstoneWorkflow(ctx workflow.Context) (rejectedUpdateTombstoneResult, error) {
	ctx = imageAgentActivityContext(ctx)
	plan := sevenSlotPlan()
	input := manualWorkflowInput(plan)
	results := make([]SlotWorkflowResult, len(plan.Slots))
	for index, slot := range plan.Slots {
		results[index] = SlotWorkflowResult{Execution: successfulSlotResult(slot.ID, 1), Status: imageagent.SlotStatusAccepted}
	}
	projection := WorkflowResult{
		Status: imageagent.RunStatusAwaitingFinalApproval, Plan: plan,
		Slots: slotProjections(plan, results), ResultDigest: sevenSlotResultDigest,
	}
	state := newWorkflowUpdateState(ctx, &input, &projection, &results, newWorkflowEffectOwner(ctx))
	if err := state.register(ctx); err != nil {
		return rejectedUpdateTombstoneResult{}, err
	}
	if err := workflow.Sleep(ctx, 5*time.Second); err != nil {
		return rejectedUpdateTombstoneResult{}, err
	}
	rejected := state.actions["approval-rejected-once"]
	fresh := state.actions["approval-fresh"]
	return rejectedUpdateTombstoneResult{
		RejectedActionRecorded: rejected != nil,
		RejectedActionState: func() signalIngressState {
			if rejected == nil {
				return ""
			}
			return rejected.ingressState
		}(),
		FreshActionCompleted: fresh != nil && fresh.completed,
	}, nil
}

func TestRejectedUpdateConsumesActionIDInHandlerAndFreshIDCanSucceed(t *testing.T) {
	env := newWorkflowEnv(t)
	env.RegisterWorkflow(rejectedUpdateTombstoneWorkflow)
	wrongAccepted := false
	var wrongCompleted, sameCompleted, freshCompleted error
	wrong := validApproval("approval-rejected-once")
	wrong.ResultDigest = "wrong-digest"
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalApproveResults, "transport-wrong", &testsuite.TestUpdateCallback{
			OnReject:   func(err error) { wrongCompleted = err },
			OnAccept:   func() { wrongAccepted = true },
			OnComplete: func(_ interface{}, err error) { wrongCompleted = err },
		}, wrong)
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		same := validApproval("approval-rejected-once")
		env.UpdateWorkflow(signalApproveResults, "transport-same", &testsuite.TestUpdateCallback{
			OnReject:   func(err error) { sameCompleted = err },
			OnAccept:   func() {},
			OnComplete: func(_ interface{}, err error) { sameCompleted = err },
		}, same)
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		fresh := validApproval("approval-fresh")
		env.UpdateWorkflow(signalApproveResults, "transport-fresh", &testsuite.TestUpdateCallback{
			OnReject:   func(err error) { freshCompleted = err },
			OnAccept:   func() {},
			OnComplete: func(_ interface{}, err error) { freshCompleted = err },
		}, fresh)
	}, 3*time.Second)

	env.ExecuteWorkflow(rejectedUpdateTombstoneWorkflow)

	require.NoError(t, env.GetWorkflowError())
	var result rejectedUpdateTombstoneResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.True(t, wrongAccepted, "shape-valid update must reach the handler so its ActionID can be consumed")
	require.Error(t, wrongCompleted)
	require.Error(t, sameCompleted, "state changes cannot make a rejected ActionID reusable")
	require.NoError(t, freshCompleted)
	require.True(t, result.RejectedActionRecorded)
	require.Equal(t, signalIngressRejected, result.RejectedActionState)
	require.True(t, result.FreshActionCompleted)
}

func TestManualWorkflowRejectsPlanReferencesOutsideImmutableCatalogBeforeActivities(t *testing.T) {
	env := newWorkflowEnv(t)
	input := manualWorkflowInput(sevenSlotPlan())
	input.AssetCatalog = imageagent.AssetCatalog{}

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.ErrorContains(t, env.GetWorkflowError(), "not authorized")
	env.AssertNotCalled(t, activityExecuteSlot, mock.Anything, mock.Anything)
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
		return input.Projection.Status == imageagent.RunStatusCompleted
	})).Run(func(mock.Arguments) { completedPersisted = true }).Return(nil).Once()
	env.OnActivity(activityPersistRunState, mock.Anything, mock.Anything).Return(nil)
	var nonCanonicalRejected, validRejected, validErr error
	validCompleted := false
	env.RegisterDelayedCallback(func() {
		nonCanonical := validApproval("approve-update-space")
		nonCanonical.ResultDigest = " " + nonCanonical.ResultDigest
		env.UpdateWorkflow(signalApproveResults, "approve-space-request", &testsuite.TestUpdateCallback{
			OnReject:   func(err error) { nonCanonicalRejected = err },
			OnAccept:   func() {},
			OnComplete: func(_ interface{}, err error) { nonCanonicalRejected = err },
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
		return input.Projection.Status == imageagent.RunStatusCompleted
	})
	durableStatus := imageagent.RunStatusAwaitingFinalApproval
	env.OnActivity(activityPersistRunState, mock.Anything, completed).
		Return(sdktemporal.NewNonRetryableApplicationError("completed state write failed after publication", "terminal_test_failure", nil)).Once()
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
			OnAccept:   func() {},
			OnComplete: func(_ interface{}, err error) { conflictingErr = err },
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
		env.UpdateWorkflow(updateResumeCommand, "approve-resume-second", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { resumedErr = err }, OnAccept: func() {},
			OnComplete: func(_ interface{}, errorValue error) {
				resumedErr = errorValue
				encoded, queryErr := env.QueryWorkflow(QueryWorkflowProjection)
				require.NoError(t, queryErr)
				var projection WorkflowResult
				require.NoError(t, encoded.Get(&projection))
				require.Equal(t, durableStatus, projection.Status)
			},
		}, ResumeCommandInput{RunID: "run-1", ActorID: "user-a", ActionID: command.ActionID})
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

func TestManualWorkflowRejectsCancelAfterApprovalPublicationBeforeProjection(t *testing.T) {
	env := newWorkflowEnv(t)
	env.OnGetVersion(externalEffectFinalizationPatch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
			Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	publishCalls := 0
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { publishCalls++ }).Return(nil).Once()
	completed := mock.MatchedBy(func(input PersistRunStateActivityInput) bool {
		return input.Projection.Status == imageagent.RunStatusCompleted
	})
	env.OnActivity(activityPersistRunState, mock.Anything, completed).
		Return(sdktemporal.NewNonRetryableApplicationError("completed state write failed after publication", "terminal_test_failure", nil)).Once()
	env.OnActivity(activityPersistRunState, mock.Anything, completed).Return(nil).Once()
	cancelledWrites := 0
	env.OnActivity(activityPersistRunState, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		if activityInputFromArgs[PersistRunStateActivityInput](t, args).Projection.Status == imageagent.RunStatusCancelled {
			cancelledWrites++
		}
	}).Return(nil)

	command := validApproval("approve-after-publication")
	var approveErr, cancelErr, resumeErr error
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalApproveResults, "approve-after-publication", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { approveErr = err }, OnAccept: func() {},
			OnComplete: func(_ interface{}, err error) { approveErr = err },
		}, command)
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		require.Error(t, approveErr)
		env.UpdateWorkflow(signalCancel, "cancel-after-publication", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { cancelErr = err }, OnAccept: func() {},
			OnComplete: func(_ interface{}, err error) { cancelErr = err },
		}, CancelSignal{RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "cancel-after-publication"})
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(updateResumeCommand, "resume-approval-after-rejected-cancel", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { resumeErr = err }, OnAccept: func() {},
			OnComplete: func(_ interface{}, err error) { resumeErr = err },
		}, ResumeCommandInput{RunID: "run-1", ActorID: "user-a", ActionID: command.ActionID})
	}, 3*time.Second)
	env.RegisterDelayedCallback(func() { env.CancelWorkflow() }, 5*time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

	require.NoError(t, env.GetWorkflowError())
	require.Error(t, approveErr)
	require.ErrorContains(t, cancelErr, "approval publication is already committed")
	require.NoError(t, resumeErr)
	require.Equal(t, 1, publishCalls)
	require.Zero(t, cancelledWrites)
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCompleted, result.Status)
	env.AssertExpectations(t)
}

func TestManualWorkflowCancelSupersedesFailedApprovalCommand(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
			Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).
		Return(sdktemporal.NewNonRetryableApplicationError("publication permanently rejected", "publication_permanent", nil)).Once()
	env.OnActivity(activityPersistRunState, mock.Anything, mock.Anything).Return(nil)

	var approvalErr, cancelRejected, cancelErr error
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalApproveResults, "approve-permanent-failure", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { approvalErr = err }, OnAccept: func() {},
			OnComplete: func(_ interface{}, err error) { approvalErr = err },
		}, validApproval("approve-permanent-failure"))
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		require.Error(t, approvalErr)
		env.UpdateWorkflow(signalCancel, "cancel-after-approval-failure", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { cancelRejected = err }, OnAccept: func() {},
			OnComplete: func(_ interface{}, err error) { cancelErr = err },
		}, CancelSignal{RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "cancel-after-approval-failure"})
	}, 2*time.Second)

	input := manualWorkflowInput(plan)
	input.WaitForCommands = true
	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	require.Error(t, approvalErr)
	require.NoError(t, cancelRejected)
	require.NoError(t, cancelErr)
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCancelled, result.Status)
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

func TestManualWorkflowRejectedLegacySignalTombstonesWorkflowUpdateActionID(t *testing.T) {
	env := newWorkflowEnv(t)
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).Return(nil).Once()
	command := validApproval("approval-shared-with-update")
	var rejected error
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(signalApproveResults, command) }, 0)
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalApproveResults, "approval-after-rejected-signal", &testsuite.TestUpdateCallback{
			OnReject:   func(err error) { rejected = err },
			OnAccept:   func() {},
			OnComplete: func(_ interface{}, err error) { rejected = err },
		}, command)
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalApproveResults, validApproval("approval-fresh-after-tombstone"))
	}, 2*time.Second)

	env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))

	require.NoError(t, env.GetWorkflowError())
	require.Error(t, rejected)
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

func TestManualWorkflowWrongActorRetrySignalDoesNotPoisonActionID(t *testing.T) {
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
	require.True(t, oldActionApplied, "an identity-rejected command must not poison the legitimate owner's action ID")
	require.Equal(t, 1, providerRetryCalls)
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCompleted, result.Status)
	env.AssertExpectations(t)
}

func TestManualWorkflowReplacementRejectsAssetsOutsideImmutableCatalogAndConsumesActionID(t *testing.T) {
	env := newWorkflowEnv(t)
	initial := sevenSlotPlan()
	validReplacement := sevenSlotPlan()
	validReplacement.Revision = 2
	validReplacement.ParentRevision = 1
	validReplacement.IdempotencyKey = "plan-key-authorized-2"
	for _, slot := range initial.Slots {
		slot := slot
		if slot.ID == "scene-2" {
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).
				Return(imageagent.SlotExecutionResult{}, sdktemporal.NewNonRetryableApplicationError("failed", "slot_rejected", nil)).Once()
		} else {
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).Return(successfulSlotResult(slot.ID, 1), nil).Once()
		}
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlotRevision(slot.ID, 1, 2)).Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	planWrites := 0
	env.OnActivity(activityPersistPlanRevision, mock.Anything, mock.Anything).Run(func(mock.Arguments) { planWrites++ }).Return(nil).Once()
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).Return(nil).Once()
	unauthorized := validReplacement
	unauthorized.SourceAssetIDs = []string{"source-not-in-catalog"}
	unauthorized.Slots = append([]imageagent.Slot(nil), validReplacement.Slots...)
	unauthorized.Slots[0].SourceAssetIDs = []string{"source-not-in-catalog"}
	command := ReplacePlanSignal{
		RunID: "run-1", ExpectedRevision: 1, Plan: unauthorized, ActorID: "user-a", ActionID: "replace-unauthorized",
	}
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(signalReplacePlan, command) }, time.Second)
	env.RegisterDelayedCallback(func() {
		correctedSameID := command
		correctedSameID.Plan = validReplacement
		env.SignalWorkflow(signalReplacePlan, correctedSameID)
	}, 1500*time.Millisecond)
	env.RegisterDelayedCallback(func() {
		fresh := command
		fresh.ActionID = "replace-authorized"
		fresh.Plan = validReplacement
		env.SignalWorkflow(signalReplacePlan, fresh)
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalApproveResults, ApproveResultsSignal{
			RunID: "run-1", PlanRevision: 2, ResultDigest: sevenSlotResultDigest, ActorID: "user-a", ActionID: "approve-authorized-replacement",
		})
	}, 3*time.Second)
	input := manualWorkflowInput(initial)
	input.WaitForCommands = true

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 1, planWrites, "the direct workflow boundary must never persist the unauthorized replacement")
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.EqualValues(t, 2, result.Plan.Revision)
	require.Equal(t, imageagent.RunStatusCompleted, result.Status)
	env.AssertExpectations(t)
}

func TestReplacementBusinessValidationEnforcesBlockedActionPolicyForNewHistories(t *testing.T) {
	input := manualWorkflowInput(sevenSlotPlan())
	projection := WorkflowResult{
		Status: imageagent.RunStatusBlocked,
		Plan:   input.Plan,
		Block:  &imageagent.Block{Code: imageagent.BudgetQuoteUnavailableCode, SlotID: "scene-2"},
	}
	results := []SlotWorkflowResult(nil)
	replacement := sevenSlotPlan()
	replacement.Revision = 2
	replacement.ParentRevision = 1
	replacement.IdempotencyKey = "plan-key-2"
	signal := ReplacePlanSignal{RunID: input.RunID, ExpectedRevision: 1, Plan: replacement, ActorID: input.Identity.UserID, ActionID: "replace-cancel-only"}
	state := workflowUpdateState{input: &input, projection: &projection, results: &results, enforceIngressPlanPolicy: true}

	require.Error(t, state.validateReplacePlanBusiness(signal))
	state.enforceIngressPlanPolicy = false
	require.NoError(t, state.validateReplacePlanBusiness(signal), "historical workflows retain their recorded validation contract")
}

func TestManualWorkflowRejectsApprovalWithWrongActorOrDigest(t *testing.T) {
	for _, test := range []struct {
		name             string
		mutate           func(*ApproveResultsSignal)
		expectTombstoned bool
	}{
		{name: "wrong actor", mutate: func(signal *ApproveResultsSignal) { signal.ActorID = "attacker" }},
		{name: "missing digest", mutate: func(signal *ApproveResultsSignal) { signal.ResultDigest = "" }, expectTombstoned: true},
		{name: "mismatched digest", mutate: func(signal *ApproveResultsSignal) { signal.ResultDigest = "wrong-digest" }, expectTombstoned: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			env := newWorkflowEnv(t)
			plan := sevenSlotPlan()
			for _, slot := range plan.Slots {
				slot := slot
				env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).Return(successfulSlotResult(slot.ID, 1), nil).Once()
			}
			published := 0
			pendingReceiptWrites := 0
			env.OnActivity(activityPersistPendingCommand, mock.Anything, mock.Anything).
				Run(func(mock.Arguments) { pendingReceiptWrites++ }).Return(nil).Once()
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
			require.Equal(t, test.expectTombstoned, freshSent, "only an authenticated owner's business rejection may consume the action ID")
			require.Equal(t, 1, published)
			require.Equal(t, 1, pendingReceiptWrites, "a rejected command must never be projected as a recoverable pending command")
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
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(signalApproveResults, validApproval("approve-final-1")) }, 3*time.Minute)
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

func TestManualWorkflowWrongActorCancelSignalDoesNotPoisonActionID(t *testing.T) {
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
	require.False(t, freshSent, "the legitimate owner must be able to reuse an ID that an attacker could not reserve")
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
		if activityInputFromArgs[PersistRunStateActivityInput](t, args).Projection.Status == imageagent.RunStatusCancelled {
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
		return input.Projection.Status == imageagent.RunStatusExecuting
	})).After(time.Minute).Run(func(mock.Arguments) {
		mu.Lock()
		defer mu.Unlock()
		durableStatus = imageagent.RunStatusExecuting
		persisted = append(persisted, imageagent.RunStatusExecuting)
	}).Return(nil).Once()
	env.OnActivity(activityPersistRunState, mock.Anything, mock.MatchedBy(func(input PersistRunStateActivityInput) bool {
		return input.Projection.Status == imageagent.RunStatusCancelled
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

func TestManualWorkflowV3CancellationWaitsForStartedSlotFinalization(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetWorkerOptions(worker.Options{DeadlockDetectionTimeout: 5 * time.Second})
	var childStarted atomic.Bool
	var childFinalized atomic.Bool
	var eventMu sync.Mutex
	var events []string
	env.RegisterWorkflowWithOptions(
		func(ctx workflow.Context, input SlotWorkflowV3Input) (SlotWorkflowV3Result, error) {
			childStarted.Store(true)
			_ = workflow.NewTimer(ctx, time.Hour).Get(ctx, nil)
			finalizationCtx, cancelFinalization := workflow.NewDisconnectedContext(ctx)
			defer cancelFinalization()
			if err := workflow.NewTimer(finalizationCtx, time.Second).Get(finalizationCtx, nil); err != nil {
				return SlotWorkflowV3Result{}, err
			}
			childFinalized.Store(true)
			eventMu.Lock()
			events = append(events, "child_finalized")
			eventMu.Unlock()
			return SlotWorkflowV3Result{
				Published: imageagent.SlotEffectV3PublishedResult{SlotID: input.Slot.ID, Attempt: input.Attempt},
				Status:    imageagent.SlotStatusBlocked, ErrorCode: imageagent.SlotProviderOutcomeUnknownCode,
			}, nil
		},
		workflow.RegisterOptions{Name: "ImageSlotWorkflowV3"},
	)
	env.OnGetVersion(activityWireV2Patch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	env.OnGetVersion(slotExecutionWireV3Patch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	env.OnGetVersion(externalEffectFinalizationPatch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	var terminalMu sync.Mutex
	var terminalStatuses []imageagent.RunStatus
	env.RegisterActivityWithOptions(func(_ context.Context, input PersistRunStateActivityInput) error {
		if isTerminalRunStatus(input.Projection.Status) {
			terminalMu.Lock()
			terminalStatuses = append(terminalStatuses, input.Projection.Status)
			terminalMu.Unlock()
			eventMu.Lock()
			events = append(events, "terminal_cancelled")
			eventMu.Unlock()
		}
		return nil
	}, sdkactivity.RegisterOptions{Name: activityPersistRunState})
	env.RegisterActivityWithOptions(
		func(context.Context, PersistPendingCommandActivityInput) error { return nil },
		sdkactivity.RegisterOptions{Name: activityPersistPendingCommand},
	)
	env.RegisterDelayedCallback(func() {
		require.True(t, childStarted.Load())
		env.SignalWorkflow(signalCancel, CancelSignal{
			RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "cancel-after-provider-dispatch",
		})
	}, time.Second)

	plan := sevenSlotPlan()
	plan.Slots = plan.Slots[:1]
	input := manualWorkflowInput(plan)
	input.MaxConcurrentSlots = 1
	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	require.True(t, childFinalized.Load(), "the started child must report its provider outcome before the parent returns")
	eventMu.Lock()
	require.Equal(t, []string{"child_finalized", "terminal_cancelled"}, events)
	eventMu.Unlock()
	terminalMu.Lock()
	require.Equal(t, []imageagent.RunStatus{imageagent.RunStatusCancelled}, terminalStatuses)
	terminalMu.Unlock()
}

func TestImageSlotWorkflowV3RealActivityHeartbeatsWhileProviderIsInFlight(t *testing.T) {
	const runID = "run-v3-temporal-heartbeat"
	repository, activityInput := initializedSlotEffectV3Activity(t, runID)
	effects := &cancellationRejectingV3Repository{SlotExternalEffectV3Repository: repository.(imageagent.SlotExternalEffectV3Repository)}
	provider := &temporalCancellationStagedExecutor{
		recordingStagedExecutor: &recordingStagedExecutor{},
		waitTimeout:             2 * time.Second,
	}
	activities, err := NewActivities(ActivityDependencies{
		Repository: repository, SlotEffects: repository.(imageagent.SlotExternalEffectRepository), SlotExecutor: provider,
		SlotEffectsV3: effects, StagedSlotExecutor: provider, ArtifactStore: &recordingArtifactStore{},
		Publisher: &identityCheckingPublisher{t: t}, PublisherV3: &identityCheckingPublisher{t: t},
		PublicationOwner: func(context.Context) (string, error) { return "workflow-run/activity/1", nil },
	})
	require.NoError(t, err)

	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetWorkerOptions(worker.Options{DeadlockDetectionTimeout: 5 * time.Second})
	env.RegisterWorkflow(ImageSlotWorkflowV3)
	env.RegisterActivityWithOptions(activities.ExecuteSlotV3, sdkactivity.RegisterOptions{Name: activityExecuteSlotV3})
	var heartbeatCount atomic.Int32
	env.SetOnActivityHeartbeatListener(func(info *sdkactivity.Info, _ sdkconverter.EncodedValues) {
		if info.ActivityType.Name == activityExecuteSlotV3 {
			heartbeatCount.Add(1)
		}
	})
	env.ExecuteWorkflow(ImageSlotWorkflowV3, SlotWorkflowV3Input{
		RunID: activityInput.RunID, Identity: activityInput.Identity, PlanRevision: activityInput.PlanRevision,
		Slot: activityInput.Slot, Attempt: activityInput.Attempt, AssetCatalog: activityInput.AssetCatalog,
		ExecuteActivityName: activityExecuteSlotV3, ExternalEffectFinalization: true,
	})

	require.NoError(t, env.GetWorkflowError())
	var result SlotWorkflowV3Result
	require.NoError(t, env.GetWorkflowResult(&result))
	require.True(t, provider.Called(), "the real v3 activity must invoke the blocking provider")
	require.Positive(t, heartbeatCount.Load(), "the real Temporal activity must heartbeat while the provider is in flight")
	require.Equal(t, imageagent.SlotStatusBlocked, result.Status)
	require.Equal(t, imageagent.SlotProviderOutcomeUnknownCode, result.ErrorCode)
	require.False(t, effects.WriteSawCancelledContext(), "provider finalization must use a detached context")
	stored, err := repository.(imageagent.SlotExternalEffectV3Repository).GetSlotExternalEffectV3(context.Background(), v3Reservation(activityInput).Identity)
	require.NoError(t, err)
	require.Equal(t, imageagent.SlotEffectV3ProviderUnknown, stored.Phase)
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
		return input.Projection.Status == imageagent.RunStatusCancelled
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
		return input.Projection.Status == imageagent.RunStatusCancelled
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
			if persist.Projection.Status == imageagent.RunStatusBlocked || persist.Projection.Status == imageagent.RunStatusAwaitingFinalApproval {
				forbiddenTransitions++
			}
			durableStatus = persist.Projection.Status
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
			OnAccept:   func() {},
			OnComplete: func(_ interface{}, err error) { conflictingErr = err },
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
			env.OnGetVersion(externalEffectFinalizationPatch, workflow.DefaultVersion, 1).Return(workflow.DefaultVersion)
			terminalCalls := 0
			forbiddenCalls := 0
			env.OnActivity(activityPersistRunState, mock.Anything, mock.MatchedBy(func(input PersistRunStateActivityInput) bool {
				return input.Projection.Status == status
			})).Run(func(mock.Arguments) { terminalCalls++ }).
				After(time.Second).
				Return(sdktemporal.NewNonRetryableApplicationError("durable terminal write returned an ambiguous error", "terminal_test_failure", nil)).Once()
			env.OnActivity(activityPersistRunState, mock.Anything, mock.MatchedBy(func(input PersistRunStateActivityInput) bool {
				return input.Projection.Status == status
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

func TestWorkflowEffectOwnerDoesNotFenceFailedTerminalPersistence(t *testing.T) {
	env := newWorkflowEnv(t)
	env.OnGetVersion(externalEffectFinalizationPatch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	terminalCalls := 0
	env.OnActivity(activityPersistRunState, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { terminalCalls++ }).
		Return(sdktemporal.NewNonRetryableApplicationError("durable terminal write returned an ambiguous error", "terminal_test_failure", nil)).Once()
	env.OnActivity(activityPersistRunState, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { terminalCalls++ }).Return(nil).Once()

	env.ExecuteWorkflow(workflowEffectOwnerFailedTerminalWorkflow)

	require.NoError(t, env.GetWorkflowError())
	var result workflowEffectOwnerFailedTerminalResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.ErrorContains(t, errors.New(result.FirstError), "ambiguous error")
	require.Empty(t, result.FollowUpError)
	require.Equal(t, 2, terminalCalls)
	env.AssertExpectations(t)
}

func TestManualWorkflowProjectsFailureBeforeReturningWorkflowLevelError(t *testing.T) {
	env := newWorkflowEnv(t)
	input := manualWorkflowInput(sevenSlotPlan())
	persistedFailures := 0
	env.RegisterActivityWithOptions(func(_ context.Context, failure PersistWorkflowFailureV2ActivityInput) error {
		persistedFailures++
		require.Equal(t, input.RunID, failure.RunID)
		require.Equal(t, input.Identity, failure.Identity)
		require.Equal(t, "workflow_failed", failure.FailureCode)
		require.Equal(t, "图像任务执行失败，可使用相同请求重试", failure.FailureMessage)
		require.NotEmpty(t, failure.CommitID)
		return nil
	}, sdkactivity.RegisterOptions{Name: activityPersistWorkflowFailureV2})
	env.OnActivity(activityPersistRunState, mock.Anything, mock.MatchedBy(func(input PersistRunStateActivityInput) bool {
		return input.Projection.Status == imageagent.RunStatusExecuting
	})).Return(sdktemporal.NewNonRetryableApplicationError("database write exhausted", "persistence_exhausted", nil)).Once()

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.ErrorContains(t, env.GetWorkflowError(), "database write exhausted")
	require.Equal(t, 1, persistedFailures)
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
		name     string
		result   imageagent.SlotExecutionResult
		wantCode string
	}{
		{name: "wrong slot", result: successfulSlotResult("different-slot", 1), wantCode: "invalid_slot_result"},
		{name: "wrong attempt", result: successfulSlotResult("slot-1", 2), wantCode: "invalid_slot_result"},
		{name: "empty candidates", result: imageagent.SlotExecutionResult{SlotID: "slot-1", Attempt: 1}, wantCode: "invalid_slot_result"},
		{name: "whitespace candidate ID", result: imageagent.SlotExecutionResult{SlotID: "slot-1", Attempt: 1, Candidates: []imageagent.AssetCandidate{{AssetID: " \t "}}}, wantCode: "invalid_slot_result"},
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
			require.Equal(t, test.wantCode, result.ErrorCode)
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
	plan := sevenSlotPlan()
	slot := plan.Slots[0]
	initializeActivityProjection(t, repository, imageagent.Run{
		ID: "run-1", BusinessTaskID: "task-1", TenantID: identity.TenantID, UserID: identity.UserID,
		Mode: imageagent.RunModeManual, IdempotencyKey: "run-key-1", Status: imageagent.RunStatusExecuting, Version: 1,
	}, plan)

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

func TestPersistWorkflowFailureProjectsTerminalStateIdempotently(t *testing.T) {
	repository := store.NewMemoryRepository()
	identity := imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}
	plan := sevenSlotPlan()
	run := imageagent.Run{
		ID: "run-1", BusinessTaskID: "task-1", TenantID: identity.TenantID, UserID: identity.UserID,
		Mode: imageagent.RunModeManual, IdempotencyKey: "run-key-1", Status: imageagent.RunStatusPlanning, Version: 1,
	}
	initializeActivityProjection(t, repository, run, plan)
	activities, err := NewActivities(ActivityDependencies{Repository: repository, SlotExecutor: &identityCheckingExecutor{t: t}, Publisher: &identityCheckingPublisher{t: t}})
	require.NoError(t, err)
	input := PersistWorkflowFailureActivityInput{
		RunID: "run-1", Identity: identity, FailureCode: "workflow_failed",
		FailureMessage: "图像任务执行失败，可使用相同请求重试",
	}

	require.NoError(t, activities.PersistWorkflowFailure(context.Background(), input))
	require.NoError(t, activities.PersistWorkflowFailure(context.Background(), input))

	projection, err := repository.GetProjection(context.Background(), imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-a", RunID: "run-1"})
	require.NoError(t, err)
	require.Equal(t, imageagent.RunStatusFailed, projection.Run.Status)
	require.Equal(t, "workflow_failed", projection.Run.CurrentNode)
	require.Equal(t, &imageagent.Block{Code: "workflow_failed", Message: "图像任务执行失败，可使用相同请求重试"}, projection.Run.Block)
	events, err := repository.ListEvents(context.Background(), imageagent.ScopeForRun(projection.Run), 0, 100)
	require.NoError(t, err)
	failureEvents := 0
	for _, event := range events {
		if event.Type == "run.failed" {
			failureEvents++
		}
	}
	require.Equal(t, 1, failureEvents)
}

func TestRestartedExecutionCanRepersistProjectionAndReplayedSlotResult(t *testing.T) {
	repository := store.NewMemoryRepository()
	identity := imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}
	plan := sevenSlotPlan()
	run := imageagent.Run{
		ID: "run-restart", BusinessTaskID: "task-1", TenantID: identity.TenantID, UserID: identity.UserID,
		Mode: imageagent.RunModeManual, IdempotencyKey: "run-key-restart", Status: imageagent.RunStatusPlanning, Version: 1,
	}
	initializeActivityProjection(t, repository, run, plan)
	activities, err := NewActivities(ActivityDependencies{Repository: repository, SlotExecutor: &identityCheckingExecutor{t: t}, Publisher: &identityCheckingPublisher{t: t}})
	require.NoError(t, err)
	projection := WorkflowResult{Status: imageagent.RunStatusExecuting, Plan: plan, Slots: slotProjections(plan, nil)}
	result := SlotWorkflowResult{Execution: successfulSlotResult("slot-1", 1), Status: imageagent.SlotStatusAccepted}
	attemptKey := slotAttemptKey(plan.Revision, plan.Slots[0], 1)

	firstInput := manualWorkflowInput(plan)
	firstInput.RunID = run.ID
	firstInput.Identity = identity
	firstInput.projectionExecutionID = "temporal-run-a"
	firstRunCommit, err := runProjectionCommitID(firstInput, projection, "execute_slots")
	require.NoError(t, err)
	require.NoError(t, activities.PersistRunState(context.Background(), PersistRunStateActivityInput{RunID: run.ID, Identity: identity, PlanRevision: plan.Revision, Projection: projection, CurrentNode: "execute_slots", CommitID: firstRunCommit}))
	require.NoError(t, activities.PersistSlotResult(context.Background(), PersistSlotResultActivityInput{RunID: run.ID, Identity: identity, PlanRevision: plan.Revision, Result: result, AttemptKey: attemptKey}))
	firstFailureCommit, err := workflowFailureCommitID(firstInput)
	require.NoError(t, err)
	require.NoError(t, activities.PersistWorkflowFailureV2(context.Background(), PersistWorkflowFailureV2ActivityInput{RunID: run.ID, Identity: identity, FailureCode: "workflow_failed", FailureMessage: "retry", CommitID: firstFailureCommit}))

	secondInput := firstInput
	secondInput.projectionExecutionID = "temporal-run-b"
	secondRunCommit, err := runProjectionCommitID(secondInput, projection, "execute_slots")
	require.NoError(t, err)
	require.NoError(t, activities.PersistRunState(context.Background(), PersistRunStateActivityInput{RunID: run.ID, Identity: identity, PlanRevision: plan.Revision, Projection: projection, CurrentNode: "execute_slots", CommitID: secondRunCommit}))
	require.NoError(t, activities.PersistSlotResult(context.Background(), PersistSlotResultActivityInput{RunID: run.ID, Identity: identity, PlanRevision: plan.Revision, Result: result, AttemptKey: attemptKey}))

	current, err := repository.GetProjection(context.Background(), imageagent.RunScope{TenantID: identity.TenantID, OwnerUserID: identity.UserID, RunID: run.ID})
	require.NoError(t, err)
	require.Equal(t, imageagent.RunStatusExecuting, current.Run.Status)
	require.Equal(t, imageagent.SlotStatusAccepted, current.Slots[0].Slot.Status)
}

func TestActivitiesPersistTerminalSlotResultIdempotently(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:image-agent-activity-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, store.AutoMigrate(db))
	repository := store.NewGormRepository(db)
	identity := imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}
	plan := sevenSlotPlan()
	run := &imageagent.Run{ID: "run-1", BusinessTaskID: "task-1", TenantID: identity.TenantID, UserID: identity.UserID, Mode: imageagent.RunModeManual, IdempotencyKey: "run-key-1", Status: imageagent.RunStatusExecuting, Version: 1}
	initializeActivityProjection(t, repository, *run, plan)
	activities, err := NewActivities(ActivityDependencies{Repository: repository, SlotExecutor: &identityCheckingExecutor{t: t}, Publisher: &identityCheckingPublisher{t: t}})
	require.NoError(t, err)
	input := PersistSlotResultActivityInput{
		RunID: "run-1", Identity: identity, PlanRevision: 1, AttemptKey: "slot-key-slot-1:plan:1:attempt:1",
		Result: SlotWorkflowResult{Execution: successfulSlotResult("slot-1", 1), Status: imageagent.SlotStatusAccepted},
	}

	require.NoError(t, activities.PersistSlotResult(context.Background(), input))
	require.NoError(t, activities.PersistSlotResult(context.Background(), input))
	var attemptCount, resultCount, eventCount int64
	require.NoError(t, db.Table("image_agent_v2_attempts").Where("tenant_id = ? AND owner_user_id = ? AND run_id = ? AND plan_revision = ? AND slot_id = ? AND attempt = ?", "tenant-a", "user-a", "run-1", 1, "slot-1", 1).Count(&attemptCount).Error)
	require.NoError(t, db.Table("image_agent_v2_slots").Where("tenant_id = ? AND owner_user_id = ? AND run_id = ? AND plan_revision = ? AND id = ? AND attempt = ? AND status = ?", "tenant-a", "user-a", "run-1", 1, "slot-1", 1, string(imageagent.SlotStatusAccepted)).Count(&resultCount).Error)
	require.NoError(t, db.Table("image_agent_v2_events").Where("tenant_id = ? AND owner_user_id = ? AND run_id = ? AND type = ?", "tenant-a", "user-a", "run-1", slotResultPersistedEventType).Count(&eventCount).Error)
	require.EqualValues(t, 1, attemptCount)
	require.EqualValues(t, 1, resultCount)
	require.EqualValues(t, 1, eventCount)
}

func TestPersistPlanRevisionActivityCommitsRunPlanSnapshotEventAndReceiptTogether(t *testing.T) {
	for _, test := range []struct {
		name string
		new  func(t *testing.T) imageagent.Repository
	}{
		{name: "memory", new: func(t *testing.T) imageagent.Repository { return store.NewMemoryRepository() }},
		{name: "gorm sqlite", new: func(t *testing.T) imageagent.Repository {
			t.Helper()
			db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:image-agent-replace-activity-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
			require.NoError(t, err)
			require.NoError(t, store.AutoMigrate(db))
			return store.NewGormRepository(db)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := test.new(t)
			identity := imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}
			plan1 := sevenSlotPlan()
			run := imageagent.Run{
				ID: "run-replace-activity", BusinessTaskID: "task-1", TenantID: identity.TenantID, UserID: identity.UserID,
				Mode: imageagent.RunModeManual, IdempotencyKey: "run-key-replace-activity", Status: imageagent.RunStatusBlocked,
				CurrentNode: "retry_slot", ActivePlanRevision: 1, Version: 1,
				Block: &imageagent.Block{Code: "slot_failed", SlotID: "scene-2"},
			}
			initial := initializeActivityProjection(t, repository, run, plan1)
			replacement := sevenSlotPlan()
			replacement.Revision = 2
			replacement.ParentRevision = 1
			replacement.IdempotencyKey = "plan-key-2"
			activities, err := NewActivities(ActivityDependencies{Repository: repository, SlotExecutor: &identityCheckingExecutor{t: t}, Publisher: &identityCheckingPublisher{t: t}})
			require.NoError(t, err)

			err = activities.PersistPlanRevision(context.Background(), PersistPlanRevisionActivityInput{
				RunID: run.ID, Identity: identity, ExpectedRevision: 1, Plan: replacement,
			})
			require.NoError(t, err)
			stored, err := repository.GetProjection(context.Background(), imageagent.ScopeForRun(run))
			require.NoError(t, err)
			require.EqualValues(t, initial.ProjectionVersion+1, stored.ProjectionVersion)
			require.EqualValues(t, 2, stored.Plan.Revision)
			require.EqualValues(t, 2, stored.Run.ActivePlanRevision)
			require.EqualValues(t, 2, stored.Run.Version)
			require.Equal(t, imageagent.RunStatusExecuting, stored.Run.Status)
			require.Nil(t, stored.Run.Block)
			require.Nil(t, stored.PendingCommand)
			require.Len(t, stored.Slots, len(replacement.Slots))
			events, err := repository.ListEvents(context.Background(), imageagent.ScopeForRun(run), 0, 10)
			require.NoError(t, err)
			require.Len(t, events, 2)
			require.Equal(t, "plan.replaced", events[1].Type)
			require.Equal(t, stored.ProjectionVersion, events[1].Cursor)
		})
	}
}

func TestPersistPendingCommandActivityMakesLedgerExhaustionDurableWithoutLosingPendingOwner(t *testing.T) {
	repository := store.NewMemoryRepository()
	identity := imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}
	plan := sevenSlotPlan()
	run := imageagent.Run{
		ID: "run-ingress-cap", BusinessTaskID: "task-1", TenantID: identity.TenantID, UserID: identity.UserID,
		Mode: imageagent.RunModeManual, IdempotencyKey: "run-key-ingress-cap", Status: imageagent.RunStatusBlocked,
		ActivePlanRevision: 1, Version: 1,
	}
	initial := initializeActivityProjection(t, repository, run, plan)
	activities, err := NewActivities(ActivityDependencies{Repository: repository, SlotExecutor: &identityCheckingExecutor{t: t}, Publisher: &identityCheckingPublisher{t: t}})
	require.NoError(t, err)
	receipt := &imageagent.PendingCommandReceipt{
		ActionID: "retry-at-cap", Kind: signalRetrySlot, Phase: string(updatePhaseRetryPersistResult),
		Status: "pending", PlanRevision: 1, SlotID: "scene-2", Attempt: 2,
		FailureCode: "persistence_failed", FailureCategory: "persistence", FailureMessage: "运行状态保存暂时失败",
	}
	ingress := imageagent.CommandIngress{Used: maxActionLedgerEntries, Limit: maxActionLedgerEntries, Exhausted: true, Reason: "command_capacity_exhausted"}
	input := PersistPendingCommandActivityInput{
		RunID: run.ID, Identity: identity, Receipt: receipt, CommandIngress: ingress, CommitID: "command-ingress:exhausted",
	}

	require.NoError(t, activities.PersistPendingCommand(context.Background(), input))
	require.NoError(t, activities.PersistPendingCommand(context.Background(), input), "exact retry must use the stored commit receipt")
	stored, err := repository.GetProjection(context.Background(), imageagent.ScopeForRun(run))
	require.NoError(t, err)
	require.EqualValues(t, initial.ProjectionVersion+1, stored.ProjectionVersion)
	require.Equal(t, ingress, stored.CommandIngress)
	require.Equal(t, receipt, stored.PendingCommand, "capacity exhaustion must not evict the resumable pending owner")
	events, err := repository.ListEvents(context.Background(), imageagent.ScopeForRun(run), initial.LastEventID, 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "command.ingress.exhausted", events[0].Type)
	require.Equal(t, stored.ProjectionVersion, events[0].Cursor)
}

func TestManualWorkflowReplacePathUsesRealProjectionActivityAndRepository(t *testing.T) {
	repository := store.NewMemoryRepository()
	identity := imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}
	plan1 := sevenSlotPlan()
	run := imageagent.Run{
		ID: "run-real-replace", BusinessTaskID: "task-1", TenantID: identity.TenantID, UserID: identity.UserID,
		Mode: imageagent.RunModeManual, IdempotencyKey: "run-key-real-replace", Status: imageagent.RunStatusPlanning,
	}
	initializeActivityProjection(t, repository, run, plan1)
	executor := &revisionFailingExecutor{failedRevision: 1, failedSlotID: "scene-2"}
	activities, err := NewActivities(ActivityDependencies{Repository: repository, SlotExecutor: executor, Publisher: &identityCheckingPublisher{t: t}})
	require.NoError(t, err)

	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetWorkerOptions(worker.Options{DeadlockDetectionTimeout: 5 * time.Second})
	env.OnGetVersion(activityWireV2Patch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	env.OnGetVersion(slotExecutionWireV3Patch, workflow.DefaultVersion, 1).Return(workflow.DefaultVersion)
	env.OnGetVersion(projectionExecutionCommitPatch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	env.OnGetVersion(approvalActionIDV3Patch, workflow.DefaultVersion, 1).Return(workflow.DefaultVersion)
	env.OnGetVersion(approvalPublicationWireV3Patch, workflow.DefaultVersion, 1).Return(workflow.DefaultVersion)
	env.OnGetVersion(resultDigestV3Patch, workflow.DefaultVersion, 1).Return(workflow.DefaultVersion)
	env.OnGetVersion(approvalPublicationScopePatch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	env.RegisterWorkflow(ImageSlotWorkflow)
	env.RegisterActivityWithOptions(activities.ExecuteSlot, sdkactivity.RegisterOptions{Name: activityExecuteSlot})
	env.RegisterActivityWithOptions(activities.PersistSlotResult, sdkactivity.RegisterOptions{Name: activityPersistSlotResult})
	env.RegisterActivityWithOptions(activities.PersistRunState, sdkactivity.RegisterOptions{Name: activityPersistRunState})
	env.RegisterActivityWithOptions(activities.PersistPlanRevision, sdkactivity.RegisterOptions{Name: activityPersistPlanRevision})
	env.RegisterActivityWithOptions(activities.PersistPendingCommand, sdkactivity.RegisterOptions{Name: activityPersistPendingCommand})
	env.RegisterActivityWithOptions(activities.PublishApproved, sdkactivity.RegisterOptions{Name: activityPublishApproved})
	replacement := sevenSlotPlan()
	replacement.Revision = 2
	replacement.ParentRevision = 1
	replacement.IdempotencyKey = "plan-key-real-replace-2"
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalReplacePlan, ReplacePlanSignal{
			RunID: run.ID, ExpectedRevision: 1, Plan: replacement, ActorID: identity.UserID, ActionID: "replace-real-activity",
		})
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalApproveResults, ApproveResultsSignal{
			RunID: run.ID, PlanRevision: 2, ResultDigest: sevenSlotResultDigest, ActorID: identity.UserID, ActionID: "approve-real-replacement",
		})
	}, 2*time.Second)
	input := manualWorkflowInput(plan1)
	input.RunID = run.ID
	input.WaitForCommands = true

	env.ExecuteWorkflow(ImageAgentWorkflow, input)

	require.NoError(t, env.GetWorkflowError())
	stored, err := repository.GetProjection(context.Background(), imageagent.ScopeForRun(run))
	require.NoError(t, err)
	require.EqualValues(t, 2, stored.Plan.Revision)
	require.EqualValues(t, 2, stored.Run.ActivePlanRevision)
	require.Equal(t, imageagent.RunStatusCompleted, stored.Run.Status)
	require.Nil(t, stored.PendingCommand)
	require.EqualValues(t, 1, executor.failedCalls.Load())
	require.EqualValues(t, len(plan1.Slots)+len(replacement.Slots)-1, executor.successCalls.Load())
}

func TestActivitiesPersistEachSlotResultWithItsOwnDurableProjectionCursor(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:image-agent-slot-events-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, store.AutoMigrate(db))
	repository := store.NewGormRepository(db)
	identity := imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}
	plan := sevenSlotPlan()
	run := &imageagent.Run{ID: "run-cursors", BusinessTaskID: "task-1", TenantID: identity.TenantID, UserID: identity.UserID, Mode: imageagent.RunModeManual, IdempotencyKey: "run-cursors-key", Status: imageagent.RunStatusExecuting, Version: 1}
	initializeActivityProjection(t, repository, *run, plan)
	activities, err := NewActivities(ActivityDependencies{Repository: repository, SlotExecutor: &identityCheckingExecutor{t: t}, Publisher: &identityCheckingPublisher{t: t}})
	require.NoError(t, err)
	for _, slotID := range []string{"slot-1", "slot-2"} {
		require.NoError(t, activities.PersistSlotResult(context.Background(), PersistSlotResultActivityInput{
			RunID: run.ID, Identity: identity, PlanRevision: 1, AttemptKey: "slot-key-" + slotID + ":plan:1:attempt:1",
			Result: SlotWorkflowResult{Execution: successfulSlotResult(slotID, 1), Status: imageagent.SlotStatusAccepted},
		}))
	}
	events, err := repository.ListEvents(context.Background(), imageagent.RunScope{TenantID: identity.TenantID, OwnerUserID: identity.UserID, RunID: run.ID}, 0, 10)
	require.NoError(t, err)
	require.Len(t, events, 3)
	require.Equal(t, []int64{1, 2, 3}, []int64{events[0].Cursor, events[1].Cursor, events[2].Cursor})
	require.Equal(t, []int64{1, 2, 3}, []int64{events[0].ProjectionVersion, events[1].ProjectionVersion, events[2].ProjectionVersion})
}

func TestNewActivitiesRejectsMissingDependencies(t *testing.T) {
	_, err := NewActivities(ActivityDependencies{})
	require.ErrorContains(t, err, "repository")
	_, err = NewActivities(ActivityDependencies{Repository: store.NewMemoryRepository()})
	require.ErrorContains(t, err, "slot executor")
}

func initializeActivityProjection(t *testing.T, repository imageagent.Repository, run imageagent.Run, plan imageagent.Plan) imageagent.RunProjection {
	t.Helper()
	plan = pendingPlanForTest(plan)
	catalog, err := (workflowCatalogResolver{}).Resolve(context.Background(), imageagent.AssetCatalogScope{
		TenantID: run.TenantID, OwnerUserID: run.UserID, BusinessTaskID: run.BusinessTaskID, RunID: run.ID,
	})
	require.NoError(t, err)
	normalized, err := imageagent.NormalizeAssetCatalog(catalog)
	require.NoError(t, err)
	scope := imageagent.ScopeForRun(run)
	projection, err := repository.InitializeRun(context.Background(), imageagent.ProjectionInitialization{
		Scope: scope, Run: run, Plan: plan, Catalog: normalized,
		Snapshot: imageagent.RunProjection{Run: run, Plan: plan},
		CommitID: "start:" + run.IdempotencyKey, EventType: "run.initialized", EventPayload: []byte(`{}`),
	})
	require.NoError(t, err)
	return projection
}

func TestServiceCapturesVerifiedIdentityAndRejectsNonManualStarts(t *testing.T) {
	repository := store.NewMemoryRepository()
	workflows := &recordingDomainWorkflowClient{}
	service, err := imageagent.NewService(repository, workflows, workflowCatalogResolver{})
	require.NoError(t, err)
	input := imageagent.StartRunInput{RunID: "run-1", BusinessTaskID: "task-1", Mode: imageagent.RunModeManual, IdempotencyKey: "run-key-1", Plan: sevenSlotPlan()}

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
	require.Equal(t, imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a", BusinessTaskID: "task-1"}, workflows.starts[0].Identity)
	require.Equal(t, "user-a", workflows.starts[0].Plan.CreatedBy)
	scope := imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-a", RunID: "run-1"}
	projection, err := repository.GetProjection(context.Background(), scope)
	require.NoError(t, err)
	run := projection.Run
	require.Equal(t, "user-a", run.UserID)
	require.Equal(t, imageagent.RunModeManual, run.Mode)
	current, err := repository.GetProjection(context.Background(), scope)
	require.NoError(t, err)
	updated := current
	updated.Run.Status = imageagent.RunStatusAwaitingFinalApproval
	updated.Run.CurrentNode = "approve_results"
	updated.Run.Version++
	updated.ResultDigest = sevenSlotResultDigest
	_, err = repository.CommitProjection(context.Background(), imageagent.ProjectionCommit{
		Scope: scope, CommitID: "test:awaiting-approval", ExpectedProjectionVersion: current.ProjectionVersion,
		Snapshot: updated, EventType: "run.updated", EventPayload: []byte(`{}`), ExpectedRunVersion: current.Run.Version,
		RunMutation: &imageagent.RunMutation{Status: imageagent.RunStatusAwaitingFinalApproval, CurrentNode: "approve_results", ActivePlanRevision: 1},
	})
	require.NoError(t, err)
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
	require.Equal(t, "image-agent:tenant-a:user-a:run-1", raw.startOptions.ID)
	require.Equal(t, TaskQueueV3, raw.startOptions.TaskQueue)
	require.Zero(t, raw.startOptions.WorkflowExecutionTimeout)
	require.Equal(t, enumspb.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING, raw.startOptions.WorkflowIDConflictPolicy)
	require.Equal(t, enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY, raw.startOptions.WorkflowIDReusePolicy)
	require.Equal(t, workflowNameImageAgent, raw.workflowName)
	require.Equal(t, imageagent.RunModeManual, raw.workflowInput.Mode)
	require.True(t, raw.workflowInput.WaitForCommands)

	projection, err := client.GetProjection(context.Background(), imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-a", RunID: "run-1"}, start.Identity)
	require.NoError(t, err)
	require.Equal(t, raw.queryValue, projection)
	require.Equal(t, "image-agent:tenant-a:user-a:run-1", raw.queryWorkflowID)
	require.Equal(t, QueryWorkflowProjection, raw.queryType)

	require.ErrorContains(t, client.ApproveResults(context.Background(), imageagent.ApproveResultsCommand{RunID: "run-1", PlanRevision: 1, ResultDigest: sevenSlotResultDigest, ActorID: "attacker", ActionID: "spoofed", Identity: start.Identity}), "actor")
	require.ErrorContains(t, client.ApproveResults(context.Background(), imageagent.ApproveResultsCommand{RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "missing-digest", Identity: start.Identity}), "digest")
	require.ErrorContains(t, client.ApproveResults(context.Background(), imageagent.ApproveResultsCommand{RunID: "run-1", PlanRevision: 1, ResultDigest: " " + sevenSlotResultDigest, ActorID: "user-a", ActionID: "spaced-digest", Identity: start.Identity}), "digest")
}

func TestRunProjectionCommitIDIsScopedToTemporalExecution(t *testing.T) {
	input := manualWorkflowInput(sevenSlotPlan())
	projection := WorkflowResult{Status: imageagent.RunStatusExecuting, Plan: input.Plan}

	input.projectionExecutionID = "temporal-run-a"
	first, err := runProjectionCommitID(input, projection, "execute_slots")
	require.NoError(t, err)
	firstFailure, err := workflowFailureCommitID(input)
	require.NoError(t, err)
	input.projectionExecutionID = "temporal-run-b"
	second, err := runProjectionCommitID(input, projection, "execute_slots")
	require.NoError(t, err)
	secondFailure, err := workflowFailureCommitID(input)
	require.NoError(t, err)

	require.NotEqual(t, first, second, "a failed workflow restart must not replay the prior execution's projection commit")
	require.NotEqual(t, firstFailure, secondFailure, "each failed execution must own its terminal failure projection")
}

func TestMaximumSlotIdempotencyKeyFitsDerivedAttemptAndProjectionCommitColumns(t *testing.T) {
	slot := imageagent.Slot{IdempotencyKey: fmt.Sprintf("%0128d", 0)}
	attemptKey := slotAttemptKey(math.MaxInt64, slot, int(^uint(0)>>1))

	require.LessOrEqual(t, len(attemptKey), 192)
	require.LessOrEqual(t, len("slot-v3:"+attemptKey), 192)
}

func TestTemporalClientRejectsUnsafeConcurrencyBeforeStartingWorkflow(t *testing.T) {
	raw := &recordingSDKClient{}
	client := NewClient(raw)
	start := imageagent.WorkflowStart{
		Run: imageagent.Run{
			ID:                 "run-1",
			TenantID:           "tenant-a",
			UserID:             "user-a",
			Mode:               imageagent.RunModeManual,
			MaxConcurrentSlots: imageagent.MaxConcurrentSlots + 1,
		},
		Plan:     sevenSlotPlan(),
		Identity: imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"},
	}

	require.ErrorIs(t, client.StartManual(context.Background(), start), imageagent.ErrValidation)
	require.Empty(t, raw.workflowName)
}

func TestV3ManualApprovalLifetimeIsNotBoundedByAServerExecutionTimeout(t *testing.T) {
	require.Zero(t, V3WorkflowExecutionTimeout)
	require.Equal(t, 45*24*time.Hour, V3MinimumStagingLifecycleRetention)
}

func TestTemporalClientRetryReturnsAfterWorkflowUpdateIsAccepted(t *testing.T) {
	raw := &recordingSDKClient{updateResultErr: errors.New("accepted retry must not wait for workflow completion")}
	client := NewClient(raw)
	identity := imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}

	err := client.RetrySlot(context.Background(), imageagent.RetrySlotCommand{
		RunID: "run-1", PlanRevision: 2, SlotID: "scene-2", ActorID: "user-a", ActionID: "retry-accepted", Identity: identity,
	})

	require.NoError(t, err)
	require.Len(t, raw.updateOptions, 1)
	require.Equal(t, sdkclient.WorkflowUpdateStageAccepted, raw.updateOptions[0].WaitForStage)
	require.Zero(t, raw.updateGetCalls)
}

func TestTemporalClientRetryWaitsForAcceptedWhileOtherCommandsWaitForCompleted(t *testing.T) {
	raw := &recordingSDKClient{updateAck: imageagent.CommandAcknowledgement{RunID: "run-1", PlanRevision: 2, ActionID: "retry-1", Status: imageagent.RunStatusExecuting}}
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
	ack, err := client.Resume(context.Background(), imageagent.ResumeCommand{
		RunID: "run-1", ActorID: "user-a", ActionID: "retry-1", Identity: identity,
	})
	require.NoError(t, err)
	require.Equal(t, raw.updateAck, ack)

	require.Len(t, raw.updateOptions, 5)
	require.Equal(t, []string{"replace_plan", "retry_slot", "approve_results", "cancel", "resume_command"}, []string{
		raw.updateOptions[0].UpdateName, raw.updateOptions[1].UpdateName, raw.updateOptions[2].UpdateName, raw.updateOptions[3].UpdateName, raw.updateOptions[4].UpdateName,
	})
	for index, options := range raw.updateOptions {
		require.Equal(t, "image-agent:tenant-a:user-a:run-1", options.WorkflowID)
		if index == 1 || index == 3 {
			require.Equal(t, sdkclient.WorkflowUpdateStageAccepted, options.WaitForStage)
		} else {
			require.Equal(t, sdkclient.WorkflowUpdateStageCompleted, options.WaitForStage)
		}
		require.NotEmpty(t, options.UpdateID)
		require.Len(t, options.Args, 1)
	}
	require.Equal(t, 3, raw.updateGetCalls)
	require.Empty(t, raw.signalName)
}

func TestTemporalClientUsesUniqueTransportUpdateIDForEachResumeAttempt(t *testing.T) {
	raw := &recordingSDKClient{
		updateAck:          imageagent.CommandAcknowledgement{RunID: "run-1", PlanRevision: 1, ActionID: "retry-1", Status: imageagent.RunStatusExecuting},
		updateResultErrors: []error{errors.New("first resume transport lost after workflow activity failure"), nil},
	}
	client := NewClient(raw)
	command := imageagent.ResumeCommand{
		RunID: "run-1", ActorID: "user-a", ActionID: "retry-1",
		Identity: imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"},
	}
	require.Error(t, func() error { _, err := client.Resume(context.Background(), command); return err }())
	require.NoError(t, func() error { _, err := client.Resume(context.Background(), command); return err }())
	require.Len(t, raw.updateOptions, 2)
	require.NotEqual(t, raw.updateOptions[0].UpdateID, raw.updateOptions[1].UpdateID)
	require.Equal(t, []string{"retry-1", "retry-1"}, []string{
		raw.updateOptions[0].Args[0].(ResumeCommandInput).ActionID,
		raw.updateOptions[1].Args[0].(ResumeCommandInput).ActionID,
	})
	require.Equal(t, 2, raw.updateGetCalls, "a new transport UpdateID must actually execute the second resume attempt")
}

func TestTemporalClientMapsWorkflowUpdateErrorsToApplicationContracts(t *testing.T) {
	identity := imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}
	command := imageagent.CancelRunCommand{RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "cancel-1", Identity: identity}
	for _, tt := range []struct {
		name             string
		directErr        error
		resultErr        error
		completedCommand bool
		want             error
	}{
		{name: "missing run", directErr: serviceerror.NewNotFound("missing"), want: imageagent.ErrRunNotFound},
		{name: "closed workflow", directErr: serviceerror.NewFailedPrecondition("workflow completed"), want: imageagent.ErrCommandBlocked},
		{name: "revision", resultErr: sdktemporal.NewNonRetryableApplicationError("stale", "imageagent_revision_conflict", nil), completedCommand: true, want: imageagent.ErrRevisionConflict},
		{name: "blocked", resultErr: sdktemporal.NewNonRetryableApplicationError("blocked", "imageagent_command_blocked", nil), completedCommand: true, want: imageagent.ErrCommandBlocked},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw := &recordingSDKClient{updateErr: tt.directErr, updateResultErr: tt.resultErr}
			client := NewClient(raw)
			var err error
			if tt.completedCommand {
				err = client.ApproveResults(context.Background(), imageagent.ApproveResultsCommand{
					RunID: "run-1", PlanRevision: 1, ResultDigest: sevenSlotResultDigest,
					ActorID: "user-a", ActionID: "approve-1", Identity: identity,
				})
			} else {
				err = client.Cancel(context.Background(), command)
			}
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

func TestRegisterWorkerPreservesFrozenV2Compatibility(t *testing.T) {
	activities, err := NewActivities(ActivityDependencies{Repository: store.NewMemoryRepository(), SlotExecutor: &identityCheckingExecutor{t: t}, Publisher: &identityCheckingPublisher{t: t}})
	require.NoError(t, err)
	registrar := &recordingWorkerRegistrar{}

	require.NoError(t, RegisterWorker(registrar, activities))

	require.Equal(t, []string{workflowNameImageAgent, workflowNameImageSlot}, registrar.workflows)
	require.Equal(t, []string{
		"imageagent.execute_slot", "imageagent.persist_slot_result", "imageagent.persist_run_state", "imageagent.persist_plan_revision", "imageagent.persist_pending_command", "imageagent.publish_approved",
		"imageagent.execute_slot.v2", "imageagent.persist_slot_result.v2", "imageagent.persist_run_state.v2", "imageagent.persist_workflow_failure.v1", "imageagent.persist_workflow_failure.v2", "imageagent.persist_plan_revision.v2", "imageagent.persist_pending_command.v2", "imageagent.publish_approved.v2",
	}, registrar.activities)
}

func TestRegisterWorkerUsesExactModeBoundWorkflowAndActivitySets(t *testing.T) {
	activities, err := NewActivities(ActivityDependencies{Repository: store.NewMemoryRepository(), SlotExecutor: &identityCheckingExecutor{t: t}, Publisher: &identityCheckingPublisher{t: t}})
	require.NoError(t, err)
	tests := []struct {
		name           string
		mode           WorkerWireMode
		wantWorkflows  []string
		wantActivities []string
	}{
		{
			name: "v2", mode: WorkerWireModeV2,
			wantWorkflows: []string{workflowNameImageAgent, workflowNameImageSlot},
			wantActivities: []string{
				"imageagent.execute_slot", "imageagent.persist_slot_result", "imageagent.persist_run_state", "imageagent.persist_plan_revision", "imageagent.persist_pending_command", "imageagent.publish_approved",
				"imageagent.execute_slot.v2", "imageagent.persist_slot_result.v2", "imageagent.persist_run_state.v2", "imageagent.persist_workflow_failure.v1", "imageagent.persist_workflow_failure.v2", "imageagent.persist_plan_revision.v2", "imageagent.persist_pending_command.v2", "imageagent.publish_approved.v2",
			},
		},
		{
			name: "v3", mode: WorkerWireModeV3,
			wantWorkflows: []string{workflowNameImageAgent, "ImageSlotWorkflowV3", workflowNameCompatibilityCanary},
			wantActivities: []string{
				"imageagent.persist_run_state.v2", "imageagent.persist_workflow_failure.v1", "imageagent.persist_workflow_failure.v2", "imageagent.persist_plan_revision.v2", "imageagent.persist_pending_command.v2",
				"imageagent.execute_slot.v3", "imageagent.persist_slot_result.v3", "imageagent.publish_approved.v3",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registrar := &recordingWorkerRegistrar{}
			require.NoError(t, RegisterWorkerForMode(registrar, activities, test.mode))
			require.Equal(t, test.wantWorkflows, registrar.workflows)
			require.Equal(t, test.wantActivities, registrar.activities)
		})
	}
}

func TestWorkerConfigBindsExplicitModesToCompatibleQueues(t *testing.T) {
	tests := []struct {
		name    string
		config  WorkerConfig
		queue   string
		wantErr string
	}{
		{name: "v2", config: WorkerConfig{WireMode: WorkerWireModeV2}, queue: TaskQueue},
		{name: "v3", config: WorkerConfig{WireMode: WorkerWireModeV3}, queue: TaskQueueV3},
		{name: "custom v2", config: WorkerConfig{WireMode: WorkerWireModeV2, TaskQueue: "image-agent-manual-v2-custom"}, queue: "image-agent-manual-v2-custom"},
		{name: "custom v3", config: WorkerConfig{WireMode: WorkerWireModeV3, TaskQueue: "image-agent-manual-v3-custom"}, queue: "image-agent-manual-v3-custom"},
		{name: "v2 rejects v3 default", config: WorkerConfig{WireMode: WorkerWireModeV2, TaskQueue: TaskQueueV3}, wantErr: "v2 wire mode cannot use v3 task queue"},
		{name: "v3 rejects v2 default", config: WorkerConfig{WireMode: WorkerWireModeV3, TaskQueue: TaskQueue}, wantErr: "v3 wire mode cannot use v2 task queue"},
		{name: "missing explicit mode", config: WorkerConfig{TaskQueue: "image-agent-custom"}, wantErr: "wire mode is required"},
		{name: "invalid mode", config: WorkerConfig{WireMode: "invalid"}, wantErr: "unsupported image agent temporal wire mode"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			queue, err := test.config.selectedTaskQueue()
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.queue, queue)
		})
	}
}

func TestLegacyPersistRunStateDecodesFrozenPayloadAndFailsWithoutWritingV2State(t *testing.T) {
	repository := store.NewMemoryRepository()
	activities, err := NewActivities(ActivityDependencies{
		Repository:   repository,
		SlotExecutor: &identityCheckingExecutor{t: t},
		Publisher:    &identityCheckingPublisher{t: t},
	})
	require.NoError(t, err)

	raw := []byte(`{"RunID":"run-legacy","Identity":{"TenantID":"tenant-a","UserID":"user-a"},"PlanRevision":1,"Status":"executing","CurrentNode":"execute_slots","Block":{"Code":"legacy_block","Message":"legacy payload"}}`)
	payloads := &commonpb.Payloads{Payloads: []*commonpb.Payload{{
		Metadata: map[string][]byte{"encoding": []byte("json/plain")},
		Data:     raw,
	}}}
	var input LegacyPersistRunStateActivityInput
	require.NoError(t, sdkconverter.GetDefaultDataConverter().FromPayloads(payloads, &input))
	require.Equal(t, imageagent.RunStatusExecuting, input.Status)
	require.Equal(t, "legacy_block", input.Block.Code)

	err = activities.LegacyPersistRunState(context.Background(), input)
	var applicationError *sdktemporal.ApplicationError
	require.ErrorAs(t, err, &applicationError)
	require.Equal(t, updateErrorLegacyMigrationRequired, applicationError.Type())

	_, err = repository.GetProjection(context.Background(), imageagent.RunScope{
		TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID,
	})
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
}

func TestImageAgentWorkflowReplaysAfterPersistStateWorkflowTaskRestart(t *testing.T) {
	input := manualWorkflowInput(sevenSlotPlan())
	workflowPayloads, err := sdkconverter.GetDefaultDataConverter().ToPayloads(input)
	require.NoError(t, err)
	persistInput := LegacyPersistRunStateActivityInput{
		RunID: "run-1", Identity: input.Identity, PlanRevision: 1,
		Status: imageagent.RunStatusExecuting, CurrentNode: "execute_slots", Block: &imageagent.Block{Code: "legacy_block", Message: "legacy payload"},
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
				ActivityId: "5", ActivityType: &commonpb.ActivityType{Name: activityPersistRunStateLegacy},
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

type workflowEffectOwnerFailedTerminalResult struct {
	FirstError    string
	FollowUpError string
}

func workflowEffectOwnerFailedTerminalWorkflow(ctx workflow.Context) (workflowEffectOwnerFailedTerminalResult, error) {
	ctx = imageAgentActivityContext(ctx)
	owner := newWorkflowEffectOwner(ctx)
	input := manualWorkflowInput(sevenSlotPlan())
	firstErr := owner.persistTerminalRunState(
		ctx, input, WorkflowResult{Status: imageagent.RunStatusCompleted, Plan: input.Plan}, "complete", "approval-action",
	)
	followUpErr := owner.persistTerminalRunState(
		ctx, input, WorkflowResult{Status: imageagent.RunStatusCancelled, Plan: input.Plan}, "cancelled", "cancel-action",
	)
	return workflowEffectOwnerFailedTerminalResult{
		FirstError:    workflowTestErrorString(firstErr),
		FollowUpError: workflowTestErrorString(followUpErr),
	}, nil
}

func workflowEffectOwnerFenceWorkflow(ctx workflow.Context, status imageagent.RunStatus) (workflowEffectOwnerFenceResult, error) {
	ctx = imageAgentActivityContext(ctx)
	owner := newWorkflowEffectOwner(ctx)
	input := manualWorkflowInput(sevenSlotPlan())
	node := string(status)
	firstDone := workflow.NewBufferedChannel(ctx, 1)
	workflow.Go(ctx, func(effectCtx workflow.Context) {
		firstDone.Send(effectCtx, workflowTestErrorString(
			owner.persistTerminalRunState(effectCtx, input, WorkflowResult{Status: status, Plan: input.Plan}, node, "action-a"),
		))
	})
	if err := workflow.Sleep(ctx, 100*time.Millisecond); err != nil {
		return workflowEffectOwnerFenceResult{}, err
	}
	nonTerminalErr := owner.persistRunState(ctx, input, WorkflowResult{Status: imageagent.RunStatusBlocked, Plan: input.Plan}, "retry_slot")
	var firstErr string
	firstDone.Receive(ctx, &firstErr)
	differentActionErr := owner.persistTerminalRunState(ctx, input, WorkflowResult{Status: status, Plan: input.Plan}, node, "action-b")
	differentStatus := imageagent.RunStatusCancelled
	if status == imageagent.RunStatusCancelled {
		differentStatus = imageagent.RunStatusCompleted
	}
	differentTerminalErr := owner.persistTerminalRunState(ctx, input, WorkflowResult{Status: differentStatus, Plan: input.Plan}, string(differentStatus), "action-a")
	exactRetryErr := owner.persistTerminalRunState(ctx, input, WorkflowResult{Status: status, Plan: input.Plan}, node, "action-a")
	afterSuccessErr := owner.persistRunState(ctx, input, WorkflowResult{Status: imageagent.RunStatusAwaitingFinalApproval, Plan: input.Plan}, "approve_results")
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
	env.OnGetVersion(activityWireV2Patch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	env.OnGetVersion(slotExecutionWireV3Patch, workflow.DefaultVersion, 1).Return(workflow.DefaultVersion)
	env.OnGetVersion(approvalActionIDV3Patch, workflow.DefaultVersion, 1).Return(workflow.DefaultVersion)
	env.OnGetVersion(approvalPublicationWireV3Patch, workflow.DefaultVersion, 1).Return(workflow.DefaultVersion)
	env.OnGetVersion(resultDigestV3Patch, workflow.DefaultVersion, 1).Return(workflow.DefaultVersion)
	env.OnGetVersion(approvalPublicationScopePatch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
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
		func(context.Context, PersistPendingCommandActivityInput) error { return nil },
		sdkactivity.RegisterOptions{Name: activityPersistPendingCommand},
	)
	env.RegisterActivityWithOptions(
		func(context.Context, PublishApprovedActivityInput) error { return nil },
		sdkactivity.RegisterOptions{Name: activityPublishApproved},
	)
	return env
}

func manualWorkflowInput(plan imageagent.Plan) WorkflowInput {
	plan = pendingPlanForTest(plan)
	return WorkflowInput{
		RunID:              "run-1",
		Mode:               imageagent.RunModeManual,
		Identity:           imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"},
		Plan:               plan,
		MaxConcurrentSlots: 3,
		AssetCatalog:       imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/source.png"}}},
	}
}

func pendingPlanForTest(plan imageagent.Plan) imageagent.Plan {
	plan.Slots = append([]imageagent.Slot(nil), plan.Slots...)
	for index := range plan.Slots {
		if plan.Slots[index].Status == "" {
			plan.Slots[index].Status = imageagent.SlotStatusPending
		}
	}
	return plan
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
			in.PlanRevision == revision && in.Slot.ID == slotID && in.Attempt == attempt && in.IdempotencyKey == fmt.Sprintf("slot-key-%s:plan:%d:attempt:%d", slotID, revision, attempt)
	})
}

func successfulSlotResult(slotID string, attempt int) imageagent.SlotExecutionResult {
	return imageagent.SlotExecutionResult{
		SlotID:  slotID,
		Attempt: attempt,
		Candidates: []imageagent.AssetCandidate{{
			AssetID: "candidate-" + slotID,
			URL:     "https://generated.example/" + slotID + ".png",
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

type temporalCancellationStagedExecutor struct {
	*recordingStagedExecutor
	mu          sync.Mutex
	waitTimeout time.Duration
	called      bool
}

func (e *temporalCancellationStagedExecutor) GenerateSlot(ctx context.Context, _ imageagent.SlotExecutionInput) (imageagent.SlotGeneratedOutput, error) {
	e.mu.Lock()
	e.called = true
	e.mu.Unlock()
	timer := time.NewTimer(e.waitTimeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
	return imageagent.SlotGeneratedOutput{}, &imageagent.ProviderDispatchError{
		State: imageagent.ProviderDispatchedUnknown,
		Err:   errors.New("provider request may have been dispatched"),
	}
}

func (e *temporalCancellationStagedExecutor) Called() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.called
}

type revisionFailingExecutor struct {
	failedRevision int64
	failedSlotID   string
	failedCalls    atomic.Int64
	successCalls   atomic.Int64
}

func (e *revisionFailingExecutor) ExecuteSlot(_ context.Context, input imageagent.SlotExecutionInput) (imageagent.SlotExecutionResult, error) {
	if input.PlanRevision == e.failedRevision && input.Slot.ID == e.failedSlotID {
		e.failedCalls.Add(1)
		return imageagent.SlotExecutionResult{}, sdktemporal.NewNonRetryableApplicationError("provider rejected slot", "slot_rejected", nil)
	}
	e.successCalls.Add(1)
	return successfulSlotResult(input.Slot.ID, input.Attempt), nil
}

func (e *revisionFailingExecutor) GenerateSlot(_ context.Context, input imageagent.SlotExecutionInput) (imageagent.SlotGeneratedOutput, error) {
	if input.PlanRevision == e.failedRevision && input.Slot.ID == e.failedSlotID {
		e.failedCalls.Add(1)
		return imageagent.SlotGeneratedOutput{}, sdktemporal.NewNonRetryableApplicationError("provider rejected slot", "slot_rejected", nil)
	}
	e.successCalls.Add(1)
	return testGeneratedOutput(input), nil
}

func (e *revisionFailingExecutor) PublishSlot(_ context.Context, input imageagent.SlotExecutionInput, _ imageagent.SlotGeneratedOutput) (imageagent.SlotExecutionResult, error) {
	return successfulSlotResult(input.Slot.ID, input.Attempt), nil
}

func (e *identityCheckingExecutor) ExecuteSlot(ctx context.Context, input imageagent.SlotExecutionInput) (imageagent.SlotExecutionResult, error) {
	e.calls++
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	require.True(e.t, ok)
	require.Equal(e.t, input.TenantID, identity.TenantID)
	require.Equal(e.t, input.UserID, identity.UserID)
	return successfulSlotResult(input.Slot.ID, input.Attempt), nil
}

func (e *identityCheckingExecutor) GenerateSlot(ctx context.Context, input imageagent.SlotExecutionInput) (imageagent.SlotGeneratedOutput, error) {
	e.calls++
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	require.True(e.t, ok)
	require.Equal(e.t, input.TenantID, identity.TenantID)
	require.Equal(e.t, input.UserID, identity.UserID)
	return testGeneratedOutput(input), nil
}

func (e *identityCheckingExecutor) PublishSlot(_ context.Context, input imageagent.SlotExecutionInput, _ imageagent.SlotGeneratedOutput) (imageagent.SlotExecutionResult, error) {
	return successfulSlotResult(input.Slot.ID, input.Attempt), nil
}

func testGeneratedOutput(input imageagent.SlotExecutionInput) imageagent.SlotGeneratedOutput {
	return imageagent.SlotGeneratedOutput{SlotID: input.Slot.ID, Attempt: input.Attempt, SourceAssetID: input.Slot.SourceAssetIDs[0], Assets: []imageagent.GeneratedAsset{{URL: "C:/generated/" + input.Slot.ID + ".png", Metadata: map[string]string{"local_path": "C:/generated/" + input.Slot.ID + ".png"}}}}
}

type identityCheckingPublisher struct {
	t     *testing.T
	calls int
}

func (p *identityCheckingPublisher) PublishApproved(ctx context.Context, input imageagent.PublishApprovedInput) (imageagent.PublicationAcknowledgement, error) {
	p.calls++
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	require.True(p.t, ok)
	require.Equal(p.t, input.TenantID, identity.TenantID)
	require.Equal(p.t, "user-a", identity.UserID)
	return imageagent.PublicationAcknowledgement{}, nil
}

func (p *identityCheckingPublisher) PublishApprovedV3(ctx context.Context, input imageagent.PublishApprovedV3Input) (imageagent.PublicationAcknowledgement, error) {
	p.calls++
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	require.True(p.t, ok)
	require.Equal(p.t, input.TenantID, identity.TenantID)
	require.Equal(p.t, "user-a", identity.UserID)
	return imageagent.PublicationAcknowledgement{}, nil
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

func (*recordingDomainWorkflowClient) Resume(context.Context, imageagent.ResumeCommand) (imageagent.CommandAcknowledgement, error) {
	return imageagent.CommandAcknowledgement{}, nil
}

type workflowCatalogResolver struct{}

func (workflowCatalogResolver) Resolve(context.Context, imageagent.AssetCatalogScope) (imageagent.AssetCatalog, error) {
	assets := make([]imageagent.AuthorizedAsset, 0, 10)
	for index := 1; index <= 9; index++ {
		assets = append(assets, imageagent.AuthorizedAsset{ID: fmt.Sprintf("source-%d", index), Type: imageagent.AuthorizedAssetSource, Width: 1200, Height: 900})
	}
	assets = append(assets, imageagent.AuthorizedAsset{ID: "style-modern", Type: imageagent.AuthorizedAssetStyle})
	return imageagent.AssetCatalog{Assets: assets}, nil
}

type recordingSDKClient struct {
	startOptions       sdkclient.StartWorkflowOptions
	workflowName       string
	workflowInput      WorkflowInput
	signalWorkflowID   string
	signalName         string
	signalArg          interface{}
	queryWorkflowID    string
	queryType          string
	queryValue         imageagent.WorkflowProjection
	updateOptions      []sdkclient.UpdateWorkflowOptions
	updateGetCalls     int
	updateErr          error
	updateResultErr    error
	updateResultErrors []error
	updateAck          imageagent.CommandAcknowledgement
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
	return &recordingWorkflowUpdateHandle{client: c, workflowID: options.WorkflowID, updateID: options.UpdateID, callIndex: len(c.updateOptions) - 1}, nil
}

type recordingWorkflowUpdateHandle struct {
	client     *recordingSDKClient
	workflowID string
	updateID   string
	callIndex  int
}

func (h *recordingWorkflowUpdateHandle) WorkflowID() string { return h.workflowID }
func (*recordingWorkflowUpdateHandle) RunID() string        { return "" }
func (h *recordingWorkflowUpdateHandle) UpdateID() string   { return h.updateID }
func (h *recordingWorkflowUpdateHandle) Get(_ context.Context, target interface{}) error {
	h.client.updateGetCalls++
	if acknowledgement, ok := target.(*imageagent.CommandAcknowledgement); ok {
		*acknowledgement = h.client.updateAck
	}
	if h.callIndex < len(h.client.updateResultErrors) {
		return h.client.updateResultErrors[h.callIndex]
	}
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
