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
	"task-processor/internal/listingkit/core"
)

func (r *taskRepository) MarkProcessing(ctx context.Context, taskID string) error {
	updated := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.
			Model(&listingkit.Task{}).
			Scopes(taskAccessScope(ctx)).
			Where("id = ? AND status = ?", taskID, core.TaskStatusPending).
			Updates(map[string]any{
				"status":     core.TaskStatusProcessing,
				"updated_at": currentTimestampValue(tx),
			})
		if result.Error != nil {
			return fmt.Errorf("failed to update task: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return nil
		}
		finalTask, err := loadTaskForSheinPODImageLookupIndex(ctx, tx, taskID)
		if err != nil {
			return err
		}
		if err := syncSheinPODImageLookupIndex(ctx, tx, finalTask); err != nil {
			return err
		}
		updated = true
		return nil
	})
	if err != nil {
		return err
	}
	if updated {
		return nil
	}
	task, err := r.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if task.Status != core.TaskStatusPending {
		return core.ErrTaskNotPending
	}
	return core.ErrTaskNotFound
}

func (r *taskRepository) MarkCompleted(ctx context.Context, taskID string, result *listingkit.ListingKitResult) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"result":          result,
		"status":          core.TaskStatusCompleted,
		"retryable_block": nil,
		"error":           "",
	})
}

func (r *taskRepository) MarkNeedsReview(ctx context.Context, taskID string, result *listingkit.ListingKitResult, reason string) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"result":          result,
		"status":          core.TaskStatusNeedsReview,
		"retryable_block": nil,
		"error":           reason,
	})
}

func (r *taskRepository) MarkFailed(ctx context.Context, taskID string, errorMsg string) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"status":          core.TaskStatusFailed,
		"retryable_block": nil,
		"error":           errorMsg,
	})
}

func (r *taskRepository) MarkBlockedRetryable(ctx context.Context, taskID string, block *listingkit.RetryableBlock, errorMsg string) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"status":          core.TaskStatusBlockedRetryable,
		"retryable_block": copyRetryableBlock(block),
		"error":           errorMsg,
	})
}

func (r *taskRepository) MarkBlockedRetryableIfCurrent(ctx context.Context, taskID string, expected, next *listingkit.RetryableBlock, errorMsg string) (bool, error) {
	if expected == nil || next == nil {
		return false, core.ErrTaskNotRecoverable
	}
	expectedValue, err := expected.Value()
	if err != nil {
		return false, err
	}
	expectedJSON, ok := expectedValue.([]byte)
	if !ok {
		return false, fmt.Errorf("retryable block value has unexpected type %T", expectedValue)
	}
	condition := "retryable_block = ?"
	switch r.db.Dialector.Name() {
	case "postgres":
		condition = "retryable_block::jsonb = ?::jsonb"
	case "sqlite":
		condition = "json(retryable_block) = json(?)"
	}
	result := r.db.WithContext(ctx).
		Model(&listingkit.Task{}).
		Scopes(taskAccessScope(ctx)).
		Where("id = ? AND status = ? AND "+condition, taskID, core.TaskStatusBlockedRetryable, string(expectedJSON)).
		Updates(map[string]any{
			"status":          core.TaskStatusBlockedRetryable,
			"retryable_block": copyRetryableBlock(next),
			"error":           errorMsg,
			"updated_at":      currentTimestampValue(r.db),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func (r *taskRepository) ResolveUsageSettlement(ctx context.Context, taskID string) error {
	task, err := r.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if task.RetryableBlock == nil || task.RetryableBlock.ReasonCode != "usage_commit_pending" {
		return core.ErrTaskNotRecoverable
	}
	if task.Result == nil || (task.Result.Status != string(core.TaskStatusCompleted) && task.Result.Status != string(core.TaskStatusNeedsReview)) {
		return core.ErrTaskNotRecoverable
	}
	errorMsg := ""
	if task.Result.Status == string(core.TaskStatusNeedsReview) {
		errorMsg = listingkit.TaskNeedsReviewReason(task.Result)
	}
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"status":                             core.TaskStatus(task.Result.Status),
		"retryable_block":                    nil,
		"error":                              errorMsg,
		"generation_usage_reservation_state": "",
		"generation_usage_reservation_lease_until": nil,
	})
}

func (r *taskRepository) BeginGenerationUsageReservation(ctx context.Context, taskID string, leaseUntil time.Time) error {
	return r.updateGenerationUsageReservation(ctx, taskID, leaseUntil, func(task *listingkit.Task) error {
		if task.GenerationUsageReservationState == "" {
			task.GenerationUsageReservationState = listingkit.GenerationUsageReservationStatePending
		}
		return nil
	})
}

func (r *taskRepository) MarkGenerationUsageReserved(ctx context.Context, taskID string, leaseUntil time.Time) error {
	return r.updateGenerationUsageReservation(ctx, taskID, leaseUntil, func(task *listingkit.Task) error {
		if task.GenerationUsageReservationState == "" {
			return core.ErrTaskNotRecoverable
		}
		task.GenerationUsageReservationState = listingkit.GenerationUsageReservationStateReserved
		return nil
	})
}

func (r *taskRepository) RenewGenerationUsageReservation(ctx context.Context, taskID string, leaseUntil time.Time) error {
	return r.updateGenerationUsageReservation(ctx, taskID, leaseUntil, func(task *listingkit.Task) error {
		if task.GenerationUsageReservationState == "" {
			return core.ErrTaskNotRecoverable
		}
		return nil
	})
}

func (r *taskRepository) ClearGenerationUsageReservation(ctx context.Context, taskID string) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"generation_usage_reservation_state":       "",
		"generation_usage_reservation_lease_until": nil,
	})
}

func (r *taskRepository) FinalizeGenerationUsageAdmission(ctx context.Context, taskID string, status core.TaskStatus, block *listingkit.RetryableBlock, errorMsg string) error {
	if status != core.TaskStatusFailed && (status != core.TaskStatusBlockedRetryable || block == nil) {
		return core.ErrTaskNotRecoverable
	}
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"status":                             status,
		"retryable_block":                    copyRetryableBlock(block),
		"error":                              errorMsg,
		"generation_usage_reservation_state": "",
		"generation_usage_reservation_lease_until": nil,
	})
}

func (r *taskRepository) PrepareGenerationUsageRelease(ctx context.Context, taskID string, block *listingkit.RetryableBlock, errorMsg string, taskResult *listingkit.ListingKitResult) error {
	if block == nil || block.ReasonCode != "usage_release_pending" {
		return core.ErrTaskNotRecoverable
	}
	updates := map[string]any{
		"status":          core.TaskStatusBlockedRetryable,
		"retryable_block": copyRetryableBlock(block),
		"error":           errorMsg,
	}
	if taskResult != nil {
		updates["result"] = taskResult
	}
	return r.updateTaskFields(ctx, taskID, updates)
}

func (r *taskRepository) ResolveGenerationUsageRelease(ctx context.Context, taskID, terminalError string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task listingkit.Task
		if err := applyTaskAccessScope(tx.Clauses(clause.Locking{Strength: "UPDATE"}), ctx).Where("id = ?", taskID).First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return core.ErrTaskNotFound
			}
			return err
		}
		if task.Status != core.TaskStatusBlockedRetryable || task.RetryableBlock == nil || task.RetryableBlock.ReasonCode != "usage_release_pending" {
			return core.ErrTaskNotRecoverable
		}
		result := tx.Model(&listingkit.Task{}).
			Scopes(taskAccessScope(ctx)).
			Where("id = ? AND status = ?", taskID, core.TaskStatusBlockedRetryable).
			Updates(map[string]any{
				"status":                             core.TaskStatusFailed,
				"retryable_block":                    nil,
				"error":                              terminalError,
				"generation_usage_reservation_state": "",
				"generation_usage_reservation_lease_until": nil,
				"updated_at": currentTimestampValue(tx),
			})
		if result.Error != nil {
			return fmt.Errorf("resolve generation usage release: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return core.ErrTaskNotRecoverable
		}
		finalTask, err := loadTaskForSheinPODImageLookupIndex(ctx, tx, taskID)
		if err != nil {
			return err
		}
		return syncSheinPODImageLookupIndex(ctx, tx, finalTask)
	})
}

func (r *taskRepository) ListExpiredGenerationUsageReservations(ctx context.Context, dueBefore time.Time, limit int) ([]listingkit.Task, error) {
	if dueBefore.IsZero() {
		dueBefore = time.Now().UTC()
	}
	var tasks []listingkit.Task
	db := applyTaskAccessScope(r.db.WithContext(ctx).Model(&listingkit.Task{}), ctx).
		Where("status IN ? AND generation_usage_reservation_state <> '' AND generation_usage_reservation_lease_until IS NOT NULL AND generation_usage_reservation_lease_until <= ?", []core.TaskStatus{core.TaskStatusPending, core.TaskStatusProcessing, core.TaskStatusCompleted, core.TaskStatusNeedsReview}, dueBefore).
		Order("generation_usage_reservation_lease_until ASC").
		Order("id ASC")
	if limit > 0 {
		db = db.Limit(normalizeRecoverableTaskLimitFromValue(limit))
	}
	if err := db.Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("list expired generation usage reservations: %w", err)
	}
	return tasks, nil
}

func (r *taskRepository) ResolveExpiredGenerationUsageReservation(ctx context.Context, taskID string, expectedStatus core.TaskStatus, dueBefore time.Time, block *listingkit.RetryableBlock, errorMsg string, clearReservation bool) error {
	if block == nil || dueBefore.IsZero() || !generationUsageReservationMayNeedSettlement(expectedStatus) {
		return core.ErrTaskNotRecoverable
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task listingkit.Task
		if err := applyTaskAccessScope(tx.Clauses(clause.Locking{Strength: "UPDATE"}), ctx).Where("id = ?", taskID).First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return core.ErrTaskNotFound
			}
			return err
		}
		if task.Status != expectedStatus || task.GenerationUsageReservationState == "" || task.GenerationUsageReservationLeaseUntil == nil || task.GenerationUsageReservationLeaseUntil.After(dueBefore) {
			return core.ErrTaskNotRecoverable
		}
		updates := map[string]any{
			"status":          core.TaskStatusBlockedRetryable,
			"retryable_block": copyRetryableBlock(block),
			"error":           errorMsg,
			"updated_at":      currentTimestampValue(tx),
		}
		if clearReservation {
			updates["generation_usage_reservation_state"] = ""
			updates["generation_usage_reservation_lease_until"] = nil
		}
		result := tx.Model(&listingkit.Task{}).Scopes(taskAccessScope(ctx)).Where("id = ? AND status = ? AND generation_usage_reservation_state <> '' AND generation_usage_reservation_lease_until <= ?", taskID, expectedStatus, dueBefore).Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("resolve expired generation usage reservation: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return core.ErrTaskNotRecoverable
		}
		finalTask, err := loadTaskForSheinPODImageLookupIndex(ctx, tx, taskID)
		if err != nil {
			return err
		}
		return syncSheinPODImageLookupIndex(ctx, tx, finalTask)
	})
}

func (r *taskRepository) updateGenerationUsageReservation(ctx context.Context, taskID string, leaseUntil time.Time, mutate func(*listingkit.Task) error) error {
	if leaseUntil.IsZero() {
		return core.ErrTaskNotRecoverable
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task listingkit.Task
		if err := applyTaskAccessScope(tx.Clauses(clause.Locking{Strength: "UPDATE"}), ctx).Where("id = ?", taskID).First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return core.ErrTaskNotFound
			}
			return err
		}
		if task.Status != core.TaskStatusProcessing {
			return core.ErrTaskNotRecoverable
		}
		if err := mutate(&task); err != nil {
			return err
		}
		return tx.Model(&listingkit.Task{}).Scopes(taskAccessScope(ctx)).Where("id = ?", taskID).Updates(map[string]any{
			"generation_usage_reservation_state":       task.GenerationUsageReservationState,
			"generation_usage_reservation_lease_until": leaseUntil,
			"updated_at": currentTimestampValue(tx),
		}).Error
	})
}

func (r *taskRepository) ListRecoverableTasks(ctx context.Context, query *listingkit.RecoverableTaskQuery) ([]listingkit.Task, error) {
	var tasks []listingkit.Task
	db := applyTaskAccessScope(r.db.WithContext(ctx).Model(&listingkit.Task{}), ctx)
	db = db.Where("status = ?", core.TaskStatusBlockedRetryable)
	db, bounded := applyBoundedRecoverableTaskScope(db, query)
	limit := normalizeRecoverableTaskLimit(query)
	if bounded && limit > 0 {
		db = db.Limit(limit)
	}
	if err := db.Find(&tasks).Error; err != nil {
		return nil, err
	}

	dueBefore := time.Time{}
	if query != nil {
		dueBefore = query.DueBefore
	}
	items := collectRecoverableTasks(tasks, dueBefore)
	items = filterRecoverableTaskReasonCodes(items, query)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func applyBoundedRecoverableTaskScope(db *gorm.DB, query *listingkit.RecoverableTaskQuery) (*gorm.DB, bool) {
	if query == nil || query.DueBefore.IsZero() {
		return db, false
	}
	var reasonExpr string
	switch db.Dialector.Name() {
	case "postgres":
		reasonExpr = "COALESCE(retryable_block::jsonb ->> 'reason_code', '')"
		db = db.
			Where("retryable_block IS NOT NULL").
			Where("COALESCE((retryable_block::jsonb ->> 'auto_resume_enabled')::boolean, FALSE) = TRUE").
			Where("COALESCE((retryable_block::jsonb ->> 'auto_retry_paused')::boolean, FALSE) = FALSE").
			Where("(retryable_block::jsonb ->> 'next_retry_at') IS NOT NULL").
			Where("(retryable_block::jsonb ->> 'next_retry_at')::timestamptz <= ?", query.DueBefore.UTC()).
			Order("(retryable_block::jsonb ->> 'next_retry_at')::timestamptz ASC, created_at ASC, id ASC")
	case "sqlite":
		reasonExpr = "COALESCE(json_extract(retryable_block, '$.reason_code'), '')"
		db = db.
			Where("retryable_block IS NOT NULL").
			Where("COALESCE(json_extract(retryable_block, '$.auto_resume_enabled'), 0) = 1").
			Where("COALESCE(json_extract(retryable_block, '$.auto_retry_paused'), 0) = 0").
			Where("json_extract(retryable_block, '$.next_retry_at') IS NOT NULL").
			Where("datetime(json_extract(retryable_block, '$.next_retry_at')) <= datetime(?)", query.DueBefore.UTC().Format(time.RFC3339Nano)).
			Order("datetime(json_extract(retryable_block, '$.next_retry_at')) ASC, created_at ASC, id ASC")
	default:
		return db, false
	}
	if len(query.ReasonCodes) > 0 {
		db = db.Where(reasonExpr+" IN ?", query.ReasonCodes)
	}
	if len(query.ExcludeReasonCodes) > 0 {
		db = db.Where(reasonExpr+" NOT IN ?", query.ExcludeReasonCodes)
	}
	return db, true
}

func filterRecoverableTaskReasonCodes(tasks []listingkit.Task, query *listingkit.RecoverableTaskQuery) []listingkit.Task {
	if query == nil || (len(query.ReasonCodes) == 0 && len(query.ExcludeReasonCodes) == 0) {
		return tasks
	}
	items := make([]listingkit.Task, 0, len(tasks))
	for i := range tasks {
		if taskMatchesRecoverableReasonCodes(&tasks[i], query) {
			items = append(items, tasks[i])
		}
	}
	return items
}

func taskMatchesRecoverableReasonCodes(task *listingkit.Task, query *listingkit.RecoverableTaskQuery) bool {
	if task == nil || task.RetryableBlock == nil {
		return false
	}
	if len(query.ReasonCodes) > 0 {
		matched := false
		for _, code := range query.ReasonCodes {
			if task.RetryableBlock.ReasonCode == code {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	for _, code := range query.ExcludeReasonCodes {
		if task.RetryableBlock.ReasonCode == code {
			return false
		}
	}
	return true
}

func (r *taskRepository) RecoverBlockedTaskNow(ctx context.Context, taskID string, recoveredAt time.Time) error {
	task, err := r.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	force := recoveredAt.IsZero()
	effectiveRecoveredAt := normalizeRecoverTimestamp(recoveredAt)
	if !taskIsRecoverable(task, effectiveRecoveredAt, force) {
		return core.ErrTaskNotRecoverable
	}
	block := listingkit.BuildRecoveredRetryableBlock(task.RetryableBlock, effectiveRecoveredAt)
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"status":          core.TaskStatusPending,
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
			if errors.Is(err, core.ErrTaskNotRecoverable) {
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
		"status": core.TaskStatusPending,
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
				return core.ErrTaskNotFound
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
				return core.ErrTaskNotFound
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
			return core.ErrTaskNotFound
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
	if task == nil || task.Status != core.TaskStatusBlockedRetryable || task.RetryableBlock == nil {
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
