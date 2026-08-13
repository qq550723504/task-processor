package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
	"task-processor/internal/shared/tenantctx"
)

// The in-memory implementation keeps local and fallback deployments on the
// same retry path as the GORM repository.
func (r *MemTaskRepository) ensureSDSChildRetryJobsLocked() map[string]listingkit.SDSChildRetryJob {
	if r.sdsChildRetryJobs == nil {
		r.sdsChildRetryJobs = make(map[string]listingkit.SDSChildRetryJob)
	}
	return r.sdsChildRetryJobs
}

func (r *MemTaskRepository) ScheduleSDSChildRetry(_ context.Context, job *listingkit.SDSChildRetryJob) (*listingkit.SDSChildRetryJob, error) {
	if job == nil || strings.TrimSpace(job.TaskID) == "" || job.Kind == "" {
		return nil, fmt.Errorf("SDS child retry job requires task ID and kind")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	jobs := r.ensureSDSChildRetryJobsLocked()
	now := time.Now().UTC()
	for _, existing := range jobs {
		if existing.TaskID == job.TaskID && existing.Status == listingkit.SDSChildRetryJobStatusRepairing && existing.LeaseUntil != nil && existing.LeaseUntil.After(now) {
			return nil, listingkit.ErrSDSRepairRetryInProgress
		}
	}
	for _, existing := range jobs {
		if existing.TaskID == job.TaskID && existing.Kind == job.Kind {
			if existing.Status == listingkit.SDSChildRetryJobStatusCompleted || existing.Status == listingkit.SDSChildRetryJobStatusExhausted || existing.Status == listingkit.SDSChildRetryJobStatusCancelled || existing.Status == listingkit.SDSChildRetryJobStatusRepairing {
				existing.Attempt = job.Attempt
				existing.NextRetryAt = job.NextRetryAt
				existing.ReasonCode = job.ReasonCode
				existing.LastError = job.LastError
				existing.Status = listingkit.SDSChildRetryJobStatusPending
				existing.LeaseOwner = ""
				existing.LeaseUntil = nil
				r.sdsChildRetryJobs[existing.ID] = existing
			}
			copy := existing
			return &copy, nil
		}
	}
	copy := *job
	if copy.ID == "" {
		copy.ID = uuid.NewString()
	}
	if copy.Status == "" {
		copy.Status = listingkit.SDSChildRetryJobStatusPending
	}
	jobs[copy.ID] = copy
	return &copy, nil
}

func (r *MemTaskRepository) ListDueSDSChildRetries(ctx context.Context, dueBefore time.Time, limit int) ([]listingkit.SDSChildRetryJob, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	jobs := make([]listingkit.SDSChildRetryJob, 0)
	for _, job := range r.sdsChildRetryJobs {
		if job.Status != listingkit.SDSChildRetryJobStatusPending || job.NextRetryAt.After(dueBefore) || !matchesTenantScope(ctx, job.TenantID) {
			continue
		}
		jobs = append(jobs, job)
	}
	sortSDSChildRetryJobs(jobs)
	if limit > 0 && len(jobs) > limit {
		jobs = jobs[:limit]
	}
	return jobs, nil
}

func (r *MemTaskRepository) ListSDSChildRetries(ctx context.Context, taskID string) ([]listingkit.SDSChildRetryJob, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	jobs := make([]listingkit.SDSChildRetryJob, 0)
	for _, job := range r.ensureSDSChildRetryJobsLocked() {
		if job.TaskID == taskID && matchesTenantScope(ctx, job.TenantID) {
			jobs = append(jobs, job)
		}
	}
	sort.SliceStable(jobs, func(i, j int) bool {
		if jobs[i].UpdatedAt.Equal(jobs[j].UpdatedAt) {
			return jobs[i].ID < jobs[j].ID
		}
		return jobs[i].UpdatedAt.After(jobs[j].UpdatedAt)
	})
	return jobs, nil
}

func (r *MemTaskRepository) BeginSDSChildRetryRepair(ctx context.Context, taskID string, kind listingkit.SDSChildRetryKind) (*listingkit.SDSChildRetryRepairLease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	jobs := r.ensureSDSChildRetryJobsLocked()
	for _, job := range jobs {
		if job.TaskID != taskID {
			continue
		}
		if (job.Status == listingkit.SDSChildRetryJobStatusPending || job.Status == listingkit.SDSChildRetryJobStatusRepairing) && job.LeaseUntil != nil && job.LeaseUntil.After(now) {
			return nil, listingkit.ErrSDSRepairRetryInProgress
		}
	}
	for id, job := range jobs {
		if job.TaskID != taskID || job.Kind != kind {
			continue
		}
		if job.Status == listingkit.SDSChildRetryJobStatusPending || job.Status == listingkit.SDSChildRetryJobStatusExhausted {
			job.Status = listingkit.SDSChildRetryJobStatusCancelled
			job.LeaseOwner = ""
			job.LeaseUntil = nil
			r.sdsChildRetryJobs[id] = job
		}
	}
	owner := uuid.NewString()
	leaseUntil := now.Add(30 * time.Minute)
	var marker *listingkit.SDSChildRetryJob
	for id, job := range jobs {
		if job.TaskID == taskID && job.Kind == kind {
			job.Status = listingkit.SDSChildRetryJobStatusRepairing
			job.LeaseOwner = owner
			job.LeaseUntil = &leaseUntil
			job.ReasonCode = "sds_repair_in_progress"
			job.LastError = ""
			jobs[id] = job
			copy := job
			marker = &copy
			break
		}
	}
	if marker == nil {
		job := listingkit.SDSChildRetryJob{
			ID: uuid.NewString(), TenantID: tenantctx.TenantIDFromContext(ctx), TaskID: taskID,
			Kind: kind, NextRetryAt: now, ReasonCode: "sds_repair_in_progress",
			Status: listingkit.SDSChildRetryJobStatusRepairing, LeaseOwner: owner, LeaseUntil: &leaseUntil,
		}
		jobs[job.ID] = job
		marker = &job
	}
	return &listingkit.SDSChildRetryRepairLease{JobID: marker.ID, Owner: owner}, nil
}

func (r *MemTaskRepository) EndSDSChildRetryRepair(_ context.Context, lease *listingkit.SDSChildRetryRepairLease) error {
	if lease == nil || strings.TrimSpace(lease.JobID) == "" || strings.TrimSpace(lease.Owner) == "" {
		return fmt.Errorf("SDS repair lease is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.ensureSDSChildRetryJobsLocked()[lease.JobID]
	if !ok || job.Status != listingkit.SDSChildRetryJobStatusRepairing || job.LeaseOwner != lease.Owner {
		return nil
	}
	job.Status = listingkit.SDSChildRetryJobStatusCancelled
	job.LeaseOwner = ""
	job.LeaseUntil = nil
	r.sdsChildRetryJobs[job.ID] = job
	return nil
}

func (r *MemTaskRepository) ClaimDueSDSChildRetries(ctx context.Context, dueBefore time.Time, limit int, owner string, leaseUntil time.Time) ([]listingkit.SDSChildRetryJob, error) {
	if strings.TrimSpace(owner) == "" {
		return nil, fmt.Errorf("SDS child retry lease owner is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	jobs := make([]listingkit.SDSChildRetryJob, 0)
	claimedTaskIDs := make(map[string]struct{})
	for _, job := range r.ensureSDSChildRetryJobsLocked() {
		if job.Status != listingkit.SDSChildRetryJobStatusPending || job.NextRetryAt.After(dueBefore) || (job.LeaseUntil != nil && job.LeaseUntil.After(dueBefore)) || !matchesTenantScope(ctx, job.TenantID) {
			continue
		}
		if _, claimed := claimedTaskIDs[job.TaskID]; claimed {
			continue
		}
		blocked := false
		for _, sibling := range r.sdsChildRetryJobs {
			if sibling.TaskID == job.TaskID && sibling.Status == listingkit.SDSChildRetryJobStatusRepairing && sibling.LeaseUntil != nil && sibling.LeaseUntil.After(dueBefore) {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		jobs = append(jobs, job)
		claimedTaskIDs[job.TaskID] = struct{}{}
	}
	sortSDSChildRetryJobs(jobs)
	if limit > 0 && len(jobs) > limit {
		jobs = jobs[:limit]
	}
	for index := range jobs {
		jobs[index].LeaseOwner = owner
		lease := leaseUntil
		jobs[index].LeaseUntil = &lease
		r.sdsChildRetryJobs[jobs[index].ID] = jobs[index]
	}
	return jobs, nil
}

func (r *MemTaskRepository) SaveSDSChildRetry(ctx context.Context, job *listingkit.SDSChildRetryJob) error {
	if job == nil || strings.TrimSpace(job.ID) == "" {
		return fmt.Errorf("SDS child retry job is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.ensureSDSChildRetryJobsLocked()[job.ID]; !ok || !matchesTenantScope(ctx, job.TenantID) {
		return core.ErrTaskNotFound
	}
	copy := *job
	r.sdsChildRetryJobs[job.ID] = copy
	return nil
}

func sortSDSChildRetryJobs(jobs []listingkit.SDSChildRetryJob) {
	sort.SliceStable(jobs, func(i, j int) bool {
		if jobs[i].NextRetryAt.Equal(jobs[j].NextRetryAt) {
			return jobs[i].ID < jobs[j].ID
		}
		return jobs[i].NextRetryAt.Before(jobs[j].NextRetryAt)
	})
}
