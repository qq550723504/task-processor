package podruntime

import (
	"context"
	"errors"
	enums "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	podapp "task-processor/internal/app/pod"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/pod"
	"time"
)

const TaskQueue = "product-pod-current"
const WorkflowName = "ProductPODDesignV1"
const processActivity = "ProductPODProcessV1"
const observeActivity = "ProductPODObserveV1"

func WorkflowID(id string) string { return "pod-design/" + id }
func DesignWorkflow(ctx workflow.Context, in podapp.Execution) error {
	if in.Scope.Validate() != nil || !collection.ValidID(in.OperationID) {
		return temporal.NewNonRetryableApplicationError("invalid POD identity", "invalid", nil)
	}
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 3 * time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}})
	var result podapp.ExecutionResult
	if e := workflow.ExecuteActivity(ctx, processActivity, in).Get(ctx, &result); e != nil {
		return e
	}
	if result.Done || result.Unknown {
		return nil
	}
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}})
	deadline := workflow.Now(ctx).Add(15 * time.Minute)
	for workflow.Now(ctx).Before(deadline) {
		if e := workflow.Sleep(ctx, 5*time.Second); e != nil {
			return e
		}
		if !workflow.Now(ctx).Before(deadline) {
			break
		}
		if e := workflow.ExecuteActivity(ctx, observeActivity, in).Get(ctx, &result); e != nil {
			return e
		}
		if result.Done || result.Unknown {
			return nil
		}
	}
	return nil // The original durable intent/fence remains; query is read-only.
}

type WorkflowClient interface {
	ExecuteWorkflow(context.Context, client.StartWorkflowOptions, interface{}, ...interface{}) (client.WorkflowRun, error)
	DescribeWorkflowExecution(context.Context, string, string) (*workflowservice.DescribeWorkflowExecutionResponse, error)
}
type TemporalStarter struct{ Client WorkflowClient }

func (s TemporalStarter) Ensure(ctx context.Context, in podapp.Execution) error {
	if ctx == nil || s.Client == nil || in.Scope.Validate() != nil || !collection.ValidID(in.OperationID) {
		return pod.ErrUnavailable
	}
	id := WorkflowID(in.OperationID)
	description, e := s.Client.DescribeWorkflowExecution(ctx, id, "")
	if e == nil {
		return describedOriginal(description, id)
	}
	var missing *serviceerror.NotFound
	if !errors.As(e, &missing) {
		return pod.ErrUnknown
	}
	_, e = s.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: id, TaskQueue: TaskQueue, WorkflowExecutionTimeout: 30 * time.Minute, WorkflowIDConflictPolicy: enums.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING, WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE}, WorkflowName, in)
	var exists *serviceerror.WorkflowExecutionAlreadyStarted
	if e != nil && !errors.As(e, &exists) {
		return pod.ErrUnknown
	}
	description, e = s.Client.DescribeWorkflowExecution(ctx, id, "")
	if e != nil {
		return pod.ErrUnknown
	}
	return describedOriginal(description, id)
}
func describedOriginal(description *workflowservice.DescribeWorkflowExecutionResponse, id string) error {
	if description == nil || description.WorkflowExecutionInfo == nil {
		return pod.ErrUnknown
	}
	info := description.WorkflowExecutionInfo
	if info.Execution == nil || info.Execution.WorkflowId != id || info.Type == nil || info.Type.Name != WorkflowName {
		return pod.ErrUnknown
	}
	if info.Status != enums.WORKFLOW_EXECUTION_STATUS_RUNNING && info.Status != enums.WORKFLOW_EXECUTION_STATUS_COMPLETED {
		return pod.ErrUnknown
	}
	return nil
}
func NewWorker(c client.Client, p *podapp.Processor) (worker.Worker, error) {
	if c == nil || p == nil || p.Repository == nil || p.Kernel == nil || p.Mutations == nil || p.Observer == nil {
		return nil, pod.ErrUnavailable
	}
	w := worker.New(c, TaskQueue, worker.Options{MaxConcurrentActivityExecutionSize: 2})
	w.RegisterWorkflowWithOptions(DesignWorkflow, workflow.RegisterOptions{Name: WorkflowName})
	w.RegisterActivityWithOptions(p.Process, activity.RegisterOptions{Name: processActivity})
	w.RegisterActivityWithOptions(p.Observe, activity.RegisterOptions{Name: observeActivity})
	return w, nil
}
