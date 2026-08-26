package temporal

import (
	"context"
	"fmt"
	"strings"

	"go.temporal.io/api/enums/v1"
	sdkclient "go.temporal.io/sdk/client"
	sdkworker "go.temporal.io/sdk/worker"
	sdkworkflow "go.temporal.io/sdk/workflow"

	"task-processor/internal/imageagent"
)

type sdkWorkflowClient interface {
	ExecuteWorkflow(context.Context, sdkclient.StartWorkflowOptions, interface{}, ...interface{}) (sdkclient.WorkflowRun, error)
	SignalWorkflow(context.Context, string, string, string, interface{}) error
}

type Client struct {
	client sdkWorkflowClient
}

func NewClient(client sdkWorkflowClient) *Client {
	return &Client{client: client}
}

func (c *Client) StartManual(ctx context.Context, start imageagent.WorkflowStart) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("image agent temporal client is not configured")
	}
	if start.Run.Mode != imageagent.RunModeManual {
		return fmt.Errorf("image agent workflow mode must be manual")
	}
	if err := validateCommandIdentity(start.Identity, start.Run.ID); err != nil {
		return err
	}
	_, err := c.client.ExecuteWorkflow(ctx, sdkclient.StartWorkflowOptions{
		ID:                       WorkflowID(start.Identity.TenantID, start.Run.ID),
		TaskQueue:                TaskQueueName(),
		WorkflowIDConflictPolicy: enums.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING,
		WorkflowIDReusePolicy:    enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
	}, workflowNameImageAgent, WorkflowInput{
		RunID: start.Run.ID, Mode: imageagent.RunModeManual, Identity: start.Identity,
		Plan: start.Plan, MaxConcurrentSlots: start.MaxConcurrentSlots, WaitForCommands: true,
	})
	return err
}

func (c *Client) RetrySlot(ctx context.Context, command imageagent.RetrySlotCommand) error {
	if err := c.validateSignal(command.Identity, command.RunID, command.PlanRevision, command.ActorID, command.ActionID); err != nil {
		return err
	}
	if strings.TrimSpace(command.SlotID) == "" {
		return fmt.Errorf("image agent retry slot ID is required")
	}
	return c.client.SignalWorkflow(ctx, WorkflowID(command.Identity.TenantID, command.RunID), "", signalRetrySlot, RetrySlotSignal{
		RunID: command.RunID, PlanRevision: command.PlanRevision, SlotID: command.SlotID,
		ActorID: command.ActorID, ActionID: command.ActionID,
	})
}

func (c *Client) ApproveResults(ctx context.Context, command imageagent.ApproveResultsCommand) error {
	if err := c.validateSignal(command.Identity, command.RunID, command.PlanRevision, command.ActorID, command.ActionID); err != nil {
		return err
	}
	return c.client.SignalWorkflow(ctx, WorkflowID(command.Identity.TenantID, command.RunID), "", signalApproveResults, ApproveResultsSignal{
		RunID: command.RunID, PlanRevision: command.PlanRevision,
		ActorID: command.ActorID, ActionID: command.ActionID,
	})
}

func (c *Client) Cancel(ctx context.Context, command imageagent.CancelRunCommand) error {
	if err := c.validateSignal(command.Identity, command.RunID, command.PlanRevision, command.ActorID, command.ActionID); err != nil {
		return err
	}
	return c.client.SignalWorkflow(ctx, WorkflowID(command.Identity.TenantID, command.RunID), "", signalCancel, CancelSignal{
		RunID: command.RunID, PlanRevision: command.PlanRevision,
		ActorID: command.ActorID, ActionID: command.ActionID,
	})
}

func (c *Client) validateSignal(identity imageagent.ExecutionIdentity, runID string, revision int64, actorID, actionID string) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("image agent temporal client is not configured")
	}
	if err := validateCommandIdentity(identity, runID); err != nil {
		return err
	}
	if revision <= 0 || strings.TrimSpace(actionID) == "" {
		return fmt.Errorf("image agent signal requires positive plan revision and action ID")
	}
	if strings.TrimSpace(actorID) != strings.TrimSpace(identity.UserID) {
		return fmt.Errorf("image agent signal actor must match verified user identity")
	}
	return nil
}

func validateCommandIdentity(identity imageagent.ExecutionIdentity, runID string) error {
	if strings.TrimSpace(identity.TenantID) == "" || strings.TrimSpace(identity.UserID) == "" || strings.TrimSpace(runID) == "" {
		return fmt.Errorf("image agent run and verified execution identity are required")
	}
	return nil
}

type WorkerConfig struct {
	Client     sdkclient.Client
	Activities *Activities
}

func NewWorker(config WorkerConfig) (sdkworker.Worker, error) {
	if config.Client == nil {
		return nil, fmt.Errorf("temporal client is required")
	}
	if config.Activities == nil {
		return nil, fmt.Errorf("image agent activities are required")
	}
	worker := sdkworker.New(config.Client, TaskQueueName(), sdkworker.Options{})
	if err := RegisterWorker(worker, config.Activities); err != nil {
		return nil, err
	}
	return worker, nil
}

type workerRegistrar interface {
	activityRegistrar
	RegisterWorkflowWithOptions(interface{}, sdkworkflow.RegisterOptions)
}

func RegisterWorker(registrar workerRegistrar, activities *Activities) error {
	if registrar == nil {
		return fmt.Errorf("temporal worker registrar is required")
	}
	if activities == nil {
		return fmt.Errorf("image agent activities are required")
	}
	registrar.RegisterWorkflowWithOptions(ImageAgentWorkflow, sdkworkflow.RegisterOptions{Name: workflowNameImageAgent})
	registrar.RegisterWorkflowWithOptions(ImageSlotWorkflow, sdkworkflow.RegisterOptions{Name: workflowNameImageSlot})
	return RegisterActivities(registrar, activities)
}
