package temporal

import (
	"fmt"
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
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &sdktemporal.RetryPolicy{
			InitialInterval: time.Second, BackoffCoefficient: 2,
			MaximumInterval: 10 * time.Second, MaximumAttempts: 5,
		},
	})
	cancelChannel := workflow.GetSignalChannel(ctx, signalCancel)
	retryChannel := workflow.GetSignalChannel(ctx, signalRetrySlot)
	seenActions := map[string]bool{}
	if err := persistRunState(ctx, input, imageagent.RunStatusExecuting, "execute_slots", nil); err != nil {
		return WorkflowResult{}, err
	}

	limit := input.MaxConcurrentSlots
	if limit <= 0 {
		limit = defaultMaxConcurrentSlots
	}
	results, cancelled, err := executeInitialSlots(ctx, input, limit, cancelChannel, seenActions)
	if err != nil {
		return WorkflowResult{}, err
	}
	if cancelled {
		completed := summarizeResults(input.Plan, results).CompletedSlotIDs
		return cancelledResult(ctx, input, completed)
	}

	result := summarizeResults(input.Plan, results)
	if result.Block != nil {
		if err := persistRunState(ctx, input, imageagent.RunStatusBlocked, "retry_slot", result.Block); err != nil {
			return WorkflowResult{}, err
		}
		for result.Block != nil {
			results, cancelled, err = applyQueuedRetries(ctx, input, results, retryChannel, cancelChannel, seenActions)
			if err != nil {
				return WorkflowResult{}, err
			}
			result = summarizeResults(input.Plan, results)
			if cancelled {
				return cancelledResult(ctx, input, result.CompletedSlotIDs)
			}
			if result.Block == nil {
				break
			}
			if !input.WaitForCommands {
				return result, nil
			}
			var retry RetrySlotSignal
			gotRetry := false
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
			selector.Select(ctx)
			if cancelled {
				return cancelledResult(ctx, input, result.CompletedSlotIDs)
			}
			if gotRetry {
				oneRetry := workflow.NewBufferedChannel(ctx, 1)
				oneRetry.SendAsync(retry)
				results, cancelled, err = applyQueuedRetries(ctx, input, results, oneRetry, cancelChannel, seenActions)
				if err != nil {
					return WorkflowResult{}, err
				}
				result = summarizeResults(input.Plan, results)
			}
		}
	}

	if err := persistRunState(ctx, input, imageagent.RunStatusAwaitingFinalApproval, "approve_results", nil); err != nil {
		return WorkflowResult{}, err
	}
	approveChannel := workflow.GetSignalChannel(ctx, signalApproveResults)
	for {
		selector := workflow.NewSelector(ctx)
		approved := false
		cancelled = false
		selector.AddReceive(approveChannel, func(channel workflow.ReceiveChannel, _ bool) {
			var signal ApproveResultsSignal
			channel.Receive(ctx, &signal)
			if signal.RunID == input.RunID && signal.PlanRevision == input.Plan.Revision && signal.ActionID != "" && !seenActions[signal.ActionID] {
				seenActions[signal.ActionID] = true
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
		selector.Select(ctx)
		if cancelled {
			return cancelledResult(ctx, input, result.CompletedSlotIDs)
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
		return result, nil
	}
}

type childCompletion struct {
	Index  int
	Result SlotWorkflowResult
	Failed bool
}

func executeInitialSlots(ctx workflow.Context, input WorkflowInput, limit int, cancelChannel workflow.ReceiveChannel, seenActions map[string]bool) ([]SlotWorkflowResult, bool, error) {
	results := make([]SlotWorkflowResult, len(input.Plan.Slots))
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
		selector.Select(ctx)
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
	for {
		var signal CancelSignal
		if !channel.ReceiveAsync(&signal) {
			return false
		}
		if validCancel(input, signal, seenActions) {
			return true
		}
	}
}

func validCancel(input WorkflowInput, signal CancelSignal, seenActions map[string]bool) bool {
	if signal.RunID != input.RunID || signal.PlanRevision != input.Plan.Revision || signal.ActionID == "" || seenActions[signal.ActionID] {
		return false
	}
	seenActions[signal.ActionID] = true
	return true
}

func validRetry(input WorkflowInput, signal RetrySlotSignal, seenActions map[string]bool) bool {
	if signal.RunID != input.RunID || signal.PlanRevision != input.Plan.Revision || signal.SlotID == "" || signal.ActionID == "" || seenActions[signal.ActionID] {
		return false
	}
	seenActions[signal.ActionID] = true
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

func cancelledResult(ctx workflow.Context, input WorkflowInput, completed []string) (WorkflowResult, error) {
	if err := persistRunState(ctx, input, imageagent.RunStatusCancelled, "cancelled", nil); err != nil {
		return WorkflowResult{}, err
	}
	return WorkflowResult{Status: imageagent.RunStatusCancelled, CompletedSlotIDs: completed}, nil
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

func summarizeResults(plan imageagent.Plan, results []SlotWorkflowResult) WorkflowResult {
	result := WorkflowResult{Status: imageagent.RunStatusAwaitingFinalApproval}
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

func candidateAssetIDs(plan imageagent.Plan, results []SlotWorkflowResult) []string {
	seen := map[string]bool{}
	var ids []string
	for index := range plan.Slots {
		for _, candidate := range results[index].Execution.Candidates {
			if candidate.AssetID == "" || seen[candidate.AssetID] {
				continue
			}
			seen[candidate.AssetID] = true
			ids = append(ids, candidate.AssetID)
		}
	}
	return ids
}

func findSlot(plan imageagent.Plan, slotID string) imageagent.Slot {
	for _, slot := range plan.Slots {
		if slot.ID == slotID {
			return slot
		}
	}
	return imageagent.Slot{ID: slotID, IdempotencyKey: "unknown-slot"}
}
