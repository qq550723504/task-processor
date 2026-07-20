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
		for index := range jobs {
			jobs[index].LeaseOwner = owner
			jobs[index].LeaseUntil = &leaseUntil
			if err := tx.Save(&jobs[index]).Error; err != nil {
				return err
			}
		}
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
