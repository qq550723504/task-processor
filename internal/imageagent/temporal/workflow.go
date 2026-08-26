package temporal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.temporal.io/api/enums/v1"
	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"task-processor/internal/imageagent"
)

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
	ctx = imageAgentActivityContext(ctx)
	projection := WorkflowResult{Status: imageagent.RunStatusPlanning, Plan: input.Plan, Slots: slotProjections(input.Plan, nil)}
	var updates *workflowUpdateState
	cancelAndProject := func(results []SlotWorkflowResult) (WorkflowResult, error) {
		var result WorkflowResult
		var err error
		if updates != nil && updates.cancelCommitted {
			result = cancelledProjection(input, results)
		} else {
			result, err = cancelledResult(ctx, input, results)
		}
		if err != nil {
			return result, err
		}
		projection = result
		if updates != nil && updates.cancelRequested {
			err = workflow.Await(ctx, func() bool { return workflow.AllHandlersFinished(ctx) })
		}
		return result, err
	}
	if err := workflow.SetQueryHandler(ctx, QueryWorkflowProjection, func() (WorkflowResult, error) {
		return projection, nil
	}); err != nil {
		return WorkflowResult{}, fmt.Errorf("register image agent projection query: %w", err)
	}
	cancelChannel := workflow.GetSignalChannel(ctx, signalCancel)
	retryChannel := workflow.GetSignalChannel(ctx, signalRetrySlot)
	replaceChannel := workflow.GetSignalChannel(ctx, signalReplacePlan)
	approveChannel := workflow.GetSignalChannel(ctx, signalApproveResults)
	var results []SlotWorkflowResult
	updates = newWorkflowUpdateState(ctx, &input, &projection, &results)
	if err := updates.register(ctx); err != nil {
		return WorkflowResult{}, fmt.Errorf("register image agent update handlers: %w", err)
	}
	seenActions := map[string]bool{}
	rejectQueuedApprovals(approveChannel, seenActions)
	rejectQueuedReplacements(replaceChannel, seenActions)
	if receiveQueuedCancel(cancelChannel, input, seenActions) {
		return cancelAndProject(nil)
	}

runPlan:
	for {
		projection = WorkflowResult{Status: imageagent.RunStatusExecuting, Plan: input.Plan, Slots: slotProjections(input.Plan, nil)}
		if err := persistRunState(ctx, input, imageagent.RunStatusExecuting, "execute_slots", nil); err != nil {
			return WorkflowResult{}, err
		}

		limit := input.MaxConcurrentSlots
		if limit <= 0 {
			limit = defaultMaxConcurrentSlots
		}
		var cancelled bool
		var err error
		results, cancelled, err = executeInitialSlots(ctx, input, limit, cancelChannel, seenActions, updates, func(results []SlotWorkflowResult) {
			projection = executingProjection(input.Plan, results)
		})
		if err != nil {
			return WorkflowResult{}, err
		}
		if cancelled {
			return cancelAndProject(results)
		}
		rejectQueuedApprovals(approveChannel, seenActions)

		result := summarizeResults(input.Plan, results)
		if result.Block != nil {
			rejectQueuedReplacements(replaceChannel, seenActions)
			if err := persistRunState(ctx, input, imageagent.RunStatusBlocked, "retry_slot", result.Block); err != nil {
				return WorkflowResult{}, err
			}
			projection = result
			for result.Block != nil {
				rejectQueuedApprovals(approveChannel, seenActions)
				if replacement, ok := receiveQueuedReplacement(replaceChannel, input, seenActions); ok {
					if err := persistPlanRevision(ctx, input, replacement); err != nil {
						return WorkflowResult{}, err
					}
					input.Plan = replacement.Plan
					continue runPlan
				}
				results, cancelled, err = applyQueuedRetries(ctx, input, results, retryChannel, cancelChannel, seenActions)
				if err != nil {
					return WorkflowResult{}, err
				}
				result = summarizeResults(input.Plan, results)
				projection = result
				if cancelled {
					return cancelAndProject(results)
				}
				if result.Block == nil {
					break
				}
				if !input.WaitForCommands {
					return result, nil
				}
				var retry RetrySlotSignal
				var replacement ReplacePlanSignal
				gotRetry := false
				gotReplacement := false
				gotUpdateWake := false
				selector := workflow.NewSelector(ctx)
				selector.AddReceive(retryChannel, func(channel workflow.ReceiveChannel, _ bool) {
					channel.Receive(ctx, &retry)
					gotRetry = true
				})
				selector.AddReceive(cancelChannel, func(channel workflow.ReceiveChannel, _ bool) {
					var cancelSignal CancelSignal
					channel.Receive(ctx, &cancelSignal)
					if validCancel(input, cancelSignal, seenActions) {
						cancelled = true
					}
				})
				selector.AddReceive(replaceChannel, func(channel workflow.ReceiveChannel, _ bool) {
					channel.Receive(ctx, &replacement)
					gotReplacement = true
				})
				selector.AddReceive(updates.wake, func(channel workflow.ReceiveChannel, _ bool) {
					channel.Receive(ctx, nil)
					gotUpdateWake = true
				})
				selector.Select(ctx)
				if cancelled {
					return cancelAndProject(results)
				}
				if gotReplacement {
					if !validReplacement(input, replacement, seenActions) {
						continue
					}
					if err := persistPlanRevision(ctx, input, replacement); err != nil {
						return WorkflowResult{}, err
					}
					input.Plan = replacement.Plan
					continue runPlan
				}
				if gotUpdateWake {
					if updates.cancelRequested {
						return cancelAndProject(results)
					}
					if updates.restartPlan {
						updates.restartPlan = false
						continue runPlan
					}
					result = projection
					continue
				}
				if gotRetry {
					oneRetry := workflow.NewBufferedChannel(ctx, 1)
					oneRetry.SendAsync(retry)
					results, cancelled, err = applyQueuedRetries(ctx, input, results, oneRetry, cancelChannel, seenActions)
					if err != nil {
						return WorkflowResult{}, err
					}
					result = summarizeResults(input.Plan, results)
					projection = result
				}
			}
		}

		rejectQueuedApprovals(approveChannel, seenActions)
		if err := persistRunState(ctx, input, imageagent.RunStatusAwaitingFinalApproval, "approve_results", nil); err != nil {
			return WorkflowResult{}, err
		}
		rejectQueuedApprovals(approveChannel, seenActions)
		digest, err := resultDigest(input.Plan, results)
		if err != nil {
			return WorkflowResult{}, err
		}
		result.Status = imageagent.RunStatusAwaitingFinalApproval
		result.Block = nil
		result.ResultDigest = digest
		projection = result
		for {
			selector := workflow.NewSelector(ctx)
			approved := false
			updateWoke := false
			cancelled = false
			selector.AddReceive(approveChannel, func(channel workflow.ReceiveChannel, _ bool) {
				var signal ApproveResultsSignal
				channel.Receive(ctx, &signal)
				if validApprovalSignal(input, signal, projection.Status, digest, seenActions) {
					approved = true
				}
			})
			selector.AddReceive(cancelChannel, func(channel workflow.ReceiveChannel, _ bool) {
				var signal CancelSignal
				channel.Receive(ctx, &signal)
				if validCancel(input, signal, seenActions) {
					cancelled = true
				}
			})
			selector.AddReceive(updates.wake, func(channel workflow.ReceiveChannel, _ bool) {
				channel.Receive(ctx, nil)
				updateWoke = true
			})
			selector.Select(ctx)
			if updateWoke {
				if updates.cancelRequested {
					return cancelAndProject(results)
				}
				if projection.Status == imageagent.RunStatusCompleted {
					if err := workflow.Await(ctx, func() bool { return workflow.AllHandlersFinished(ctx) }); err != nil {
						return WorkflowResult{}, err
					}
					return projection, nil
				}
				continue
			}
			if cancelled {
				return cancelAndProject(results)
			}
			if !approved {
				continue
			}
			publishInput := PublishApprovedActivityInput{
				RunID: input.RunID, Identity: input.Identity, PlanRevision: input.Plan.Revision,
				CandidateAssetIDs: candidateAssetIDs(input.Plan, results),
				IdempotencyKey:    publicationKey(input.RunID, input.Plan.Revision),
			}
			if err := workflow.ExecuteActivity(ctx, activityPublishApproved, publishInput).Get(ctx, nil); err != nil {
				return WorkflowResult{}, fmt.Errorf("publish approved assets: %w", err)
			}
			if err := persistRunState(ctx, input, imageagent.RunStatusCompleted, "complete", nil); err != nil {
				return WorkflowResult{}, err
			}
			result.Status = imageagent.RunStatusCompleted
			projection = result
			if err := workflow.Await(ctx, func() bool { return workflow.AllHandlersFinished(ctx) }); err != nil {
				return WorkflowResult{}, err
			}
			return result, nil
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
	fingerprint     string
	phase           workflowUpdatePhase
	running         bool
	future          workflow.Future
	setter          workflow.Settable
	retryResult     *SlotWorkflowResult
	acknowledgement CommandAcknowledgement
	completed       bool
}

type workflowUpdateState struct {
	input           *WorkflowInput
	projection      *WorkflowResult
	results         *[]SlotWorkflowResult
	wake            workflow.Channel
	actions         map[string]*workflowUpdateRecord
	pendingActionID string
	restartPlan     bool
	cancelRequested bool
	cancelCommitted bool
}

func newWorkflowUpdateState(ctx workflow.Context, input *WorkflowInput, projection *WorkflowResult, results *[]SlotWorkflowResult) *workflowUpdateState {
	return &workflowUpdateState{
		input: input, projection: projection, results: results,
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
	return workflow.SetUpdateHandlerWithOptions(ctx, signalCancel, s.handleCancel, workflow.UpdateHandlerOptions{Validator: s.validateCancel})
}

func (s *workflowUpdateState) validateReplacePlan(signal ReplacePlanSignal) error {
	fingerprint, err := updateFingerprint(signalReplacePlan, signal)
	if err != nil {
		return updateBlockedError("replacement command cannot be encoded")
	}
	if duplicate, err := s.validateAction(signal.ActionID, fingerprint); duplicate || err != nil {
		return err
	}
	if err := validateUpdateIdentity(*s.input, signal.RunID, signal.ExpectedRevision, signal.ActorID, signal.ActionID); err != nil {
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
	return nil
}

func (s *workflowUpdateState) handleReplacePlan(ctx workflow.Context, signal ReplacePlanSignal) (CommandAcknowledgement, error) {
	ctx = imageAgentActivityContext(ctx)
	fingerprint, err := updateFingerprint(signalReplacePlan, signal)
	if err != nil {
		return CommandAcknowledgement{}, updateBlockedError("replacement command cannot be encoded")
	}
	if err := s.validateReplacePlan(signal); err != nil {
		return CommandAcknowledgement{}, err
	}
	record := s.actions[signal.ActionID]
	if record == nil {
		record = s.beginAction(signal.ActionID, fingerprint, updatePhaseReplacePersistPlan)
	}
	return s.runActionAttempt(ctx, signal.ActionID, record, func() (CommandAcknowledgement, error) {
		return s.applyReplacePlan(ctx, signal, record)
	})
}

func (s *workflowUpdateState) applyReplacePlan(ctx workflow.Context, signal ReplacePlanSignal, record *workflowUpdateRecord) (CommandAcknowledgement, error) {
	if record.phase == updatePhaseReplacePersistPlan || record.phase == updatePhaseReplacePersistTransition {
		if err := persistPlanRevision(ctx, *s.input, signal); err != nil {
			return CommandAcknowledgement{}, err
		}
		record.phase = updatePhaseReplacePersistTransition
	}
	transitionInput := *s.input
	transitionInput.Plan = signal.Plan
	if err := persistRunState(ctx, transitionInput, imageagent.RunStatusExecuting, "execute_slots", nil); err != nil {
		return CommandAcknowledgement{}, err
	}
	s.input.Plan = signal.Plan
	*s.results = nil
	*s.projection = executingProjection(signal.Plan, nil)
	s.restartPlan = true
	s.wake.SendAsync(struct{}{})
	return CommandAcknowledgement{
		RunID: signal.RunID, PlanRevision: signal.Plan.Revision, ActionID: signal.ActionID, Status: imageagent.RunStatusExecuting,
	}, nil
}

func (s *workflowUpdateState) validateRetrySlot(signal RetrySlotSignal) error {
	fingerprint, err := updateFingerprint(signalRetrySlot, signal)
	if err != nil {
		return updateBlockedError("retry command cannot be encoded")
	}
	if duplicate, err := s.validateAction(signal.ActionID, fingerprint); duplicate || err != nil {
		return err
	}
	if err := validateUpdateIdentity(*s.input, signal.RunID, signal.PlanRevision, signal.ActorID, signal.ActionID); err != nil {
		return err
	}
	if s.projection.Status != imageagent.RunStatusBlocked || s.projection.Block == nil || s.projection.Block.SlotID != signal.SlotID {
		return updateBlockedError("retry is not valid for the current blocked slot")
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
	if err := s.validateRetrySlot(signal); err != nil {
		return CommandAcknowledgement{}, err
	}
	record := s.actions[signal.ActionID]
	if record == nil {
		record = s.beginAction(signal.ActionID, fingerprint, updatePhaseRetryExecuteChild)
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
		if err := persistSlotResult(ctx, *s.input, *record.retryResult); err != nil {
			return CommandAcknowledgement{}, err
		}
		record.phase = updatePhaseRetryPersistTransition
	}
	stagedResults := append([]SlotWorkflowResult(nil), (*s.results)...)
	stagedResults[index] = *record.retryResult
	result := summarizeResults(s.input.Plan, stagedResults)
	if result.Block != nil {
		if err := persistRunState(ctx, *s.input, imageagent.RunStatusBlocked, "retry_slot", result.Block); err != nil {
			return CommandAcknowledgement{}, err
		}
	} else {
		digest, err := resultDigest(s.input.Plan, stagedResults)
		if err != nil {
			return CommandAcknowledgement{}, err
		}
		result.Status = imageagent.RunStatusAwaitingFinalApproval
		result.ResultDigest = digest
		if err := persistRunState(ctx, *s.input, imageagent.RunStatusAwaitingFinalApproval, "approve_results", nil); err != nil {
			return CommandAcknowledgement{}, err
		}
	}
	*s.results = stagedResults
	*s.projection = result
	s.wake.SendAsync(struct{}{})
	return CommandAcknowledgement{
		RunID: signal.RunID, PlanRevision: signal.PlanRevision, ActionID: signal.ActionID, Status: result.Status,
	}, nil
}

func (s *workflowUpdateState) validateApproveResults(signal ApproveResultsSignal) error {
	fingerprint, err := updateFingerprint(signalApproveResults, signal)
	if err != nil {
		return updateBlockedError("approval command cannot be encoded")
	}
	if duplicate, err := s.validateAction(signal.ActionID, fingerprint); duplicate || err != nil {
		return err
	}
	if err := validateUpdateIdentity(*s.input, signal.RunID, signal.PlanRevision, signal.ActorID, signal.ActionID); err != nil {
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
	if err := s.validateApproveResults(signal); err != nil {
		return CommandAcknowledgement{}, err
	}
	record := s.actions[signal.ActionID]
	if record == nil {
		record = s.beginAction(signal.ActionID, fingerprint, updatePhaseApprovalPublish)
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
			IdempotencyKey:    publicationKey(s.input.RunID, s.input.Plan.Revision),
		}
		if err := workflow.ExecuteActivity(ctx, activityPublishApproved, publishInput).Get(ctx, nil); err != nil {
			return CommandAcknowledgement{}, fmt.Errorf("publish approved assets: %w", err)
		}
		record.phase = updatePhaseApprovalPersistComplete
	}
	if err := persistRunState(ctx, *s.input, imageagent.RunStatusCompleted, "complete", nil); err != nil {
		return CommandAcknowledgement{}, err
	}
	result := *s.projection
	result.Status = imageagent.RunStatusCompleted
	*s.projection = result
	s.wake.SendAsync(struct{}{})
	return CommandAcknowledgement{
		RunID: signal.RunID, PlanRevision: signal.PlanRevision, ActionID: signal.ActionID, Status: imageagent.RunStatusCompleted,
	}, nil
}

func (s *workflowUpdateState) validateCancel(signal CancelSignal) error {
	fingerprint, err := updateFingerprint(signalCancel, signal)
	if err != nil {
		return updateBlockedError("cancel command cannot be encoded")
	}
	if duplicate, err := s.validateAction(signal.ActionID, fingerprint); duplicate || err != nil {
		return err
	}
	if err := validateUpdateIdentity(*s.input, signal.RunID, signal.PlanRevision, signal.ActorID, signal.ActionID); err != nil {
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
	if err := s.validateCancel(signal); err != nil {
		return CommandAcknowledgement{}, err
	}
	record := s.actions[signal.ActionID]
	if record == nil {
		record = s.beginAction(signal.ActionID, fingerprint, updatePhaseCancelPersist)
	}
	return s.runActionAttempt(ctx, signal.ActionID, record, func() (CommandAcknowledgement, error) {
		return s.applyCancel(ctx, signal)
	})
}

func (s *workflowUpdateState) applyCancel(ctx workflow.Context, signal CancelSignal) (CommandAcknowledgement, error) {
	if err := persistRunState(ctx, *s.input, imageagent.RunStatusCancelled, "cancelled", nil); err != nil {
		return CommandAcknowledgement{}, err
	}
	result := *s.projection
	result.Status = imageagent.RunStatusCancelled
	result.Block = nil
	result.ResultDigest = ""
	*s.projection = result
	s.cancelCommitted = true
	s.cancelRequested = true
	s.wake.SendAsync(struct{}{})
	return CommandAcknowledgement{RunID: signal.RunID, PlanRevision: signal.PlanRevision, ActionID: signal.ActionID, Status: imageagent.RunStatusCancelled}, nil
}

func (s *workflowUpdateState) beginAction(actionID, fingerprint string, phase workflowUpdatePhase) *workflowUpdateRecord {
	record := &workflowUpdateRecord{fingerprint: fingerprint, phase: phase}
	s.actions[actionID] = record
	s.pendingActionID = actionID
	return record
}

func (s *workflowUpdateState) runActionAttempt(ctx workflow.Context, actionID string, record *workflowUpdateRecord, apply func() (CommandAcknowledgement, error)) (CommandAcknowledgement, error) {
	if record.completed {
		return record.acknowledgement, nil
	}
	if record.running {
		var acknowledgement CommandAcknowledgement
		return acknowledgement, record.future.Get(ctx, &acknowledgement)
	}
	record.future, record.setter = workflow.NewFuture(ctx)
	record.running = true
	acknowledgement, err := apply()
	record.running = false
	if err == nil {
		record.phase = updatePhaseCompleted
		record.acknowledgement = acknowledgement
		record.completed = true
		if s.pendingActionID == actionID {
			s.pendingActionID = ""
		}
	}
	record.setter.Set(acknowledgement, err)
	record.future = nil
	record.setter = nil
	return acknowledgement, err
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
		return true, nil
	}
	if s.pendingActionID != "" {
		return false, updateBlockedError("another image agent command is pending")
	}
	return false, nil
}

func validateUpdateIdentity(input WorkflowInput, runID string, revision int64, actorID, actionID string) error {
	if strings.TrimSpace(actionID) == "" || runID != input.RunID || actorID != input.Identity.UserID {
		return updateBlockedError("workflow command identity does not match")
	}
	if revision != input.Plan.Revision {
		return sdktemporal.NewNonRetryableApplicationError("image agent plan revision is stale", updateErrorRevisionConflict, nil)
	}
	return nil
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

func executeInitialSlots(ctx workflow.Context, input WorkflowInput, limit int, cancelChannel workflow.ReceiveChannel, seenActions map[string]bool, updates *workflowUpdateState, progress func([]SlotWorkflowResult)) ([]SlotWorkflowResult, bool, error) {
	results := make([]SlotWorkflowResult, len(input.Plan.Slots))
	if (updates != nil && updates.cancelRequested) || receiveQueuedCancel(cancelChannel, input, seenActions) {
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
		selector.AddReceive(cancelChannel, func(channel workflow.ReceiveChannel, _ bool) {
			var signal CancelSignal
			channel.Receive(ctx, &signal)
			if validCancel(input, signal, seenActions) {
				cancelled = true
				cancelChildren()
			}
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
			results[completion.Index] = completion.Result
			if err := persistSlotResult(ctx, input, completion.Result); err != nil {
				return nil, false, err
			}
			if progress != nil {
				progress(results)
			}
		}
		if !cancelled && receiveQueuedCancel(cancelChannel, input, seenActions) {
			cancelled = true
			cancelChildren()
		}
		if !cancelled && next < len(input.Plan.Slots) {
			launch(next)
		}
	}
	return results, cancelled, nil
}

func startChild(ctx workflow.Context, input WorkflowInput, index, attempt int, completionChannel workflow.SendChannel) {
	slotInput := SlotWorkflowInput{RunID: input.RunID, Identity: input.Identity, PlanRevision: input.Plan.Revision, Slot: input.Plan.Slots[index], Attempt: attempt}
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

func applyQueuedRetries(ctx workflow.Context, input WorkflowInput, results []SlotWorkflowResult, retryChannel, cancelChannel workflow.ReceiveChannel, seenActions map[string]bool) ([]SlotWorkflowResult, bool, error) {
	for {
		if receiveQueuedCancel(cancelChannel, input, seenActions) {
			return results, true, nil
		}
		var signal RetrySlotSignal
		if !retryChannel.ReceiveAsync(&signal) {
			return results, false, nil
		}
		if !validRetry(input, signal, seenActions) {
			continue
		}
		index := slotIndex(input.Plan, signal.SlotID)
		if index < 0 || results[index].Status != imageagent.SlotStatusBlocked {
			continue
		}
		attempt := results[index].Execution.Attempt + 1
		completionChannel := workflow.NewBufferedChannel(ctx, 1)
		childCtx, cancelChild := workflow.WithCancel(ctx)
		startChild(childCtx, input, index, attempt, completionChannel)
		var completion childCompletion
		gotCompletion, cancelled := false, false
		for !gotCompletion && !cancelled {
			selector := workflow.NewSelector(ctx)
			selector.AddReceive(completionChannel, func(channel workflow.ReceiveChannel, _ bool) {
				channel.Receive(ctx, &completion)
				gotCompletion = true
			})
			selector.AddReceive(cancelChannel, func(channel workflow.ReceiveChannel, _ bool) {
				var cancelSignal CancelSignal
				channel.Receive(ctx, &cancelSignal)
				if validCancel(input, cancelSignal, seenActions) {
					cancelled = true
					cancelChild()
				}
			})
			selector.Select(ctx)
		}
		if cancelled {
			return results, true, nil
		}
		if completion.Failed {
			completion.Result = SlotWorkflowResult{Execution: imageagent.SlotExecutionResult{SlotID: signal.SlotID, Attempt: attempt}, Status: imageagent.SlotStatusBlocked, ErrorCode: "slot_workflow_failed"}
		}
		results[index] = completion.Result
		if err := persistSlotResult(ctx, input, completion.Result); err != nil {
			return nil, false, err
		}
	}
}

func receiveQueuedCancel(channel workflow.ReceiveChannel, input WorkflowInput, seenActions map[string]bool) bool {
	cancelled := false
	for {
		var signal CancelSignal
		if !channel.ReceiveAsync(&signal) {
			return cancelled
		}
		if validCancel(input, signal, seenActions) {
			cancelled = true
		}
	}
}

func validCancel(input WorkflowInput, signal CancelSignal, seenActions map[string]bool) bool {
	if !consumeAction(signal.ActionID, seenActions) {
		return false
	}
	return signal.RunID == input.RunID && signal.PlanRevision == input.Plan.Revision && signal.ActorID == input.Identity.UserID
}

func validRetry(input WorkflowInput, signal RetrySlotSignal, seenActions map[string]bool) bool {
	if !consumeAction(signal.ActionID, seenActions) {
		return false
	}
	return signal.RunID == input.RunID && signal.PlanRevision == input.Plan.Revision && signal.SlotID != "" && signal.ActorID == input.Identity.UserID
}

func validReplacement(input WorkflowInput, signal ReplacePlanSignal, seenActions map[string]bool) bool {
	if !consumeAction(signal.ActionID, seenActions) {
		return false
	}
	if signal.RunID != input.RunID || signal.ExpectedRevision != input.Plan.Revision || signal.ActorID != input.Identity.UserID ||
		signal.Plan.ParentRevision != signal.ExpectedRevision || signal.Plan.Revision <= signal.ExpectedRevision || signal.Plan.CreatedBy != input.Identity.UserID {
		return false
	}
	return imageagent.ValidatePlan(signal.Plan) == nil
}

func receiveQueuedReplacement(channel workflow.ReceiveChannel, input WorkflowInput, seenActions map[string]bool) (ReplacePlanSignal, bool) {
	for {
		var signal ReplacePlanSignal
		if !channel.ReceiveAsync(&signal) {
			return ReplacePlanSignal{}, false
		}
		if validReplacement(input, signal, seenActions) {
			return signal, true
		}
	}
}

func rejectQueuedReplacements(channel workflow.ReceiveChannel, seenActions map[string]bool) {
	for {
		var signal ReplacePlanSignal
		if !channel.ReceiveAsync(&signal) {
			return
		}
		consumeAction(signal.ActionID, seenActions)
	}
}

func validApprovalSignal(input WorkflowInput, signal ApproveResultsSignal, status imageagent.RunStatus, digest string, seenActions map[string]bool) bool {
	if !consumeAction(signal.ActionID, seenActions) {
		return false
	}
	return status == imageagent.RunStatusAwaitingFinalApproval &&
		signal.RunID == input.RunID && signal.PlanRevision == input.Plan.Revision &&
		signal.ActorID == input.Identity.UserID && signal.ResultDigest != "" && signal.ResultDigest == digest
}

func rejectQueuedApprovals(channel workflow.ReceiveChannel, seenActions map[string]bool) {
	for {
		var signal ApproveResultsSignal
		if !channel.ReceiveAsync(&signal) {
			return
		}
		consumeAction(signal.ActionID, seenActions)
	}
}

func consumeAction(actionID string, seenActions map[string]bool) bool {
	if actionID == "" || seenActions[actionID] {
		return false
	}
	seenActions[actionID] = true
	return true
}

func slotIndex(plan imageagent.Plan, slotID string) int {
	for index, slot := range plan.Slots {
		if slot.ID == slotID {
			return index
		}
	}
	return -1
}

func cancelledResult(ctx workflow.Context, input WorkflowInput, results []SlotWorkflowResult) (WorkflowResult, error) {
	if err := persistRunState(ctx, input, imageagent.RunStatusCancelled, "cancelled", nil); err != nil {
		return WorkflowResult{}, err
	}
	return cancelledProjection(input, results), nil
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

func persistSlotResult(ctx workflow.Context, input WorkflowInput, result SlotWorkflowResult) error {
	activityInput := PersistSlotResultActivityInput{
		RunID: input.RunID, Identity: input.Identity, PlanRevision: input.Plan.Revision,
		Result: result, AttemptKey: slotAttemptKey(findSlot(input.Plan, result.Execution.SlotID), result.Execution.Attempt),
	}
	if err := workflow.ExecuteActivity(ctx, activityPersistSlotResult, activityInput).Get(ctx, nil); err != nil {
		return fmt.Errorf("persist slot %s result: %w", result.Execution.SlotID, err)
	}
	return nil
}

func persistRunState(ctx workflow.Context, input WorkflowInput, status imageagent.RunStatus, node string, block *imageagent.Block) error {
	err := workflow.ExecuteActivity(ctx, activityPersistRunState, PersistRunStateActivityInput{
		RunID: input.RunID, Identity: input.Identity, PlanRevision: input.Plan.Revision,
		Status: status, CurrentNode: node, Block: block,
	}).Get(ctx, nil)
	if err != nil {
		return fmt.Errorf("persist run state %s: %w", status, err)
	}
	return nil
}

func persistPlanRevision(ctx workflow.Context, input WorkflowInput, replacement ReplacePlanSignal) error {
	err := workflow.ExecuteActivity(ctx, activityPersistPlanRevision, PersistPlanRevisionActivityInput{
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

type digestSlotResult struct {
	SlotID            string   `json:"slot_id"`
	CandidateAssetIDs []string `json:"candidate_asset_ids"`
}

func resultDigest(plan imageagent.Plan, results []SlotWorkflowResult) (string, error) {
	if len(results) != len(plan.Slots) {
		return "", fmt.Errorf("final image agent results do not match declared slots")
	}
	payload := make([]digestSlotResult, 0, len(plan.Slots))
	for index, slot := range plan.Slots {
		candidateIDs := make([]string, 0, len(results[index].Execution.Candidates))
		for _, candidate := range results[index].Execution.Candidates {
			id := strings.TrimSpace(candidate.AssetID)
			if id == "" {
				return "", fmt.Errorf("slot %s final result contains an empty candidate asset ID", slot.ID)
			}
			candidateIDs = append(candidateIDs, id)
		}
		payload = append(payload, digestSlotResult{SlotID: strings.TrimSpace(slot.ID), CandidateAssetIDs: candidateIDs})
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode final image agent result digest: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func findSlot(plan imageagent.Plan, slotID string) imageagent.Slot {
	for _, slot := range plan.Slots {
		if slot.ID == slotID {
			return slot
		}
	}
	return imageagent.Slot{ID: slotID, IdempotencyKey: "unknown-slot"}
}
