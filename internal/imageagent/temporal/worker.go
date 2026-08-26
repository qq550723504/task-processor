package temporal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	sdkclient "go.temporal.io/sdk/client"
	sdkconverter "go.temporal.io/sdk/converter"
	sdktemporal "go.temporal.io/sdk/temporal"
	sdkworker "go.temporal.io/sdk/worker"
	sdkworkflow "go.temporal.io/sdk/workflow"

	"task-processor/internal/imageagent"
)

type sdkWorkflowClient interface {
	ExecuteWorkflow(context.Context, sdkclient.StartWorkflowOptions, interface{}, ...interface{}) (sdkclient.WorkflowRun, error)
	QueryWorkflow(context.Context, string, string, string, ...interface{}) (sdkconverter.EncodedValue, error)
	SignalWorkflow(context.Context, string, string, string, interface{}) error
	UpdateWorkflow(context.Context, sdkclient.UpdateWorkflowOptions) (sdkclient.WorkflowUpdateHandle, error)
}

func (c *Client) GetProjection(ctx context.Context, scope imageagent.RunScope, identity imageagent.ExecutionIdentity) (imageagent.WorkflowProjection, error) {
	if c == nil || c.client == nil {
		return imageagent.WorkflowProjection{}, fmt.Errorf("image agent temporal client is not configured")
	}
	if err := validateCommandIdentity(identity, scope.RunID); err != nil {
		return imageagent.WorkflowProjection{}, err
	}
	if strings.TrimSpace(scope.TenantID) != strings.TrimSpace(identity.TenantID) {
		return imageagent.WorkflowProjection{}, imageagent.ErrRunNotFound
	}
	encoded, err := c.client.QueryWorkflow(ctx, WorkflowID(identity.TenantID, scope.RunID), "", QueryWorkflowProjection)
	if err != nil {
		return imageagent.WorkflowProjection{}, err
	}
	var projection imageagent.WorkflowProjection
	if err := encoded.Get(&projection); err != nil {
		return imageagent.WorkflowProjection{}, fmt.Errorf("decode image agent workflow projection: %w", err)
	}
	return projection, nil
}

func (c *Client) ReplacePlan(ctx context.Context, command imageagent.ReplacePlanCommand) error {
	if err := c.validateSignal(command.Identity, command.RunID, command.ExpectedRevision, command.ActorID, command.ActionID); err != nil {
		return err
	}
	if command.Plan.ParentRevision != command.ExpectedRevision || command.Plan.Revision <= command.ExpectedRevision {
		return fmt.Errorf("image agent replacement plan must advance expected revision")
	}
	if err := imageagent.ValidatePlan(command.Plan); err != nil {
		return fmt.Errorf("validate image agent replacement plan: %w", err)
	}
	return c.executeCommandUpdate(ctx, command.Identity, command.RunID, signalReplacePlan, command.ActionID, ReplacePlanSignal{
		RunID: command.RunID, ExpectedRevision: command.ExpectedRevision, Plan: command.Plan,
		ActorID: command.ActorID, ActionID: command.ActionID,
	})
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
		AssetCatalog: start.AssetCatalog,
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
	return c.executeCommandUpdate(ctx, command.Identity, command.RunID, signalRetrySlot, command.ActionID, RetrySlotSignal{
		RunID: command.RunID, PlanRevision: command.PlanRevision, SlotID: command.SlotID,
		ActorID: command.ActorID, ActionID: command.ActionID,
	})
}

func (c *Client) ApproveResults(ctx context.Context, command imageagent.ApproveResultsCommand) error {
	if err := c.validateSignal(command.Identity, command.RunID, command.PlanRevision, command.ActorID, command.ActionID); err != nil {
		return err
	}
	if command.ResultDigest == "" || command.ResultDigest != strings.TrimSpace(command.ResultDigest) {
		return fmt.Errorf("image agent approval result digest is required")
	}
	return c.executeCommandUpdate(ctx, command.Identity, command.RunID, signalApproveResults, command.ActionID, ApproveResultsSignal{
		RunID: command.RunID, PlanRevision: command.PlanRevision, ResultDigest: command.ResultDigest,
		ActorID: command.ActorID, ActionID: command.ActionID,
	})
}

func (c *Client) Cancel(ctx context.Context, command imageagent.CancelRunCommand) error {
	if err := c.validateSignal(command.Identity, command.RunID, command.PlanRevision, command.ActorID, command.ActionID); err != nil {
		return err
	}
	return c.executeCommandUpdate(ctx, command.Identity, command.RunID, signalCancel, command.ActionID, CancelSignal{
		RunID: command.RunID, PlanRevision: command.PlanRevision,
		ActorID: command.ActorID, ActionID: command.ActionID,
	})
}

func (c *Client) Resume(ctx context.Context, command imageagent.ResumeCommand) (imageagent.CommandAcknowledgement, error) {
	if c == nil || c.client == nil {
		return imageagent.CommandAcknowledgement{}, fmt.Errorf("image agent temporal client is not configured")
	}
	if err := validateCommandIdentity(command.Identity, command.RunID); err != nil {
		return imageagent.CommandAcknowledgement{}, err
	}
	if strings.TrimSpace(command.ActorID) != strings.TrimSpace(command.Identity.UserID) || strings.TrimSpace(command.ActionID) == "" {
		return imageagent.CommandAcknowledgement{}, fmt.Errorf("image agent resume actor and action must match verified identity")
	}
	input := ResumeCommandInput{RunID: command.RunID, ActorID: command.ActorID, ActionID: command.ActionID}
	handle, err := c.client.UpdateWorkflow(ctx, sdkclient.UpdateWorkflowOptions{
		UpdateID:   "resume:" + command.ActionID,
		WorkflowID: WorkflowID(command.Identity.TenantID, command.RunID),
		UpdateName: updateResumeCommand, Args: []interface{}{input},
		WaitForStage: sdkclient.WorkflowUpdateStageCompleted,
	})
	if err != nil {
		return imageagent.CommandAcknowledgement{}, mapCommandUpdateError(err)
	}
	var acknowledgement imageagent.CommandAcknowledgement
	if err := handle.Get(ctx, &acknowledgement); err != nil {
		return imageagent.CommandAcknowledgement{}, mapCommandUpdateError(err)
	}
	return acknowledgement, nil
}

func (c *Client) executeCommandUpdate(ctx context.Context, identity imageagent.ExecutionIdentity, runID, updateName, actionID string, arg interface{}) error {
	encoded, err := json.Marshal(arg)
	if err != nil {
		return fmt.Errorf("encode image agent workflow update: %w", err)
	}
	sum := sha256.Sum256(append([]byte(updateName+":"), encoded...))
	handle, err := c.client.UpdateWorkflow(ctx, sdkclient.UpdateWorkflowOptions{
		UpdateID:   actionID + ":" + hex.EncodeToString(sum[:]),
		WorkflowID: WorkflowID(identity.TenantID, runID),
		UpdateName: updateName, Args: []interface{}{arg},
		WaitForStage: sdkclient.WorkflowUpdateStageCompleted,
	})
	if err != nil {
		return mapCommandUpdateError(err)
	}
	var acknowledgement CommandAcknowledgement
	if err := handle.Get(ctx, &acknowledgement); err != nil {
		return mapCommandUpdateError(err)
	}
	return nil
}

func mapCommandUpdateError(err error) error {
	if err == nil {
		return nil
	}
	var notFound *serviceerror.NotFound
	if errors.As(err, &notFound) {
		return fmt.Errorf("%w: %v", imageagent.ErrRunNotFound, err)
	}
	var failedPrecondition *serviceerror.FailedPrecondition
	if errors.As(err, &failedPrecondition) {
		return fmt.Errorf("%w: %v", imageagent.ErrCommandBlocked, err)
	}
	var applicationError *sdktemporal.ApplicationError
	if errors.As(err, &applicationError) {
		switch applicationError.Type() {
		case updateErrorRevisionConflict:
			return fmt.Errorf("%w: %v", imageagent.ErrRevisionConflict, err)
		case updateErrorCommandBlocked:
			return fmt.Errorf("%w: %v", imageagent.ErrCommandBlocked, err)
		case updateErrorRunNotFound:
			return fmt.Errorf("%w: %v", imageagent.ErrRunNotFound, err)
		}
	}
	return err
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
