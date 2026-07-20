package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
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
