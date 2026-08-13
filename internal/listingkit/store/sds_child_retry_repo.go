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
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "listingkit_task_id"}, {Name: "kind"}},
		DoNothing: true,
	}).Create(&copy)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected > 0 {
		return &copy, nil
	}
	var existing listingkit.SDSChildRetryJob
	if err := r.db.WithContext(ctx).Where("listingkit_task_id = ? AND kind = ?", copy.TaskID, copy.Kind).First(&existing).Error; err != nil {
		return nil, err
	}
	if existing.Status == listingkit.SDSChildRetryJobStatusCompleted || existing.Status == listingkit.SDSChildRetryJobStatusExhausted || existing.Status == listingkit.SDSChildRetryJobStatusCancelled {
		result := r.db.WithContext(ctx).Model(&listingkit.SDSChildRetryJob{}).
			Where("id = ? AND status IN ?", existing.ID, []listingkit.SDSChildRetryJobStatus{
				listingkit.SDSChildRetryJobStatusCompleted,
				listingkit.SDSChildRetryJobStatusExhausted,
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
			return nil, result.Error
		}
		if result.RowsAffected > 0 {
			if err := r.db.WithContext(ctx).Where("id = ?", existing.ID).First(&existing).Error; err != nil {
				return nil, err
			}
		} else if err := r.db.WithContext(ctx).Where("id = ?", existing.ID).First(&existing).Error; err != nil {
			return nil, err
		}
	}
	return &existing, nil
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
	db := r.db.WithContext(ctx).Where("listingkit_task_id = ?", taskID)
	if tenantID := tenantctx.TenantIDFromContext(ctx); tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	var jobs []listingkit.SDSChildRetryJob
	return jobs, db.Order("updated_at DESC, id ASC").Find(&jobs).Error
}

func (r *taskRepository) CancelSDSChildRetry(ctx context.Context, taskID string, kind listingkit.SDSChildRetryKind) error {
	return r.db.WithContext(ctx).Model(&listingkit.SDSChildRetryJob{}).
		Where("listingkit_task_id = ? AND kind = ? AND status = ?", taskID, kind, listingkit.SDSChildRetryJobStatusPending).
		Updates(map[string]any{
			"status":      listingkit.SDSChildRetryJobStatusCancelled,
			"lease_owner": "",
			"lease_until": nil,
		}).Error
}

func (r *taskRepository) ClaimDueSDSChildRetries(ctx context.Context, dueBefore time.Time, limit int, owner string, leaseUntil time.Time) ([]listingkit.SDSChildRetryJob, error) {
	if strings.TrimSpace(owner) == "" {
		return nil, fmt.Errorf("SDS child retry lease owner is required")
	}
	var jobs []listingkit.SDSChildRetryJob
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		db := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND next_retry_at <= ? AND (lease_until IS NULL OR lease_until <= ?)", listingkit.SDSChildRetryJobStatusPending, dueBefore, dueBefore).
			Order("next_retry_at ASC, id ASC")
		if limit > 0 {
			db = db.Limit(limit)
		}
		if err := db.Find(&jobs).Error; err != nil {
			return err
		}
		claimed := make([]listingkit.SDSChildRetryJob, 0, len(jobs))
		claimedTaskIDs := make(map[string]struct{})
		for index := range jobs {
			job := &jobs[index]
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
