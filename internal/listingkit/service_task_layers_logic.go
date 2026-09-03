package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
	"task-processor/internal/listingkit/core"
)

func (s *service) ProcessStandardProductLayer(ctx context.Context, taskID string) (*StandardProductSnapshot, error) {
	ctx, task, err := s.loadTaskExecutionContext(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if err := s.markTaskProcessingIfPending(ctx, task); err != nil {
		return nil, err
	}
	state, err := s.runStandardProductWorkflow(ctx, task)
	if err != nil {
		var saveErr error
		if state != nil && state.result != nil {
			state.result.Status = string(core.TaskStatusProcessing)
			saveErr = s.repo.SaveTaskResult(ctx, task.ID, mergeStandardProductLayerResult(task.Result, state.result))
		}
		return nil, errors.Join(err, saveErr)
	}
	state.result.Status = string(core.TaskStatusProcessing)
	if err := s.repo.SaveTaskResult(ctx, task.ID, mergeStandardProductLayerResult(task.Result, state.result)); err != nil {
		return nil, err
	}
	if state.blocked {
		now := time.Now().UTC()
		classified := &submissiondomain.RetryableBlockState{
			ReasonCode:           standardProductReadinessBlockReason,
			ReasonMessage:        standardProductReadinessBlockMessage,
			MaxAutoRetryAttempts: 8,
			RecoveryScope:        submissiondomain.RetryableRecoveryScopeTask,
			AutoResumeEnabled:    true,
		}
		block := adaptSubmissionRetryableBlock(submissiondomain.BuildReblockedRetryableBlock(
			adaptRetryableBlockState(task.RetryableBlock),
			classified,
			now,
			submissiondomain.RetryableRecoveryScopeTask,
		))
		if block.MaxAutoRetryAttempts == 0 {
			block.MaxAutoRetryAttempts = classified.MaxAutoRetryAttempts
		}
		block.AutoResumeEnabled = true
		if err := s.repo.MarkBlockedRetryable(ctx, task.ID, block, standardProductReadinessBlockMessage); err != nil {
			return nil, err
		}
		return state.snapshot, nil
	}
	if client, enabled := resolvePlatformAdaptWorkflowClient(s); enabled && client != nil {
		if err := client.StartPlatformAdaptation(ctx, PlatformAdaptWorkflowStartInput{
			TaskID:      strings.TrimSpace(task.ID),
			Platform:    "all",
			RequestedAt: time.Now().UTC(),
		}); err != nil {
			return nil, err
		}
	}
	return state.snapshot, nil
}

func (s *service) ProcessPlatformAdaptationLayer(ctx context.Context, taskID string, platform string) (*ListingKitResult, error) {
	ctx, task, err := s.loadTaskExecutionContext(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if err := s.markTaskProcessingIfPending(ctx, task); err != nil {
		return nil, err
	}
	snapshot, err := standardSnapshotFromTask(task)
	if err != nil {
		return nil, err
	}
	if normalized := strings.ToLower(strings.TrimSpace(platform)); normalized != "" && normalized != "all" {
		adaptationTask := *task
		adaptationRequest := *task.Request
		adaptationRequest.Platforms = []string{normalized}
		adaptationTask.Request = &adaptationRequest
		task = &adaptationTask
	}
	result, err := s.runPlatformAdaptation(ctx, task, snapshot)
	if err != nil {
		return nil, err
	}
	if err := s.persistProcessedTaskResult(ctx, task.ID, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *service) PersistLayerFailure(ctx context.Context, taskID string, errorMessage string) error {
	taskID = strings.TrimSpace(taskID)
	errorMessage = strings.TrimSpace(errorMessage)
	if taskID == "" {
		return fmt.Errorf("task ID is required")
	}
	if errorMessage == "" {
		return fmt.Errorf("failure message is required")
	}
	failureRepo, ok := s.repo.(ProcessingFailureRepository)
	if !ok {
		return fmt.Errorf("mark layer task failed: processing failure repository is required")
	}
	updated, err := failureRepo.MarkFailedIfProcessing(ctx, taskID, errorMessage)
	if err != nil {
		return fmt.Errorf("mark layer task failed: %w", err)
	}
	if updated {
		return nil
	}
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("load layer task after failure conflict: %w", err)
	}
	switch task.Status {
	case core.TaskStatusCompleted, core.TaskStatusNeedsReview, core.TaskStatusFailed:
		return nil
	default:
		return fmt.Errorf("mark layer task failed from status %q: %w", task.Status, core.ErrTaskNotRecoverable)
	}
}
