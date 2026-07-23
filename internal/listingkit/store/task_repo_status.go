package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/listingkit"
)

func (r *taskRepository) MarkProcessing(ctx context.Context, taskID string) error {
	result := r.db.WithContext(ctx).
		Model(&listingkit.Task{}).
		Scopes(taskAccessScope(ctx)).
		Where("id = ? AND status = ?", taskID, listingkit.TaskStatusPending).
		Updates(map[string]any{
			"status":     listingkit.TaskStatusProcessing,
			"updated_at": currentTimestampValue(r.db),
		})
	if result.Error != nil {
		return fmt.Errorf("failed to update task: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		return nil
	}
	task, err := r.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if task.Status != listingkit.TaskStatusPending {
		return listingkit.ErrTaskNotPending
	}
	return listingkit.ErrTaskNotFound
}

func (r *taskRepository) MarkCompleted(ctx context.Context, taskID string, result *listingkit.ListingKitResult) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"result": result,
		"status": listingkit.TaskStatusCompleted,
		"error":  "",
	})
}

func (r *taskRepository) MarkNeedsReview(ctx context.Context, taskID string, result *listingkit.ListingKitResult, reason string) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"result": result,
		"status": listingkit.TaskStatusNeedsReview,
		"error":  reason,
	})
}

func (r *taskRepository) MarkFailed(ctx context.Context, taskID string, errorMsg string) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"status": listingkit.TaskStatusFailed,
		"error":  errorMsg,
	})
}

func (r *taskRepository) MarkBlockedRetryable(ctx context.Context, taskID string, block *listingkit.RetryableBlock, errorMsg string) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"status":          listingkit.TaskStatusBlockedRetryable,
		"retryable_block": copyRetryableBlock(block),
		"error":           errorMsg,
	})
}

func (r *taskRepository) ListRecoverableTasks(ctx context.Context, query *listingkit.RecoverableTaskQuery) ([]listingkit.Task, error) {
	var tasks []listingkit.Task
	db := applyTaskAccessScope(r.db.WithContext(ctx).Model(&listingkit.Task{}), ctx)
	if err := db.Where("status = ?", listingkit.TaskStatusBlockedRetryable).Find(&tasks).Error; err != nil {
		return nil, err
	}

	dueBefore := time.Time{}
	if query != nil {
		dueBefore = query.DueBefore
	}
	items := collectRecoverableTasks(tasks, dueBefore)
	limit := normalizeRecoverableTaskLimit(query)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *taskRepository) RecoverBlockedTaskNow(ctx context.Context, taskID string, recoveredAt time.Time) error {
	task, err := r.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	force := recoveredAt.IsZero()
	effectiveRecoveredAt := normalizeRecoverTimestamp(recoveredAt)
	if !taskIsRecoverable(task, effectiveRecoveredAt, force) {
		return listingkit.ErrTaskNotRecoverable
	}
	block := listingkit.BuildRecoveredRetryableBlock(task.RetryableBlock, effectiveRecoveredAt)
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"status":          listingkit.TaskStatusPending,
		"retryable_block": block,
		"error":           "",
	})
}

func (r *taskRepository) BulkRecoverBlockedTasks(ctx context.Context, query *listingkit.RecoverBlockedTasksQuery) (int64, error) {
	listQuery := &listingkit.RecoverableTaskQuery{}
	if query != nil {
		listQuery.DueBefore = query.DueBefore
		listQuery.Limit = normalizeRecoverableTaskLimitFromValue(query.Limit)
	}
	tasks, err := r.ListRecoverableTasks(ctx, listQuery)
	if err != nil {
		return 0, err
	}
	recoverAt := time.Now().UTC()
	if query != nil && !query.RecoverAt.IsZero() {
		recoverAt = query.RecoverAt
	}
	recoverAt = normalizeRecoverTimestamp(recoverAt)
	var recovered int64
	for i := range tasks {
		if err := r.RecoverBlockedTaskNow(ctx, tasks[i].ID, recoverAt); err != nil {
			if errors.Is(err, listingkit.ErrTaskNotRecoverable) {
				continue
			}
			return recovered, err
		}
		recovered++
	}
	return recovered, nil
}

func (r *taskRepository) PrepareRetry(ctx context.Context, taskID string) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"status": listingkit.TaskStatusPending,
		"error":  "",
	})
}

func (r *taskRepository) IncrementRetryCount(ctx context.Context, taskID string) error {
	return r.db.WithContext(ctx).Model(&listingkit.Task{}).Scopes(taskAccessScope(ctx)).Where("id = ?", taskID).UpdateColumn("retry_count", gorm.Expr("retry_count + ?", 1)).Error
}

func (r *taskRepository) SaveTaskResult(ctx context.Context, taskID string, result *listingkit.ListingKitResult) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{"result": result})
}

func (r *taskRepository) MutateTaskResult(ctx context.Context, taskID string, mutate listingkit.TaskResultMutation) (*listingkit.Task, error) {
	var out *listingkit.Task
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task listingkit.Task
		if err := applyTaskAccessScope(tx.Clauses(clause.Locking{Strength: "UPDATE"}), ctx).Where("id = ?", taskID).First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return listingkit.ErrTaskNotFound
			}
			return err
		}
		copied := task
		out = &copied
		if mutate != nil {
			if err := mutate(&task); err != nil {
				return err
			}
		}
		task.UpdatedAt = time.Now()
		if err := tx.Model(&listingkit.Task{}).
			Scopes(taskAccessScope(ctx)).
			Where("id = ?", taskID).
			Updates(map[string]any{
				"status":          task.Status,
				"error":           task.Error,
				"result":          task.Result,
				"retryable_block": task.RetryableBlock,
				"updated_at":      currentTimestampValue(tx),
			}).Error; err != nil {
			return fmt.Errorf("failed to update task result: %w", err)
		}
		finalTask, err := loadTaskForSheinPODImageLookupIndex(ctx, tx, taskID)
		if err != nil {
			return err
		}
		if err := syncSheinPODImageLookupIndex(ctx, tx, finalTask); err != nil {
			return err
		}
		out = finalTask
		return nil
	})
	return out, err
}

func (r *taskRepository) ReplaceTaskSDSOptionsForRetry(ctx context.Context, taskID string, options *listingkit.SDSSyncOptions, audit listingkit.PodExecutionAuditEvent) (*listingkit.Task, error) {
	var out *listingkit.Task
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task listingkit.Task
		if err := applyTaskAccessScope(tx.Clauses(clause.Locking{Strength: "UPDATE"}), ctx).Where("id = ?", taskID).First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return listingkit.ErrTaskNotFound
			}
			return err
		}
		if !listingkit.TaskEligibleForSDSRepair(&task) || task.Request == nil || task.Request.Options == nil || options == nil {
			return listingkit.ErrSDSRepairNotEligible
		}
		task.Request.Options.SDS = options
		if task.Result.PodExecution == nil {
			task.Result.PodExecution = &listingkit.PodExecutionSummary{}
		}
		task.Result.PodExecution.History = append(task.Result.PodExecution.History, audit)
		task.UpdatedAt = time.Now()
		if err := tx.Model(&listingkit.Task{}).
			Scopes(taskAccessScope(ctx)).
			Where("id = ?", taskID).
			Updates(map[string]any{
				"request":    task.Request,
				"result":     task.Result,
				"updated_at": currentTimestampValue(tx),
			}).Error; err != nil {
			return fmt.Errorf("failed to replace task SDS options: %w", err)
		}
		finalTask, err := loadTaskForSheinPODImageLookupIndex(ctx, tx, taskID)
		if err != nil {
			return err
		}
		if err := syncSheinPODImageLookupIndex(ctx, tx, finalTask); err != nil {
			return err
		}
		out = finalTask
		return nil
	})
	return out, err
}

func (r *taskRepository) updateTaskFields(ctx context.Context, taskID string, updates map[string]any) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates["updated_at"] = currentTimestampValue(tx)
		result := tx.Model(&listingkit.Task{}).Scopes(taskAccessScope(ctx)).Where("id = ?", taskID).Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("failed to update task: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return listingkit.ErrTaskNotFound
		}
		finalTask, err := loadTaskForSheinPODImageLookupIndex(ctx, tx, taskID)
		if err != nil {
			return err
		}
		return syncSheinPODImageLookupIndex(ctx, tx, finalTask)
	})
}

func collectRecoverableTasks(tasks []listingkit.Task, dueBefore time.Time) []listingkit.Task {
	items := make([]listingkit.Task, 0, len(tasks))
	for i := range tasks {
		if !taskIsRecoverable(&tasks[i], dueBefore, false) {
			continue
		}
		items = append(items, tasks[i])
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i].RetryableBlock.NextRetryAt
		right := items[j].RetryableBlock.NextRetryAt
		switch {
		case left == nil && right == nil:
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		case left == nil:
			return false
		case right == nil:
			return true
		case !left.Equal(*right):
			return left.Before(*right)
		case !items[i].CreatedAt.Equal(items[j].CreatedAt):
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		default:
			return items[i].ID < items[j].ID
		}
	})
	return items
}

func taskIsRecoverable(task *listingkit.Task, dueBefore time.Time, force bool) bool {
	if task == nil || task.Status != listingkit.TaskStatusBlockedRetryable || task.RetryableBlock == nil {
		return false
	}
	if force {
		return true
	}
	block := task.RetryableBlock
	if !block.AutoResumeEnabled || block.AutoRetryPaused || block.NextRetryAt == nil {
		return false
	}
	if dueBefore.IsZero() {
		return true
	}
	return !block.NextRetryAt.After(dueBefore)
}

func normalizeRecoverTimestamp(recoveredAt time.Time) time.Time {
	if recoveredAt.IsZero() {
		return time.Now().UTC()
	}
	return recoveredAt
}

func normalizeRecoverableTaskLimitFromValue(limit int) int {
	if limit <= 0 {
		return 0
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func normalizeRecoverableTaskLimit(query *listingkit.RecoverableTaskQuery) int {
	if query == nil {
		return 0
	}
	return normalizeRecoverableTaskLimitFromValue(query.Limit)
}

func timestampPointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copied := value
	return &copied
}

func copyRetryableBlock(src *listingkit.RetryableBlock) *listingkit.RetryableBlock {
	if src == nil {
		return nil
	}
	cloned := *src
	cloned.LastRetryAt = timestampPointerValue(src.LastRetryAt)
	cloned.NextRetryAt = timestampPointerValue(src.NextRetryAt)
	return &cloned
}

func timestampPointerValue(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
