package temporal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

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
	if strings.TrimSpace(scope.OwnerUserID) != strings.TrimSpace(identity.UserID) {
		return imageagent.WorkflowProjection{}, imageagent.ErrRunNotFound
	}
	encoded, err := c.client.QueryWorkflow(ctx, WorkflowID(identity.TenantID, identity.UserID, scope.RunID), "", QueryWorkflowProjection)
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
	if err := imageagent.ValidateSubmittedPlan(command.Plan); err != nil {
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
	if err := imageagent.ValidateMaxConcurrentSlots(start.Run.MaxConcurrentSlots); err != nil {
		return err
	}
	if err := imageagent.ValidateSubmittedPlan(start.Plan); err != nil {
		return fmt.Errorf("validate image agent workflow plan: %w", err)
	}
	if err := validateCommandIdentity(start.Identity, start.Run.ID); err != nil {
		return err
	}
	policy, err := start.Run.Budget.Policy()
	if err != nil {
		return fmt.Errorf("validate image agent workflow budget: %w", err)
	}
	startedAt := start.Run.StartedAt.UTC()
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	var deadlineAt time.Time
	if policy.MaxElapsed.Enabled {
		deadlineAt = startedAt.Add(time.Duration(policy.MaxElapsed.Value))
	}
	_, err = c.client.ExecuteWorkflow(ctx, sdkclient.StartWorkflowOptions{
		ID:                       WorkflowID(start.Identity.TenantID, start.Identity.UserID, start.Run.ID),
		TaskQueue:                TaskQueueV3,
		WorkflowExecutionTimeout: V3WorkflowExecutionTimeout,
		WorkflowIDConflictPolicy: enums.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING,
		WorkflowIDReusePolicy:    enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
	}, workflowNameImageAgent, WorkflowInput{
		RunID: start.Run.ID, Mode: imageagent.RunModeManual, Identity: start.Identity,
		Plan: start.Plan, MaxConcurrentSlots: imageagent.NormalizeMaxConcurrentSlots(start.Run.MaxConcurrentSlots), WaitForCommands: true,
		AssetCatalog: start.AssetCatalog,
		BudgetPolicy: policy, StartedAt: startedAt, DeadlineAt: deadlineAt,
	})
	return err
}

func (c *Client) RecoverEffect(ctx context.Context, command imageagent.RecoverEffectCommand, projection imageagent.RunProjection) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("image agent temporal client is not configured")
	}
	if err := validateCommandIdentity(command.Identity, command.RunID); err != nil {
		return err
	}
	if command.PlanRevision <= 0 || command.PlanRevision > imageagent.MaxJSONSafePlanRevision || command.Attempt <= 0 || imageagent.ValidateActionID(command.ActionID) != nil {
		return fmt.Errorf("image agent recovery requires a valid revision, attempt, and action ID")
	}
	if strings.TrimSpace(projection.Run.ID) != strings.TrimSpace(command.RunID) {
		return imageagent.ErrRunNotFound
	}
	if strings.TrimSpace(projection.Run.TenantID) != strings.TrimSpace(command.Identity.TenantID) || strings.TrimSpace(projection.Run.UserID) != strings.TrimSpace(command.Identity.UserID) {
		return imageagent.ErrRunNotFound
	}
	if projection.Plan.Revision != command.PlanRevision {
		return imageagent.ErrRevisionConflict
	}
	var slot imageagent.Slot
	found := false
	for _, candidate := range projection.Plan.Slots {
		if candidate.ID == strings.TrimSpace(command.SlotID) {
			slot = candidate
			found = true
			break
		}
	}
	if !found {
		return imageagent.ErrCommandBlocked
	}
	return newRecoveryWorkflowStarter(c.client, TaskQueueV3)(ctx, EffectRecoveryWorkflowInput{
		RunID: command.RunID, Identity: command.Identity, PlanRevision: command.PlanRevision,
		Slot: slot, Attempt: command.Attempt, AssetCatalog: projection.AssetCatalog,
	})
}

func (c *Client) RetrySlot(ctx context.Context, command imageagent.RetrySlotCommand) error {
	if err := c.validateSignal(command.Identity, command.RunID, command.PlanRevision, command.ActorID, command.ActionID); err != nil {
		return err
	}
	if strings.TrimSpace(command.SlotID) == "" {
		return fmt.Errorf("image agent retry slot ID is required")
	}
	return c.executeAcceptedCommandUpdate(ctx, command.Identity, command.RunID, signalRetrySlot, RetrySlotSignal{
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
	return c.executeAcceptedCommandUpdate(ctx, command.Identity, command.RunID, signalCancel, CancelSignal{
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
	if strings.TrimSpace(command.ActorID) != strings.TrimSpace(command.Identity.UserID) || imageagent.ValidateActionID(command.ActionID) != nil {
		return imageagent.CommandAcknowledgement{}, fmt.Errorf("image agent resume actor and action must match verified identity")
	}
	input := ResumeCommandInput{RunID: command.RunID, ActorID: command.ActorID, ActionID: command.ActionID}
	transportID, err := newTransportUpdateID("resume")
	if err != nil {
		return imageagent.CommandAcknowledgement{}, err
	}
	handle, err := c.client.UpdateWorkflow(ctx, sdkclient.UpdateWorkflowOptions{
		UpdateID:   transportID,
		WorkflowID: WorkflowID(command.Identity.TenantID, command.Identity.UserID, command.RunID),
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

func (c *Client) executeAcceptedCommandUpdate(ctx context.Context, identity imageagent.ExecutionIdentity, runID, updateName string, arg interface{}) error {
	transportID, err := newTransportUpdateID(updateName)
	if err != nil {
		return err
	}
	_, err = c.client.UpdateWorkflow(ctx, sdkclient.UpdateWorkflowOptions{
		UpdateID:   transportID,
		WorkflowID: WorkflowID(identity.TenantID, identity.UserID, runID),
		UpdateName: updateName, Args: []interface{}{arg},
		WaitForStage: sdkclient.WorkflowUpdateStageAccepted,
	})
	return mapCommandUpdateError(err)
}

func (c *Client) executeCommandUpdate(ctx context.Context, identity imageagent.ExecutionIdentity, runID, updateName, actionID string, arg interface{}) error {
	transportID, err := newTransportUpdateID(updateName)
	if err != nil {
		return err
	}
	handle, err := c.client.UpdateWorkflow(ctx, sdkclient.UpdateWorkflowOptions{
		UpdateID:   transportID,
		WorkflowID: WorkflowID(identity.TenantID, identity.UserID, runID),
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

func newTransportUpdateID(prefix string) (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate temporal update transport ID: %w", err)
	}
	return strings.TrimSpace(prefix) + ":" + hex.EncodeToString(random[:]), nil
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
	if revision <= 0 || imageagent.ValidateActionID(actionID) != nil {
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
	WireMode   WorkerWireMode
	TaskQueue  string
}

func NewWorker(config WorkerConfig) (sdkworker.Worker, error) {
	if config.Client == nil {
		return nil, fmt.Errorf("temporal client is required")
	}
	if config.Activities == nil {
		return nil, fmt.Errorf("image agent activities are required")
	}
	queue, err := config.selectedTaskQueue()
	if err != nil {
		return nil, err
	}
	if config.Activities.recoveryWorkflowStarter == nil {
		config.Activities.recoveryWorkflowStarter = newRecoveryWorkflowStarter(config.Client, queue)
	}
	worker := sdkworker.New(config.Client, queue, sdkworker.Options{})
	if err := RegisterWorkerForMode(worker, config.Activities, config.WireMode); err != nil {
		return nil, err
	}
	return worker, nil
}

func (config WorkerConfig) selectedTaskQueue() (string, error) {
	if err := validateWorkerWireMode(config.WireMode); err != nil {
		return "", err
	}
	if queue := strings.TrimSpace(config.TaskQueue); queue != "" {
		if config.WireMode == WorkerWireModeV2 && queue == TaskQueueV3 {
			return "", fmt.Errorf("image agent v2 wire mode cannot use v3 task queue %q", queue)
		}
		if config.WireMode == WorkerWireModeV3 && queue == TaskQueue {
			return "", fmt.Errorf("image agent v3 wire mode cannot use v2 task queue %q", queue)
		}
		return queue, nil
	}
	return config.WireMode.DefaultTaskQueue()
}

func validateWorkerWireMode(mode WorkerWireMode) error {
	switch mode {
	case WorkerWireModeV2, WorkerWireModeV3:
		return nil
	case "":
		return fmt.Errorf("image agent temporal wire mode is required")
	default:
		return fmt.Errorf("unsupported image agent temporal wire mode %q", mode)
	}
}

type workerRegistrar interface {
	activityRegistrar
	RegisterWorkflowWithOptions(interface{}, sdkworkflow.RegisterOptions)
}

func RegisterWorker(registrar workerRegistrar, activities *Activities) error {
	return RegisterWorkerForMode(registrar, activities, WorkerWireModeV2)
}

func RegisterWorkerForMode(registrar workerRegistrar, activities *Activities, mode WorkerWireMode) error {
	if registrar == nil {
		return fmt.Errorf("temporal worker registrar is required")
	}
	if activities == nil {
		return fmt.Errorf("image agent activities are required")
	}
	if err := validateWorkerWireMode(mode); err != nil {
		return err
	}
	registrar.RegisterWorkflowWithOptions(ImageAgentWorkflow, sdkworkflow.RegisterOptions{Name: workflowNameImageAgent})
	if mode == WorkerWireModeV2 {
		registrar.RegisterWorkflowWithOptions(ImageSlotWorkflow, sdkworkflow.RegisterOptions{Name: workflowNameImageSlot})
	} else {
		registrar.RegisterWorkflowWithOptions(ImageSlotWorkflowV3, sdkworkflow.RegisterOptions{Name: "ImageSlotWorkflowV3"})
		registrar.RegisterWorkflowWithOptions(ImageAgentEffectRecoveryWorkflow, sdkworkflow.RegisterOptions{Name: EffectRecoveryWorkflowName})
		registrar.RegisterWorkflowWithOptions(ImageAgentCompatibilityCanaryWorkflow, sdkworkflow.RegisterOptions{Name: workflowNameCompatibilityCanary})
	}
	return RegisterActivitiesForMode(registrar, activities, mode)
}

func newRecoveryWorkflowStarter(client sdkWorkflowClient, taskQueue string) RecoveryWorkflowStarter {
	return func(ctx context.Context, input EffectRecoveryWorkflowInput) error {
		if client == nil {
			return fmt.Errorf("image agent recovery workflow temporal client is not configured")
		}
		taskQueue = strings.TrimSpace(taskQueue)
		if taskQueue == "" {
			return fmt.Errorf("image agent recovery workflow task queue is required")
		}
		_, err := client.ExecuteWorkflow(ctx, sdkclient.StartWorkflowOptions{
			ID:                       EffectRecoveryWorkflowID(input.Identity, input.PlanRevision, input.RunID+":"+input.Slot.ID, input.Attempt),
			TaskQueue:                taskQueue,
			WorkflowIDConflictPolicy: enums.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING,
			WorkflowIDReusePolicy:    enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
		}, EffectRecoveryWorkflowName, input)
		return err
	}
}
