package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"task-processor/internal/listingkit"
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
	for _, existing := range jobs {
		if existing.TaskID == job.TaskID && existing.Kind == job.Kind {
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

func (r *MemTaskRepository) ClaimDueSDSChildRetries(ctx context.Context, dueBefore time.Time, limit int, owner string, leaseUntil time.Time) ([]listingkit.SDSChildRetryJob, error) {
	if strings.TrimSpace(owner) == "" {
		return nil, fmt.Errorf("SDS child retry lease owner is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	jobs := make([]listingkit.SDSChildRetryJob, 0)
	for _, job := range r.ensureSDSChildRetryJobsLocked() {
		if job.Status != listingkit.SDSChildRetryJobStatusPending || job.NextRetryAt.After(dueBefore) || (job.LeaseUntil != nil && job.LeaseUntil.After(dueBefore)) || !matchesTenantScope(ctx, job.TenantID) {
			continue
		}
		jobs = append(jobs, job)
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
		return listingkit.ErrTaskNotFound
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
