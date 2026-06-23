package listingadmin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
)

type GormImportTaskRepository struct {
	db *gorm.DB
}

func NewGormImportTaskRepository(db *gorm.DB) *GormImportTaskRepository {
	return &GormImportTaskRepository{db: db}
}

func AutoMigrateImportTaskRepository(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	return ensureOwnerAuditColumns(db, (listingProductImportTask{}).TableName())
}

func (r *GormImportTaskRepository) ListImportTasks(ctx context.Context, query ImportTaskQuery) (*ImportTaskPage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("import task repository database is not configured")
	}
	rows, total, page, pageSize, err := findImportTaskRows(ctx, r.db.WithContext(ctx).Table("listing_product_import_task"), query)
	if err != nil {
		return nil, err
	}
	items := make([]ImportTask, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toImportTask())
	}
	return &ImportTaskPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *GormImportTaskRepository) BatchCreateImportTasks(ctx context.Context, tasks []ImportTask) ([]ImportTask, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("import task repository database is not configured")
	}
	rows := make([]listingProductImportTask, 0, len(tasks))
	for _, task := range tasks {
		row := listingProductImportTaskFromImportTask(task)
		applyImportTaskDefaults(&row)
		if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
			applyImportTaskAuditFields(&row, ownerUserID, true)
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return []ImportTask{}, nil
	}
	if err := r.db.WithContext(ctx).Table("listing_product_import_task").Create(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ImportTask, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toImportTask())
	}
	return out, nil
}

func (r *GormImportTaskRepository) DeleteImportTask(ctx context.Context, tenantID, id int64) error {
	updates := map[string]any{"deleted": 1}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	return updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_product_import_task"), tenantID, id, "owner_user_id", updates, ErrImportTaskNotFound)
}

func (r *GormImportTaskRepository) GetImportTaskByID(ctx context.Context, id int64) (*ImportTask, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("import task repository database is not configured")
	}
	var row listingProductImportTask
	err := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_product_import_task").Where("id = ? AND deleted = 0", id),
		ctx,
		"owner_user_id",
	).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	task := row.toImportTask()
	return &task, nil
}

func (r *GormImportTaskRepository) ListPendingAndRetryTasks(ctx context.Context, limit int, tenantID int64, storeIDs []int64) ([]ImportTask, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("import task repository database is not configured")
	}
	if limit <= 0 {
		limit = 20
	}
	statuses := []int16{
		model.TaskStatusPending.Int16(),
		model.TaskStatusPendingRetry.Int16(),
		model.TaskStatusCrawled.Int16(),
	}
	query := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_product_import_task").Where("deleted = 0").Where("status IN ?", statuses),
		ctx,
		"owner_user_id",
	)
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if len(storeIDs) > 0 {
		query = query.Where("store_id IN ?", storeIDs)
	}
	var rows []listingProductImportTask
	if err := query.Order("priority asc, update_time asc, id asc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]ImportTask, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toImportTask())
	}
	return items, nil
}

func (r *GormImportTaskRepository) CountTimedOutProcessingTasks(ctx context.Context, timeoutBefore time.Time) (int64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("import task repository database is not configured")
	}
	var count int64
	err := r.db.WithContext(ctx).
		Table("listing_product_import_task").
		Where("deleted = 0").
		Where("status = ?", model.TaskStatusProcessing.Int16()).
		Where("update_time < ?", timeoutBefore).
		Count(&count).Error
	return count, err
}

func (r *GormImportTaskRepository) ListTimedOutProcessingTasks(ctx context.Context, timeoutBefore time.Time, limit int) ([]ImportTask, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("import task repository database is not configured")
	}
	if limit <= 0 {
		limit = 100
	}
	var rows []listingProductImportTask
	if err := r.db.WithContext(ctx).
		Table("listing_product_import_task").
		Where("deleted = 0").
		Where("status = ?", model.TaskStatusProcessing.Int16()).
		Where("update_time < ?", timeoutBefore).
		Order("update_time asc, id asc").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]ImportTask, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toImportTask())
	}
	return items, nil
}

func (r *GormImportTaskRepository) RecoverTimedOutProcessingTasks(ctx context.Context, ids []int64, recovery ProcessingTimeoutRecovery) (int, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("import task repository database is not configured")
	}
	if len(ids) == 0 {
		return 0, nil
	}
	timeoutMinutes := recovery.TimeoutMinutes
	if timeoutMinutes <= 0 {
		timeoutMinutes = 30
	}
	remark := recovery.Remark
	if remark == "" {
		remark = fmt.Sprintf("Recovered after processing timeout watchdog (%d minutes)", timeoutMinutes)
	}
	res := r.db.WithContext(ctx).
		Table("listing_product_import_task").
		Where("deleted = 0").
		Where("id IN ?", ids).
		Where("status = ?", model.TaskStatusProcessing.Int16()).
		Where("update_time < ?", time.Now().Add(-time.Duration(timeoutMinutes)*time.Minute)).
		Updates(map[string]any{
			"status":        model.TaskStatusPendingRetry.Int16(),
			"error_message": recovery.ErrorMessage,
			"reason_code":   recovery.ReasonCode,
			"stage":         recovery.Stage,
			"remark":        remark,
			"update_time":   time.Now(),
		})
	if res.Error != nil {
		return 0, res.Error
	}
	return int(res.RowsAffected), nil
}

func (r *GormImportTaskRepository) CountStaleQueuedTasks(ctx context.Context, timeoutBefore time.Time) (int64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("import task repository database is not configured")
	}
	var count int64
	err := r.db.WithContext(ctx).
		Table("listing_product_import_task").
		Where("deleted = 0").
		Where("status = ?", model.TaskStatusQueued.Int16()).
		Where("update_time < ?", timeoutBefore).
		Count(&count).Error
	return count, err
}

func (r *GormImportTaskRepository) ListStaleQueuedTasks(ctx context.Context, timeoutBefore time.Time, limit int) ([]ImportTask, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("import task repository database is not configured")
	}
	if limit <= 0 {
		limit = 500
	}
	var rows []listingProductImportTask
	if err := r.db.WithContext(ctx).
		Table("listing_product_import_task").
		Where("deleted = 0").
		Where("status = ?", model.TaskStatusQueued.Int16()).
		Where("update_time < ?", timeoutBefore).
		Order("update_time asc, id asc").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]ImportTask, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toImportTask())
	}
	return items, nil
}

func (r *GormImportTaskRepository) RecoverStaleQueuedTasks(ctx context.Context, ids []int64, recovery StaleQueuedRecovery) (int, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("import task repository database is not configured")
	}
	if len(ids) == 0 {
		return 0, nil
	}
	timeoutMinutes := recovery.TimeoutMinutes
	if timeoutMinutes <= 0 {
		timeoutMinutes = 120
	}
	remark := recovery.Remark
	if remark == "" {
		remark = fmt.Sprintf("Recovered from stale queued state by scheduler watchdog (%d minutes)", timeoutMinutes)
	}
	res := r.db.WithContext(ctx).
		Table("listing_product_import_task").
		Where("deleted = 0").
		Where("id IN ?", ids).
		Where("status = ?", model.TaskStatusQueued.Int16()).
		Where("update_time < ?", time.Now().Add(-time.Duration(timeoutMinutes)*time.Minute)).
		Updates(map[string]any{
			"status":        model.TaskStatusPending.Int16(),
			"error_message": recovery.ErrorMessage,
			"reason_code":   recovery.ReasonCode,
			"stage":         recovery.Stage,
			"remark":        remark,
			"update_time":   time.Now(),
		})
	if res.Error != nil {
		return 0, res.Error
	}
	return int(res.RowsAffected), nil
}

func (r *GormImportTaskRepository) UpdateImportTaskStatus(ctx context.Context, req *api.ProductImportTaskUpdateReqDTO) (bool, error) {
	if r == nil || r.db == nil {
		return false, errors.New("import task repository database is not configured")
	}
	if req == nil {
		return false, nil
	}
	var row listingProductImportTask
	err := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_product_import_task").Where("id = ? AND deleted = 0", req.ID),
		ctx,
		"owner_user_id",
	).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	if req.ExpectedCurrentStatus != nil && row.Status != *req.ExpectedCurrentStatus {
		return true, fmt.Errorf("管理端拒绝更新任务状态: taskId=%d, currentStatus=%d, expectedCurrentStatus=%d", req.ID, row.Status, *req.ExpectedCurrentStatus)
	}
	current, parseErr := model.ParseTaskStatus(row.Status)
	if parseErr == nil {
		target := model.TaskStatus(req.Status)
		if current != target {
			if err := model.ValidateTaskStatusTransition(current, target); err != nil {
				return true, fmt.Errorf("管理端拒绝更新任务状态: taskId=%d, invalid transition %d -> %d", req.ID, row.Status, req.Status)
			}
		}
	}
	updates := map[string]any{
		"status":        req.Status,
		"error_message": req.ErrorMessage,
		"reason_code":   req.ReasonCode,
		"stage":         req.Stage,
		"remark":        req.Remark,
		"update_time":   time.Now(),
	}
	if req.RetryCount != nil {
		updates["retry_count"] = *req.RetryCount
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	res := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_product_import_task").Where("id = ? AND deleted = 0", req.ID),
		ctx,
		"owner_user_id",
	).Updates(updates)
	if res.Error != nil {
		return true, res.Error
	}
	return res.RowsAffected > 0, nil
}
