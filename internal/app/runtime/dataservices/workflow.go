package dataservicesruntime

import (
	"context"
	"errors"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/dataacquisition"
	"time"
)

const (
	DataTaskQueue       = "data-services-v1"
	DataWorkflowName    = "DataAcquisitionV1"
	RunActivityName     = "DataAcquisitionRunV1"
	CleanupActivityName = "DataAcquisitionCleanupV1"
)

type Execution struct {
	Scope            collection.Scope
	JobID, InputHash string
	Deadline         time.Time
}

func execution(job dataacquisition.Job) Execution {
	return Execution{job.Scope, job.ID, job.InputHash, job.Deadline}
}
func (e Execution) valid() bool {
	return e.Scope.Validate() == nil && collection.ValidID(e.JobID) && len(e.InputHash) == 64 && !e.Deadline.IsZero()
}
func (e Execution) same(other Execution) bool {
	return e.Scope == other.Scope && e.JobID == other.JobID && e.InputHash == other.InputHash && e.Deadline.Equal(other.Deadline)
}
func verifyMemo(described *workflowservice.DescribeWorkflowExecutionResponse, input Execution) error {
	if described == nil || described.WorkflowExecutionInfo == nil || described.WorkflowExecutionInfo.Memo == nil {
		return dataacquisition.ErrConflict
	}
	info := described.WorkflowExecutionInfo
	if info.Execution == nil || info.Execution.WorkflowId != "data-v1-"+collection.StableID(input.Scope.OrganizationID, input.Scope.ActorID, input.JobID) || info.Type == nil || info.Type.Name != DataWorkflowName {
		return dataacquisition.ErrConflict
	}
	var original Execution
	if converter.GetDefaultDataConverter().FromPayload(described.WorkflowExecutionInfo.Memo.Fields["dataExecution"], &original) != nil || !original.same(input) {
		return dataacquisition.ErrConflict
	}
	return nil
}

type WorkflowClient interface {
	ExecuteWorkflow(context.Context, client.StartWorkflowOptions, interface{}, ...interface{}) (client.WorkflowRun, error)
	DescribeWorkflowExecution(context.Context, string, string) (*workflowservice.DescribeWorkflowExecutionResponse, error)
}
type TemporalStarter struct{ Client WorkflowClient }

func (s TemporalStarter) EnsureExecution(ctx context.Context, job dataacquisition.Job) error {
	input := execution(job)
	if s.Client == nil || !input.valid() {
		return dataacquisition.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	id := "data-v1-" + collection.StableID(job.Scope.OrganizationID, job.Scope.ActorID, job.ID)
	described, err := s.Client.DescribeWorkflowExecution(ctx, id, "")
	var missing *serviceerror.NotFound
	if err != nil && !errors.As(err, &missing) {
		return dataacquisition.ErrUnavailable
	}
	if err == nil {
		if err := verifyMemo(described, input); err != nil {
			return err
		}
		state := described.WorkflowExecutionInfo.Status
		if state == enums.WORKFLOW_EXECUTION_STATUS_RUNNING || state == enums.WORKFLOW_EXECUTION_STATUS_COMPLETED {
			return nil
		}
	}
	_, err = s.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: id, TaskQueue: DataTaskQueue, WorkflowExecutionTimeout: 40 * time.Minute, WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY, Memo: map[string]interface{}{"dataExecution": input}}, DataWorkflowName, input)
	var exists *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &exists) {
		// A concurrent caller started the original execution. Verify its immutable
		// memo instead of treating arbitrary AlreadyStarted as sufficient evidence.
		described, err := s.Client.DescribeWorkflowExecution(ctx, id, "")
		if err != nil {
			return dataacquisition.ErrUnknown
		}
		return verifyMemo(described, input)
	}
	if err != nil {
		return dataacquisition.ErrUnavailable
	}
	return nil
}
func DataWorkflow(ctx workflow.Context, input Execution) error {
	if !input.valid() {
		return temporal.NewNonRetryableApplicationError("invalid original data execution", "invalid", nil)
	}
	for workflow.Now(ctx).Before(input.Deadline) {
		remaining := input.Deadline.Sub(workflow.Now(ctx))
		window := 6 * time.Minute
		if remaining < window {
			window = remaining
		}
		attempt := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: window, ScheduleToCloseTimeout: window, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}})
		if err := workflow.ExecuteActivity(attempt, RunActivityName, input).Get(ctx, nil); err == nil {
			return nil
		}
		remaining = input.Deadline.Sub(workflow.Now(ctx))
		if remaining <= 0 {
			break
		}
		if remaining > 20*time.Second {
			remaining = 20 * time.Second
		}
		if err := workflow.Sleep(ctx, remaining); err != nil {
			return err
		}
	}
	// Keep original terminal fencing retryable for the remaining execution
	// lifetime (bounded by EnsureExecution's 40 minutes). This phase cannot
	// extend the persisted provider deadline or start a new reservation.
	cleanup := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 5 * time.Minute, RetryPolicy: &temporal.RetryPolicy{InitialInterval: 10 * time.Second, MaximumInterval: time.Minute}})
	return workflow.ExecuteActivity(cleanup, CleanupActivityName, input).Get(ctx, nil)
}

type JobRunner interface {
	Run(context.Context, collection.Scope, string) error
}
type activities struct{ runner JobRunner }

func (a activities) Run(ctx context.Context, input Execution) error {
	if !input.valid() {
		return dataacquisition.ErrInvalid
	}
	return a.runner.Run(ctx, input.Scope, input.JobID)
}

// RegisterWorker is consumed by the public runtime owner. It starts no worker
// or service by itself and never installs an alternate source of business facts.
func RegisterWorker(w worker.Worker, runner JobRunner) error {
	if w == nil || runner == nil {
		return dataacquisition.ErrUnavailable
	}
	w.RegisterWorkflowWithOptions(DataWorkflow, workflow.RegisterOptions{Name: DataWorkflowName})
	a := activities{runner}
	w.RegisterActivityWithOptions(a.Run, activity.RegisterOptions{Name: RunActivityName})
	w.RegisterActivityWithOptions(a.Run, activity.RegisterOptions{Name: CleanupActivityName})
	return nil
}
