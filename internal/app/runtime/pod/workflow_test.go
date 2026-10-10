package podruntime

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	common "go.temporal.io/api/common/v1"
	enums "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	info "go.temporal.io/api/workflow/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/testsuite"
	podapp "task-processor/internal/app/pod"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/pod"
	"testing"
	"time"
)

func TestObservationBudgetIncludesActivityTime(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	started := time.Now().UTC()
	env.SetStartTime(started)
	process := func(context.Context, podapp.Execution) (podapp.ExecutionResult, error) {
		return podapp.ExecutionResult{Wait: true}, nil
	}
	observe := func(context.Context, podapp.Execution) (podapp.ExecutionResult, error) {
		return podapp.ExecutionResult{Wait: true}, nil
	}
	env.RegisterActivityWithOptions(process, activity.RegisterOptions{Name: processActivity})
	env.RegisterActivityWithOptions(observe, activity.RegisterOptions{Name: observeActivity})
	env.OnActivity(observeActivity, mock.Anything, mock.Anything).After(20*time.Second).Return(podapp.ExecutionResult{Wait: true}, nil)
	env.ExecuteWorkflow(DesignWorkflow, podapp.Execution{Scope: collection.Scope{"org", "actor", "member"}, OperationID: uuid.NewString()})
	require.NoError(t, env.GetWorkflowError())
	require.LessOrEqual(t, env.Now().Sub(started), 15*time.Minute+30*time.Second)
}

func TestProvenPreSendFailureRetriesWithinOriginalWorkflow(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	calls := 0
	process := func(context.Context, podapp.Execution) (podapp.ExecutionResult, error) {
		calls++
		if calls == 1 {
			return podapp.ExecutionResult{NotStarted: true}, nil
		}
		return podapp.ExecutionResult{Done: true}, nil
	}
	env.RegisterActivityWithOptions(process, activity.RegisterOptions{Name: processActivity})
	env.ExecuteWorkflow(DesignWorkflow, podapp.Execution{Scope: collection.Scope{"org", "actor", "member"}, OperationID: uuid.NewString()})
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 2, calls)
}

func TestPreSendBudgetFailsWithoutReplacingWorkflowOrObserving(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	started := time.Now().UTC()
	env.SetStartTime(started)
	process := func(context.Context, podapp.Execution) (podapp.ExecutionResult, error) {
		return podapp.ExecutionResult{NotStarted: true}, nil
	}
	env.RegisterActivityWithOptions(process, activity.RegisterOptions{Name: processActivity})
	env.ExecuteWorkflow(DesignWorkflow, podapp.Execution{Scope: collection.Scope{"org", "actor", "member"}, OperationID: uuid.NewString()})
	require.Error(t, env.GetWorkflowError())
	require.LessOrEqual(t, env.Now().Sub(started), 15*time.Minute+time.Second)
}

func TestUnknownAttemptNeverRetriesProcess(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	calls := 0
	process := func(context.Context, podapp.Execution) (podapp.ExecutionResult, error) {
		calls++
		return podapp.ExecutionResult{Unknown: true}, nil
	}
	env.RegisterActivityWithOptions(process, activity.RegisterOptions{Name: processActivity})
	env.ExecuteWorkflow(DesignWorkflow, podapp.Execution{Scope: collection.Scope{"org", "actor", "member"}, OperationID: uuid.NewString()})
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 1, calls)
}

func TestWorkflowTimeoutCoversLateProcessAndFullObservationBudget(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	started := time.Now().UTC()
	env.SetStartTime(started)
	calls := 0
	process := func(context.Context, podapp.Execution) (podapp.ExecutionResult, error) {
		calls++
		if calls < 5 {
			return podapp.ExecutionResult{NotStarted: true}, nil
		}
		return podapp.ExecutionResult{Wait: true}, nil
	}
	observe := func(context.Context, podapp.Execution) (podapp.ExecutionResult, error) {
		return podapp.ExecutionResult{Wait: true}, nil
	}
	env.RegisterActivityWithOptions(process, activity.RegisterOptions{Name: processActivity})
	env.RegisterActivityWithOptions(observe, activity.RegisterOptions{Name: observeActivity})
	env.OnActivity(processActivity, mock.Anything, mock.Anything).After(3 * time.Minute).Return(process)
	env.OnActivity(observeActivity, mock.Anything, mock.Anything).After(30*time.Second).Return(podapp.ExecutionResult{Wait: true}, nil)
	in := podapp.Execution{Scope: collection.Scope{"org", "actor", "member"}, OperationID: uuid.NewString()}
	f := &temporalFixture{missing: true, state: enums.WORKFLOW_EXECUTION_STATUS_RUNNING, id: WorkflowID(in.OperationID)}
	require.NoError(t, (TemporalStarter{Client: f}).Ensure(context.Background(), in))
	env.ExecuteWorkflow(DesignWorkflow, in)
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 5, calls)
	require.Greater(t, env.Now().Sub(started), 30*time.Minute, "late send must still receive its full observation window")
	require.Less(t, env.Now().Sub(started), f.starts[0].WorkflowExecutionTimeout)
}

type temporalFixture struct {
	starts  []client.StartWorkflowOptions
	in      []podapp.Execution
	missing bool
	state   enums.WorkflowExecutionStatus
	id      string
}

func (f *temporalFixture) ExecuteWorkflow(_ context.Context, o client.StartWorkflowOptions, _ interface{}, args ...interface{}) (client.WorkflowRun, error) {
	f.starts = append(f.starts, o)
	f.in = append(f.in, args[0].(podapp.Execution))
	f.missing = false
	return nil, nil
}
func (f *temporalFixture) DescribeWorkflowExecution(context.Context, string, string) (*workflowservice.DescribeWorkflowExecutionResponse, error) {
	if f.missing {
		return nil, &serviceerror.NotFound{Message: "not started"}
	}
	return &workflowservice.DescribeWorkflowExecutionResponse{WorkflowExecutionInfo: &info.WorkflowExecutionInfo{Execution: &common.WorkflowExecution{WorkflowId: f.id}, Type: &common.WorkflowType{Name: WorkflowName}, Status: f.state}}, nil
}
func TestEnsureStartsOnlyMissingOriginalWorkflow(t *testing.T) {
	in := podapp.Execution{Scope: collection.Scope{"org", "actor", "member"}, OperationID: uuid.NewString()}
	f := &temporalFixture{missing: true, state: enums.WORKFLOW_EXECUTION_STATUS_RUNNING, id: WorkflowID(in.OperationID)}
	s := TemporalStarter{Client: f}
	require.NoError(t, s.Ensure(context.Background(), in))
	require.NoError(t, s.Ensure(context.Background(), in))
	require.Len(t, f.starts, 1)
	require.Equal(t, in, f.in[0])
	require.Equal(t, f.id, f.starts[0].ID)
	require.Equal(t, enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE, f.starts[0].WorkflowIDReusePolicy)
	f.state = enums.WORKFLOW_EXECUTION_STATUS_FAILED
	require.ErrorIs(t, s.Ensure(context.Background(), in), pod.ErrUnknown)
	require.Len(t, f.starts, 1)
	f.state = enums.WORKFLOW_EXECUTION_STATUS_COMPLETED
	require.ErrorIs(t, s.Ensure(context.Background(), in), pod.ErrUnknown, "closed workflows cannot silently represent an unstarted QUEUED operation")
	require.Len(t, f.starts, 1)
}
