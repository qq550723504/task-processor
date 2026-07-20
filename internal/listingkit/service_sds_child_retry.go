package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

func (s *service) runSDSChildRetry(ctx context.Context, job *SDSChildRetryJob) error {
	result, err := s.RetryTaskChildTask(ctx, job.TaskID, &RetryChildTaskRequest{Kind: string(job.Kind)})
	if err == nil && (result == nil || result.Result == nil || childTaskHasFailed(result.Result, string(job.Kind))) {
		err = fmt.Errorf("SDS child retry did not complete")
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

// ScheduleStudioBatchSDSChildRetries queues failed SDS child tasks already
// created by a Studio batch. It deliberately does not recreate designs or
// ListingKit tasks, and is therefore safe for batches whose generation stage
// has completed.
func (s *service) ScheduleStudioBatchSDSChildRetries(ctx context.Context, batchID string) (*StudioBatchSDSChildRetryResult, error) {
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return nil, NewStudioBatchActionValidationError("batch ID is required")
	}
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("listingkit task repository is not configured")
	}
	repo, ok := s.repo.(SDSChildRetryJobRepository)
	if !ok {
		return nil, fmt.Errorf("SDS child retry repository is not configured")
	}
	linkRepo := resolveStudioBatchTaskLinkRepo(s)
	if linkRepo == nil {
		return nil, fmt.Errorf("studio batch task link repository is not configured")
	}
	links, err := linkRepo.ListStudioBatchTaskLinksByBatchID(ctx, batchID)
	if err != nil {
		return nil, err
	}
	result := &StudioBatchSDSChildRetryResult{BatchID: batchID}
	for _, link := range links {
		taskID := strings.TrimSpace(link.ListingKitTaskID)
		if taskID == "" {
			result.Skipped++
			continue
		}
		task, err := s.repo.GetTask(ctx, taskID)
		if err != nil {
			result.Failures = append(result.Failures, StudioBatchSDSChildRetryFail{TaskID: taskID, Message: err.Error()})
			continue
		}
		if task == nil || task.Result == nil {
			result.Skipped++
			continue
		}
		state, found := childTaskStateByKind(task.Result, string(SDSChildRetryKindDesignSync))
		if !found || state.Status != string(TaskStatusFailed) {
			result.Skipped++
			continue
		}
		storeID := link.SheinStoreID
		if task.Request != nil && task.Request.SheinStoreID > 0 {
			storeID = task.Request.SheinStoreID
		}
		_, err = repo.ScheduleSDSChildRetry(ctx, &SDSChildRetryJob{
			TenantID: task.TenantID, TaskID: task.ID, StoreID: storeID,
			Kind: SDSChildRetryKindDesignSync, NextRetryAt: time.Now().UTC(),
			ReasonCode: "manual_studio_batch_sds_retry", LastError: errors.New("manual SDS retry scheduled from Studio batch").Error(),
			Status: SDSChildRetryJobStatusPending,
		})
		if err != nil {
			result.Failures = append(result.Failures, StudioBatchSDSChildRetryFail{TaskID: taskID, Message: err.Error()})
			continue
		}
		result.Scheduled++
	}
	return result, nil
}
