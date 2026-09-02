package listingkit

import (
	"context"
	"strings"
	"time"

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
		if state != nil && state.result != nil {
			state.result.Status = string(core.TaskStatusProcessing)
			_ = s.repo.SaveTaskResult(ctx, task.ID, mergeStandardProductLayerResult(task.Result, state.result))
		}
		_ = s.repo.MarkFailed(ctx, task.ID, err.Error())
		return nil, err
	}
	state.result.Status = string(core.TaskStatusProcessing)
	if err := s.repo.SaveTaskResult(ctx, task.ID, mergeStandardProductLayerResult(task.Result, state.result)); err != nil {
		return nil, err
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
	result := s.runPlatformAdaptation(ctx, task, snapshot)
	if err := s.persistProcessedTaskResult(ctx, task.ID, result); err != nil {
		return nil, err
	}
	return result, nil
}
