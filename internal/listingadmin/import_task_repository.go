package listingadmin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
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
	table := (listingProductImportTask{}).TableName()
	if err := ensureOwnerAuditColumns(db, table); err != nil {
		return err
	}
	if !db.Migrator().HasColumn(table, "processing_node") {
		if err := db.Exec(fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN "processing_node" varchar(128)`, table)).Error; err != nil {
			return err
		}
	}
	return db.AutoMigrate(&listingDispatchEvent{})
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

func (r *GormImportTaskRepository) ListDispatchCandidatesFair(ctx context.Context, req DispatchCandidateRequest) ([]ImportTask, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("import task repository database is not configured")
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.PerStoreLimit <= 0 {
		req.PerStoreLimit = 1
	}
	statuses := []int16{
		model.TaskStatusPending.Int16(),
		model.TaskStatusCrawled.Int16(),
		model.TaskStatusPendingRetry.Int16(),
	}
	platform := strings.TrimSpace(req.Platform)
	if platform == "" {
		return []ImportTask{}, nil
	}
	ranked := r.db.WithContext(ctx).
		Table("listing_product_import_task AS t").
		Select(`t.*, row_number() over (
			partition by t.tenant_id, t.store_id
			order by t.priority desc, t.update_time asc, t.id asc
		) as rn`).
		Where("t.deleted = 0").
		Where("COALESCE(t.target_platform, t.platform) = ?", platform).
		Where("t.status IN ?", statuses).
		Where("t.store_id IS NOT NULL")
	if len(req.ExcludedStoreIDs) > 0 {
		ranked = ranked.Where("t.store_id NOT IN ?", req.ExcludedStoreIDs)
	}

	var rows []listingProductImportTask
	err := r.db.WithContext(ctx).
		Table("(?) AS ranked", ranked).
		Where("ranked.rn <= ?", req.PerStoreLimit).
		Order("ranked.rn asc, ranked.priority desc, ranked.update_time asc, ranked.id asc").
		Limit(req.Limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	items := make([]ImportTask, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toImportTask())
	}
	return items, nil
}

func (r *GormImportTaskRepository) ClaimForDispatch(ctx context.Context, claim DispatchClaim) (bool, error) {
	if r == nil || r.db == nil {
		return false, errors.New("import task repository database is not configured")
	}
	processingNode := strings.TrimSpace(claim.ProcessingNode)
	if processingNode == "" {
		return false, errors.New("dispatch claim processing node is required")
	}
	if !isDispatchableImportTaskStatus(claim.PreviousStatus) {
		return false, fmt.Errorf("dispatch claim previous status %d is not dispatchable", claim.PreviousStatus)
	}
	res := r.db.WithContext(ctx).
		Table("listing_product_import_task").
		Where("id = ?", claim.TaskID).
		Where("status = ?", claim.PreviousStatus).
		Where("deleted = 0").
		Updates(map[string]any{
			"status":          model.TaskStatusQueued.Int16(),
			"processing_node": processingNode,
			"error_message":   nil,
			"reason_code":     nil,
			"remark":          strings.TrimSpace(claim.Remark),
			"update_time":     time.Now(),
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

func isDispatchableImportTaskStatus(status int16) bool {
	switch status {
	case model.TaskStatusPending.Int16(), model.TaskStatusCrawled.Int16(), model.TaskStatusPendingRetry.Int16():
		return true
	default:
		return false
	}
}

func (r *GormImportTaskRepository) RecordDispatchDelay(ctx context.Context, delay DispatchDelay) (bool, error) {
	if r == nil || r.db == nil {
		return false, errors.New("import task repository database is not configured")
	}
	if !isDispatchableImportTaskStatus(delay.CurrentStatus) {
		return false, fmt.Errorf("dispatch delay current status %d is not dispatchable", delay.CurrentStatus)
	}
	reasonCode := strings.TrimSpace(delay.ReasonCode)
	if reasonCode == "" {
		reasonCode = "dispatch_delayed"
	}
	stage := strings.TrimSpace(delay.Stage)
	if stage == "" {
		stage = "dispatch"
	}
	message := strings.TrimSpace(delay.ErrorMessage)
	if message == "" {
		message = fmt.Sprintf("Dispatch delayed: %s", reasonCode)
	}
	remark := strings.TrimSpace(delay.Remark)
	if remark == "" {
		remark = message
	}
	res := r.db.WithContext(ctx).
		Table("listing_product_import_task").
		Where("id = ?", delay.TaskID).
		Where("status = ?", delay.CurrentStatus).
		Where("deleted = 0").
		Updates(map[string]any{
			"error_message": message,
			"reason_code":   reasonCode,
			"stage":         stage,
			"remark":        remark,
			"update_time":   time.Now(),
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

func (r *GormImportTaskRepository) CountDailyDispatchUsage(ctx context.Context, platform string, tenantID, storeID int64, day time.Time) (DailyDispatchUsage, error) {
	if r == nil || r.db == nil {
		return DailyDispatchUsage{}, errors.New("import task repository database is not configured")
	}
	platform = strings.TrimSpace(platform)
	if platform == "" {
		return DailyDispatchUsage{}, nil
	}
	if day.IsZero() {
		day = time.Now()
	}
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	end := start.AddDate(0, 0, 1)
	var rows []struct {
		Status int16
		Count  int64
	}
	err := r.db.WithContext(ctx).
		Table("listing_product_import_task").
		Select("status, count(*) as count").
		Where("deleted = 0").
		Where("tenant_id = ? AND store_id = ?", tenantID, storeID).
		Where("COALESCE(NULLIF(target_platform, ''), platform) = ?", platform).
		Where("create_time >= ? AND create_time < ?", start, end).
		Where("status IN ?", []int16{
			model.TaskStatusProcessing.Int16(),
			model.TaskStatusRepublishing.Int16(),
			model.TaskStatusResuming.Int16(),
			model.TaskStatusQueued.Int16(),
			model.TaskStatusPublished.Int16(),
			model.TaskStatusDraft.Int16(),
		}).
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return DailyDispatchUsage{}, err
	}
	var usage DailyDispatchUsage
	for _, row := range rows {
		switch row.Status {
		case model.TaskStatusPublished.Int16(), model.TaskStatusDraft.Int16():
			usage.Completed += int(row.Count)
		case model.TaskStatusProcessing.Int16(), model.TaskStatusRepublishing.Int16(), model.TaskStatusResuming.Int16():
			usage.Processing += int(row.Count)
		case model.TaskStatusQueued.Int16():
			usage.Queued += int(row.Count)
		}
	}
	return usage, nil
}

func (r *GormImportTaskRepository) RecordDispatchEvent(ctx context.Context, event DispatchEvent) error {
	if r == nil || r.db == nil {
		return errors.New("import task repository database is not configured")
	}
	row := listingDispatchEvent{
		TaskID:         event.TaskID,
		TenantID:       event.TenantID,
		StoreID:        event.StoreID,
		Platform:       strings.TrimSpace(event.Platform),
		Action:         strings.TrimSpace(event.Action),
		ReasonCode:     strings.TrimSpace(event.ReasonCode),
		Stage:          strings.TrimSpace(event.Stage),
		Capacity:       event.Capacity,
		Queued:         event.Queued,
		Processing:     event.Processing,
		CompletedToday: event.CompletedToday,
		DailyLimit:     event.DailyLimit,
		OwnerNode:      strings.TrimSpace(event.OwnerNode),
	}
	if row.Stage == "" {
		row.Stage = "dispatch"
	}
	return r.db.WithContext(ctx).Table(row.TableName()).Create(&row).Error
}

func (r *GormImportTaskRepository) RollbackDispatch(ctx context.Context, taskID int64, previousStatus int16, processingNode, reason string) error {
	if r == nil || r.db == nil {
		return errors.New("import task repository database is not configured")
	}
	trimmedProcessingNode := strings.TrimSpace(processingNode)
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		trimmedReason = "Dispatch rollback"
	}
	res := r.db.WithContext(ctx).
		Table("listing_product_import_task").
		Where("id = ?", taskID).
		Where("status = ?", model.TaskStatusQueued.Int16()).
		Where("processing_node = ?", trimmedProcessingNode).
		Where("deleted = 0").
		Updates(map[string]any{
			"status":          previousStatus,
			"processing_node": "",
			"error_message":   trimmedReason,
			"remark":          trimmedReason,
			"update_time":     time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrImportTaskNotFound
	}
	return nil
}

func (r *GormImportTaskRepository) CountQueuedByStore(ctx context.Context, platform string, storeIDs []int64) (map[int64]int64, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("import task repository database is not configured")
	}
	trimmedPlatform := strings.TrimSpace(platform)
	if trimmedPlatform == "" {
		return map[int64]int64{}, nil
	}
	type storeCount struct {
		StoreID int64
		Count   int64
	}
	query := r.db.WithContext(ctx).
		Table("listing_product_import_task").
		Select("store_id, count(*) as count").
		Where("deleted = 0").
		Where("status = ?", model.TaskStatusQueued.Int16()).
		Where("COALESCE(target_platform, platform) = ?", trimmedPlatform).
		Group("store_id")
	if len(storeIDs) > 0 {
		query = query.Where("store_id IN ?", storeIDs)
	}
	var rows []storeCount
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[int64]int64, len(rows))
	for _, row := range rows {
		counts[row.StoreID] = row.Count
	}
	return counts, nil
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
	timeoutBefore := recovery.TimeoutBefore
	if timeoutBefore.IsZero() {
		timeoutBefore = time.Now().Add(-time.Duration(timeoutMinutes) * time.Minute)
	}
	res := r.db.WithContext(ctx).
		Table("listing_product_import_task").
		Where("deleted = 0").
		Where("id IN ?", ids).
		Where("status = ?", model.TaskStatusProcessing.Int16()).
		Where("update_time < ?", timeoutBefore).
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
	timeoutBefore := recovery.TimeoutBefore
	if timeoutBefore.IsZero() {
		timeoutBefore = time.Now().Add(-time.Duration(timeoutMinutes) * time.Minute)
	}
	res := r.db.WithContext(ctx).
		Table("listing_product_import_task").
		Where("deleted = 0").
		Where("id IN ?", ids).
		Where("status = ?", model.TaskStatusQueued.Int16()).
		Where("update_time < ?", timeoutBefore).
		Updates(map[string]any{
			"status":          model.TaskStatusPending.Int16(),
			"processing_node": "",
			"error_message":   recovery.ErrorMessage,
			"reason_code":     recovery.ReasonCode,
			"stage":           recovery.Stage,
			"remark":          remark,
			"update_time":     time.Now(),
		})
	if res.Error != nil {
		return 0, res.Error
	}
	return int(res.RowsAffected), nil
}

func (r *GormImportTaskRepository) UpdateImportTaskStatus(ctx context.Context, req *ImportTaskStatusUpdate) (bool, error) {
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
	now := time.Now()
	updates := map[string]any{
		"status":        req.Status,
		"error_message": req.ErrorMessage,
		"reason_code":   req.ReasonCode,
		"stage":         req.Stage,
		"remark":        req.Remark,
		"update_time":   now,
	}
	if shouldSetImportTaskPublishedTime(row, req.Status) {
		updates["published_time"] = now
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

func shouldSetImportTaskPublishedTime(row listingProductImportTask, targetStatus int16) bool {
	if row.PublishedTime != nil {
		return false
	}
	if isImportTaskCompletedStatus(row.Status) {
		return false
	}
	return isImportTaskCompletedStatus(targetStatus)
}

func isImportTaskCompletedStatus(status int16) bool {
	switch status {
	case model.TaskStatusPublished.Int16(), model.TaskStatusDraft.Int16():
		return true
	default:
		return false
	}
}
