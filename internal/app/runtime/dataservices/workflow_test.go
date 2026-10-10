package dataservicesruntime

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	workflowpb "go.temporal.io/api/workflow/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/mocks"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"sync/atomic"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/dataacquisition"
	"testing"
	"time"
)

func TestWorkflowRetriesOriginalJobAndEndsWithBoundedDeadlineCleanup(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(func(context.Context, Execution) error { return nil }, activity.RegisterOptions{Name: RunActivityName})
	env.RegisterActivityWithOptions(func(context.Context, Execution) error { return nil }, activity.RegisterOptions{Name: CleanupActivityName})
	input := Execution{Scope: collection.Scope{OrganizationID: "org", ActorID: "creator", MemberID: "original"}, JobID: uuid.NewString(), InputHash: collection.Digest("original"), Deadline: time.Now().UTC().Add(15 * time.Second)}
	env.OnActivity(RunActivityName, mock.Anything, input).Return(func(context.Context, Execution) error { return context.DeadlineExceeded })
	env.OnActivity(CleanupActivityName, mock.Anything, input).Return(nil).Once()
	env.ExecuteWorkflow(DataWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	env.AssertExpectations(t)
}
func TestRunActivityCoversBoundedDiscoveryWithoutExtendingOriginalDeadline(t *testing.T) {
	for _, remaining := range []time.Duration{30 * time.Minute, 40 * time.Second} {
		t.Run(remaining.String(), func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			env := suite.NewTestWorkflowEnvironment()
			now := time.Now().UTC()
			env.SetStartTime(now)
			window := make(chan time.Duration, 1)
			env.RegisterActivityWithOptions(func(ctx context.Context, _ Execution) error {
				info := activity.GetInfo(ctx)
				window <- info.Deadline.Sub(info.StartedTime)
				return nil
			}, activity.RegisterOptions{Name: RunActivityName})
			input := Execution{Scope: collection.Scope{OrganizationID: "org", ActorID: "creator", MemberID: "original"}, JobID: uuid.NewString(), InputHash: collection.Digest("original"), Deadline: now.Add(remaining)}
			env.ExecuteWorkflow(DataWorkflow, input)
			require.True(t, env.IsWorkflowCompleted())
			require.NoError(t, env.GetWorkflowError())
			actual := <-window
			if remaining >= 6*time.Minute {
				require.Greater(t, actual, 5*time.Minute, "activity must cover the existing five-minute discovery plus persistence")
				require.LessOrEqual(t, actual, 6*time.Minute)
			} else {
				require.LessOrEqual(t, actual, remaining, "activity cannot extend the original job deadline")
				require.Positive(t, actual)
			}
		})
	}
}
func TestDeadlineCleanupRetriesOriginalExecutionAfterRepeatedUnavailable(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetStartWorkflowOptions(client.StartWorkflowOptions{WorkflowExecutionTimeout: 40 * time.Minute})
	env.RegisterActivityWithOptions(func(context.Context, Execution) error { return nil }, activity.RegisterOptions{Name: CleanupActivityName})
	input := Execution{Scope: collection.Scope{OrganizationID: "org", ActorID: "creator", MemberID: "original"}, JobID: uuid.NewString(), InputHash: collection.Digest("original"), Deadline: time.Now().UTC().Add(-time.Minute)}
	env.OnActivity(CleanupActivityName, mock.Anything, input).Return(errors.New("Product database temporarily unavailable")).Times(4)
	env.OnActivity(CleanupActivityName, mock.Anything, input).Return(nil).Once()
	env.ExecuteWorkflow(DataWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError(), "cleanup must survive more than three failures within the original execution window")
	env.AssertExpectations(t)
}

func TestDeadlineCleanupRecoversAfterNineMinuteDatabaseOutage(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetStartWorkflowOptions(client.StartWorkflowOptions{WorkflowExecutionTimeout: 40 * time.Minute})
	var available atomic.Bool
	env.RegisterDelayedCallback(func() { available.Store(true) }, 9*time.Minute)
	env.RegisterActivityWithOptions(func(context.Context, Execution) error { return nil }, activity.RegisterOptions{Name: CleanupActivityName})
	input := Execution{Scope: collection.Scope{OrganizationID: "org", ActorID: "creator", MemberID: "original"}, JobID: uuid.NewString(), InputHash: collection.Digest("original"), Deadline: time.Now().UTC().Add(-time.Minute)}
	env.OnActivity(CleanupActivityName, mock.Anything, input).Return(func(context.Context, Execution) error {
		if !available.Load() {
			return temporal.NewApplicationErrorWithOptions("Product database temporarily unavailable", "fixture", temporal.ApplicationErrorOptions{NextRetryDelay: 3 * time.Minute})
		}
		return nil
	})
	env.ExecuteWorkflow(DataWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError(), "cleanup must use the remaining workflow window rather than stop after eight minutes")
}

func TestStarterBoundsOriginalExecutionIncludingCleanup(t *testing.T) {
	job := dataacquisition.Job{Scope: collection.Scope{OrganizationID: "org", ActorID: "creator", MemberID: "original"}, ID: uuid.NewString(), InputHash: collection.Digest("original"), Deadline: time.Now().UTC().Add(time.Minute)}
	input := execution(job)
	id := "data-v1-" + collection.StableID(job.Scope.OrganizationID, job.Scope.ActorID, job.ID)
	c := &mocks.Client{}
	c.On("DescribeWorkflowExecution", mock.Anything, id, "").Return(nil, serviceerror.NewNotFound("original workflow not yet started")).Once()
	c.On("ExecuteWorkflow", mock.Anything, mock.MatchedBy(func(options client.StartWorkflowOptions) bool {
		memo, ok := options.Memo["dataExecution"].(Execution)
		return options.ID == id && options.TaskQueue == DataTaskQueue && options.WorkflowExecutionTimeout == 40*time.Minute && options.WorkflowIDReusePolicy == enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY && ok && memo.same(input)
	}), DataWorkflowName, input).Return(nil, nil).Once()
	require.NoError(t, (TemporalStarter{Client: c}).EnsureExecution(context.Background(), job))
	c.AssertExpectations(t)
}

func TestExecutionLookupVerifiesMemoWorkflowTypeAndOriginalID(t *testing.T) {
	input := Execution{Scope: collection.Scope{OrganizationID: "org", ActorID: "creator", MemberID: "member"}, JobID: uuid.NewString(), InputHash: collection.Digest("input"), Deadline: time.Now().UTC().Add(time.Hour)}
	payload, err := converter.GetDefaultDataConverter().ToPayload(input)
	require.NoError(t, err)
	described := &workflowservice.DescribeWorkflowExecutionResponse{WorkflowExecutionInfo: &workflowpb.WorkflowExecutionInfo{Memo: &commonpb.Memo{Fields: map[string]*commonpb.Payload{"dataExecution": payload}}, Execution: &commonpb.WorkflowExecution{WorkflowId: "data-v1-" + collection.StableID(input.Scope.OrganizationID, input.Scope.ActorID, input.JobID)}, Type: &commonpb.WorkflowType{Name: "AnotherWorkflow"}}}
	require.ErrorIs(t, verifyMemo(described, input), dataacquisition.ErrConflict)
	described.WorkflowExecutionInfo.Type.Name = DataWorkflowName
	require.NoError(t, verifyMemo(described, input))
	described.WorkflowExecutionInfo.Execution.WorkflowId = "different"
	require.ErrorIs(t, verifyMemo(described, input), dataacquisition.ErrConflict)
}
