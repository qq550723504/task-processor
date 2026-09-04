package temporal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.temporal.io/api/enums/v1"
	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"task-processor/internal/imageagent"
)

const (
	activityWireV2Patch               = "image-agent-atomic-command-boundaries-v2"
	slotExecutionWireV3Patch          = "image-agent-slot-execution-wire-v3"
	approvalActionIDV3Patch           = "image-agent-approval-action-id-v3"
	approvalPublicationWireV3Patch    = "image-agent-approval-publication-wire-v3"
	resultDigestV3Patch               = "image-agent-result-digest-v3"
	budgetAuthorizationPatch          = "image-agent-budget-authorization-v1"
	workflowFailureProjectionPatch    = "image-agent-workflow-failure-projection-v1"
	projectionExecutionCommitPatch    = "image-agent-projection-execution-commit-v1"
	commandIngressPlanPolicyPatch     = "image-agent-command-ingress-plan-policy-v1"
	approvalPublicationScopePatch     = "image-agent-approval-publication-scope-v1"
	approvalPublicationKeyLengthPatch = "image-agent-approval-publication-key-length-v1"
	externalEffectFinalizationPatch   = "image-agent-external-effect-finalization-v1"
	effectRecoveryStartWireV1Patch    = "image-agent-effect-recovery-start-wire-v1"
	recoveryRequestedBlockCode        = "recovery_requested"
	recoveryStartFailedBlockCode      = "recovery_start_failed"
)

type workflowActivityWire struct {
	executeSlot, reviewStagedSlot, persistSlotResult, persistRunState, persistPlanRevision, persistPendingCommand, publishApproved, startEffectRecovery string
	useV3Slot, useV3Approval                                                                                                                            bool
	useRunScopedApprovalKey                                                                                                                             bool
	useBoundedApprovalKey                                                                                                                               bool
}

func activityWireForWorkflow(ctx workflow.Context) workflowActivityWire {
	useV3Slot := workflow.GetVersion(ctx, slotExecutionWireV3Patch, workflow.DefaultVersion, 1) != workflow.DefaultVersion
	useV3ApprovalActionID := workflow.GetVersion(ctx, approvalActionIDV3Patch, workflow.DefaultVersion, 1) != workflow.DefaultVersion
	useV3ApprovalPublication := workflow.GetVersion(ctx, approvalPublicationWireV3Patch, workflow.DefaultVersion, 1) != workflow.DefaultVersion
	useV3ResultDigest := workflow.GetVersion(ctx, resultDigestV3Patch, workflow.DefaultVersion, 1) != workflow.DefaultVersion
	useRunScopedApprovalKey := workflow.GetVersion(ctx, approvalPublicationScopePatch, workflow.DefaultVersion, 1) != workflow.DefaultVersion
	useBoundedApprovalKey := false
	if useV3ApprovalActionID && useV3ApprovalPublication && useV3ResultDigest && useRunScopedApprovalKey {
		useBoundedApprovalKey = workflow.GetVersion(ctx, approvalPublicationKeyLengthPatch, workflow.DefaultVersion, 1) != workflow.DefaultVersion
	}
	version := workflow.GetVersion(ctx, activityWireV2Patch, workflow.DefaultVersion, 1)
	if version == workflow.DefaultVersion {
		return workflowActivityWire{
			executeSlot: activityExecuteSlotLegacy, persistSlotResult: activityPersistSlotResultLegacy,
			persistRunState: activityPersistRunStateLegacy, persistPlanRevision: activityPersistPlanRevisionLegacy,
			persistPendingCommand: activityPersistPendingCommandLegacy, publishApproved: activityPublishApprovedLegacy,
		}
	}
	wire := workflowActivityWire{
		executeSlot: activityExecuteSlot, persistSlotResult: activityPersistSlotResult,
		reviewStagedSlot: activityReviewStagedSlotV3,
		persistRunState:  activityPersistRunState, persistPlanRevision: activityPersistPlanRevision,
		persistPendingCommand: activityPersistPendingCommand, publishApproved: activityPublishApproved,
	}
	if useV3Slot {
		wire.executeSlot = activityExecuteSlotV3
		wire.persistSlotResult = activityPersistSlotResultV3
		wire.useV3Slot = true
	}
	if useV3ApprovalActionID && useV3ApprovalPublication && useV3ResultDigest {
		wire.publishApproved = activityPublishApprovedV3
		wire.useV3Approval = true
		wire.useRunScopedApprovalKey = useRunScopedApprovalKey
		wire.useBoundedApprovalKey = useBoundedApprovalKey
	}
	if useV3Slot && workflow.GetVersion(ctx, effectRecoveryStartWireV1Patch, workflow.DefaultVersion, 1) != workflow.DefaultVersion {
		wire.startEffectRecovery = activityStartEffectRecoveryV3
	}
	return wire
}

func ImageAgentWorkflow(ctx workflow.Context, input WorkflowInput) (WorkflowResult, error) {
	if input.Mode != imageagent.RunModeManual {
		return WorkflowResult{}, fmt.Errorf("image agent workflow mode must be manual")
	}
	if input.RunID == "" || input.Identity.TenantID == "" || input.Identity.UserID == "" {
		return WorkflowResult{}, fmt.Errorf("run ID and verified execution identity are required")
	}
	if input.TargetPlatform != "" || input.ImagePolicyContext != nil {
		if input.ImagePolicyContext == nil || imageagent.ValidateImagePolicyContext(input.TargetPlatform, *input.ImagePolicyContext) != nil {
			return WorkflowResult{}, fmt.Errorf("validate image policy context: %w", imageagent.ErrValidation)
		}
	}
	input.enforceIngressPlanPolicy = workflow.GetVersion(ctx, commandIngressPlanPolicyPatch, workflow.DefaultVersion, 1) != workflow.DefaultVersion
	validatePlan := imageagent.ValidatePlan
	if input.enforceIngressPlanPolicy {
		validatePlan = imageagent.ValidateInitialSubmittedPlan
	}
	if err := validatePlan(input.Plan); err != nil {
		return WorkflowResult{}, fmt.Errorf("validate plan: %w", err)
	}
	catalog, err := imageagent.NormalizeAssetCatalog(input.AssetCatalog)
	if err != nil {
		return WorkflowResult{}, fmt.Errorf("validate immutable asset catalog: %w", err)
	}
	if err := imageagent.ValidatePlanAgainstCatalog(input.Plan, catalog); err != nil {
		return WorkflowResult{}, fmt.Errorf("validate plan against immutable asset catalog: %w", err)
	}
	input.AssetCatalog = catalog
	input.BudgetAuthorization = workflow.GetVersion(ctx, budgetAuthorizationPatch, workflow.DefaultVersion, 1) != workflow.DefaultVersion
	input.externalEffectFinalization = workflow.GetVersion(ctx, externalEffectFinalizationPatch, workflow.DefaultVersion, 1) != workflow.DefaultVersion
	if workflow.GetVersion(ctx, projectionExecutionCommitPatch, workflow.DefaultVersion, 1) != workflow.DefaultVersion {
		input.projectionExecutionID = strings.TrimSpace(workflow.GetInfo(ctx).WorkflowExecution.RunID)
		if input.projectionExecutionID == "" {
			return WorkflowResult{}, fmt.Errorf("temporal workflow execution identity is required")
		}
	}
	if input.BudgetAuthorization {
		if err := input.BudgetPolicy.Allows(imageagent.UsageVector{}, imageagent.UsageVector{}, imageagent.UsageVector{}); err != nil {
			return WorkflowResult{}, fmt.Errorf("validate workflow budget policy: %w", err)
		}
		if input.BudgetPolicy.MaxElapsed.Enabled && (input.StartedAt.IsZero() || input.DeadlineAt.IsZero() || !input.DeadlineAt.Equal(input.StartedAt.Add(time.Duration(input.BudgetPolicy.MaxElapsed.Value)))) {
			return WorkflowResult{}, fmt.Errorf("validate workflow budget deadline: %w", imageagent.ErrValidation)
		}
	}
	if !input.LifecycleDeadlineAt.IsZero() && (input.StartedAt.IsZero() || !input.LifecycleDeadlineAt.Equal(input.StartedAt.Add(V3WorkflowExecutionTimeout-V3LifecycleDeadlineSafetyMargin))) {
		return WorkflowResult{}, fmt.Errorf("validate workflow lifecycle deadline: %w", imageagent.ErrValidation)
	}
	result, runErr := runImageAgentWorkflow(ctx, input)
	if runErr == nil {
		return result, nil
	}
	if workflow.GetVersion(ctx, workflowFailureProjectionPatch, workflow.DefaultVersion, 1) == workflow.DefaultVersion {
		return result, runErr
	}
	if failureErr := persistWorkflowFailure(ctx, input); failureErr != nil {
		return result, fmt.Errorf("%w: persist workflow failure: %v", runErr, failureErr)
	}
	return result, runErr
}

func runImageAgentWorkflow(ctx workflow.Context, input WorkflowInput) (WorkflowResult, error) {
	ctx = imageAgentActivityContext(ctx)
	effects := newWorkflowEffectOwner(ctx)
	projection := WorkflowResult{Status: imageagent.RunStatusPlanning, Plan: input.Plan, Slots: slotProjections(input.Plan, nil), CommandIngress: imageagent.CommandIngress{Limit: maxActionLedgerEntries}}
	var updates *workflowUpdateState
	cancelAndProject := func(results []SlotWorkflowResult) (WorkflowResult, bool, error) {
		if updates == nil || !updates.cancelRequested {
			return WorkflowResult{}, false, fmt.Errorf("image agent cancellation was not committed by the command saga")
		}
		for !updates.cancelCommitted {
			if updates.cancelPending {
				updates.commitPendingCancellation(ctx, results)
				continue
			}
			if updates.cancelBlocked && updates.cancelCommitErr == nil {
				if err := workflow.Await(ctx, func() bool {
					return !updates.cancelRequested && updates.pendingActionID == ""
				}); err != nil {
					return projection, false, err
				}
				updates.cancelBlocked = false
				return projection, false, nil
			}
			updates.wake.Receive(ctx, nil)
		}
		result := cancelledProjection(input, results)
		result.CommandIngress = updates.commandIngress()
		projection = result
		if updates.cancelRequested {
			if err := workflow.Await(ctx, func() bool { return workflow.AllHandlersFinished(ctx) }); err != nil {
				return result, true, err
			}
		}
		return result, true, nil
	}
	cancelChannel := workflow.GetSignalChannel(ctx, signalCancel)
	retryChannel := workflow.GetSignalChannel(ctx, signalRetrySlot)
	replaceChannel := workflow.GetSignalChannel(ctx, signalReplacePlan)
	approveChannel := workflow.GetSignalChannel(ctx, signalApproveResults)
	recoveryCompletedChannel := workflow.GetSignalChannel(ctx, signalEffectRecoveryCompleted)
	var results []SlotWorkflowResult
	updates = newWorkflowUpdateState(ctx, &input, &projection, &results, effects)
	if err := workflow.SetQueryHandler(ctx, QueryWorkflowProjection, func() (WorkflowResult, error) {
		return updates.projectionSnapshot(), nil
	}); err != nil {
		return WorkflowResult{}, fmt.Errorf("register image agent projection query: %w", err)
	}
	if err := updates.register(ctx); err != nil {
		return WorkflowResult{}, fmt.Errorf("register image agent update handlers: %w", err)
	}
	updates.startSignalHandlers(ctx, cancelChannel, retryChannel, replaceChannel, approveChannel, recoveryCompletedChannel)
	awaitTerminalIntent := func(results []SlotWorkflowResult) (WorkflowResult, error) {
		if err := workflow.Await(ctx, func() bool {
			return updates.cancelCommitted || isTerminalRunStatus(projection.Status)
		}); err != nil {
			return WorkflowResult{}, err
		}
		if updates.cancelCommitted {
			result, _, err := cancelAndProject(results)
			return result, err
		}
		if err := workflow.Await(ctx, func() bool { return workflow.AllHandlersFinished(ctx) }); err != nil {
			return WorkflowResult{}, err
		}
		return projection, nil
	}
	persistLifecycleDeadlineBlock := func(result WorkflowResult) (WorkflowResult, error) {
		result.Status = imageagent.RunStatusBlocked
		result.Block = &imageagent.Block{Code: imageagent.WorkflowLifecycleElapsedCode, Message: imageagent.WorkflowLifecycleElapsedCode}
		result.CommandIngress = updates.commandIngress()
		if err := effects.persistRunState(ctx, input, result, "lifecycle_deadline"); err != nil {
			if errors.Is(err, errWorkflowEffectFenced) {
				return awaitTerminalIntent(nil)
			}
			return WorkflowResult{}, err
		}
		projection = result
		return result, nil
	}
	lifecycleTimer := func() workflow.Future {
		if input.LifecycleDeadlineAt.IsZero() || !workflow.Now(ctx).Before(input.LifecycleDeadlineAt) {
			return nil
		}
		return workflow.NewTimer(ctx, input.LifecycleDeadlineAt.Sub(workflow.Now(ctx)))
	}
	lifecycleElapsed := func() bool {
		return !input.LifecycleDeadlineAt.IsZero() && !workflow.Now(ctx).Before(input.LifecycleDeadlineAt)
	}

runPlan:
	for {
		if lifecycleElapsed() {
			return persistLifecycleDeadlineBlock(projection)
		}
		executing := WorkflowResult{Status: imageagent.RunStatusExecuting, Plan: input.Plan, Slots: slotProjections(input.Plan, nil)}
		executing.CommandIngress = updates.commandIngress()
		if !updates.consumeExecutingHandoff(input.Plan.Revision) {
			if err := effects.persistRunState(ctx, input, executing, "execute_slots"); err != nil {
				if errors.Is(err, errWorkflowEffectFenced) {
					return awaitTerminalIntent(nil)
				}
				return WorkflowResult{}, err
			}
		}
		projection = executing
		if updates.pendingActionID != "" {
			for updates.pendingActionID != "" && !updates.cancelRequested {
				wait := workflow.NewSelector(ctx)
				wait.AddReceive(updates.wake, func(channel workflow.ReceiveChannel, _ bool) { channel.Receive(ctx, nil) })
				if timer := lifecycleTimer(); timer != nil {
					wait.AddFuture(timer, func(workflow.Future) {})
				}
				wait.Select(ctx)
				if lifecycleElapsed() {
					return persistLifecycleDeadlineBlock(projection)
				}
			}
		}
		if updates.cancelRequested {
			result, _, err := cancelAndProject(nil)
			return result, err
		}

		limit := input.MaxConcurrentSlots
		if limit <= 0 {
			limit = defaultMaxConcurrentSlots
		}
		var cancelled bool
		var err error
		results, cancelled, err = executeInitialSlots(ctx, input, limit, updates, func(results []SlotWorkflowResult) {
			projection = executingProjection(input.Plan, results)
			projection.CommandIngress = updates.commandIngress()
		})
		if err != nil {
			if errors.Is(err, errWorkflowEffectFenced) {
				return awaitTerminalIntent(results)
			}
			return WorkflowResult{}, err
		}
		var result WorkflowResult
		cancellationBlocked := false
		if cancelled {
			cancelResult, terminal, cancelErr := cancelAndProject(results)
			if cancelErr != nil || terminal {
				return cancelResult, cancelErr
			}
			result = cancelResult
			cancellationBlocked = true
		} else {
			result = summarizeResultsForWire(input.Plan, results, effects.activities)
			result.CommandIngress = updates.commandIngress()
		}
		if result.Block != nil {
			if !cancellationBlocked {
				if err := effects.persistRunState(ctx, input, result, "retry_slot"); err != nil {
					if errors.Is(err, errWorkflowEffectFenced) {
						return awaitTerminalIntent(results)
					}
					return WorkflowResult{}, err
				}
			}
			projection = result
			legacyRetryPending := updates.dispatchDeferredRetries(ctx)
			for result.Block != nil {
				if !input.WaitForCommands && !legacyRetryPending && !cancellationBlocked {
					return result, nil
				}
				wait := workflow.NewSelector(ctx)
				wait.AddReceive(updates.wake, func(channel workflow.ReceiveChannel, _ bool) { channel.Receive(ctx, nil) })
				if timer := lifecycleTimer(); timer != nil {
					wait.AddFuture(timer, func(workflow.Future) {})
				}
				wait.Select(ctx)
				if lifecycleElapsed() {
					return persistLifecycleDeadlineBlock(result)
				}
				legacyRetryPending = false
				if updates.cancelRequested {
					cancelResult, terminal, cancelErr := cancelAndProject(results)
					if cancelErr != nil || terminal {
						return cancelResult, cancelErr
					}
					result = cancelResult
					cancellationBlocked = true
					continue
				}
				if updates.restartPlan {
					updates.restartPlan = false
					continue runPlan
				}
				result = projection
			}
		}

		digest, err := resultDigestForWire(input.Plan, results, effects.activities)
		if err != nil {
			return WorkflowResult{}, err
		}
		result.Status = imageagent.RunStatusAwaitingFinalApproval
		result.Block = nil
		result.ResultDigest = digest
		result.CommandIngress = updates.commandIngress()
		if !updates.consumeAwaitingApprovalHandoff(input.Plan.Revision) {
			if err := effects.persistRunState(ctx, input, result, "approve_results"); err != nil {
				if errors.Is(err, errWorkflowEffectFenced) {
					return awaitTerminalIntent(results)
				}
				return WorkflowResult{}, err
			}
		}
		projection = result
		for {
			wait := workflow.NewSelector(ctx)
			wait.AddReceive(updates.wake, func(channel workflow.ReceiveChannel, _ bool) { channel.Receive(ctx, nil) })
			if timer := lifecycleTimer(); timer != nil {
				wait.AddFuture(timer, func(workflow.Future) {})
			}
			wait.Select(ctx)
			if lifecycleElapsed() {
				return persistLifecycleDeadlineBlock(result)
			}
			if updates.cancelRequested {
				cancelResult, terminal, cancelErr := cancelAndProject(results)
				if cancelErr != nil || terminal {
					return cancelResult, cancelErr
				}
				result = cancelResult
				continue
			}
			if projection.Status == imageagent.RunStatusCompleted {
				if err := workflow.Await(ctx, func() bool { return workflow.AllHandlersFinished(ctx) }); err != nil {
					return WorkflowResult{}, err
				}
				return projection, nil
			}
		}
	}
}

func persistWorkflowFailure(ctx workflow.Context, input WorkflowInput) error {
	failureCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &sdktemporal.RetryPolicy{
			InitialInterval: time.Second, BackoffCoefficient: 2,
			MaximumInterval: 30 * time.Second, MaximumAttempts: 0,
		},
	})
	if input.projectionExecutionID == "" {
		return workflow.ExecuteActivity(failureCtx, activityPersistWorkflowFailure, PersistWorkflowFailureActivityInput{
			RunID: input.RunID, Identity: input.Identity, FailureCode: "workflow_failed",
			FailureMessage: "图像任务执行失败，可使用相同请求重试",
		}).Get(failureCtx, nil)
	}
	commitID, err := workflowFailureCommitID(input)
	if err != nil {
		return err
	}
	return workflow.ExecuteActivity(failureCtx, activityPersistWorkflowFailureV2, PersistWorkflowFailureV2ActivityInput{
		RunID: input.RunID, Identity: input.Identity, FailureCode: "workflow_failed",
		FailureMessage: "图像任务执行失败，可使用相同请求重试", CommitID: commitID,
	}).Get(failureCtx, nil)
}

func imageAgentActivityContext(ctx workflow.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &sdktemporal.RetryPolicy{
			InitialInterval: time.Second, BackoffCoefficient: 2,
			MaximumInterval: 10 * time.Second, MaximumAttempts: 5,
		},
	})
}

type workflowEffectRequest struct {
	execute          func(workflow.Context) error
	terminalIdentity string
	allowAfterIntent bool
	done             workflow.Channel
}

type workflowEffectResult struct {
	err error
}

type workflowEffectOwner struct {
	requests   workflow.Channel
	activities workflowActivityWire
}

var errWorkflowEffectFenced = errors.New("workflow terminal effect fence rejected request")

func newWorkflowEffectOwner(ctx workflow.Context) *workflowEffectOwner {
	owner := &workflowEffectOwner{requests: workflow.NewChannel(ctx), activities: activityWireForWorkflow(ctx)}
	commitTerminalIntentAfterSuccess := workflow.GetVersion(ctx, externalEffectFinalizationPatch, workflow.DefaultVersion, 1) != workflow.DefaultVersion
	workflow.Go(ctx, func(ownerCtx workflow.Context) {
		terminalIntentIdentity := ""
		terminalSucceeded := false
		for {
			var request workflowEffectRequest
			owner.requests.Receive(ownerCtx, &request)
			if terminalIntentIdentity != "" && request.terminalIdentity != terminalIntentIdentity && !request.allowAfterIntent {
				request.done.Send(ownerCtx, workflowEffectResult{err: fmt.Errorf(
					"%w: terminal intent %s does not match request %s",
					errWorkflowEffectFenced, terminalIntentIdentity, request.terminalIdentity,
				)})
				continue
			}
			if !commitTerminalIntentAfterSuccess && request.terminalIdentity != "" && terminalIntentIdentity == "" {
				terminalIntentIdentity = request.terminalIdentity
			}
			if terminalSucceeded {
				request.done.Send(ownerCtx, workflowEffectResult{})
				continue
			}
			err := request.execute(ownerCtx)
			if err == nil && request.terminalIdentity != "" {
				if terminalIntentIdentity == "" {
					terminalIntentIdentity = request.terminalIdentity
				}
				terminalSucceeded = true
			}
			request.done.Send(ownerCtx, workflowEffectResult{err: err})
		}
	})
	return owner
}

func (o *workflowEffectOwner) execute(ctx workflow.Context, terminalIdentity string, effect func(workflow.Context) error) error {
	return o.executeRequest(ctx, terminalIdentity, false, effect)
}

func (o *workflowEffectOwner) executeRequest(ctx workflow.Context, terminalIdentity string, allowAfterIntent bool, effect func(workflow.Context) error) error {
	done := workflow.NewBufferedChannel(ctx, 1)
	o.requests.Send(ctx, workflowEffectRequest{execute: effect, terminalIdentity: terminalIdentity, allowAfterIntent: allowAfterIntent, done: done})
	var result workflowEffectResult
	done.Receive(ctx, &result)
	return result.err
}

func (o *workflowEffectOwner) persistSlotResult(ctx workflow.Context, input WorkflowInput, result SlotWorkflowResult) error {
	return o.execute(ctx, "", func(ownerCtx workflow.Context) error {
		return executePersistSlotResult(ownerCtx, o.activities.persistSlotResult, input, result)
	})
}

func (o *workflowEffectOwner) persistSlotResultV3(ctx workflow.Context, input WorkflowInput, result SlotWorkflowV3Result) error {
	return o.execute(ctx, "", func(ownerCtx workflow.Context) error {
		activityInput := PersistSlotResultV3ActivityInput{
			RunID: input.RunID, Identity: input.Identity, PlanRevision: input.Plan.Revision, Result: result,
			AttemptKey: slotAttemptKey(input.Plan.Revision, findSlot(input.Plan, result.Published.SlotID), result.Published.Attempt),
		}
		if err := workflow.ExecuteActivity(ownerCtx, o.activities.persistSlotResult, activityInput).Get(ownerCtx, nil); err != nil {
			return fmt.Errorf("persist v3 slot %s result: %w", result.Published.SlotID, err)
		}
		return nil
	})
}

func (o *workflowEffectOwner) reviewStagedSlotV3(ctx workflow.Context, input WorkflowInput, index, attempt int) (SlotWorkflowV3Result, error) {
	if !o.activities.useV3Slot || strings.TrimSpace(o.activities.reviewStagedSlot) == "" {
		return SlotWorkflowV3Result{}, fmt.Errorf("staged review activity is not configured")
	}
	slot := input.Plan.Slots[index]
	activityInput := ExecuteSlotV3ActivityInput{
		RunID: input.RunID, Identity: input.Identity, PlanRevision: input.Plan.Revision,
		TargetPlatform: input.TargetPlatform, ImagePolicyContext: clonePolicyContext(input.ImagePolicyContext),
		Slot: slot, Attempt: attempt, IdempotencyKey: slotAttemptKey(input.Plan.Revision, slot, attempt),
		AssetCatalog: input.AssetCatalog, ExternalEffectFinalization: input.externalEffectFinalization,
		BudgetAuthorization: input.BudgetAuthorization, BudgetPolicy: input.BudgetPolicy,
		DeadlineAt: input.DeadlineAt, LifecycleDeadlineAt: input.LifecycleDeadlineAt,
	}
	var published imageagent.SlotEffectV3PublishedResult
	if err := workflow.ExecuteActivity(ctx, o.activities.reviewStagedSlot, activityInput).Get(ctx, &published); err != nil {
		return SlotWorkflowV3Result{}, err
	}
	normalized, err := imageagent.NormalizeSlotEffectV3PublishedResult(published)
	if err != nil || normalized.SlotID != slot.ID || normalized.Attempt != attempt {
		return SlotWorkflowV3Result{}, fmt.Errorf("staged review returned an invalid slot result")
	}
	return SlotWorkflowV3Result{Published: normalized, Status: imageagent.SlotStatusAccepted, EffectPhase: imageagent.SlotEffectV3PublicationComplete}, nil
}

func (o *workflowEffectOwner) persistRunState(
	ctx workflow.Context,
	input WorkflowInput,
	projection WorkflowResult,
	node string,
	commitIdentity ...string,
) error {
	if isTerminalRunStatus(projection.Status) {
		return fmt.Errorf("terminal run-state effect %s requires a stable action identity", projection.Status)
	}
	commitID, err := runProjectionCommitID(input, projection, node, commitIdentity...)
	if err != nil {
		return err
	}
	return o.execute(ctx, "", func(ownerCtx workflow.Context) error {
		return executePersistRunState(ownerCtx, o.activities.persistRunState, input, projection, node, commitID)
	})
}

func (o *workflowEffectOwner) persistTerminalRunState(
	ctx workflow.Context,
	input WorkflowInput,
	projection WorkflowResult,
	node string,
	actionFingerprint string,
) error {
	if !isTerminalRunStatus(projection.Status) || actionFingerprint == "" {
		return fmt.Errorf("terminal run-state effect requires terminal status and stable action identity")
	}
	identity, err := terminalRunStateEffectIdentity(input, projection.Status, node, projection.Block, actionFingerprint)
	if err != nil {
		return err
	}
	return o.execute(ctx, identity, func(ownerCtx workflow.Context) error {
		return executePersistRunState(ownerCtx, o.activities.persistRunState, input, projection, node, identity)
	})
}

func (o *workflowEffectOwner) startEffectRecoveryV3(ctx workflow.Context, input EffectRecoveryWorkflowInput) error {
	if strings.TrimSpace(o.activities.startEffectRecovery) == "" {
		return fmt.Errorf("effect recovery starter activity is not configured")
	}
	return o.execute(ctx, "", func(ownerCtx workflow.Context) error {
		if err := workflow.ExecuteActivity(ownerCtx, o.activities.startEffectRecovery, input).Get(ownerCtx, nil); err != nil {
			return fmt.Errorf("start effect recovery for slot %s: %w", input.Slot.ID, err)
		}
		return nil
	})
}

func runProjectionCommitID(input WorkflowInput, projection WorkflowResult, node string, identities ...string) (string, error) {
	identity := ""
	if len(identities) > 0 {
		identity = identities[0]
	}
	if input.projectionExecutionID == "" {
		return updateFingerprint("public_projection", struct {
			RunID    string
			Revision int64
			Status   imageagent.RunStatus
			Node     string
			Block    *imageagent.Block
			Identity string
		}{input.RunID, input.Plan.Revision, projection.Status, node, projection.Block, identity})
	}
	return updateFingerprint("public_projection", struct {
		RunID       string
		Revision    int64
		Status      imageagent.RunStatus
		Node        string
		Block       *imageagent.Block
		Identity    string
		ExecutionID string
	}{input.RunID, input.Plan.Revision, projection.Status, node, projection.Block, identity, input.projectionExecutionID})
}

func workflowFailureCommitID(input WorkflowInput) (string, error) {
	if input.projectionExecutionID == "" {
		return "workflow-failed", nil
	}
	return updateFingerprint("workflow_failure_projection", struct {
		RunID       string
		ExecutionID string
	}{input.RunID, input.projectionExecutionID})
}

func (o *workflowEffectOwner) persistPlanRevision(ctx workflow.Context, input WorkflowInput, replacement ReplacePlanSignal) error {
	return o.execute(ctx, "", func(ownerCtx workflow.Context) error {
		return executePersistPlanRevision(ownerCtx, o.activities.persistPlanRevision, input, replacement)
	})
}

func (o *workflowEffectOwner) persistPendingCommand(ctx workflow.Context, input WorkflowInput, receipt *imageagent.PendingCommandReceipt, ingress imageagent.CommandIngress, commitID string) error {
	return o.executeRequest(ctx, "", true, func(ownerCtx workflow.Context) error {
		if err := workflow.ExecuteActivity(ownerCtx, o.activities.persistPendingCommand, PersistPendingCommandActivityInput{RunID: input.RunID, Identity: input.Identity, Receipt: receipt, CommitID: commitID, CommandIngress: ingress}).Get(ownerCtx, nil); err != nil {
			return fmt.Errorf("persist pending command projection: %w", err)
		}
		return nil
	})
}

func (o *workflowEffectOwner) publishApproved(ctx workflow.Context, input PublishApprovedActivityInput) error {
	return o.execute(ctx, "", func(ownerCtx workflow.Context) error {
		if o.activities.useV3Approval {
			v3Input := PublishApprovedV3ActivityInput{
				RunID: input.RunID, Identity: input.Identity, PlanRevision: input.PlanRevision,
				CandidateAssetIDs: append([]string(nil), input.CandidateAssetIDs...), IdempotencyKey: input.IdempotencyKey,
			}
			if err := workflow.ExecuteActivity(ownerCtx, o.activities.publishApproved, v3Input).Get(ownerCtx, nil); err != nil {
				return fmt.Errorf("publish approved assets: %w", err)
			}
			return nil
		}
		if err := workflow.ExecuteActivity(ownerCtx, o.activities.publishApproved, input).Get(ownerCtx, nil); err != nil {
			return fmt.Errorf("publish approved assets: %w", err)
		}
		return nil
	})
}

func isTerminalRunStatus(status imageagent.RunStatus) bool {
	return status == imageagent.RunStatusCompleted || status == imageagent.RunStatusFailed || status == imageagent.RunStatusCancelled
}

func terminalRunStateEffectIdentity(
	input WorkflowInput,
	status imageagent.RunStatus,
	node string,
	block *imageagent.Block,
	actionFingerprint string,
) (string, error) {
	identity, err := updateFingerprint("terminal_run_state", struct {
		RunID             string
		Identity          imageagent.ExecutionIdentity
		PlanRevision      int64
		Status            imageagent.RunStatus
		Node              string
		Block             *imageagent.Block
		ActionFingerprint string
	}{
		RunID: input.RunID, Identity: input.Identity, PlanRevision: input.Plan.Revision,
		Status: status, Node: node, Block: block, ActionFingerprint: actionFingerprint,
	})
	if err != nil {
		return "", fmt.Errorf("fingerprint terminal run-state effect: %w", err)
	}
	return identity, nil
}

type workflowUpdatePhase string

const (
	updatePhaseReplacePersistPlan       workflowUpdatePhase = "replace.persist_plan"
	updatePhaseReplacePersistTransition workflowUpdatePhase = "replace.persist_transition"
	updatePhaseRetryExecuteChild        workflowUpdatePhase = "retry.execute_child"
	updatePhaseRetryPersistResult       workflowUpdatePhase = "retry.persist_result"
	updatePhaseRetryPersistTransition   workflowUpdatePhase = "retry.persist_transition"
	updatePhaseApprovalPublish          workflowUpdatePhase = "approval.publish"
	updatePhaseApprovalPersistComplete  workflowUpdatePhase = "approval.persist_complete"
	updatePhaseCancelPersist            workflowUpdatePhase = "cancel.persist"
	updatePhaseCompleted                workflowUpdatePhase = "completed"
)

type workflowUpdateRecord struct {
	fingerprint       string
	kind              string
	command           interface{}
	phase             workflowUpdatePhase
	running           bool
	future            workflow.Future
	setter            workflow.Settable
	retryResult       *SlotWorkflowResult
	retryResultV3     *SlotWorkflowV3Result
	acknowledgement   CommandAcknowledgement
	completed         bool
	ingressState      signalIngressState
	attempt           int
	readyAttempt      bool
	failureCode       string
	failureCategory   string
	failureMessage    string
	lastFailedAt      *time.Time
	businessValidated bool
}

type signalIngressState string

const (
	signalIngressRejected   signalIngressState = "rejected"
	signalIngressDeferred   signalIngressState = "deferred"
	signalIngressAccepted   signalIngressState = "accepted"
	signalIngressSuperseded signalIngressState = "superseded"
)

const maxActionLedgerEntries = 1024

type workflowUpdateState struct {
	input                           *WorkflowInput
	projection                      *WorkflowResult
	results                         *[]SlotWorkflowResult
	effects                         *workflowEffectOwner
	wake                            workflow.Channel
	actions                         map[string]*workflowUpdateRecord
	pendingActionID                 string
	deferredRetries                 []RetrySlotSignal
	restartPlan                     bool
	executingHandoffRevision        int64
	awaitingApprovalHandoffRevision int64
	cancelRequested                 bool
	cancelPending                   bool
	cancelCommitted                 bool
	cancelCommitErr                 error
	cancelBlocked                   bool
	lastBlockedCancelFingerprint    string
	cancelActionFingerprint         string
	ingressExhausted                bool
	enforceIngressPlanPolicy        bool
}

func newWorkflowUpdateState(
	ctx workflow.Context,
	input *WorkflowInput,
	projection *WorkflowResult,
	results *[]SlotWorkflowResult,
	effects *workflowEffectOwner,
) *workflowUpdateState {
	return &workflowUpdateState{
		input: input, projection: projection, results: results, effects: effects,
		wake: workflow.NewBufferedChannel(ctx, 8), actions: make(map[string]*workflowUpdateRecord),
		enforceIngressPlanPolicy: input.enforceIngressPlanPolicy,
	}
}

func (s *workflowUpdateState) register(ctx workflow.Context) error {
	if err := workflow.SetUpdateHandlerWithOptions(ctx, signalReplacePlan, s.handleReplacePlan, workflow.UpdateHandlerOptions{Validator: s.validateReplacePlan}); err != nil {
		return err
	}
	if err := workflow.SetUpdateHandlerWithOptions(ctx, signalRetrySlot, s.handleRetrySlot, workflow.UpdateHandlerOptions{Validator: s.validateRetrySlot}); err != nil {
		return err
	}
	if err := workflow.SetUpdateHandlerWithOptions(ctx, signalApproveResults, s.handleApproveResults, workflow.UpdateHandlerOptions{Validator: s.validateApproveResults}); err != nil {
		return err
	}
	if err := workflow.SetUpdateHandlerWithOptions(ctx, signalCancel, s.handleCancel, workflow.UpdateHandlerOptions{Validator: s.validateCancel}); err != nil {
		return err
	}
	return workflow.SetUpdateHandlerWithOptions(ctx, updateResumeCommand, s.handleResume, workflow.UpdateHandlerOptions{Validator: s.validateResume})
}

func (s *workflowUpdateState) startSignalHandlers(
	ctx workflow.Context,
	cancelChannel, retryChannel, replaceChannel, approveChannel, recoveryCompletedChannel workflow.ReceiveChannel,
) {
	workflow.Go(ctx, func(signalCtx workflow.Context) {
		for {
			var signal CancelSignal
			cancelChannel.Receive(signalCtx, &signal)
			s.dispatchCancelSignal(signalCtx, signal)
		}
	})
	workflow.Go(ctx, func(signalCtx workflow.Context) {
		for {
			var signal RetrySlotSignal
			retryChannel.Receive(signalCtx, &signal)
			s.dispatchRetrySignal(signalCtx, signal)
		}
	})
	workflow.Go(ctx, func(signalCtx workflow.Context) {
		for {
			var signal ReplacePlanSignal
			replaceChannel.Receive(signalCtx, &signal)
			s.dispatchReplaceSignal(signalCtx, signal)
		}
	})
	workflow.Go(ctx, func(signalCtx workflow.Context) {
		for {
			var signal ApproveResultsSignal
			approveChannel.Receive(signalCtx, &signal)
			s.dispatchApprovalSignal(signalCtx, signal)
		}
	})
	workflow.Go(ctx, func(signalCtx workflow.Context) {
		for {
			var signal EffectRecoveryCompletedSignal
			recoveryCompletedChannel.Receive(signalCtx, &signal)
			s.dispatchEffectRecoveryCompleted(signal)
		}
	})
}

func (s *workflowUpdateState) dispatchEffectRecoveryCompleted(signal EffectRecoveryCompletedSignal) {
	if s.applyEffectRecoveryCompleted(signal) {
		s.wake.SendAsync(struct{}{})
	}
}

// applyEffectRecoveryCompleted reconciles the parent workflow's in-memory
// state after the recovery activity has committed the same transition to the
// projection store. The identity tuple makes delivery safe to replay and
// prevents a late recovery from mutating a newer attempt.
func (s *workflowUpdateState) applyEffectRecoveryCompleted(signal EffectRecoveryCompletedSignal) bool {
	if strings.TrimSpace(signal.RunID) == "" || signal.RunID != s.input.RunID || signal.PlanRevision != s.input.Plan.Revision || strings.TrimSpace(signal.SlotID) == "" || signal.Attempt <= 0 {
		return false
	}
	index := slotIndex(s.input.Plan, signal.SlotID)
	if index < 0 || index >= len(*s.results) || index >= len(s.projection.Slots) {
		return false
	}
	current := (*s.results)[index]
	if current.Execution.SlotID != signal.SlotID || current.Execution.Attempt != signal.Attempt {
		return false
	}
	ownerIndex := recoverableEffectIndex(s.projection.RecoverableEffects, signal.SlotID, signal.Attempt)
	clearOwner := false
	if signal.Result.EffectPhase == imageagent.SlotEffectV3PublicationComplete && signal.Result.Outcome == EffectRecoveryOutcomePublished {
		published, err := imageagent.NormalizeSlotEffectV3PublishedResult(signal.Result.Published)
		if err != nil {
			return false
		}
		if ownerIndex < 0 && recoveredSlotProjectionMatches(s.projection.Slots[index], published) {
			return false
		}
		candidates := make([]imageagent.AssetCandidate, 0, len(published.Candidates))
		for _, candidate := range published.Candidates {
			candidates = append(candidates, imageagent.AssetCandidate{
				AssetID: candidate.AssetID, SourceAssetID: candidate.SourceAssetID, DurableAsset: candidate.DurableAsset,
				Width: candidate.Width, Height: candidate.Height, Operations: append([]string(nil), candidate.Operations...),
			})
		}
		(*s.results)[index] = SlotWorkflowResult{
			Execution: imageagent.SlotExecutionResult{SlotID: published.SlotID, Attempt: published.Attempt, Candidates: candidates},
			Status:    imageagent.SlotStatusAccepted, EffectPhase: imageagent.SlotEffectV3PublicationComplete,
		}
		s.projection.Slots[index].Attempt = published.Attempt
		s.projection.Slots[index].Candidates = append([]imageagent.AssetCandidate(nil), candidates...)
		s.projection.Slots[index].ErrorCode = ""
		s.projection.Slots[index].Slot.Status = imageagent.SlotStatusAccepted
		clearOwner = true
	} else {
		code := strings.TrimSpace(signal.Result.BlockedCode)
		if code == "" || signal.Result.EffectPhase == imageagent.SlotEffectV3PublicationComplete {
			return false
		}
		if ownerIndex >= 0 && current.Status == imageagent.SlotStatusBlocked && current.ErrorCode == code && current.EffectPhase == signal.Result.EffectPhase && s.projection.Slots[index].ErrorCode == code {
			return false
		}
		(*s.results)[index].Status = imageagent.SlotStatusBlocked
		(*s.results)[index].ErrorCode = code
		(*s.results)[index].EffectPhase = signal.Result.EffectPhase
		s.projection.Slots[index].Slot.Status = imageagent.SlotStatusBlocked
		s.projection.Slots[index].ErrorCode = code
		if ownerIndex >= 0 {
			s.projection.RecoverableEffects[ownerIndex].Code = code
		}
	}

	if clearOwner && ownerIndex >= 0 {
		effects := make([]imageagent.RecoverableEffect, 0, len(s.projection.RecoverableEffects)-1)
		effects = append(effects, s.projection.RecoverableEffects[:ownerIndex]...)
		effects = append(effects, s.projection.RecoverableEffects[ownerIndex+1:]...)
		s.projection.RecoverableEffects = effects
	}
	s.projection.Block = recoveryParentBlock(s.projection.RecoverableEffects)
	if len(s.projection.RecoverableEffects) == 0 {
		if s.lastBlockedCancelFingerprint != "" {
			s.cancelRequested = true
			s.cancelPending = true
			s.cancelCommitted = false
			s.cancelCommitErr = nil
			s.cancelBlocked = false
			s.cancelActionFingerprint = s.lastBlockedCancelFingerprint
		} else if s.projection.Status == imageagent.RunStatusBlocked {
			s.projection.Status = imageagent.RunStatusAwaitingFinalApproval
		}
	}
	return true
}

func (s *workflowUpdateState) dispatchReplaceSignal(ctx workflow.Context, signal ReplacePlanSignal) {
	if !s.signalCanEnter(signalReplacePlan, signal.RunID, signal.ActorID, signal.ActionID, signal) {
		return
	}
	_, _ = s.handleReplacePlan(ctx, signal)
}

func (s *workflowUpdateState) dispatchRetrySignal(ctx workflow.Context, signal RetrySlotSignal) {
	fingerprint, err := updateFingerprint(signalRetrySlot, signal)
	if err != nil || strings.TrimSpace(signal.ActionID) == "" {
		return
	}
	if !s.signalCanEnterWithFingerprint(signal.RunID, signal.ActorID, signal.ActionID, fingerprint) {
		return
	}
	if validateCommandOwner(*s.input, signal.RunID, signal.ActorID, signal.ActionID) == nil {
		if _, exists := s.actions[signal.ActionID]; !exists && s.pendingActionID == "" && len(s.actions) < maxActionLedgerEntries &&
			(s.projection.Status == imageagent.RunStatusPlanning || s.projection.Status == imageagent.RunStatusExecuting) {
			record := &workflowUpdateRecord{fingerprint: fingerprint, kind: signalRetrySlot, command: signal, phase: updatePhaseRetryExecuteChild, ingressState: signalIngressDeferred}
			s.actions[signal.ActionID] = record
			s.deferredRetries = append(s.deferredRetries, signal)
			return
		}
	}
	_, _ = s.handleRetrySlot(ctx, signal)
}

func (s *workflowUpdateState) dispatchApprovalSignal(ctx workflow.Context, signal ApproveResultsSignal) {
	if !s.signalCanEnter(signalApproveResults, signal.RunID, signal.ActorID, signal.ActionID, signal) {
		return
	}
	_, _ = s.handleApproveResults(ctx, signal)
}

func (s *workflowUpdateState) dispatchCancelSignal(ctx workflow.Context, signal CancelSignal) {
	if !s.signalCanEnter(signalCancel, signal.RunID, signal.ActorID, signal.ActionID, signal) {
		return
	}
	_, _ = s.handleCancel(ctx, signal)
}

func (s *workflowUpdateState) signalCanEnter(kind, runID, actorID, actionID string, command interface{}) bool {
	fingerprint, err := updateFingerprint(kind, command)
	if err != nil {
		return false
	}
	return s.signalCanEnterWithFingerprint(runID, actorID, actionID, fingerprint)
}

func (s *workflowUpdateState) signalCanEnterWithFingerprint(runID, actorID, actionID, fingerprint string) bool {
	if validateCommandOwner(*s.input, runID, actorID, actionID) != nil {
		return false
	}
	if existing := s.actions[actionID]; existing != nil {
		// Compatibility Signals are ingress-only. An exact pending action can only
		// be retried through the authenticated resume Update; Signals never resume
		// a saga or replace its canonical payload.
		_ = fingerprint
		return false
	}
	return true
}

func initialActionPhase(kind string) workflowUpdatePhase {
	switch kind {
	case signalReplacePlan:
		return updatePhaseReplacePersistPlan
	case signalRetrySlot:
		return updatePhaseRetryExecuteChild
	case signalApproveResults:
		return updatePhaseApprovalPublish
	case signalCancel:
		return updatePhaseCancelPersist
	default:
		return ""
	}
}

func (s *workflowUpdateState) dispatchDeferredRetries(ctx workflow.Context) bool {
	deferred := s.deferredRetries
	s.deferredRetries = nil
	started := false
	for _, signal := range deferred {
		fingerprint, err := updateFingerprint(signalRetrySlot, signal)
		if err != nil {
			continue
		}
		ingress := s.actions[signal.ActionID]
		if ingress == nil || ingress.fingerprint != fingerprint || ingress.ingressState != signalIngressDeferred {
			continue
		}
		if s.pendingActionID != "" {
			ingress.ingressState = signalIngressRejected
			ingress.command = nil
			continue
		}
		ingress.ingressState = signalIngressAccepted
		ingress.attempt = 1
		s.pendingActionID = signal.ActionID
		if err := s.validateRetrySlotBusiness(signal); err != nil {
			s.rejectPreparedAction(signal.ActionID, ingress)
			continue
		}
		ingress.businessValidated = true
		if err := s.startReservedAction(ctx, signal.ActionID, ingress, "promote"); err != nil {
			started = true // The exact pending action can be resumed after the persistence failure.
			continue
		}
		command := signal
		workflow.Go(ctx, func(commandCtx workflow.Context) {
			_, _ = s.handleRetrySlot(commandCtx, command)
		})
		started = true
	}
	return started
}

func (s *workflowUpdateState) consumeExecutingHandoff(revision int64) bool {
	if s.executingHandoffRevision != revision {
		return false
	}
	s.executingHandoffRevision = 0
	return true
}

func (s *workflowUpdateState) consumeAwaitingApprovalHandoff(revision int64) bool {
	if s.awaitingApprovalHandoffRevision != revision {
		return false
	}
	s.awaitingApprovalHandoffRevision = 0
	return true
}

func (s *workflowUpdateState) validateReplacePlan(signal ReplacePlanSignal) error {
	if strings.TrimSpace(signal.RunID) == "" || strings.TrimSpace(signal.ActorID) == "" || strings.TrimSpace(signal.ActionID) == "" || signal.ExpectedRevision <= 0 {
		return updateBlockedError("replacement command shape is invalid")
	}
	return nil
}

func (s *workflowUpdateState) validateReplacePlanBusiness(signal ReplacePlanSignal) error {
	if err := validateCommandRevision(*s.input, signal.ExpectedRevision); err != nil {
		return err
	}
	if s.projection.Status != imageagent.RunStatusBlocked {
		return updateBlockedError("replacement plan is only valid while blocked")
	}
	if s.enforceIngressPlanPolicy && !imageagent.BlockAllowsAction(s.projection.Block, imageagent.ActionEditPlan) {
		return updateBlockedError("the current block does not permit plan replacement")
	}
	if signal.Plan.CreatedBy != s.input.Identity.UserID {
		return updateBlockedError("replacement plan revision, parent, or actor is invalid")
	}
	var err error
	if s.enforceIngressPlanPolicy {
		err = imageagent.ValidateReplacementSubmittedPlan(signal.ExpectedRevision, signal.Plan)
	} else {
		if signal.Plan.ParentRevision != signal.ExpectedRevision || signal.Plan.Revision <= signal.ExpectedRevision {
			return updateBlockedError("replacement plan revision, parent, or actor is invalid")
		}
		err = imageagent.ValidatePlan(signal.Plan)
	}
	if err != nil {
		return updateBlockedError("replacement plan is invalid")
	}
	if err := imageagent.ValidatePlanAgainstCatalog(signal.Plan, s.input.AssetCatalog); err != nil {
		return updateBlockedError("replacement plan references an unauthorized asset")
	}
	return nil
}

func (s *workflowUpdateState) handleReplacePlan(ctx workflow.Context, signal ReplacePlanSignal) (CommandAcknowledgement, error) {
	ctx = imageAgentActivityContext(ctx)
	fingerprint, err := updateFingerprint(signalReplacePlan, signal)
	if err != nil {
		return CommandAcknowledgement{}, updateBlockedError("replacement command cannot be encoded")
	}
	if err := validateCommandOwner(*s.input, signal.RunID, signal.ActorID, signal.ActionID); err != nil {
		return CommandAcknowledgement{}, err
	}
	record, created, err := s.prepareAction(ctx, signal.ActionID, fingerprint, updatePhaseReplacePersistPlan, signalReplacePlan, signal)
	if err != nil {
		return CommandAcknowledgement{}, err
	}
	if created {
		if err := s.validateReplacePlanBusiness(signal); err != nil {
			s.rejectPreparedAction(signal.ActionID, record)
			return CommandAcknowledgement{}, err
		}
		record.businessValidated = true
		if err := s.startReservedAction(ctx, signal.ActionID, record, "start"); err != nil {
			return CommandAcknowledgement{}, err
		}
	}
	return s.runActionAttempt(ctx, signal.ActionID, record, func() (CommandAcknowledgement, error) {
		return s.applyReplacePlan(ctx, signal, record)
	})
}

func (s *workflowUpdateState) applyReplacePlan(ctx workflow.Context, signal ReplacePlanSignal, record *workflowUpdateRecord) (CommandAcknowledgement, error) {
	if record.phase == updatePhaseReplacePersistPlan || record.phase == updatePhaseReplacePersistTransition {
		if err := s.effects.persistPlanRevision(ctx, *s.input, signal); err != nil {
			return CommandAcknowledgement{}, err
		}
		record.phase = updatePhaseCompleted
	}
	nextProjection := executingProjection(signal.Plan, nil)
	nextProjection.CommandIngress = s.commandIngress()
	s.input.Plan = signal.Plan
	*s.results = nil
	*s.projection = nextProjection
	s.executingHandoffRevision = signal.Plan.Revision
	s.restartPlan = true
	return CommandAcknowledgement{
		RunID: signal.RunID, PlanRevision: signal.Plan.Revision, ActionID: signal.ActionID, Status: imageagent.RunStatusExecuting,
	}, nil
}

func (s *workflowUpdateState) validateRetrySlot(signal RetrySlotSignal) error {
	if strings.TrimSpace(signal.RunID) == "" || strings.TrimSpace(signal.ActorID) == "" || strings.TrimSpace(signal.ActionID) == "" || strings.TrimSpace(signal.SlotID) == "" || signal.PlanRevision <= 0 {
		return updateBlockedError("retry command shape is invalid")
	}
	if err := validateCommandOwner(*s.input, signal.RunID, signal.ActorID, signal.ActionID); err != nil {
		return err
	}
	fingerprint, err := updateFingerprint(signalRetrySlot, signal)
	if err != nil {
		return updateBlockedError("retry command cannot be encoded")
	}
	known, err := s.validateAction(signal.ActionID, fingerprint)
	if err != nil || known {
		return err
	}
	return s.validateRetrySlotBusiness(signal)
}

func (s *workflowUpdateState) validateRetrySlotBusiness(signal RetrySlotSignal) error {
	if err := validateCommandRevision(*s.input, signal.PlanRevision); err != nil {
		return err
	}
	if s.projection.Status != imageagent.RunStatusBlocked || s.projection.Block == nil || s.projection.Block.SlotID != signal.SlotID {
		return updateBlockedError("retry is not valid for the current blocked slot")
	}
	if !imageagent.BlockAllowsAction(s.projection.Block, imageagent.ActionRetrySlot) {
		return updateBlockedError("retry is not permitted for the current blocked effect")
	}
	index := slotIndex(s.input.Plan, signal.SlotID)
	if index < 0 || index >= len(*s.results) || (*s.results)[index].Status != imageagent.SlotStatusBlocked {
		return updateBlockedError("retry slot is not blocked")
	}
	return nil
}

func (s *workflowUpdateState) handleRetrySlot(ctx workflow.Context, signal RetrySlotSignal) (CommandAcknowledgement, error) {
	ctx = imageAgentActivityContext(ctx)
	fingerprint, err := updateFingerprint(signalRetrySlot, signal)
	if err != nil {
		return CommandAcknowledgement{}, updateBlockedError("retry command cannot be encoded")
	}
	if err := validateCommandOwner(*s.input, signal.RunID, signal.ActorID, signal.ActionID); err != nil {
		return CommandAcknowledgement{}, err
	}
	record, created, err := s.prepareAction(ctx, signal.ActionID, fingerprint, updatePhaseRetryExecuteChild, signalRetrySlot, signal)
	if err != nil {
		return CommandAcknowledgement{}, err
	}
	if created {
		if err := s.validateRetrySlotBusiness(signal); err != nil {
			s.rejectPreparedAction(signal.ActionID, record)
			return CommandAcknowledgement{}, err
		}
		record.businessValidated = true
		if err := s.startReservedAction(ctx, signal.ActionID, record, "start"); err != nil {
			return CommandAcknowledgement{}, err
		}
	}
	return s.runActionAttempt(ctx, signal.ActionID, record, func() (CommandAcknowledgement, error) {
		return s.applyRetrySlot(ctx, signal, record)
	})
}

func (s *workflowUpdateState) applyRetrySlot(ctx workflow.Context, signal RetrySlotSignal, record *workflowUpdateRecord) (CommandAcknowledgement, error) {
	index := slotIndex(s.input.Plan, signal.SlotID)
	if record.phase == updatePhaseRetryExecuteChild {
		currentAttempt := (*s.results)[index].Execution.Attempt
		if s.effects.activities.useV3Slot && (*s.results)[index].ErrorCode == imageagent.SlotReviewTransportRequiredCode {
			if currentAttempt <= 0 {
				return CommandAcknowledgement{}, fmt.Errorf("review retry is missing its staged attempt")
			}
			reviewed, reviewErr := s.effects.reviewStagedSlotV3(ctx, *s.input, index, currentAttempt)
			if reviewErr != nil {
				code := slotExecutionV3ErrorCode(reviewErr)
				reviewed = SlotWorkflowV3Result{
					Published: imageagent.SlotEffectV3PublishedResult{SlotID: signal.SlotID, Attempt: currentAttempt},
					Status:    imageagent.SlotStatusBlocked, ErrorCode: code,
					EffectPhase: terminalEffectPhaseForErrorCode(code),
				}
			}
			record.retryResultV3 = &reviewed
			pendingResult := SlotWorkflowResult{
				Execution: imageagent.SlotExecutionResult{SlotID: signal.SlotID, Attempt: currentAttempt},
				Status:    reviewed.Status, ErrorCode: reviewed.ErrorCode, EffectPhase: reviewed.EffectPhase,
			}
			if reviewed.Status == imageagent.SlotStatusAccepted {
				pendingResult.Execution.Attempt = reviewed.Published.Attempt
				for _, candidate := range reviewed.Published.Candidates {
					pendingResult.Execution.Candidates = append(pendingResult.Execution.Candidates, imageagent.AssetCandidate{
						AssetID: candidate.AssetID, SourceAssetID: candidate.SourceAssetID, DurableAsset: candidate.DurableAsset,
						Width: candidate.Width, Height: candidate.Height, Operations: append([]string(nil), candidate.Operations...),
					})
				}
			}
			record.retryResult = &pendingResult
			record.phase = updatePhaseRetryPersistResult
		} else {
			attempt := currentAttempt + 1
			completionChannel := workflow.NewBufferedChannel(ctx, 1)
			if s.input.BudgetAuthorization && s.input.BudgetPolicy.AllowsRepairAttempt(attempt-1) != nil {
				completionChannel.Send(ctx, blockedSlotCompletion(*s.input, index, attempt, imageagent.BudgetExhaustedCode, s.effects.activities.useV3Slot))
			} else {
				startChild(ctx, *s.input, index, attempt, completionChannel, s.effects.activities)
			}
			var completion childCompletion
			completionChannel.Receive(ctx, &completion)
			if completion.Failed {
				completion.Result = SlotWorkflowResult{
					Execution: imageagent.SlotExecutionResult{SlotID: signal.SlotID, Attempt: attempt},
					Status:    imageagent.SlotStatusBlocked, ErrorCode: "slot_workflow_failed",
				}
				completion.V3Result = &SlotWorkflowV3Result{
					Published: imageagent.SlotEffectV3PublishedResult{SlotID: signal.SlotID, Attempt: attempt},
					Status:    imageagent.SlotStatusBlocked, ErrorCode: "slot_workflow_failed",
				}
			}
			pendingResult := completion.Result
			record.retryResult = &pendingResult
			if s.effects.activities.useV3Slot && completion.V3Result != nil {
				pendingV3Result := *completion.V3Result
				record.retryResultV3 = &pendingV3Result
			}
			record.phase = updatePhaseRetryPersistResult
		}
	}
	if record.retryResult == nil {
		return CommandAcknowledgement{}, fmt.Errorf("retry update is missing its deterministic child result")
	}
	if record.phase == updatePhaseRetryPersistResult {
		if record.retryResultV3 != nil {
			if err := s.effects.persistSlotResultV3(ctx, *s.input, *record.retryResultV3); err != nil {
				return CommandAcknowledgement{}, err
			}
		} else if err := s.effects.persistSlotResult(ctx, *s.input, *record.retryResult); err != nil {
			return CommandAcknowledgement{}, err
		}
		record.phase = updatePhaseRetryPersistTransition
	}
	stagedResults := append([]SlotWorkflowResult(nil), (*s.results)...)
	stagedResults[index] = *record.retryResult
	result := summarizeResultsForWire(s.input.Plan, stagedResults, s.effects.activities)
	result.CommandIngress = s.commandIngress()
	if result.Block != nil {
		if err := s.effects.persistRunState(ctx, *s.input, result, "retry_slot", record.fingerprint); err != nil {
			return CommandAcknowledgement{}, err
		}
	} else {
		digest, err := resultDigestForWire(s.input.Plan, stagedResults, s.effects.activities)
		if err != nil {
			return CommandAcknowledgement{}, err
		}
		result.Status = imageagent.RunStatusAwaitingFinalApproval
		result.ResultDigest = digest
		if err := s.effects.persistRunState(ctx, *s.input, result, "approve_results", record.fingerprint); err != nil {
			return CommandAcknowledgement{}, err
		}
		s.awaitingApprovalHandoffRevision = s.input.Plan.Revision
	}
	*s.results = stagedResults
	*s.projection = result
	return CommandAcknowledgement{
		RunID: signal.RunID, PlanRevision: signal.PlanRevision, ActionID: signal.ActionID, Status: result.Status,
	}, nil
}

func (s *workflowUpdateState) validateApproveResults(signal ApproveResultsSignal) error {
	if strings.TrimSpace(signal.RunID) == "" || strings.TrimSpace(signal.ActorID) == "" || strings.TrimSpace(signal.ActionID) == "" || signal.PlanRevision <= 0 {
		return updateBlockedError("approval command shape is invalid")
	}
	return nil
}

func (s *workflowUpdateState) validateApproveResultsBusiness(signal ApproveResultsSignal) error {
	if err := validateCommandRevision(*s.input, signal.PlanRevision); err != nil {
		return err
	}
	if s.projection.Status != imageagent.RunStatusAwaitingFinalApproval {
		return updateBlockedError("approval is not valid in the current state")
	}
	if signal.ResultDigest == "" || signal.ResultDigest != strings.TrimSpace(signal.ResultDigest) || signal.ResultDigest != s.projection.ResultDigest {
		return updateBlockedError("approval result digest does not match the current projection")
	}
	return nil
}

func (s *workflowUpdateState) handleApproveResults(ctx workflow.Context, signal ApproveResultsSignal) (CommandAcknowledgement, error) {
	ctx = imageAgentActivityContext(ctx)
	fingerprint, err := updateFingerprint(signalApproveResults, signal)
	if err != nil {
		return CommandAcknowledgement{}, updateBlockedError("approval command cannot be encoded")
	}
	if err := validateCommandOwner(*s.input, signal.RunID, signal.ActorID, signal.ActionID); err != nil {
		return CommandAcknowledgement{}, err
	}
	record, created, err := s.prepareAction(ctx, signal.ActionID, fingerprint, updatePhaseApprovalPublish, signalApproveResults, signal)
	if err != nil {
		return CommandAcknowledgement{}, err
	}
	if created {
		if err := s.validateApproveResultsBusiness(signal); err != nil {
			s.rejectPreparedAction(signal.ActionID, record)
			return CommandAcknowledgement{}, err
		}
		record.businessValidated = true
		if err := s.startReservedAction(ctx, signal.ActionID, record, "start"); err != nil {
			return CommandAcknowledgement{}, err
		}
	}
	return s.runActionAttempt(ctx, signal.ActionID, record, func() (CommandAcknowledgement, error) {
		return s.applyApproveResults(ctx, signal, record)
	})
}

func (s *workflowUpdateState) applyApproveResults(ctx workflow.Context, signal ApproveResultsSignal, record *workflowUpdateRecord) (CommandAcknowledgement, error) {
	if record.phase == updatePhaseApprovalPublish {
		publishInput := PublishApprovedActivityInput{
			RunID: s.input.RunID, Identity: s.input.Identity, PlanRevision: s.input.Plan.Revision,
			CandidateAssetIDs: candidateAssetIDs(s.input.Plan, *s.results),
			IdempotencyKey:    approvalPublicationKeyForWire(signal.ActionID, s.input.RunID, s.input.Plan.Revision, s.effects.activities),
		}
		if err := s.effects.publishApproved(ctx, publishInput); err != nil {
			return CommandAcknowledgement{}, err
		}
		record.phase = updatePhaseApprovalPersistComplete
	}
	result := *s.projection
	result.Status = imageagent.RunStatusCompleted
	result.Block = nil
	if err := s.effects.persistTerminalRunState(ctx, *s.input, result, "complete", record.fingerprint); err != nil {
		return CommandAcknowledgement{}, err
	}
	*s.projection = result
	return CommandAcknowledgement{
		RunID: signal.RunID, PlanRevision: signal.PlanRevision, ActionID: signal.ActionID, Status: imageagent.RunStatusCompleted,
	}, nil
}

func approvalPublicationCommitted(record workflowUpdateRecord) bool {
	return record.kind == signalApproveResults && record.phase == updatePhaseApprovalPersistComplete
}

func (s *workflowUpdateState) validateCancel(signal CancelSignal) error {
	if strings.TrimSpace(signal.RunID) == "" || strings.TrimSpace(signal.ActorID) == "" || strings.TrimSpace(signal.ActionID) == "" || signal.PlanRevision <= 0 {
		return updateBlockedError("cancel command shape is invalid")
	}
	return nil
}

func (s *workflowUpdateState) validateCancelBusiness(signal CancelSignal) error {
	if err := validateCommandRevision(*s.input, signal.PlanRevision); err != nil {
		return err
	}
	if s.input.externalEffectFinalization && s.cancelRequested {
		return updateBlockedError("cancel is already pending")
	}
	if s.input.externalEffectFinalization && s.pendingActionID != "" {
		pending := s.actions[s.pendingActionID]
		if pending != nil && approvalPublicationCommitted(*pending) {
			return updateBlockedError("approval publication is already committed")
		}
	}
	switch s.projection.Status {
	case imageagent.RunStatusCompleted, imageagent.RunStatusFailed, imageagent.RunStatusCancelled:
		return updateBlockedError("cancel is not valid for a terminal run")
	}
	return nil
}

func (s *workflowUpdateState) handleCancel(ctx workflow.Context, signal CancelSignal) (CommandAcknowledgement, error) {
	ctx = imageAgentActivityContext(ctx)
	fingerprint, err := updateFingerprint(signalCancel, signal)
	if err != nil {
		return CommandAcknowledgement{}, updateBlockedError("cancel command cannot be encoded")
	}
	if err := validateCommandOwner(*s.input, signal.RunID, signal.ActorID, signal.ActionID); err != nil {
		return CommandAcknowledgement{}, err
	}
	if s.actions[signal.ActionID] == nil {
		if err := s.validateCancelBusiness(signal); err != nil {
			return CommandAcknowledgement{}, err
		}
	}
	record, created, err := s.prepareAction(ctx, signal.ActionID, fingerprint, updatePhaseCancelPersist, signalCancel, signal)
	if err != nil {
		return CommandAcknowledgement{}, err
	}
	if created {
		record.businessValidated = true
		if err := s.startReservedAction(ctx, signal.ActionID, record, "start"); err != nil {
			return CommandAcknowledgement{}, err
		}
	}
	return s.runActionAttempt(ctx, signal.ActionID, record, func() (CommandAcknowledgement, error) {
		return s.applyCancel(ctx, signal, record)
	})
}

func (s *workflowUpdateState) applyCancel(ctx workflow.Context, signal CancelSignal, record *workflowUpdateRecord) (CommandAcknowledgement, error) {
	if s.input.externalEffectFinalization {
		s.cancelRequested = true
		s.cancelPending = true
		s.cancelCommitErr = nil
		s.cancelBlocked = false
		s.cancelActionFingerprint = record.fingerprint
		// Record the intent before waking the cancellation saga. The saga may
		// launch recovery workflows before this update handler reaches its
		// blocked acknowledgement, and a fast recovery completion must still
		// have the original fingerprint available to resume cancellation.
		s.lastBlockedCancelFingerprint = record.fingerprint
		s.wake.SendAsync(struct{}{})
		if err := workflow.Await(ctx, func() bool {
			return s.cancelCommitted || s.cancelBlocked || (!s.cancelPending && s.cancelCommitErr != nil)
		}); err != nil {
			return CommandAcknowledgement{}, err
		}
		if s.cancelCommitErr != nil {
			return CommandAcknowledgement{}, s.cancelCommitErr
		}
		if s.cancelBlocked {
			s.cancelRequested = false
			s.cancelActionFingerprint = ""
			return CommandAcknowledgement{RunID: signal.RunID, PlanRevision: signal.PlanRevision, ActionID: signal.ActionID, Status: imageagent.RunStatusBlocked}, nil
		}
		return CommandAcknowledgement{RunID: signal.RunID, PlanRevision: signal.PlanRevision, ActionID: signal.ActionID, Status: imageagent.RunStatusCancelled}, nil
	}
	result := *s.projection
	result.Status = imageagent.RunStatusCancelled
	result.Block = nil
	result.ResultDigest = ""
	if err := s.effects.persistTerminalRunState(ctx, *s.input, result, "cancelled", record.fingerprint); err != nil {
		return CommandAcknowledgement{}, err
	}
	*s.projection = result
	s.cancelCommitted = true
	s.cancelRequested = true
	return CommandAcknowledgement{RunID: signal.RunID, PlanRevision: signal.PlanRevision, ActionID: signal.ActionID, Status: imageagent.RunStatusCancelled}, nil
}

func (s *workflowUpdateState) commitPendingCancellation(ctx workflow.Context, results []SlotWorkflowResult) {
	if s.input.externalEffectFinalization && !cancellationResultsTerminalized(results) {
		result := blockedCancellationProjection(*s.input, results, s.effects.activities)
		recoveryHandoffEnabled := strings.TrimSpace(s.effects.activities.startEffectRecovery) != ""
		if recoveryHandoffEnabled {
			markBlockedProjectionCode(&result, recoveryRequestedBlockCode)
		}
		result.CommandIngress = s.commandIngress()
		var err error
		if recoveryHandoffEnabled {
			err = s.persistRecoveryHandoff(ctx, result)
		} else {
			err = s.effects.persistRunState(ctx, *s.input, result, "retry_slot")
		}
		s.cancelPending = false
		s.cancelCommitErr = err
		s.cancelBlocked = false
		if err == nil {
			*s.projection = result
			if recoveryHandoffEnabled {
				failed := cloneWorkflowResult(result)
				failedAny := false
				for _, recoveryInput := range effectRecoveryInputsForCancellation(*s.input, results, result.RecoverableEffects) {
					if startErr := s.effects.startEffectRecoveryV3(ctx, recoveryInput); startErr != nil {
						markRecoverableEffectCode(&failed, recoveryInput.Slot.ID, recoveryInput.Attempt, recoveryStartFailedBlockCode)
						failedAny = true
					}
				}
				if failedAny {
					if persistErr := s.persistRecoveryHandoff(ctx, failed); persistErr != nil {
						s.cancelCommitErr = persistErr
						s.wake.SendAsync(struct{}{})
						return
					}
					*s.projection = failed
				}
			}
			s.cancelBlocked = true
		}
		s.wake.SendAsync(struct{}{})
		return
	}
	result := cancelledProjection(*s.input, results)
	result.CommandIngress = s.commandIngress()
	err := s.effects.persistTerminalRunState(ctx, *s.input, result, "cancelled", s.cancelActionFingerprint)
	s.cancelPending = false
	s.cancelCommitErr = err
	s.cancelBlocked = false
	if err == nil {
		*s.projection = result
		s.cancelCommitted = true
		s.lastBlockedCancelFingerprint = ""
	}
	s.wake.SendAsync(struct{}{})
}

func (s *workflowUpdateState) persistRecoveryHandoff(ctx workflow.Context, result WorkflowResult) error {
	identity, err := recoveryHandoffCommitIdentity(result)
	if err != nil {
		return err
	}
	return s.effects.persistRunState(ctx, *s.input, result, "retry_slot", identity)
}

func recoveryHandoffCommitIdentity(result WorkflowResult) (string, error) {
	effects, err := imageagent.NormalizeRecoverableEffects(result.RecoverableEffects)
	if err != nil {
		return "", err
	}
	sort.Slice(effects, func(i, j int) bool {
		if effects[i].SlotID != effects[j].SlotID {
			return effects[i].SlotID < effects[j].SlotID
		}
		if effects[i].Attempt != effects[j].Attempt {
			return effects[i].Attempt < effects[j].Attempt
		}
		return effects[i].Code < effects[j].Code
	})
	type slotCodeProjection struct {
		SlotID    string
		Attempt   int
		ErrorCode string
	}
	slots := make([]slotCodeProjection, 0, len(result.Slots))
	for _, slot := range result.Slots {
		slots = append(slots, slotCodeProjection{SlotID: strings.TrimSpace(slot.Slot.ID), Attempt: slot.Attempt, ErrorCode: strings.TrimSpace(slot.ErrorCode)})
	}
	sort.Slice(slots, func(i, j int) bool {
		if slots[i].SlotID != slots[j].SlotID {
			return slots[i].SlotID < slots[j].SlotID
		}
		if slots[i].Attempt != slots[j].Attempt {
			return slots[i].Attempt < slots[j].Attempt
		}
		return slots[i].ErrorCode < slots[j].ErrorCode
	})
	return updateFingerprint("recovery_handoff", struct {
		Effects []imageagent.RecoverableEffect
		Slots   []slotCodeProjection
	}{effects, slots})
}

func (s *workflowUpdateState) prepareAction(ctx workflow.Context, actionID, fingerprint string, phase workflowUpdatePhase, kind string, command interface{}) (*workflowUpdateRecord, bool, error) {
	if existing := s.actions[actionID]; existing != nil {
		if existing.fingerprint != fingerprint {
			return nil, false, updateBlockedError("action ID was reused for a different command")
		}
		if existing.ingressState == signalIngressRejected || existing.ingressState == signalIngressDeferred || existing.ingressState == signalIngressSuperseded {
			return nil, false, updateBlockedError("action ID was already consumed by rejected ingress")
		}
		if existing.completed {
			return existing, false, nil
		}
		if s.pendingActionID != actionID {
			return nil, false, updateBlockedError("another image agent command owns the pending saga")
		}
		if !existing.readyAttempt {
			existing.attempt++
			existing.failureCode, existing.failureCategory, existing.failureMessage, existing.lastFailedAt = "", "", "", nil
			if err := s.persistActionReceipt(ctx, actionID, existing, fmt.Sprintf("command:%s:attempt:%d:start", actionID, existing.attempt)); err != nil {
				return nil, false, err
			}
			existing.readyAttempt = true
		}
		return existing, !existing.businessValidated, nil
	}
	if s.input.externalEffectFinalization && s.cancelRequested {
		return nil, false, updateBlockedError("image agent cancellation is pending")
	}
	if !s.canAdmitNewAction(kind) {
		if err := s.persistIngressExhaustion(ctx); err != nil {
			return nil, false, err
		}
		return nil, false, updateBlockedError("image agent action ledger capacity is exhausted")
	}
	record := &workflowUpdateRecord{
		fingerprint: fingerprint, phase: phase, kind: kind, command: command,
		ingressState: signalIngressAccepted, attempt: 1,
	}
	s.actions[actionID] = record
	s.projection.CommandIngress = s.commandIngress()
	if s.pendingActionID != "" {
		if kind == signalCancel && s.supersedeFailedPendingAction() {
			s.pendingActionID = actionID
			return record, true, nil
		}
		record.ingressState = signalIngressRejected
		record.command = nil
		return nil, false, updateBlockedError("another image agent command is pending")
	}
	s.pendingActionID = actionID
	return record, true, nil
}

func (s *workflowUpdateState) canAdmitNewAction(kind string) bool {
	if len(s.actions) < maxActionLedgerEntries {
		return true
	}
	if kind != signalCancel {
		return false
	}
	if s.pendingActionID == "" {
		return true
	}
	return s.failedPendingActionCanBeSuperseded()
}

func (s *workflowUpdateState) failedPendingActionCanBeSuperseded() bool {
	pending := s.actions[s.pendingActionID]
	return pending != nil && pending.kind != signalCancel &&
		(!s.input.externalEffectFinalization || !approvalPublicationCommitted(*pending)) &&
		!pending.completed && !pending.running && pending.lastFailedAt != nil
}

func (s *workflowUpdateState) supersedeFailedPendingAction() bool {
	if !s.failedPendingActionCanBeSuperseded() {
		return false
	}
	pending := s.actions[s.pendingActionID]
	pending.ingressState = signalIngressSuperseded
	pending.command = nil
	pending.retryResult = nil
	pending.retryResultV3 = nil
	pending.readyAttempt = false
	s.pendingActionID = ""
	return true
}

func (s *workflowUpdateState) startReservedAction(ctx workflow.Context, actionID string, record *workflowUpdateRecord, transition string) error {
	if err := s.persistActionReceipt(ctx, actionID, record, fmt.Sprintf("command:%s:attempt:%d:%s", actionID, record.attempt, transition)); err != nil {
		return err
	}
	record.readyAttempt = true
	return nil
}

func (s *workflowUpdateState) rejectPreparedAction(actionID string, record *workflowUpdateRecord) {
	record.ingressState = signalIngressRejected
	record.command = nil
	record.retryResult = nil
	record.retryResultV3 = nil
	record.readyAttempt = false
	if s.pendingActionID == actionID {
		s.pendingActionID = ""
	}
}

func (s *workflowUpdateState) persistActionReceipt(ctx workflow.Context, actionID string, record *workflowUpdateRecord, commitID string) error {
	return s.effects.persistPendingCommand(ctx, *s.input, s.pendingReceipt(actionID, record), s.commandIngress(), commitID)
}

func (s *workflowUpdateState) commandIngress() imageagent.CommandIngress {
	used := len(s.actions)
	if used > maxActionLedgerEntries {
		used = maxActionLedgerEntries
	}
	return imageagent.CommandIngress{Used: used, Limit: maxActionLedgerEntries, Exhausted: s.ingressExhausted, Reason: func() string {
		if s.ingressExhausted {
			return "command_capacity_exhausted"
		}
		return ""
	}()}
}

func (s *workflowUpdateState) persistIngressExhaustion(ctx workflow.Context) error {
	if s.ingressExhausted {
		return nil
	}
	ingress := s.commandIngress()
	ingress.Exhausted = true
	ingress.Reason = "command_capacity_exhausted"
	var receipt *imageagent.PendingCommandReceipt
	if s.pendingActionID != "" {
		if record := s.actions[s.pendingActionID]; record != nil && !record.completed {
			receipt = s.pendingReceipt(s.pendingActionID, record)
		}
	}
	if err := s.effects.persistPendingCommand(ctx, *s.input, receipt, ingress, "command-ingress:exhausted"); err != nil {
		return err
	}
	s.ingressExhausted = true
	s.projection.CommandIngress = ingress
	return nil
}

func (s *workflowUpdateState) projectionSnapshot() WorkflowResult {
	result := *s.projection
	result.CommandIngress = s.commandIngress()
	result.PendingCommand = nil
	if s.pendingActionID == "" {
		return result
	}
	record := s.actions[s.pendingActionID]
	if record == nil || record.completed {
		return result
	}
	result.PendingCommand = s.pendingReceipt(s.pendingActionID, record)
	return result
}

func (s *workflowUpdateState) validateResume(input ResumeCommandInput) error {
	if strings.TrimSpace(input.ActionID) == "" || strings.TrimSpace(input.RunID) == "" || strings.TrimSpace(input.ActorID) == "" {
		return updateBlockedError("workflow resume shape is invalid")
	}
	return nil
}

func (s *workflowUpdateState) handleResume(ctx workflow.Context, input ResumeCommandInput) (CommandAcknowledgement, error) {
	ctx = imageAgentActivityContext(ctx)
	if err := validateCommandOwner(*s.input, input.RunID, input.ActorID, input.ActionID); err != nil {
		return CommandAcknowledgement{}, err
	}
	record := s.actions[input.ActionID]
	if record == nil || record.completed || record.ingressState != signalIngressAccepted || s.pendingActionID != input.ActionID {
		return CommandAcknowledgement{}, updateBlockedError("pending image agent command was not found")
	}
	if !record.readyAttempt && !record.running {
		record.attempt++
		record.failureCode, record.failureCategory, record.failureMessage, record.lastFailedAt = "", "", "", nil
		if err := s.persistActionReceipt(ctx, input.ActionID, record, fmt.Sprintf("command:%s:attempt:%d:resume", input.ActionID, record.attempt)); err != nil {
			return CommandAcknowledgement{}, err
		}
		record.readyAttempt = true
	}
	return s.runActionAttempt(ctx, input.ActionID, record, func() (CommandAcknowledgement, error) {
		switch command := record.command.(type) {
		case ReplacePlanSignal:
			return s.applyReplacePlan(ctx, command, record)
		case RetrySlotSignal:
			return s.applyRetrySlot(ctx, command, record)
		case ApproveResultsSignal:
			return s.applyApproveResults(ctx, command, record)
		case CancelSignal:
			return s.applyCancel(ctx, command, record)
		default:
			return CommandAcknowledgement{}, updateBlockedError("pending image agent command payload is unavailable")
		}
	})
}

func (s *workflowUpdateState) runActionAttempt(ctx workflow.Context, actionID string, record *workflowUpdateRecord, apply func() (CommandAcknowledgement, error)) (CommandAcknowledgement, error) {
	if record.completed {
		return record.acknowledgement, nil
	}
	if record.running {
		var acknowledgement CommandAcknowledgement
		return acknowledgement, record.future.Get(ctx, &acknowledgement)
	}
	if record.ingressState == signalIngressRejected || record.ingressState == signalIngressDeferred || record.ingressState == signalIngressSuperseded {
		return CommandAcknowledgement{}, updateBlockedError("action ID is a rejected workflow tombstone")
	}
	if s.pendingActionID != actionID {
		return CommandAcknowledgement{}, updateBlockedError("another image agent command owns the pending saga")
	}
	if !record.readyAttempt {
		return CommandAcknowledgement{}, updateBlockedError("pending command must be resumed before another attempt")
	}
	record.ingressState = signalIngressAccepted
	record.future, record.setter = workflow.NewFuture(ctx)
	record.running = true
	record.readyAttempt = false
	acknowledgement, err := apply()
	record.running = false
	if err == nil {
		record.phase = updatePhaseCompleted
		record.acknowledgement = acknowledgement
		record.completed = true
		record.ingressState = signalIngressAccepted
		record.command = nil
		record.retryResult = nil
		record.retryResultV3 = nil
		record.kind = ""
		if s.pendingActionID == actionID {
			s.pendingActionID = ""
		}
	}
	if err != nil {
		s.setSafeCommandFailure(ctx, record, err)
		if persistErr := s.persistActionReceipt(ctx, actionID, record, fmt.Sprintf("command:%s:attempt:%d:failed", actionID, record.attempt)); persistErr != nil {
			err = fmt.Errorf("%v; persist pending receipt: %w", err, persistErr)
		}
	}
	record.setter.Set(acknowledgement, err)
	record.future = nil
	record.setter = nil
	if err == nil {
		s.wake.SendAsync(struct{}{})
	}
	return acknowledgement, err
}

func (s *workflowUpdateState) pendingReceipt(actionID string, record *workflowUpdateRecord) *imageagent.PendingCommandReceipt {
	receipt := &imageagent.PendingCommandReceipt{
		ActionID: actionID, Kind: record.kind, Phase: string(record.phase), Status: "pending", PlanRevision: s.input.Plan.Revision,
		FailureCode: record.failureCode, FailureCategory: record.failureCategory, FailureMessage: record.failureMessage,
		LastFailedAt: record.lastFailedAt, Attempt: record.attempt,
	}
	if command, ok := record.command.(RetrySlotSignal); ok {
		receipt.SlotID = command.SlotID
		receipt.PlanRevision = command.PlanRevision
	}
	if command, ok := record.command.(ReplacePlanSignal); ok {
		receipt.PlanRevision = command.ExpectedRevision
	}
	return receipt
}

func (s *workflowUpdateState) setSafeCommandFailure(ctx workflow.Context, record *workflowUpdateRecord, err error) {
	record.failureCode, record.failureCategory, record.failureMessage = safeCommandFailure(record.phase)
	failedAt := workflow.Now(ctx).UTC()
	record.lastFailedAt = &failedAt
	_ = err // Raw errors are deliberately not copied into the public receipt.
}

func safeCommandFailure(phase workflowUpdatePhase) (code, category, message string) {
	switch phase {
	case updatePhaseRetryExecuteChild:
		return "provider_unavailable", "provider", "图片生成服务暂时不可用"
	case updatePhaseApprovalPublish:
		return "publication_failed", "publication", "结果发布暂时失败"
	case updatePhaseReplacePersistPlan, updatePhaseReplacePersistTransition, updatePhaseRetryPersistResult,
		updatePhaseRetryPersistTransition, updatePhaseApprovalPersistComplete, updatePhaseCancelPersist:
		return "persistence_failed", "persistence", "运行状态保存暂时失败"
	}
	return "technical_failure", "technical", "上次操作遇到技术问题，可以恢复后继续"
}

func (s *workflowUpdateState) validateAction(actionID, fingerprint string) (bool, error) {
	if strings.TrimSpace(actionID) == "" {
		return false, updateBlockedError("action ID is required")
	}
	record, ok := s.actions[actionID]
	if ok && record.fingerprint != fingerprint {
		return true, updateBlockedError("action ID was reused for a different command")
	}
	if ok {
		if record.ingressState == signalIngressRejected || record.ingressState == signalIngressDeferred || record.ingressState == signalIngressSuperseded {
			return true, updateBlockedError("action ID was already consumed by rejected ingress")
		}
		return true, nil
	}
	if len(s.actions) >= maxActionLedgerEntries {
		return false, updateBlockedError("image agent action ledger capacity is exhausted")
	}
	if s.pendingActionID != "" {
		return false, updateBlockedError("another image agent command is pending")
	}
	return false, nil
}

func validateCommandOwner(input WorkflowInput, runID, actorID, actionID string) error {
	if imageagent.ValidateActionID(actionID) != nil {
		return updateBlockedError("action ID is invalid")
	}
	if runID != input.RunID || actorID != input.Identity.UserID {
		return sdktemporal.NewNonRetryableApplicationError("image agent run owner was not found", updateErrorRunNotFound, nil)
	}
	return nil
}

func validateCommandRevision(input WorkflowInput, revision int64) error {
	if revision != input.Plan.Revision {
		return sdktemporal.NewNonRetryableApplicationError("image agent plan revision is stale", updateErrorRevisionConflict, nil)
	}
	return nil
}

func validateUpdateIdentity(input WorkflowInput, runID string, revision int64, actorID, actionID string) error {
	if err := validateCommandOwner(input, runID, actorID, actionID); err != nil {
		return err
	}
	return validateCommandRevision(input, revision)
}

func updateFingerprint(name string, command interface{}) (string, error) {
	encoded, err := json.Marshal(command)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append([]byte(name+":"), encoded...))
	return hex.EncodeToString(sum[:]), nil
}

func updateBlockedError(message string) error {
	return sdktemporal.NewNonRetryableApplicationError(message, updateErrorCommandBlocked, nil)
}

type childCompletion struct {
	Index    int
	Result   SlotWorkflowResult
	V3Result *SlotWorkflowV3Result
	Failed   bool
}

func executeInitialSlots(ctx workflow.Context, input WorkflowInput, limit int, updates *workflowUpdateState, progress func([]SlotWorkflowResult)) ([]SlotWorkflowResult, bool, error) {
	results := make([]SlotWorkflowResult, len(input.Plan.Slots))
	if updates != nil && updates.cancelRequested {
		return results, true, nil
	}
	completionChannel := workflow.NewBufferedChannel(ctx, len(input.Plan.Slots))
	childrenCtx, cancelChildren := workflow.WithCancel(ctx)
	next, inFlight := 0, 0
	lifecycleElapsed := !input.LifecycleDeadlineAt.IsZero() && !workflow.Now(ctx).Before(input.LifecycleDeadlineAt)
	var lifecycleTimer workflow.Future
	if !lifecycleElapsed && !input.LifecycleDeadlineAt.IsZero() {
		lifecycleTimer = workflow.NewTimer(ctx, input.LifecycleDeadlineAt.Sub(workflow.Now(ctx)))
	}
	markNotDispatchedAtLifecycleDeadline := func() error {
		for next < len(input.Plan.Slots) {
			completion := blockedSlotCompletion(input, next, 1, imageagent.WorkflowLifecycleElapsedCode, updates.effects.activities.useV3Slot)
			if updates.effects.activities.useV3Slot && completion.V3Result != nil {
				if err := updates.effects.persistSlotResultV3(ctx, input, *completion.V3Result); err != nil {
					return err
				}
			} else if err := updates.effects.persistSlotResult(ctx, input, completion.Result); err != nil {
				return err
			}
			results[next] = completion.Result
			next++
			if progress != nil {
				progress(results)
			}
		}
		return nil
	}
	launch := func(index int) {
		if input.externalEffectFinalization {
			results[index] = SlotWorkflowResult{
				Execution: imageagent.SlotExecutionResult{SlotID: input.Plan.Slots[index].ID, Attempt: 1},
				Status:    imageagent.SlotStatusPending,
			}
		}
		startChild(childrenCtx, input, index, 1, completionChannel, updates.effects.activities)
		next++
		inFlight++
	}
	for !lifecycleElapsed && next < len(input.Plan.Slots) && inFlight < limit {
		launch(next)
	}
	if lifecycleElapsed {
		if err := markNotDispatchedAtLifecycleDeadline(); err != nil {
			return results, false, err
		}
	}
	cancelled := false
	for inFlight > 0 {
		selector := workflow.NewSelector(ctx)
		var completion childCompletion
		gotCompletion := false
		selector.AddReceive(completionChannel, func(channel workflow.ReceiveChannel, _ bool) {
			channel.Receive(ctx, &completion)
			gotCompletion = true
		})
		if updates != nil {
			selector.AddReceive(updates.wake, func(channel workflow.ReceiveChannel, _ bool) {
				channel.Receive(ctx, nil)
				if updates.cancelRequested {
					cancelled = true
					cancelChildren()
				}
			})
		}
		if lifecycleTimer != nil && !lifecycleElapsed {
			selector.AddFuture(lifecycleTimer, func(workflow.Future) {
				lifecycleElapsed = true
			})
		}
		selector.Select(ctx)
		if lifecycleElapsed {
			if err := markNotDispatchedAtLifecycleDeadline(); err != nil {
				return results, false, err
			}
		}
		if updates != nil && updates.cancelRequested {
			cancelled = true
			cancelChildren()
		}
		if !gotCompletion {
			continue
		}
		inFlight--
		persistCompletion := !cancelled || input.externalEffectFinalization
		if persistCompletion {
			if completion.Failed {
				completion = blockedSlotCompletion(input, completion.Index, 1, "slot_workflow_failed", updates.effects.activities.useV3Slot)
			}
			if updates.effects.activities.useV3Slot && completion.V3Result != nil {
				if err := updates.effects.persistSlotResultV3(ctx, input, *completion.V3Result); err != nil {
					return results, false, err
				}
			} else if err := updates.effects.persistSlotResult(ctx, input, completion.Result); err != nil {
				return results, false, err
			}
			results[completion.Index] = completion.Result
			if progress != nil {
				progress(results)
			}
		}
		if !cancelled && !lifecycleElapsed && next < len(input.Plan.Slots) {
			launch(next)
		}
	}
	return results, cancelled, nil
}

func startChild(ctx workflow.Context, input WorkflowInput, index, attempt int, completionChannel workflow.SendChannel, activityWire workflowActivityWire) {
	if !input.LifecycleDeadlineAt.IsZero() && !workflow.Now(ctx).Before(input.LifecycleDeadlineAt) {
		completionChannel.Send(ctx, blockedSlotCompletion(input, index, attempt, imageagent.WorkflowLifecycleElapsedCode, activityWire.useV3Slot))
		return
	}
	if input.BudgetAuthorization && !input.DeadlineAt.IsZero() && !workflow.Now(ctx).Before(input.DeadlineAt) {
		completionChannel.Send(ctx, blockedSlotCompletion(input, index, attempt, imageagent.BudgetElapsedCode, activityWire.useV3Slot))
		return
	}
	slotInput := SlotWorkflowInput{RunID: input.RunID, Identity: input.Identity, PlanRevision: input.Plan.Revision, Slot: input.Plan.Slots[index], Attempt: attempt, AssetCatalog: input.AssetCatalog}
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: childWorkflowID(slotInput), ParentClosePolicy: enums.PARENT_CLOSE_POLICY_REQUEST_CANCEL,
		WaitForCancellation: input.externalEffectFinalization,
	})
	if activityWire.useV3Slot {
		future := workflow.ExecuteChildWorkflow(childCtx, ImageSlotWorkflowV3, SlotWorkflowV3Input{
			RunID: slotInput.RunID, Identity: slotInput.Identity, PlanRevision: slotInput.PlanRevision,
			TargetPlatform: input.TargetPlatform, ImagePolicyContext: clonePolicyContext(input.ImagePolicyContext),
			Slot: slotInput.Slot, Attempt: slotInput.Attempt, AssetCatalog: slotInput.AssetCatalog,
			ExecuteActivityName: activityWire.executeSlot,
			BudgetAuthorization: input.BudgetAuthorization, BudgetPolicy: input.BudgetPolicy, DeadlineAt: input.DeadlineAt, LifecycleDeadlineAt: input.LifecycleDeadlineAt,
			ExternalEffectFinalization: input.externalEffectFinalization,
		})
		workflow.Go(ctx, func(goroutineCtx workflow.Context) {
			var v3Result SlotWorkflowV3Result
			err := future.Get(goroutineCtx, &v3Result)
			result := SlotWorkflowResult{
				Execution: imageagent.SlotExecutionResult{SlotID: slotInput.Slot.ID, Attempt: slotInput.Attempt},
				Status:    v3Result.Status, ErrorCode: v3Result.ErrorCode, EffectPhase: v3Result.EffectPhase,
			}
			if err == nil {
				result.Execution.SlotID = v3Result.Published.SlotID
				result.Execution.Attempt = v3Result.Published.Attempt
				for _, candidate := range v3Result.Published.Candidates {
					result.Execution.Candidates = append(result.Execution.Candidates, imageagent.AssetCandidate{
						AssetID: candidate.AssetID, SourceAssetID: candidate.SourceAssetID, DurableAsset: candidate.DurableAsset,
						Width: candidate.Width, Height: candidate.Height, Operations: append([]string(nil), candidate.Operations...),
					})
				}
			}
			completionChannel.Send(goroutineCtx, childCompletion{Index: index, Result: result, V3Result: &v3Result, Failed: err != nil})
		})
		return
	}
	future := workflow.ExecuteChildWorkflow(childCtx, ImageSlotWorkflow, slotInput)
	workflow.Go(ctx, func(goroutineCtx workflow.Context) {
		var result SlotWorkflowResult
		err := future.Get(goroutineCtx, &result)
		completionChannel.Send(goroutineCtx, childCompletion{Index: index, Result: result, Failed: err != nil})
	})
}

func blockedSlotCompletion(input WorkflowInput, index, attempt int, code string, useV3 bool) childCompletion {
	slot := input.Plan.Slots[index]
	phase := imageagent.SlotEffectV3Phase("")
	if useV3 && input.externalEffectFinalization {
		phase = terminalEffectPhaseForErrorCode(code)
	}
	result := SlotWorkflowResult{Execution: imageagent.SlotExecutionResult{SlotID: slot.ID, Attempt: attempt}, Status: imageagent.SlotStatusBlocked, ErrorCode: code, EffectPhase: phase}
	completion := childCompletion{Index: index, Result: result}
	if useV3 {
		v3 := SlotWorkflowV3Result{Published: imageagent.SlotEffectV3PublishedResult{SlotID: slot.ID, Attempt: attempt}, Status: imageagent.SlotStatusBlocked, ErrorCode: code, EffectPhase: phase}
		completion.V3Result = &v3
	}
	return completion
}

func slotIndex(plan imageagent.Plan, slotID string) int {
	for index, slot := range plan.Slots {
		if slot.ID == slotID {
			return index
		}
	}
	return -1
}

func cancelledProjection(input WorkflowInput, results []SlotWorkflowResult) WorkflowResult {
	result := WorkflowResult{
		Status: imageagent.RunStatusCancelled,
		Plan:   input.Plan,
		Slots:  slotProjections(input.Plan, results),
	}
	for index, slot := range input.Plan.Slots {
		if index < len(results) && results[index].Status == imageagent.SlotStatusAccepted {
			result.CompletedSlotIDs = append(result.CompletedSlotIDs, slot.ID)
		}
	}
	return result
}

func blockedCancellationProjection(input WorkflowInput, results []SlotWorkflowResult, activityWire workflowActivityWire) WorkflowResult {
	result := WorkflowResult{
		Status: imageagent.RunStatusBlocked,
		Plan:   input.Plan,
		Slots:  slotProjections(input.Plan, results),
	}
	result.RecoverableEffects = recoverableEffectsForCancellation(results, activityWire)
	for index, slot := range input.Plan.Slots {
		if index < len(results) && results[index].Status == imageagent.SlotStatusAccepted {
			result.CompletedSlotIDs = append(result.CompletedSlotIDs, slot.ID)
		}
		if result.Block != nil || index >= len(results) || strings.TrimSpace(results[index].Execution.SlotID) == "" || cancellationResultTerminalized(results[index]) {
			continue
		}
		code, message := "slot_failed", "slot_failed"
		if activityWire.useV3Slot && strings.TrimSpace(results[index].ErrorCode) != "" {
			code = imageagent.NormalizeSlotEffectV3BlockCode(results[index].ErrorCode)
			message = code
		} else if strings.TrimSpace(results[index].ErrorCode) != "" {
			message = results[index].ErrorCode
		}
		result.Block = &imageagent.Block{Code: code, Message: message, SlotID: slot.ID}
	}
	return result
}

func effectRecoveryInputsForCancellation(input WorkflowInput, results []SlotWorkflowResult, effects []imageagent.RecoverableEffect) []EffectRecoveryWorkflowInput {
	normalized, err := imageagent.NormalizeRecoverableEffects(effects)
	if err != nil || len(normalized) == 0 {
		return nil
	}
	inputs := make([]EffectRecoveryWorkflowInput, 0, len(normalized))
	for _, effect := range normalized {
		for index := range input.Plan.Slots {
			if input.Plan.Slots[index].ID != effect.SlotID || index >= len(results) || strings.TrimSpace(results[index].Execution.SlotID) == "" || results[index].Execution.Attempt != effect.Attempt {
				continue
			}
			inputs = append(inputs, EffectRecoveryWorkflowInput{
				RunID: input.RunID, Identity: input.Identity, PlanRevision: input.Plan.Revision,
				TargetPlatform: input.TargetPlatform, ImagePolicyContext: clonePolicyContext(input.ImagePolicyContext),
				Slot: input.Plan.Slots[index], Attempt: effect.Attempt, AssetCatalog: input.AssetCatalog,
			})
			break
		}
	}
	return inputs
}

func clonePolicyContext(value *imageagent.ImagePolicyContext) *imageagent.ImagePolicyContext {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func markBlockedProjectionCode(result *WorkflowResult, code string) {
	if result == nil || result.Block == nil {
		return
	}
	result.Block.Code = code
	result.Block.Message = code
	for index := range result.RecoverableEffects {
		result.RecoverableEffects[index].Code = code
		markSlotProjectionRecoverableCode(&result.Slots, result.RecoverableEffects[index])
	}
	if len(result.RecoverableEffects) == 0 {
		for index := range result.Slots {
			if result.Slots[index].Slot.ID == result.Block.SlotID && result.Slots[index].Slot.Status == imageagent.SlotStatusBlocked {
				result.Slots[index].ErrorCode = code
			}
		}
	}
}

func markRecoverableEffectCode(result *WorkflowResult, slotID string, attempt int, code string) {
	if result == nil {
		return
	}
	slotID = strings.TrimSpace(slotID)
	for index := range result.RecoverableEffects {
		if result.RecoverableEffects[index].SlotID != slotID || result.RecoverableEffects[index].Attempt != attempt {
			continue
		}
		result.RecoverableEffects[index].Code = code
		markSlotProjectionRecoverableCode(&result.Slots, result.RecoverableEffects[index])
		if result.Block != nil && strings.TrimSpace(result.Block.SlotID) == slotID {
			result.Block.Code = code
			result.Block.Message = code
		}
		return
	}
	if result.Block != nil && strings.TrimSpace(result.Block.SlotID) == slotID {
		result.Block.Code = code
		result.Block.Message = code
	}
}

func markSlotProjectionRecoverableCode(slots *[]imageagent.SlotProjection, effect imageagent.RecoverableEffect) {
	for index := range *slots {
		if (*slots)[index].Slot.ID == effect.SlotID && (*slots)[index].Attempt == effect.Attempt && (*slots)[index].Slot.Status == imageagent.SlotStatusBlocked {
			(*slots)[index].ErrorCode = effect.Code
			return
		}
	}
}

func recoverableEffectsForCancellation(results []SlotWorkflowResult, activityWire workflowActivityWire) []imageagent.RecoverableEffect {
	if !activityWire.useV3Slot {
		return nil
	}
	effects := make([]imageagent.RecoverableEffect, 0, len(results))
	for _, result := range results {
		if strings.TrimSpace(result.Execution.SlotID) == "" || cancellationResultTerminalized(result) {
			continue
		}
		code := strings.TrimSpace(result.ErrorCode)
		if code == "" {
			code = "slot_failed"
		}
		effects = append(effects, imageagent.RecoverableEffect{
			SlotID: result.Execution.SlotID, Attempt: result.Execution.Attempt, Code: imageagent.NormalizeSlotEffectV3BlockCode(code),
		})
	}
	normalized, err := imageagent.NormalizeRecoverableEffects(effects)
	if err != nil {
		return nil
	}
	return normalized
}

func cloneWorkflowResult(result WorkflowResult) WorkflowResult {
	cloned := result
	cloned.Block = cloneTemporalBlock(result.Block)
	cloned.Slots = append([]imageagent.SlotProjection(nil), result.Slots...)
	cloned.CompletedSlotIDs = append([]string(nil), result.CompletedSlotIDs...)
	cloned.RecoverableEffects = append([]imageagent.RecoverableEffect(nil), result.RecoverableEffects...)
	return cloned
}

func cancellationResultsTerminalized(results []SlotWorkflowResult) bool {
	for _, result := range results {
		if strings.TrimSpace(result.Execution.SlotID) == "" {
			continue
		}
		if !cancellationResultTerminalized(result) {
			return false
		}
	}
	return true
}

func cancellationResultTerminalized(result SlotWorkflowResult) bool {
	switch result.EffectPhase {
	case imageagent.SlotEffectV3PublicationComplete:
		return result.Status == imageagent.SlotStatusAccepted || result.Status == imageagent.SlotStatusBlocked
	case imageagent.SlotEffectV3ProviderNotDispatched:
		if result.Status != imageagent.SlotStatusBlocked {
			return false
		}
		switch result.ErrorCode {
		case imageagent.SlotProviderNotDispatchedCode, imageagent.BudgetExhaustedCode, imageagent.BudgetQuoteUnavailableCode, imageagent.BudgetElapsedCode:
			return true
		default:
			return false
		}
	case imageagent.SlotEffectV3ProviderUnknown:
		return result.Status == imageagent.SlotStatusBlocked && result.ErrorCode == imageagent.SlotProviderOutcomeUnknownCode
	case imageagent.SlotEffectV3StagingUnknown:
		return result.Status == imageagent.SlotStatusBlocked && result.ErrorCode == imageagent.SlotStagingOutcomeUnknownCode
	case imageagent.SlotEffectV3PublicationUnknown:
		return result.Status == imageagent.SlotStatusBlocked && result.ErrorCode == imageagent.SlotPublicationOutcomeUnknownCode
	case imageagent.SlotEffectV3ReviewRequired:
		return result.Status == imageagent.SlotStatusBlocked && result.ErrorCode == imageagent.SlotReviewRequiredCode
	case imageagent.SlotEffectV3ReviewTransportRequired:
		return result.Status == imageagent.SlotStatusBlocked && result.ErrorCode == imageagent.SlotReviewTransportRequiredCode
	default:
		return false
	}
}

func executePersistSlotResult(ctx workflow.Context, activityName string, input WorkflowInput, result SlotWorkflowResult) error {
	activityInput := PersistSlotResultActivityInput{
		RunID: input.RunID, Identity: input.Identity, PlanRevision: input.Plan.Revision,
		Result: result, AttemptKey: slotAttemptKey(input.Plan.Revision, findSlot(input.Plan, result.Execution.SlotID), result.Execution.Attempt),
	}
	if err := workflow.ExecuteActivity(ctx, activityName, activityInput).Get(ctx, nil); err != nil {
		return fmt.Errorf("persist slot %s result: %w", result.Execution.SlotID, err)
	}
	return nil
}

func executePersistRunState(ctx workflow.Context, activityName string, input WorkflowInput, projection WorkflowResult, node, commitID string) error {
	err := workflow.ExecuteActivity(ctx, activityName, PersistRunStateActivityInput{
		RunID: input.RunID, Identity: input.Identity, PlanRevision: input.Plan.Revision,
		Projection: projection, CurrentNode: node, CommitID: commitID,
	}).Get(ctx, nil)
	if err != nil {
		return fmt.Errorf("persist run state %s: %w", projection.Status, err)
	}
	return nil
}

func executePersistPlanRevision(ctx workflow.Context, activityName string, input WorkflowInput, replacement ReplacePlanSignal) error {
	err := workflow.ExecuteActivity(ctx, activityName, PersistPlanRevisionActivityInput{
		RunID: input.RunID, Identity: input.Identity, ExpectedRevision: replacement.ExpectedRevision, Plan: replacement.Plan,
	}).Get(ctx, nil)
	if err != nil {
		return fmt.Errorf("persist image agent replacement plan: %w", err)
	}
	return nil
}

func summarizeResults(plan imageagent.Plan, results []SlotWorkflowResult) WorkflowResult {
	result := WorkflowResult{Status: imageagent.RunStatusAwaitingFinalApproval, Plan: plan, Slots: slotProjections(plan, results)}
	for index, slot := range plan.Slots {
		if results[index].Status == imageagent.SlotStatusAccepted {
			result.CompletedSlotIDs = append(result.CompletedSlotIDs, slot.ID)
			continue
		}
		if result.Block == nil {
			result.Status = imageagent.RunStatusBlocked
			result.Block = &imageagent.Block{Code: "slot_failed", Message: results[index].ErrorCode, SlotID: slot.ID}
		}
	}
	return result
}

func summarizeResultsForWire(plan imageagent.Plan, results []SlotWorkflowResult, activityWire workflowActivityWire) WorkflowResult {
	result := summarizeResults(plan, results)
	if !activityWire.useV3Slot || result.Block == nil {
		return result
	}
	code := imageagent.NormalizeSlotEffectV3BlockCode(result.Block.Message)
	result.Block.Code = code
	result.Block.Message = code
	return result
}

func summarizeResultsV3(plan imageagent.Plan, results []SlotWorkflowV3Result) WorkflowResult {
	result := WorkflowResult{Status: imageagent.RunStatusAwaitingFinalApproval, Plan: plan, Slots: slotProjectionsV3(plan, results)}
	for index, slot := range plan.Slots {
		if results[index].Status == imageagent.SlotStatusAccepted {
			result.CompletedSlotIDs = append(result.CompletedSlotIDs, slot.ID)
			continue
		}
		if result.Block == nil {
			result.Status = imageagent.RunStatusBlocked
			code := imageagent.NormalizeSlotEffectV3BlockCode(results[index].ErrorCode)
			result.Block = &imageagent.Block{Code: code, Message: code, SlotID: slot.ID}
		}
	}
	return result
}

func slotProjectionsV3(plan imageagent.Plan, results []SlotWorkflowV3Result) []imageagent.SlotProjection {
	projections := make([]imageagent.SlotProjection, 0, len(plan.Slots))
	for index, declared := range plan.Slots {
		projection := imageagent.SlotProjection{Slot: declared}
		if index < len(results) && results[index].Published.SlotID != "" {
			result := results[index]
			projection.Attempt = result.Published.Attempt
			projection.ErrorCode = result.ErrorCode
			projection.Slot.Status = result.Status
			for _, candidate := range result.Published.Candidates {
				projection.Candidates = append(projection.Candidates, imageagent.AssetCandidate{
					AssetID: candidate.AssetID, SourceAssetID: candidate.SourceAssetID, DurableAsset: candidate.DurableAsset,
					Width: candidate.Width, Height: candidate.Height, Operations: append([]string(nil), candidate.Operations...),
				})
			}
		}
		projections = append(projections, projection)
	}
	return projections
}

func executingProjection(plan imageagent.Plan, results []SlotWorkflowResult) WorkflowResult {
	projection := WorkflowResult{Status: imageagent.RunStatusExecuting, Plan: plan, Slots: slotProjections(plan, results)}
	for index, slot := range plan.Slots {
		if index < len(results) && results[index].Status == imageagent.SlotStatusAccepted {
			projection.CompletedSlotIDs = append(projection.CompletedSlotIDs, slot.ID)
		}
	}
	return projection
}

func slotProjections(plan imageagent.Plan, results []SlotWorkflowResult) []imageagent.SlotProjection {
	projections := make([]imageagent.SlotProjection, 0, len(plan.Slots))
	for index, declared := range plan.Slots {
		slot := declared
		projection := imageagent.SlotProjection{Slot: slot}
		if index < len(results) && results[index].Execution.SlotID != "" {
			result := results[index]
			projection.Attempt = result.Execution.Attempt
			projection.Candidates = append([]imageagent.AssetCandidate(nil), result.Execution.Candidates...)
			projection.ErrorCode = result.ErrorCode
			projection.Slot.Status = result.Status
		}
		projections = append(projections, projection)
	}
	return projections
}

func candidateAssetIDs(plan imageagent.Plan, results []SlotWorkflowResult) []string {
	seen := map[string]bool{}
	var ids []string
	for index := range plan.Slots {
		for _, candidate := range results[index].Execution.Candidates {
			id := strings.TrimSpace(candidate.AssetID)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

func resultDigest(plan imageagent.Plan, results []SlotWorkflowResult) (string, error) {
	slots := make([]imageagent.SlotProjection, len(results))
	for index := range results {
		slots[index].Candidates = append([]imageagent.AssetCandidate(nil), results[index].Execution.Candidates...)
	}
	return imageagent.ResultDigestV2(plan, slots)
}

func resultDigestForWire(plan imageagent.Plan, results []SlotWorkflowResult, activityWire workflowActivityWire) (string, error) {
	slots := slotProjections(plan, results)
	if activityWire.useV3Approval {
		return imageagent.ResultDigestV3(plan, slots)
	}
	return imageagent.ResultDigestV2(plan, slots)
}

func approvalPublicationKeyForWire(actionID, runID string, revision int64, activityWire workflowActivityWire) string {
	if activityWire.useV3Approval {
		if activityWire.useRunScopedApprovalKey {
			if activityWire.useBoundedApprovalKey {
				return approvalActionPublicationKey(actionID, runID, revision)
			}
			return legacyApprovalActionPublicationKey(actionID, runID, revision)
		}
		return actionID
	}
	return publicationKey(runID, revision)
}

func approvalActionPublicationKey(actionID, runID string, revision int64) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d", strings.TrimSpace(actionID), strings.TrimSpace(runID), revision)))
	return fmt.Sprintf("image-agent:approval:%s", hex.EncodeToString(digest[:]))
}

func legacyApprovalActionPublicationKey(actionID, runID string, revision int64) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(actionID)))
	return fmt.Sprintf("image-agent:%s:plan:%d:approval:%s", strings.TrimSpace(runID), revision, hex.EncodeToString(digest[:]))
}

func findSlot(plan imageagent.Plan, slotID string) imageagent.Slot {
	for _, slot := range plan.Slots {
		if slot.ID == slotID {
			return slot
		}
	}
	return imageagent.Slot{ID: slotID, IdempotencyKey: "unknown-slot"}
}
