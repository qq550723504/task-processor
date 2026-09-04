package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"task-processor/internal/listingkit/core"
	"time"

	"github.com/google/uuid"
)

var sdsChildRetryDelays = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute}

func (s *service) RunDueSDSChildRetries(ctx context.Context, now time.Time, limit int) (int64, error) {
	repo, ok := s.repo.(SDSChildRetryJobRepository)
	if !ok {
		return 0, nil
	}
	leaseUntil := now.UTC().Add(10 * time.Minute)
	jobs, err := repo.ClaimDueSDSChildRetries(ctx, now.UTC(), limit, uuid.NewString(), leaseUntil)
	if err != nil {
		return 0, err
	}
	for i := range jobs {
		job := &jobs[i]
		if err := s.runSDSChildRetry(ctx, job); err != nil {
			return int64(i), err
		}
		job.LeaseOwner = ""
		job.LeaseUntil = nil
		if err := repo.SaveSDSChildRetry(ctx, job); err != nil {
			return int64(i), err
		}
	}
	return int64(len(jobs)), nil
}

// ScheduleTaskChildRetry validates a user-requested retry and persists it for
// the existing SDS retry sweep. It deliberately does not execute remote SDS or
// SHEIN work on the HTTP request goroutine.
func (s *service) ScheduleTaskChildRetry(ctx context.Context, taskID string, req *RetryChildTaskRequest) (*TaskChildRetryAccepted, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || req == nil || strings.TrimSpace(req.Kind) == "" {
		return nil, core.ErrChildTaskRetryInvalidRequest
	}
	if s == nil || s.repo == nil {
		return nil, core.ErrTaskNotFound
	}
	kind := strings.TrimSpace(req.Kind)
	if !childTaskRetrySupportedKind(kind) {
		return nil, core.ErrChildTaskNotRetryable
	}

	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, core.ErrTaskNotFound
	}
	if task.Status == core.TaskStatusPending || task.Status == core.TaskStatusProcessing {
		return nil, core.ErrChildTaskRetryConflict
	}
	if task.Result == nil {
		return nil, ErrTaskResultUnavailable
	}
	if task.Request == nil {
		return nil, ErrTaskResultUnavailable
	}
	state, ok := childTaskStateByKind(task.Result, kind)
	if !ok {
		return nil, core.ErrChildTaskNotFound
	}
	if state.Status != string(core.TaskStatusFailed) && state.Status != string(core.TaskStatusCompleted) {
		return nil, core.ErrChildTaskNotRetryable
	}

	repo, ok := s.repo.(SDSChildRetryJobRepository)
	if !ok {
		return nil, fmt.Errorf("SDS child retry repository is not configured")
	}
	_, err = repo.ScheduleSDSChildRetry(ctx, &SDSChildRetryJob{
		TenantID:    task.TenantID,
		TaskID:      task.ID,
		StoreID:     task.Request.SheinStoreID,
		Kind:        SDSChildRetryKind(kind),
		NextRetryAt: time.Now().UTC(),
		ReasonCode:  "manual_child_task_retry",
		LastError:   "manual child task retry queued",
		Status:      SDSChildRetryJobStatusPending,
	})
	if err != nil {
		return nil, err
	}
	return &TaskChildRetryAccepted{TaskID: task.ID, Kind: kind, Status: "queued"}, nil
}

func (s *service) runSDSChildRetry(ctx context.Context, job *SDSChildRetryJob) error {
	ctx = WithTenantID(ctx, job.TenantID)
	result, err := s.RetryTaskChildTask(ctx, job.TaskID, &RetryChildTaskRequest{Kind: string(job.Kind)})
	if err == nil && (result == nil || result.Result == nil || childTaskHasFailed(result.Result, string(job.Kind))) {
		err = childTaskRetryFailure(result, string(job.Kind))
	}
	if err == nil {
		job.Status = SDSChildRetryJobStatusCompleted
		job.LastError = ""
		return nil
	}
	job.LastError = err.Error()
	job.Attempt++
	if job.Attempt >= len(sdsChildRetryDelays) {
		job.Status = SDSChildRetryJobStatusExhausted
		return nil
	}
	job.NextRetryAt = time.Now().UTC().Add(sdsChildRetryDelays[job.Attempt])
	return nil
}

func childTaskRetryFailure(result *TaskResult, kind string) error {
	if result != nil && result.Result != nil {
		if state, ok := childTaskStateByKind(result.Result, kind); ok && strings.TrimSpace(state.Error) != "" {
			return errors.New(strings.TrimSpace(state.Error))
		}
	}
	return fmt.Errorf("SDS child retry did not complete")
}

func (s *service) ScheduleSDSChildRetry(ctx context.Context, task *Task, reasonCode string, cause error) error {
	if task == nil || task.Request == nil {
		return fmt.Errorf("task is required")
	}
	repo, ok := s.repo.(SDSChildRetryJobRepository)
	if !ok {
		return nil
	}
	_, err := repo.ScheduleSDSChildRetry(ctx, &SDSChildRetryJob{
		TenantID: task.TenantID, TaskID: task.ID, StoreID: task.Request.SheinStoreID,
		Kind: SDSChildRetryKindDesignSync, Attempt: 0, NextRetryAt: time.Now().UTC().Add(sdsChildRetryDelays[0]),
		ReasonCode: reasonCode, LastError: cause.Error(), Status: SDSChildRetryJobStatusPending,
	})
	return err
}
