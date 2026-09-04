package temporal

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	sdkactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

	"task-processor/internal/listingkit"
)

func TestPlatformAdaptWorkflowPersistsTerminalFailureAfterRetries(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	adaptAttempts := 0
	persistCalls := 0
	env.RegisterActivityWithOptions(func(context.Context, PlatformAdaptWorkflowInput) (*listingkit.ListingKitResult, error) {
		adaptAttempts++
		return nil, errors.New("build amazon draft")
	}, sdkactivity.RegisterOptions{Name: activityNameProcessPlatformAdapt})
	env.RegisterActivityWithOptions(func(_ context.Context, in LayerFailureInput) error {
		persistCalls++
		require.Equal(t, "task-1", in.TaskID)
		require.Contains(t, in.Error, "build amazon draft")
		return nil
	}, sdkactivity.RegisterOptions{Name: activityNamePersistLayerFailure})

	env.ExecuteWorkflow(PlatformAdaptWorkflow, PlatformAdaptWorkflowInput{TaskID: "task-1", Platform: "amazon"})

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	require.Equal(t, 3, adaptAttempts)
	require.Equal(t, 1, persistCalls)
}

func TestPlatformAdaptWorkflowReportsFailurePersistenceError(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(func(context.Context, PlatformAdaptWorkflowInput) (*listingkit.ListingKitResult, error) {
		return nil, errors.New("build amazon draft")
	}, sdkactivity.RegisterOptions{Name: activityNameProcessPlatformAdapt})
	env.RegisterActivityWithOptions(func(context.Context, LayerFailureInput) error {
		return errors.New("persist terminal state")
	}, sdkactivity.RegisterOptions{Name: activityNamePersistLayerFailure})

	env.ExecuteWorkflow(PlatformAdaptWorkflow, PlatformAdaptWorkflowInput{TaskID: "task-1", Platform: "amazon"})

	workflowErr := env.GetWorkflowError()
	require.Error(t, workflowErr)
	require.ErrorContains(t, workflowErr, "build amazon draft")
	require.ErrorContains(t, workflowErr, "persist terminal state")
}

func TestStandardProductWorkflowPersistsTerminalFailureAfterRetries(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	adaptAttempts := 0
	persistCalls := 0
	env.RegisterActivityWithOptions(func(context.Context, StandardProductWorkflowInput) (*listingkit.StandardProductSnapshot, error) {
		adaptAttempts++
		return nil, errors.New("build standard snapshot")
	}, sdkactivity.RegisterOptions{Name: activityNameProcessStandardProduct})
	env.RegisterActivityWithOptions(func(_ context.Context, in LayerFailureInput) error {
		persistCalls++
		require.Equal(t, "task-1", in.TaskID)
		require.Contains(t, in.Error, "build standard snapshot")
		return nil
	}, sdkactivity.RegisterOptions{Name: activityNamePersistLayerFailure})

	env.ExecuteWorkflow(StandardProductWorkflow, StandardProductWorkflowInput{TaskID: "task-1"})

	require.Error(t, env.GetWorkflowError())
	require.Equal(t, 3, adaptAttempts)
	require.Equal(t, 1, persistCalls)
}
