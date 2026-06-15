package listingkit

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"task-processor/internal/infra/worker"
	listingsubmission "task-processor/internal/listing/submission"
)

func buildTaskListSummary(tasks []Task) *TaskListSummary {
	if len(tasks) == 0 {
		return nil
	}
	summary := &TaskListSummary{
		StatusCounts:              make(map[string]int),
		SheinWorkflowStatusCounts: make(map[string]int),
		SheinWorkQueueCounts:      make(map[string]int),
		SheinActionQueueCounts:    make(map[string]int),
		SheinBlockerCounts:        make(map[string]int),
		SheinWarningCounts:        make(map[string]int),
	}
	for i := range tasks {
		item := buildTaskListItem(&tasks[i])
		incrementTaskListSummary(summary, item)
	}
	return pruneEmptyTaskListSummary(summary)
}

func incrementTaskListSummary(summary *TaskListSummary, item TaskListItem) {
	if summary == nil {
		return
	}
	if item.Status != "" {
		summary.StatusCounts[string(item.Status)]++
	}
	if item.SheinWorkflowStatus != "" {
		summary.SheinWorkflowStatusCounts[item.SheinWorkflowStatus]++
	}
	if item.SheinWorkQueue != "" {
		summary.SheinWorkQueueCounts[item.SheinWorkQueue]++
	}
	if item.SheinActionQueue != "" {
		summary.SheinActionQueueCounts[item.SheinActionQueue]++
	}
	for _, key := range item.SheinBlockingKeys {
		if key != "" {
			summary.SheinBlockerCounts[key]++
		}
	}
	for _, key := range item.SheinWarningKeys {
		if key != "" {
			summary.SheinWarningCounts[key]++
		}
	}
}

func pruneEmptyTaskListSummary(summary *TaskListSummary) *TaskListSummary {
	if summary == nil {
		return nil
	}
	if len(summary.StatusCounts) == 0 {
		summary.StatusCounts = nil
	}
	if len(summary.SheinWorkflowStatusCounts) == 0 {
		summary.SheinWorkflowStatusCounts = nil
	}
	if len(summary.SheinWorkQueueCounts) == 0 {
		summary.SheinWorkQueueCounts = nil
	}
	if len(summary.SheinActionQueueCounts) == 0 {
		summary.SheinActionQueueCounts = nil
	}
	if len(summary.SheinBlockerCounts) == 0 {
		summary.SheinBlockerCounts = nil
	}
	if len(summary.SheinWarningCounts) == 0 {
		summary.SheinWarningCounts = nil
	}
	if summary.StatusCounts == nil &&
		summary.SheinWorkflowStatusCounts == nil &&
		summary.SheinWorkQueueCounts == nil &&
		summary.SheinActionQueueCounts == nil &&
		summary.SheinBlockerCounts == nil &&
		summary.SheinWarningCounts == nil {
		return nil
	}
	return summary
}

func (s *taskLifecycleService) enqueueOrRunStudioTask(ctx context.Context, task *Task) (*Task, error) {
	return s.dispatchStudioTask(ctx, task)
}

func (s *taskLifecycleService) runTaskInline(ctx context.Context, task *Task) (*Task, error) {
	return s.runGenerateTaskInline(ctx, task)
}

func (s *taskLifecycleService) enqueueTask(ctx context.Context, task *Task) error {
	return s.enqueueGenerateTask(ctx, task)
}

func (s *taskLifecycleService) prepareGenerateTask(ctx context.Context, req *GenerateRequest) (context.Context, *Task, error) {
	if req == nil {
		return ctx, nil, fmt.Errorf("request cannot be nil")
	}
	if req.TenantID == "" {
		req.TenantID = TenantIDFromContext(ctx)
	}
	ctx = WithTenantID(ctx, req.TenantID)
	if s.requestDefaults != nil {
		applyGenerateRequestDefaults(req, s.requestDefaults())
	}
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

func (s *taskLifecycleService) applySheinStoreResolutionSnapshot(ctx context.Context, task *Task) {
	if task == nil || !taskHasPlatform(task, "shein") || s.resolveStoreSelection == nil {
		return
	}
	if selection, err := s.resolveStoreSelection(ctx, task); err == nil && selection != nil {
		task.SheinStoreResolutionSnapshot = sheinStoreResolutionSnapshotFromSelection(selection, task, nil)
	}
}

func (s *taskLifecycleService) dispatchGenerateTask(ctx context.Context, task *Task) (*Task, error) {
	if task == nil {
		return nil, nil
	}
	if s.taskSubmitter == nil || s.taskSubmitter() == nil {
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

func (s *taskLifecycleService) dispatchStudioTask(ctx context.Context, task *Task) (*Task, error) {
	if s.taskSubmitter != nil && s.taskSubmitter() != nil {
		if err := s.enqueueGenerateTask(ctx, task); err != nil {
			return nil, err
		}
		return task, nil
	}
	return s.runGenerateTaskInline(ctx, task)
}

func (s *taskLifecycleService) runGenerateTaskInline(ctx context.Context, task *Task) (*Task, error) {
	runCtx := context.WithoutCancel(ctx)
	if s.processListingKit != nil {
		if _, err := s.processListingKit(runCtx, task); err != nil {
			return s.refreshGenerateTask(runCtx, task)
		}
	}
	return s.refreshGenerateTask(runCtx, task)
}

func (s *taskLifecycleService) refreshGenerateTask(ctx context.Context, task *Task) (*Task, error) {
	if task == nil {
		return nil, nil
	}
	refreshed, err := s.repo.GetTask(ctx, task.ID)
	if err == nil {
		return refreshed, nil
	}
	return task, nil
}

func (s *taskLifecycleService) enqueueGenerateTask(ctx context.Context, task *Task) error {
	if s.standardWorkflow != nil {
		if client, enabled := s.standardWorkflow(); enabled && client != nil {
			if err := client.StartStandardProduct(ctx, StandardProductWorkflowStartInput{
				TaskID:      strings.TrimSpace(task.ID),
				RequestedAt: time.Now().UTC(),
			}); err != nil {
				if persistErr := s.persistEnqueueFailure(ctx, task.ID, fmt.Sprintf("failed to start standard product workflow: %v", err), err); persistErr != nil {
					return errors.Join(fmt.Errorf("failed to start standard product workflow: %w", err), fmt.Errorf("failed to persist task failure: %w", persistErr))
				}
				return fmt.Errorf("failed to start standard product workflow: %w", err)
			}
			return nil
		}
	}
	if s.taskSubmitter == nil || s.taskSubmitter() == nil {
		return nil
	}
	if err := s.taskSubmitter().Submit(task.ID); err != nil {
		if errors.Is(err, worker.ErrQueueFull) {
			s.scheduleAsyncEnqueueRetry(DetachedRequestContext(ctx), task.ID)
			return nil
		}
		if persistErr := s.persistEnqueueFailure(ctx, task.ID, fmt.Sprintf("failed to submit task: %v", err), err); persistErr != nil {
			return errors.Join(fmt.Errorf("failed to submit task: %w", err), fmt.Errorf("failed to persist task failure: %w", persistErr))
		}
		return fmt.Errorf("failed to submit task: %w", err)
	}
	return nil
}

func (s *taskLifecycleService) scheduleAsyncEnqueueRetry(ctx context.Context, taskID string) {
	if strings.TrimSpace(taskID) == "" || s.taskSubmitter == nil {
		return
	}
	submitter := s.taskSubmitter()
	if submitter == nil {
		return
	}

	go func() {
		delay := listingsubmission.EnqueueRetryDelay
		for {
			time.Sleep(delay)
			if err := submitter.Submit(taskID); err != nil {
				if errors.Is(err, worker.ErrQueueFull) {
					delay = listingsubmission.NextEnqueueRetryDelay(delay)
					continue
				}
				_ = s.persistEnqueueFailure(ctx, taskID, fmt.Sprintf("failed to submit task: %v", err), err)
				return
			}
			return
		}
	}()
}

func (s *taskLifecycleService) persistEnqueueFailure(ctx context.Context, taskID string, errorMsg string, cause error) error {
	return persistClassifiedTaskFailure(ctx, s.repo, taskID, errorMsg, cause)
}

func validateRequest(req *GenerateRequest) error {
	if len(req.ImageURLs) == 0 && strings.TrimSpace(req.Text) == "" && strings.TrimSpace(req.ProductURL) == "" {
		return fmt.Errorf("at least one of image_urls, text, or product_url must be provided")
	}
	if len(req.ImageURLs) > 10 {
		return fmt.Errorf("too many image URLs (max 10)")
	}
	if len(req.Platforms) == 0 {
		return fmt.Errorf("at least one platform is required")
	}
	if err := validateSheinStudioAspectRatio(req); err != nil {
		return err
	}
	return nil
}

func validateSheinStudioAspectRatio(req *GenerateRequest) error {
	if req == nil || req.Options == nil || req.Options.SheinStudio == nil || req.Options.SDS == nil {
		return nil
	}
	studio := req.Options.SheinStudio
	sds := req.Options.SDS
	if studio.SourceDesignWidth <= 0 || studio.SourceDesignHeight <= 0 || sds.PrintableWidth <= 0 || sds.PrintableHeight <= 0 {
		return nil
	}
	sourceRatio := float64(studio.SourceDesignWidth) / float64(studio.SourceDesignHeight)
	targetRatio := float64(sds.PrintableWidth) / float64(sds.PrintableHeight)
	if math.Abs(sourceRatio-targetRatio)/targetRatio > 0.25 {
		return fmt.Errorf("shein studio source image ratio differs too much from SDS printable area ratio")
	}
	return nil
}
