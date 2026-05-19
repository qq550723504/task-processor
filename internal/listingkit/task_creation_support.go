package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *service) prepareGenerateTask(ctx context.Context, req *GenerateRequest) (context.Context, *Task, error) {
	if req == nil {
		return ctx, nil, fmt.Errorf("request cannot be nil")
	}
	if req.TenantID == "" {
		req.TenantID = TenantIDFromContext(ctx)
	}
	ctx = WithTenantID(ctx, req.TenantID)
	applyGenerateRequestDefaults(req, s.requestDefaults)
	if err := validateRequest(req); err != nil {
		return ctx, nil, fmt.Errorf("invalid request: %w", err)
	}

	task := &Task{
		ID:         uuid.New().String(),
		TenantID:   TenantIDFromContext(ctx),
		UserID:     strings.TrimSpace(req.UserID),
		Request:    req,
		Status:     TaskStatusPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		RetryCount: 0,
	}
	s.applySheinStoreResolutionSnapshot(ctx, task)
	return ctx, task, nil
}

func (s *service) applySheinStoreResolutionSnapshot(ctx context.Context, task *Task) {
	if task == nil || !taskHasPlatform(task, "shein") {
		return
	}
	if selection, err := s.resolveSheinStoreSelection(ctx, task); err == nil && selection != nil {
		task.SheinStoreResolutionSnapshot = sheinStoreResolutionSnapshotFromSelection(selection, task, nil)
	}
}

func (s *service) dispatchGenerateTask(ctx context.Context, task *Task) (*Task, error) {
	if task == nil {
		return nil, nil
	}
	if s.taskSubmitter == nil {
		return s.runGenerateTaskInline(ctx, task)
	}
	if shouldRunStudioInline(task.Request) {
		return s.dispatchStudioTask(ctx, task)
	}
	if err := s.enqueueGenerateTask(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *service) dispatchStudioTask(ctx context.Context, task *Task) (*Task, error) {
	if s.taskSubmitter != nil {
		if err := s.enqueueGenerateTask(ctx, task); err != nil {
			return nil, err
		}
		return task, nil
	}
	return s.runGenerateTaskInline(ctx, task)
}

func (s *service) runGenerateTaskInline(ctx context.Context, task *Task) (*Task, error) {
	runCtx := context.WithoutCancel(ctx)
	if _, err := s.ProcessListingKit(runCtx, task); err != nil {
		return s.refreshGenerateTask(runCtx, task)
	}
	return s.refreshGenerateTask(runCtx, task)
}

func (s *service) refreshGenerateTask(ctx context.Context, task *Task) (*Task, error) {
	if task == nil {
		return nil, nil
	}
	refreshed, err := s.repo.GetTask(ctx, task.ID)
	if err == nil {
		return refreshed, nil
	}
	return task, nil
}

func (s *service) enqueueGenerateTask(ctx context.Context, task *Task) error {
	if s.standardProductWorkflowEnabled && s.standardProductWorkflowClient != nil {
		if err := s.standardProductWorkflowClient.StartStandardProduct(ctx, StandardProductWorkflowStartInput{
			TaskID:      strings.TrimSpace(task.ID),
			RequestedAt: time.Now().UTC(),
		}); err != nil {
			_ = s.repo.MarkFailed(ctx, task.ID, fmt.Sprintf("failed to start standard product workflow: %v", err))
			return fmt.Errorf("failed to start standard product workflow: %w", err)
		}
		return nil
	}
	if s.taskSubmitter == nil {
		return nil
	}
	if err := s.taskSubmitter.Submit(task.ID); err != nil {
		_ = s.repo.MarkFailed(ctx, task.ID, fmt.Sprintf("failed to submit task: %v", err))
		return fmt.Errorf("failed to submit task: %w", err)
	}
	return nil
}
