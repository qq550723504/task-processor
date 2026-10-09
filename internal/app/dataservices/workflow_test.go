package dataservicesapp

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"task-processor/internal/product/collection"
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
