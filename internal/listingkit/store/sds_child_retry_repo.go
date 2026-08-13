package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/listingkit"
	"task-processor/internal/shared/tenantctx"
)

func (r *taskRepository) ScheduleSDSChildRetry(ctx context.Context, job *listingkit.SDSChildRetryJob) (*listingkit.SDSChildRetryJob, error) {
	if job == nil || strings.TrimSpace(job.TaskID) == "" || job.Kind == "" {
		return nil, fmt.Errorf("SDS child retry job requires task ID and kind")
	}
	copy := *job
	if copy.ID == "" {
		copy.ID = uuid.NewString()
	}
	if copy.Status == "" {
		copy.Status = listingkit.SDSChildRetryJobStatusPending
	}
	var scheduled *listingkit.SDSChildRetryJob
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockRetryTask(tx, ctx, copy.TaskID); err != nil {
			return err
		}
		var repairing int64
		if err := tx.Model(&listingkit.SDSChildRetryJob{}).
			Where("listingkit_task_id = ? AND status = ? AND lease_until > ?", copy.TaskID, listingkit.SDSChildRetryJobStatusRepairing, time.Now().UTC()).
			Count(&repairing).Error; err != nil {
			return err
		}
		if repairing > 0 {
			return listingkit.ErrSDSRepairRetryInProgress
		}
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "listingkit_task_id"}, {Name: "kind"}},
			DoNothing: true,
		}).Create(&copy)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			scheduled = &copy
			return nil
		}
		var existing listingkit.SDSChildRetryJob
		if err := tx.Where("listingkit_task_id = ? AND kind = ?", copy.TaskID, copy.Kind).First(&existing).Error; err != nil {
			return err
		}
		if existing.Status == listingkit.SDSChildRetryJobStatusCompleted || existing.Status == listingkit.SDSChildRetryJobStatusExhausted || existing.Status == listingkit.SDSChildRetryJobStatusCancelled || existing.Status == listingkit.SDSChildRetryJobStatusRepairing {
			result := tx.Model(&listingkit.SDSChildRetryJob{}).
				Where("id = ? AND status IN ?", existing.ID, []listingkit.SDSChildRetryJobStatus{
					listingkit.SDSChildRetryJobStatusCompleted,
					listingkit.SDSChildRetryJobStatusExhausted,
					listingkit.SDSChildRetryJobStatusCancelled,
					listingkit.SDSChildRetryJobStatusRepairing,
				}).Updates(map[string]any{
				"attempt":       copy.Attempt,
				"next_retry_at": copy.NextRetryAt,
				"reason_code":   copy.ReasonCode,
				"last_error":    copy.LastError,
				"status":        listingkit.SDSChildRetryJobStatusPending,
				"lease_owner":   "",
				"lease_until":   nil,
			})
			if result.Error != nil {
				return result.Error
			}
		}
		if err := tx.Where("id = ?", existing.ID).First(&existing).Error; err != nil {
			return err
		}
		scheduled = &existing
		return nil
	})
	return scheduled, err
}

func (r *taskRepository) ListDueSDSChildRetries(ctx context.Context, dueBefore time.Time, limit int) ([]listingkit.SDSChildRetryJob, error) {
	var jobs []listingkit.SDSChildRetryJob
	db := r.db.WithContext(ctx).Where("status = ? AND next_retry_at <= ?", listingkit.SDSChildRetryJobStatusPending, dueBefore).Order("next_retry_at ASC, id ASC")
	if limit > 0 {
		db = db.Limit(limit)
	}
	return jobs, db.Find(&jobs).Error
}

func (r *taskRepository) ListSDSChildRetries(ctx context.Context, taskID string) ([]listingkit.SDSChildRetryJob, error) {
	db := applyTenantScope(r.db.WithContext(ctx).Where("listingkit_task_id = ?", taskID), ctx, "tenant_id")
	var jobs []listingkit.SDSChildRetryJob
	return jobs, db.Order("updated_at DESC, id ASC").Find(&jobs).Error
}

func (r *taskRepository) BeginSDSChildRetryRepair(ctx context.Context, taskID string, kind listingkit.SDSChildRetryKind) (*listingkit.SDSChildRetryRepairLease, error) {
	now := time.Now().UTC()
	owner := uuid.NewString()
	leaseUntil := now.Add(30 * time.Minute)
	lease := &listingkit.SDSChildRetryRepairLease{Owner: owner}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockRetryTask(tx, ctx, taskID); err != nil {
			return err
		}
		var jobs []listingkit.SDSChildRetryJob
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("listingkit_task_id = ?", taskID).
			Find(&jobs).Error; err != nil {
			return err
		}
		for _, job := range jobs {
			if job.Status == listingkit.SDSChildRetryJobStatusPending && job.LeaseUntil != nil && job.LeaseUntil.After(now) {
				return listingkit.ErrSDSRepairRetryInProgress
			}
			if job.Status == listingkit.SDSChildRetryJobStatusRepairing && job.LeaseUntil != nil && job.LeaseUntil.After(now) {
				return listingkit.ErrSDSRepairRetryInProgress
			}
		}
		for _, job := range jobs {
			if job.Status != listingkit.SDSChildRetryJobStatusPending && job.Status != listingkit.SDSChildRetryJobStatusExhausted {
				continue
			}
			if err := tx.Model(&listingkit.SDSChildRetryJob{}).Where("id = ?", job.ID).Updates(map[string]any{
				"status":      listingkit.SDSChildRetryJobStatusCancelled,
				"lease_owner": "",
				"lease_until": nil,
			}).Error; err != nil {
				return err
			}
		}
		var marker listingkit.SDSChildRetryJob
		markerFound := false
		for _, job := range jobs {
			if job.Kind == kind {
				marker = job
				markerFound = true
				break
			}
		}
		if !markerFound {
			marker = listingkit.SDSChildRetryJob{
				ID: owner, TenantID: tenantctx.TenantIDFromContext(ctx), TaskID: taskID,
				Kind: kind, NextRetryAt: now, ReasonCode: "sds_repair_in_progress",
				Status:     listingkit.SDSChildRetryJobStatusRepairing,
				LeaseOwner: owner, LeaseUntil: &leaseUntil,
			}
			if err := tx.Create(&marker).Error; err != nil {
				return err
			}
		} else if err := tx.Model(&listingkit.SDSChildRetryJob{}).Where("id = ?", marker.ID).Updates(map[string]any{
			"status":      listingkit.SDSChildRetryJobStatusRepairing,
			"lease_owner": owner,
			"lease_until": leaseUntil,
			"reason_code": "sds_repair_in_progress",
			"last_error":  "",
		}).Error; err != nil {
			return err
		}
		lease.JobID = marker.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return lease, nil
}

func (r *taskRepository) EndSDSChildRetryRepair(ctx context.Context, lease *listingkit.SDSChildRetryRepairLease) error {
	if lease == nil || strings.TrimSpace(lease.JobID) == "" || strings.TrimSpace(lease.Owner) == "" {
		return fmt.Errorf("SDS repair lease is required")
	}
	return r.db.WithContext(ctx).Model(&listingkit.SDSChildRetryJob{}).
		Where("id = ? AND status = ? AND lease_owner = ?", lease.JobID, listingkit.SDSChildRetryJobStatusRepairing, lease.Owner).
		Updates(map[string]any{
			"status":      listingkit.SDSChildRetryJobStatusCancelled,
			"lease_owner": "",
			"lease_until": nil,
		}).Error
}

func lockRetryTask(tx *gorm.DB, ctx context.Context, taskID string) error {
	if !tx.Migrator().HasTable(&listingkit.Task{}) {
		return nil
	}
	var task listingkit.Task
	return applyTaskAccessScope(tx.Clauses(clause.Locking{Strength: "UPDATE"}), ctx).
		Where("id = ?", taskID).First(&task).Error
}

func (r *taskRepository) ClaimDueSDSChildRetries(ctx context.Context, dueBefore time.Time, limit int, owner string, leaseUntil time.Time) ([]listingkit.SDSChildRetryJob, error) {
	if strings.TrimSpace(owner) == "" {
		return nil, fmt.Errorf("SDS child retry lease owner is required")
	}
	var jobs []listingkit.SDSChildRetryJob
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		claimed := make([]listingkit.SDSChildRetryJob, 0, len(jobs))
		claimedTaskIDs := make(map[string]struct{})
		excludedTaskIDs := make(map[string]struct{})
		for limit <= 0 || len(claimed) < limit {
			db := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
				Where("status = ? AND next_retry_at <= ? AND (lease_until IS NULL OR lease_until <= ?)", listingkit.SDSChildRetryJobStatusPending, dueBefore, dueBefore).
				Where("NOT EXISTS (SELECT 1 FROM listingkit_sds_child_retry_jobs AS sibling WHERE sibling.listingkit_task_id = listingkit_sds_child_retry_jobs.listingkit_task_id AND sibling.id <> listingkit_sds_child_retry_jobs.id AND sibling.status = ? AND sibling.lease_until > ?)", listingkit.SDSChildRetryJobStatusPending, dueBefore).
				Where("NOT EXISTS (SELECT 1 FROM listingkit_sds_child_retry_jobs AS repair WHERE repair.listingkit_task_id = listingkit_sds_child_retry_jobs.listingkit_task_id AND repair.status = ? AND repair.lease_until > ?)", listingkit.SDSChildRetryJobStatusRepairing, dueBefore).
				Order("next_retry_at ASC, id ASC")
			if len(excludedTaskIDs) > 0 {
				ids := make([]string, 0, len(excludedTaskIDs))
				for taskID := range excludedTaskIDs {
					ids = append(ids, taskID)
				}
				db = db.Where("listingkit_task_id NOT IN ?", ids)
			}
			pageSize := 0
			if limit > 0 {
				pageSize = limit - len(claimed)
			}
			if pageSize > 0 {
				db = db.Limit(pageSize)
			}
			var page []listingkit.SDSChildRetryJob
			if err := db.Find(&page).Error; err != nil {
				return err
			}
			if len(page) == 0 {
				break
			}
			for index := range page {
				job := &page[index]
				if _, alreadyClaimed := claimedTaskIDs[job.TaskID]; alreadyClaimed {
					continue
				}
				if tx.Migrator().HasTable(&listingkit.Task{}) {
					var task listingkit.Task
					if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", job.TaskID).First(&task).Error; err != nil {
						return err
					}
					var active int64
					if err := tx.Model(&listingkit.SDSChildRetryJob{}).
						Where("listingkit_task_id = ? AND id <> ? AND status = ? AND lease_until > ?", job.TaskID, job.ID, listingkit.SDSChildRetryJobStatusPending, dueBefore).
						Count(&active).Error; err != nil {
						return err
					}
					if active > 0 {
						excludedTaskIDs[job.TaskID] = struct{}{}
						continue
					}
				}
				job.LeaseOwner = owner
				job.LeaseUntil = &leaseUntil
				if err := tx.Save(job).Error; err != nil {
					return err
				}
				claimed = append(claimed, *job)
				claimedTaskIDs[job.TaskID] = struct{}{}
				excludedTaskIDs[job.TaskID] = struct{}{}
			}
			if limit <= 0 || len(page) < pageSize {
				break
			}
		}
		jobs = claimed
		return nil
	})
	return jobs, err
}

func (r *taskRepository) SaveSDSChildRetry(ctx context.Context, job *listingkit.SDSChildRetryJob) error {
	if job == nil || strings.TrimSpace(job.ID) == "" {
		return fmt.Errorf("SDS child retry job is required")
	}
	return r.db.WithContext(ctx).Save(job).Error
}

