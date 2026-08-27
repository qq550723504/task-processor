package temporal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.temporal.io/api/enums/v1"
	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"task-processor/internal/imageagent"
)

const activityWireV2Patch = "image-agent-atomic-command-boundaries-v2"

type workflowActivityWire struct {
	executeSlot, persistSlotResult, persistRunState, persistPlanRevision, persistPendingCommand, publishApproved string
}

func activityWireForWorkflow(ctx workflow.Context) workflowActivityWire {
	version := workflow.GetVersion(ctx, activityWireV2Patch, workflow.DefaultVersion, 1)
	if version == workflow.DefaultVersion {
		return workflowActivityWire{
			executeSlot: activityExecuteSlotLegacy, persistSlotResult: activityPersistSlotResultLegacy,
			persistRunState: activityPersistRunStateLegacy, persistPlanRevision: activityPersistPlanRevisionLegacy,
			persistPendingCommand: activityPersistPendingCommandLegacy, publishApproved: activityPublishApprovedLegacy,
		}
	}
	return workflowActivityWire{
		executeSlot: activityExecuteSlot, persistSlotResult: activityPersistSlotResult,
		persistRunState: activityPersistRunState, persistPlanRevision: activityPersistPlanRevision,
		persistPendingCommand: activityPersistPendingCommand, publishApproved: activityPublishApproved,
	}
}

func ImageAgentWorkflow(ctx workflow.Context, input WorkflowInput) (WorkflowResult, error) {
	if input.Mode != imageagent.RunModeManual {
		return WorkflowResult{}, fmt.Errorf("image agent workflow mode must be manual")
	}
	if input.RunID == "" || input.Identity.TenantID == "" || input.Identity.UserID == "" {
		return WorkflowResult{}, fmt.Errorf("run ID and verified execution identity are required")
	}
	if err := imageagent.ValidatePlan(input.Plan); err != nil {
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
	ctx = imageAgentActivityContext(ctx)
	effects := newWorkflowEffectOwner(ctx)
	projection := WorkflowResult{Status: imageagent.RunStatusPlanning, Plan: input.Plan, Slots: slotProjections(input.Plan, nil), CommandIngress: imageagent.CommandIngress{Limit: maxActionLedgerEntries}}
	var updates *workflowUpdateState
	cancelAndProject := func(results []SlotWorkflowResult) (WorkflowResult, error) {
		if updates == nil || !updates.cancelCommitted {
			return WorkflowResult{}, fmt.Errorf("image agent cancellation was not committed by the command saga")
		}
		result := cancelledProjection(input, results)
		result.CommandIngress = updates.commandIngress()
		projection = result
		if updates.cancelRequested {
			if err := workflow.Await(ctx, func() bool { return workflow.AllHandlersFinished(ctx) }); err != nil {
				return result, err
			}
		}
		return result, nil
	}
	cancelChannel := workflow.GetSignalChannel(ctx, signalCancel)
	retryChannel := workflow.GetSignalChannel(ctx, signalRetrySlot)
	replaceChannel := workflow.GetSignalChannel(ctx, signalReplacePlan)
	approveChannel := workflow.GetSignalChannel(ctx, signalApproveResults)
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
	updates.startSignalHandlers(ctx, cancelChannel, retryChannel, replaceChannel, approveChannel)
	awaitTerminalIntent := func(results []SlotWorkflowResult) (WorkflowResult, error) {
		if err := workflow.Await(ctx, func() bool {
			return updates.cancelCommitted || isTerminalRunStatus(projection.Status)
		}); err != nil {
			return WorkflowResult{}, err
		}
		if updates.cancelCommitted {
			return cancelAndProject(results)
		}
		if err := workflow.Await(ctx, func() bool { return workflow.AllHandlersFinished(ctx) }); err != nil {
			return WorkflowResult{}, err
		}
		return projection, nil
	}

runPlan:
	for {
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
			if err := workflow.Await(ctx, func() bool {
				return updates.pendingActionID == ""
			}); err != nil {
				return WorkflowResult{}, err
			}
		}
		if updates.cancelRequested {
			return cancelAndProject(nil)
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
		if cancelled {
			return cancelAndProject(results)
		}
		result := summarizeResults(input.Plan, results)
		result.CommandIngress = updates.commandIngress()
		if result.Block != nil {
			if err := effects.persistRunState(ctx, input, result, "retry_slot"); err != nil {
				if errors.Is(err, errWorkflowEffectFenced) {
					return awaitTerminalIntent(results)
				}
				return WorkflowResult{}, err
			}
			projection = result
			legacyRetryPending := updates.dispatchDeferredRetries(ctx)
			for result.Block != nil {
				if !input.WaitForCommands && !legacyRetryPending {
					return result, nil
				}
				updates.wake.Receive(ctx, nil)
				legacyRetryPending = false
				if updates.cancelRequested {
					return cancelAndProject(results)
				}
				if updates.restartPlan {
					updates.restartPlan = false
					continue runPlan
				}
				result = projection
			}
		}

		digest, err := resultDigest(input.Plan, results)
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
			updates.wake.Receive(ctx, nil)
			if updates.cancelRequested {
				return cancelAndProject(results)
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
			if request.terminalIdentity != "" && terminalIntentIdentity == "" {
				terminalIntentIdentity = request.terminalIdentity
			}
			if terminalSucceeded {
				request.done.Send(ownerCtx, workflowEffectResult{})
				continue
			}
			err := request.execute(ownerCtx)
			if err == nil && request.terminalIdentity != "" {
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

func runProjectionCommitID(input WorkflowInput, projection WorkflowResult, node string, identities ...string) (string, error) {
	identity := ""
	if len(identities) > 0 {
		identity = identities[0]
	}
	return updateFingerprint("public_projection", struct {
		RunID    string
		Revision int64
		Status   imageagent.RunStatus
		Node     string
		Block    *imageagent.Block
		Identity string
	}{input.RunID, input.Plan.Revision, projection.Status, node, projection.Block, identity})
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
	signalIngressRejected signalIngressState = "rejected"
	signalIngressDeferred signalIngressState = "deferred"
	signalIngressAccepted signalIngressState = "accepted"
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
	cancelCommitted                 bool
	ingressExhausted                bool
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
	cancelChannel, retryChannel, replaceChannel, approveChannel workflow.ReceiveChannel,
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
	if signal.Plan.ParentRevision != signal.ExpectedRevision || signal.Plan.Revision <= signal.ExpectedRevision || signal.Plan.CreatedBy != s.input.Identity.UserID {
		return updateBlockedError("replacement plan revision, parent, or actor is invalid")
	}
	if err := imageagent.ValidatePlan(signal.Plan); err != nil {
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
	return nil
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
		attempt := (*s.results)[index].Execution.Attempt + 1
		completionChannel := workflow.NewBufferedChannel(ctx, 1)
		startChild(ctx, *s.input, index, attempt, completionChannel)
		var completion childCompletion
		completionChannel.Receive(ctx, &completion)
		if completion.Failed {
			completion.Result = SlotWorkflowResult{
				Execution: imageagent.SlotExecutionResult{SlotID: signal.SlotID, Attempt: attempt},
				Status:    imageagent.SlotStatusBlocked, ErrorCode: "slot_workflow_failed",
			}
		}
		pendingResult := completion.Result
		record.retryResult = &pendingResult
		record.phase = updatePhaseRetryPersistResult
	}
	if record.retryResult == nil {
		return CommandAcknowledgement{}, fmt.Errorf("retry update is missing its deterministic child result")
	}
	if record.phase == updatePhaseRetryPersistResult {
		if err := s.effects.persistSlotResult(ctx, *s.input, *record.retryResult); err != nil {
			return CommandAcknowledgement{}, err
		}
		record.phase = updatePhaseRetryPersistTransition
	}
	stagedResults := append([]SlotWorkflowResult(nil), (*s.results)...)
	stagedResults[index] = *record.retryResult
	result := summarizeResults(s.input.Plan, stagedResults)
	result.CommandIngress = s.commandIngress()
	if result.Block != nil {
		if err := s.effects.persistRunState(ctx, *s.input, result, "retry_slot", record.fingerprint); err != nil {
			return CommandAcknowledgement{}, err
		}
	} else {
		digest, err := resultDigest(s.input.Plan, stagedResults)
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
			IdempotencyKey:    signal.ActionID,
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
	record, created, err := s.prepareAction(ctx, signal.ActionID, fingerprint, updatePhaseCancelPersist, signalCancel, signal)
	if err != nil {
		return CommandAcknowledgement{}, err
	}
	if created {
		if err := s.validateCancelBusiness(signal); err != nil {
			s.rejectPreparedAction(signal.ActionID, record)
			return CommandAcknowledgement{}, err
		}
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

func (s *workflowUpdateState) prepareAction(ctx workflow.Context, actionID, fingerprint string, phase workflowUpdatePhase, kind string, command interface{}) (*workflowUpdateRecord, bool, error) {
	if existing := s.actions[actionID]; existing != nil {
		if existing.fingerprint != fingerprint {
			return nil, false, updateBlockedError("action ID was reused for a different command")
		}
		if existing.ingressState == signalIngressRejected || existing.ingressState == signalIngressDeferred {
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
	if len(s.actions) >= maxActionLedgerEntries {
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
		record.ingressState = signalIngressRejected
		record.command = nil
		return nil, false, updateBlockedError("another image agent command is pending")
	}
	s.pendingActionID = actionID
	return record, true, nil
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
	record.readyAttempt = false
	if s.pendingActionID == actionID {
		s.pendingActionID = ""
	}
}

func (s *workflowUpdateState) persistActionReceipt(ctx workflow.Context, actionID string, record *workflowUpdateRecord, commitID string) error {
	return s.effects.persistPendingCommand(ctx, *s.input, s.pendingReceipt(actionID, record), s.commandIngress(), commitID)
}

func (s *workflowUpdateState) commandIngress() imageagent.CommandIngress {
	return imageagent.CommandIngress{Used: len(s.actions), Limit: maxActionLedgerEntries, Exhausted: s.ingressExhausted, Reason: func() string {
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
	ingress := imageagent.CommandIngress{Used: len(s.actions), Limit: maxActionLedgerEntries, Exhausted: true, Reason: "command_capacity_exhausted"}
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
	if record.ingressState == signalIngressRejected || record.ingressState == signalIngressDeferred {
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
		if record.ingressState == signalIngressRejected || record.ingressState == signalIngressDeferred {
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
	if strings.TrimSpace(actionID) == "" || runID != input.RunID || actorID != input.Identity.UserID {
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
	Index  int
	Result SlotWorkflowResult
	Failed bool
}

func executeInitialSlots(ctx workflow.Context, input WorkflowInput, limit int, updates *workflowUpdateState, progress func([]SlotWorkflowResult)) ([]SlotWorkflowResult, bool, error) {
	results := make([]SlotWorkflowResult, len(input.Plan.Slots))
	if updates != nil && updates.cancelRequested {
		return results, true, nil
	}
	completionChannel := workflow.NewBufferedChannel(ctx, len(input.Plan.Slots))
	childrenCtx, cancelChildren := workflow.WithCancel(ctx)
	next, inFlight := 0, 0
	launch := func(index int) {
		startChild(childrenCtx, input, index, 1, completionChannel)
		next++
		inFlight++
	}
	for next < len(input.Plan.Slots) && inFlight < limit {
		launch(next)
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
		selector.Select(ctx)
		if updates != nil && updates.cancelRequested {
			cancelled = true
			cancelChildren()
		}
		if !gotCompletion {
			continue
		}
		inFlight--
		if !cancelled {
			if completion.Failed {
				slot := input.Plan.Slots[completion.Index]
				completion.Result = SlotWorkflowResult{Execution: imageagent.SlotExecutionResult{SlotID: slot.ID, Attempt: 1}, Status: imageagent.SlotStatusBlocked, ErrorCode: "slot_workflow_failed"}
			}
			if err := updates.effects.persistSlotResult(ctx, input, completion.Result); err != nil {
				return results, false, err
			}
			results[completion.Index] = completion.Result
			if progress != nil {
				progress(results)
			}
		}
		if !cancelled && next < len(input.Plan.Slots) {
			launch(next)
		}
	}
	return results, cancelled, nil
}

func startChild(ctx workflow.Context, input WorkflowInput, index, attempt int, completionChannel workflow.SendChannel) {
	slotInput := SlotWorkflowInput{RunID: input.RunID, Identity: input.Identity, PlanRevision: input.Plan.Revision, Slot: input.Plan.Slots[index], Attempt: attempt, AssetCatalog: input.AssetCatalog}
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: childWorkflowID(slotInput), ParentClosePolicy: enums.PARENT_CLOSE_POLICY_REQUEST_CANCEL,
	})
	future := workflow.ExecuteChildWorkflow(childCtx, ImageSlotWorkflow, slotInput)
	workflow.Go(ctx, func(goroutineCtx workflow.Context) {
		var result SlotWorkflowResult
		err := future.Get(goroutineCtx, &result)
		completionChannel.Send(goroutineCtx, childCompletion{Index: index, Result: result, Failed: err != nil})
	})
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
				projection.Candidates = append(projection.Candidates, imageagent.AssetCandidate{AssetID: candidate.AssetID, SourceAssetID: candidate.SourceAssetID, DurableAsset: candidate.DurableAsset})
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

func findSlot(plan imageagent.Plan, slotID string) imageagent.Slot {
	for _, slot := range plan.Slots {
		if slot.ID == slotID {
			return slot
		}
	}
	return imageagent.Slot{ID: slotID, IdempotencyKey: "unknown-slot"}
}
