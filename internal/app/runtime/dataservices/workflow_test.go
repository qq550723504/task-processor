package dataservicesruntime

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	commonpb "go.temporal.io/api/common/v1"
	workflowpb "go.temporal.io/api/workflow/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
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
