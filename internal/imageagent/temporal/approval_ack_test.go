package temporal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	"task-processor/internal/imageagent"
)

// The Asset owner may commit while its activity acknowledgement is lost.
// Only the original publication action may reconcile that outcome.
func TestManualWorkflowApprovalLostACKCannotBeSupersededByCancel(t *testing.T) {
	env := newWorkflowEnv(t)
	env.OnGetVersion(externalEffectFinalizationPatch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	publications, receiptReads := 0, 0
	var originalKey string
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			publications++
			originalKey = activityInputFromArgs[PublishApprovedActivityInput](t, args).IdempotencyKey
		}).Return(sdktemporal.NewNonRetryableApplicationError("Asset committed, acknowledgement lost", "publication_ack_unknown", nil)).Once()
	env.OnActivity(activityPublishApproved, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			receiptReads++
			require.Equal(t, originalKey, activityInputFromArgs[PublishApprovedActivityInput](t, args).IdempotencyKey)
		}).Return(nil).Once()
	cancelledWrites := 0
	env.OnActivity(activityPersistRunState, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		if activityInputFromArgs[PersistRunStateActivityInput](t, args).Projection.Status == imageagent.RunStatusCancelled {
			cancelledWrites++
		}
	}).Return(nil)
	command := validApproval("approve-lost-ack")
	var approveErr, cancelErr, resumeErr error
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalApproveResults, "approve-lost-ack", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { approveErr = err }, OnAccept: func() {}, OnComplete: func(_ interface{}, err error) { approveErr = err },
		}, command)
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalCancel, "cancel-after-lost-ack", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { cancelErr = err }, OnAccept: func() {}, OnComplete: func(_ interface{}, err error) { cancelErr = err },
		}, CancelSignal{RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "cancel-after-lost-ack"})
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(updateResumeCommand, "resume-original-approval", &testsuite.TestUpdateCallback{
			OnReject: func(err error) { resumeErr = err }, OnAccept: func() {}, OnComplete: func(_ interface{}, err error) { resumeErr = err },
		}, ResumeCommandInput{RunID: "run-1", ActorID: "user-a", ActionID: command.ActionID})
	}, 3*time.Second)
	env.RegisterDelayedCallback(func() { env.CancelWorkflow() }, 5*time.Second)
	env.ExecuteWorkflow(ImageAgentWorkflow, manualWorkflowInput(plan))
	require.Error(t, approveErr)
	require.ErrorContains(t, cancelErr, "approval publication")
	require.NoError(t, resumeErr)
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 1, publications)
	require.Equal(t, 1, receiptReads)
	require.Zero(t, cancelledWrites)
	var result WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.RunStatusCompleted, result.Status)
	env.AssertExpectations(t)
}
