package supplychainruntime

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	common "go.temporal.io/api/common/v1"
	enums "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	info "go.temporal.io/api/workflow/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/testsuite"
	supplyapp "task-processor/internal/app/supplychain"
	"task-processor/internal/listing/preparation"
	"testing"
)

func TestSupplyWorkflowReadsEveryPageAndSkipsTerminalUnknownItems(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	seen := []string{}
	sourceIDs := []string{uuid.NewString(), uuid.NewString(), uuid.NewString()}
	id := uuid.NewString()
	env.RegisterActivityWithOptions(func(_ context.Context, input supplyapp.OperationExecution) (supplyapp.OperationPage, error) {
		require.Equal(t, "org-a", input.OrganizationID)
		require.Equal(t, id, input.OperationID)
		switch input.After {
		case "":
			return supplyapp.OperationPage{Items: []preparation.OperationItem{{SourceID: sourceIDs[0], Status: preparation.ItemPending}, {SourceID: sourceIDs[1], Status: preparation.ItemUnknown}}, NextCursor: sourceIDs[1]}, nil
		case sourceIDs[1]:
			return supplyapp.OperationPage{Items: []preparation.OperationItem{{SourceID: sourceIDs[2], Status: preparation.ItemPending}}}, nil
		default:
			t.Fatalf("unexpected cursor %s", input.After)
			return supplyapp.OperationPage{}, nil
		}
	}, activity.RegisterOptions{Name: supplyListActivity})
	env.RegisterActivityWithOptions(func(_ context.Context, input supplyapp.OperationExecution, source string) (preparation.OperationItem, error) {
		seen = append(seen, source)
		return preparation.OperationItem{SourceID: source, Status: preparation.ItemSucceeded}, nil
	}, activity.RegisterOptions{Name: supplyItemActivity})
	env.ExecuteWorkflow(SupplyOperationWorkflow, supplyapp.OperationExecution{OrganizationID: "org-a", OperationID: id})
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, []string{sourceIDs[0], sourceIDs[2]}, seen)
}
func TestSupplyWorkflowStopsAtDurableCancellation(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(func(context.Context, supplyapp.OperationExecution) (supplyapp.OperationPage, error) {
		return supplyapp.OperationPage{Cancelled: true}, nil
	}, activity.RegisterOptions{Name: supplyListActivity})
	env.ExecuteWorkflow(SupplyOperationWorkflow, supplyapp.OperationExecution{OrganizationID: "org-a", OperationID: uuid.NewString()})
	require.NoError(t, env.GetWorkflowError())
}

type operationTemporalFixture struct {
	options     []client.StartWorkflowOptions
	inputs      []supplyapp.OperationExecution
	startErr    error
	description *workflowservice.DescribeWorkflowExecutionResponse
}

func (f *operationTemporalFixture) ExecuteWorkflow(_ context.Context, options client.StartWorkflowOptions, name interface{}, args ...interface{}) (client.WorkflowRun, error) {
	f.options = append(f.options, options)
	f.inputs = append(f.inputs, args[0].(supplyapp.OperationExecution))
	return nil, f.startErr
}
func (f *operationTemporalFixture) DescribeWorkflowExecution(_ context.Context, id, run string) (*workflowservice.DescribeWorkflowExecutionResponse, error) {
	return f.description, nil
}
func TestTemporalOperationEnsureAlwaysUsesOriginalIDAndNeverRestartsClosedExecution(t *testing.T) {
	id := uuid.NewString()
	workflowID := preparation.WorkflowID("org-a", id)
	fixture := &operationTemporalFixture{description: &workflowservice.DescribeWorkflowExecutionResponse{WorkflowExecutionInfo: &info.WorkflowExecutionInfo{Execution: &common.WorkflowExecution{WorkflowId: workflowID}, Type: &common.WorkflowType{Name: SupplyWorkflowName}, Status: enums.WORKFLOW_EXECUTION_STATUS_RUNNING}}}
	starter := TemporalOperationStarter{Client: fixture}
	require.NoError(t, starter.Ensure(context.Background(), "org-a", id))
	fixture.startErr = &serviceerror.WorkflowExecutionAlreadyStarted{}
	require.NoError(t, starter.Ensure(context.Background(), "org-a", id))
	for _, options := range fixture.options {
		require.Equal(t, workflowID, options.ID)
		require.Equal(t, enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE, options.WorkflowIDReusePolicy)
		require.Equal(t, enums.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING, options.WorkflowIDConflictPolicy)
	}
	fixture.description.WorkflowExecutionInfo.Status = enums.WORKFLOW_EXECUTION_STATUS_FAILED
	require.ErrorIs(t, starter.Ensure(context.Background(), "org-a", id), preparation.ErrUnknown)
	fixture.description.WorkflowExecutionInfo.Type.Name = "other-workflow"
	require.ErrorIs(t, starter.Ensure(context.Background(), "org-a", id), preparation.ErrUnknown)
}
