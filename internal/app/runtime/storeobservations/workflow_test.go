package storeobservationsruntime

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	o "task-processor/internal/marketplace/shein/observations"
	"testing"
)

func TestWorkflowUsesPersistedScopeAndStopsOnTerminal(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	id := uuid.NewString()
	calls := 0
	env.RegisterActivityWithOptions(func(_ context.Context, in Execution) (o.Sync, error) {
		require.Equal(t, "org-a", in.OrganizationID)
		require.Equal(t, id, in.SyncID)
		calls++
		if calls == 3 {
			return o.Sync{Status: "suspended"}, nil
		}
		return o.Sync{Status: "running"}, nil
	}, activity.RegisterOptions{Name: stepActivity})
	env.ExecuteWorkflow(ObservationWorkflow, Execution{"org-a", id})
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 3, calls)
}
func TestReadRetryExhaustionPersistsFailureWithoutMorePlatformReads(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	calls, failures := 0, 0
	env.RegisterActivityWithOptions(func(context.Context, Execution) (o.Sync, error) {
		calls++
		return o.Sync{}, errors.New("temporary provider failure")
	}, activity.RegisterOptions{Name: stepActivity})
	env.RegisterActivityWithOptions(func(context.Context, Execution) (o.Sync, error) {
		failures++
		if failures <= 6 {
			return o.Sync{}, errors.New("temporary database outage")
		}
		return o.Sync{Status: "failed"}, nil
	}, activity.RegisterOptions{Name: failActivity})
	env.ExecuteWorkflow(ObservationWorkflow, Execution{"org-a", uuid.NewString()})
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 5, calls)
	require.Equal(t, 7, failures)
}
