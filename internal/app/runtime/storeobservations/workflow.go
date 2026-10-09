package storeobservationsruntime

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
	"task-processor/internal/authidentity"
	o "task-processor/internal/marketplace/shein/observations"
	"time"
)

const TaskQueue = "store-observations-current"
const WorkflowName = "StoreObservationsV1"
const stepActivity = "StoreObservationsPageV1"
const failActivity = "StoreObservationsFailV1"

type Execution struct{ OrganizationID, SyncID string }

func ObservationWorkflow(ctx workflow.Context, in Execution) error {
	if !authidentity.IsBoundedIdentifier(in.OrganizationID) || !o.ValidID(in.SyncID) {
		return temporal.NewNonRetryableApplicationError("invalid observation identity", "invalid", nil)
	}
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second, ScheduleToCloseTimeout: 3 * time.Minute, RetryPolicy: &temporal.RetryPolicy{InitialInterval: 2 * time.Second, MaximumInterval: 30 * time.Second, MaximumAttempts: 5}})
	for pages := 0; pages < 100; pages++ {
		var s o.Sync
		e := workflow.ExecuteActivity(ctx, stepActivity, in).Get(ctx, &s)
		if e != nil {
			// Persist exhaustion through the same repository owner. If SQL is down,
			// this activity retries; the sync stays discoverable, never falsely done.
			persist := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second, RetryPolicy: &temporal.RetryPolicy{InitialInterval: 2 * time.Second, MaximumInterval: time.Minute}})
			return workflow.ExecuteActivity(persist, failActivity, in).Get(ctx, nil)
		}
		if s.Terminal() {
			return nil
		}
	}
	return workflow.NewContinueAsNewError(ctx, WorkflowName, in)
}

type Activities struct{ Service *o.Service }

func (a *Activities) Step(ctx context.Context, in Execution) (o.Sync, error) {
	s, e := a.Service.Step(ctx, in.OrganizationID, in.SyncID)
	if errors.Is(e, o.ErrConflict) {
		return a.Service.Repository.ReadSync(ctx, in.OrganizationID, in.SyncID)
	}
	return s, e
}
func (a *Activities) Fail(ctx context.Context, in Execution) (o.Sync, error) {
	return a.Service.Fail(ctx, in.OrganizationID, in.SyncID)
}

type WorkflowClient interface {
	ExecuteWorkflow(context.Context, client.StartWorkflowOptions, interface{}, ...interface{}) (client.WorkflowRun, error)
	DescribeWorkflowExecution(context.Context, string, string) (*workflowservice.DescribeWorkflowExecutionResponse, error)
}
type Starter struct{ Client WorkflowClient }

func (s Starter) Ensure(ctx context.Context, org, id string) error {
	if ctx == nil || s.Client == nil || !authidentity.IsBoundedIdentifier(org) || !o.ValidID(id) {
		return o.ErrInvalid
	}
	workflowID := "store-observations/" + org + "/" + id
	_, e := s.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: workflowID, TaskQueue: TaskQueue, WorkflowExecutionTimeout: 24 * time.Hour, WorkflowIDConflictPolicy: enums.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING, WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE}, WorkflowName, Execution{org, id})
	var existing *serviceerror.WorkflowExecutionAlreadyStarted
	if e != nil && !errors.As(e, &existing) {
		return o.ErrUnavailable
	}
	description, e := s.Client.DescribeWorkflowExecution(ctx, workflowID, "")
	if e != nil || description == nil || description.WorkflowExecutionInfo == nil {
		return o.ErrUnavailable
	}
	info := description.WorkflowExecutionInfo
	if info.Execution == nil || info.Execution.WorkflowId != workflowID || info.Type == nil || info.Type.Name != WorkflowName || info.Status != enums.WORKFLOW_EXECUTION_STATUS_RUNNING && info.Status != enums.WORKFLOW_EXECUTION_STATUS_COMPLETED {
		return o.ErrUnavailable
	}
	return nil
}
func NewWorker(c client.Client, service *o.Service) (worker.Worker, error) {
	if c == nil || service == nil || service.Access == nil || service.Repository == nil {
		return nil, o.ErrUnavailable
	}
	w := worker.New(c, TaskQueue, worker.Options{MaxConcurrentActivityExecutionSize: 1})
	a := &Activities{service}
	w.RegisterWorkflowWithOptions(ObservationWorkflow, workflow.RegisterOptions{Name: WorkflowName})
	w.RegisterActivityWithOptions(a.Step, activity.RegisterOptions{Name: stepActivity})
	w.RegisterActivityWithOptions(a.Fail, activity.RegisterOptions{Name: failActivity})
	return w, nil
}
