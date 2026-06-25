package listingadmin

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
	"task-processor/internal/model"
)

type legacyImportTaskWithoutProcessingNode struct {
	ID          int64  `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID    int64  `gorm:"column:tenant_id"`
	OwnerUserID string `gorm:"column:owner_user_id"`
	StoreID     int64  `gorm:"column:store_id"`
	Platform    string `gorm:"column:platform"`
	Region      string `gorm:"column:region"`
	CategoryID  int64  `gorm:"column:category_id"`
	ProductID   string `gorm:"column:product_id"`
	Status      int16  `gorm:"column:status"`
	Deleted     int16  `gorm:"column:deleted"`
	CreatedBy   string `gorm:"column:created_by"`
	UpdatedBy   string `gorm:"column:updated_by"`
}

func (legacyImportTaskWithoutProcessingNode) TableName() string {
	return "listing_product_import_task"
}

func TestListDispatchCandidatesFairAlternatesStoresBeforeSecondTask(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	oldest := now.Add(-3 * time.Minute)
	middle := now.Add(-2 * time.Minute)
	newest := now.Add(-1 * time.Minute)
	rows := []listingProductImportTask{
		{ID: 1, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "s100-high", Status: model.TaskStatusPending.Int16(), Priority: 90, CreateTime: &oldest, UpdateTime: &oldest, Deleted: 0},
		{ID: 2, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "s100-second", Status: model.TaskStatusPendingRetry.Int16(), Priority: 80, CreateTime: &middle, UpdateTime: &middle, Deleted: 0},
		{ID: 3, TenantID: 10, StoreID: 200, Platform: "legacy", TargetPlatform: "shein", Region: "us", ProductID: "s200-high", Status: model.TaskStatusCrawled.Int16(), Priority: 70, CreateTime: &newest, UpdateTime: &newest, Deleted: 0},
		{ID: 4, TenantID: 10, StoreID: 300, Platform: "amazon", Region: "us", ProductID: "wrong-platform", Status: model.TaskStatusPending.Int16(), Priority: 100, CreateTime: &oldest, UpdateTime: &oldest, Deleted: 0},
		{ID: 5, TenantID: 10, StoreID: 400, Platform: "shein", Region: "us", ProductID: "excluded", Status: model.TaskStatusPending.Int16(), Priority: 100, CreateTime: &oldest, UpdateTime: &oldest, Deleted: 0},
		{ID: 6, TenantID: 10, StoreID: 500, Platform: "shein", TargetPlatform: "", Region: "us", ProductID: "empty-target-platform", Status: model.TaskStatusPending.Int16(), Priority: 100, CreateTime: &oldest, UpdateTime: &oldest, Deleted: 0},
	}
	seedDispatchTasks(t, db, rows)
	setNullTargetPlatforms(t, db, 1, 2, 4, 5)

	repo := NewGormImportTaskRepository(db)
	tasks, err := repo.ListDispatchCandidatesFair(context.Background(), DispatchCandidateRequest{
		Platform:         "shein",
		Limit:            3,
		PerStoreLimit:    2,
		ExcludedStoreIDs: []int64{400},
	})
	if err != nil {
		t.Fatalf("ListDispatchCandidatesFair() error = %v", err)
	}

	got := taskIDs(tasks)
	want := []int64{1, 3, 2}
	if !equalInt64s(got, want) {
		t.Fatalf("task IDs = %v, want %v", got, want)
	}
}

func TestClaimForDispatchSucceedsOnlyFromExpectedStatus(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 11, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "claim", Status: model.TaskStatusPending.Int16(), ErrorMessage: "old error", ReasonCode: "OLD", Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})

	repo := NewGormImportTaskRepository(db)
	claimed, err := repo.ClaimForDispatch(context.Background(), DispatchClaim{
		TaskID:         11,
		PreviousStatus: model.TaskStatusPendingRetry.Int16(),
		ProcessingNode: "node-a",
		Remark:         "dispatching to queue",
	})
	if err != nil {
		t.Fatalf("ClaimForDispatch(wrong status) error = %v", err)
	}
	if claimed {
		t.Fatalf("ClaimForDispatch(wrong status) = true, want false")
	}

	claimed, err = repo.ClaimForDispatch(context.Background(), DispatchClaim{
		TaskID:         11,
		PreviousStatus: model.TaskStatusPending.Int16(),
		ProcessingNode: "node-a",
		Remark:         "dispatching to queue",
	})
	if err != nil {
		t.Fatalf("ClaimForDispatch(expected status) error = %v", err)
	}
	if !claimed {
		t.Fatalf("ClaimForDispatch(expected status) = false, want true")
	}

	var row listingProductImportTask
	if err := db.Table("listing_product_import_task").Where("id = ?", int64(11)).Take(&row).Error; err != nil {
		t.Fatalf("load claimed row: %v", err)
	}
	if row.Status != model.TaskStatusQueued.Int16() || row.ProcessingNode != "node-a" || row.ErrorMessage != "" || row.ReasonCode != "" || row.Remark != "dispatching to queue" {
		t.Fatalf("claimed row = %+v, want queued with node, cleared error fields, and remark", row)
	}
}

func TestClaimForDispatchRejectsBlankProcessingNode(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 12, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "blank-node", Status: model.TaskStatusPending.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})

	repo := NewGormImportTaskRepository(db)
	claimed, err := repo.ClaimForDispatch(context.Background(), DispatchClaim{
		TaskID:         12,
		PreviousStatus: model.TaskStatusPending.Int16(),
		ProcessingNode: "  ",
		Remark:         "dispatching to queue",
	})
	if err == nil {
		t.Fatal("ClaimForDispatch(blank processing node) error = nil, want error")
	}
	if claimed {
		t.Fatal("ClaimForDispatch(blank processing node) = true, want false")
	}

	var row listingProductImportTask
	if err := db.Table("listing_product_import_task").Where("id = ?", int64(12)).Take(&row).Error; err != nil {
		t.Fatalf("load rejected claim row: %v", err)
	}
	if row.Status != model.TaskStatusPending.Int16() || row.ProcessingNode != "" || row.Remark != "" {
		t.Fatalf("rejected claim row = %+v, want unchanged pending row", row)
	}
}

func TestClaimForDispatchRejectsInvalidPreviousStatus(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 13, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "invalid-status", Status: model.TaskStatusPublished.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})

	repo := NewGormImportTaskRepository(db)
	claimed, err := repo.ClaimForDispatch(context.Background(), DispatchClaim{
		TaskID:         13,
		PreviousStatus: model.TaskStatusPublished.Int16(),
		ProcessingNode: "node-a",
		Remark:         "dispatching to queue",
	})
	if err == nil {
		t.Fatal("ClaimForDispatch(invalid previous status) error = nil, want error")
	}
	if claimed {
		t.Fatal("ClaimForDispatch(invalid previous status) = true, want false")
	}

	var row listingProductImportTask
	if err := db.Table("listing_product_import_task").Where("id = ?", int64(13)).Take(&row).Error; err != nil {
		t.Fatalf("load invalid status claim row: %v", err)
	}
	if row.Status != model.TaskStatusPublished.Int16() || row.ProcessingNode != "" || row.Remark != "" {
		t.Fatalf("invalid status claim row = %+v, want unchanged published row", row)
	}
}

func TestConcurrentClaimForDispatchOnlyOneWorkerWins(t *testing.T) {
	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 14, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "concurrent-claim", Status: model.TaskStatusPending.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})

	repo := NewGormImportTaskRepository(db)
	var successes atomic.Int32
	runConcurrently(t, 8, func(worker int) {
		claimed, err := repo.ClaimForDispatch(context.Background(), DispatchClaim{
			TaskID:         14,
			PreviousStatus: model.TaskStatusPending.Int16(),
			ProcessingNode: "node-" + string(rune('a'+worker)),
			Remark:         "dispatching concurrently",
		})
		if err != nil {
			t.Errorf("ClaimForDispatch(worker=%d) error = %v", worker, err)
			return
		}
		if claimed {
			successes.Add(1)
		}
	})

	if successes.Load() != 1 {
		t.Fatalf("successful claims = %d, want 1", successes.Load())
	}
	var row listingProductImportTask
	if err := db.Table("listing_product_import_task").Where("id = ?", int64(14)).Take(&row).Error; err != nil {
		t.Fatalf("load concurrently claimed row: %v", err)
	}
	if row.Status != model.TaskStatusQueued.Int16() || row.ProcessingNode == "" || row.Remark != "dispatching concurrently" {
		t.Fatalf("concurrently claimed row = %+v, want queued with winning processing node", row)
	}
}

func TestRollbackDispatchRestoresPreviousStatusWithVisibleReason(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 21, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "rollback", Status: model.TaskStatusQueued.Int16(), ProcessingNode: "node-a", Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})

	repo := NewGormImportTaskRepository(db)
	if err := repo.RollbackDispatch(context.Background(), 21, model.TaskStatusCrawled.Int16(), "node-a", "RabbitMQ publish failed"); err != nil {
		t.Fatalf("RollbackDispatch() error = %v", err)
	}

	var row listingProductImportTask
	if err := db.Table("listing_product_import_task").Where("id = ?", int64(21)).Take(&row).Error; err != nil {
		t.Fatalf("load rolled back row: %v", err)
	}
	if row.Status != model.TaskStatusCrawled.Int16() || row.ErrorMessage != "RabbitMQ publish failed" || row.Remark != "RabbitMQ publish failed" || row.ProcessingNode != "" {
		t.Fatalf("rolled back row = %+v, want previous status with visible reason and cleared node", row)
	}
}

func TestConcurrentRollbackDispatchOnlyOriginalQueuedClaimIsRestoredOnce(t *testing.T) {
	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 25, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "concurrent-rollback", Status: model.TaskStatusQueued.Int16(), ProcessingNode: "node-a", Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})

	repo := NewGormImportTaskRepository(db)
	var restored atomic.Int32
	var notFound atomic.Int32
	runConcurrently(t, 8, func(worker int) {
		err := repo.RollbackDispatch(context.Background(), 25, model.TaskStatusCrawled.Int16(), "node-a", "publish failed")
		switch {
		case err == nil:
			restored.Add(1)
		case errors.Is(err, ErrImportTaskNotFound):
			notFound.Add(1)
		default:
			t.Errorf("RollbackDispatch(worker=%d) error = %v", worker, err)
		}
	})

	if restored.Load() != 1 || notFound.Load() != 7 {
		t.Fatalf("rollback results restored=%d notFound=%d, want 1/7", restored.Load(), notFound.Load())
	}
	var row listingProductImportTask
	if err := db.Table("listing_product_import_task").Where("id = ?", int64(25)).Take(&row).Error; err != nil {
		t.Fatalf("load concurrently rolled back row: %v", err)
	}
	if row.Status != model.TaskStatusCrawled.Int16() || row.ProcessingNode != "" || row.ErrorMessage != "publish failed" {
		t.Fatalf("concurrently rolled back row = %+v, want restored crawled row with cleared node", row)
	}
}

func TestRollbackDispatchReturnsNotFoundWhenNoQueuedRowRestored(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 22, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "not-queued", Status: model.TaskStatusPending.Int16(), ProcessingNode: "node-a", Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})

	repo := NewGormImportTaskRepository(db)
	if err := repo.RollbackDispatch(context.Background(), 22, model.TaskStatusCrawled.Int16(), "node-a", "publish failed"); !errors.Is(err, ErrImportTaskNotFound) {
		t.Fatalf("RollbackDispatch() error = %v, want ErrImportTaskNotFound", err)
	}
}

func TestConcurrentRecoveryOnlyUpdatesStillEligibleRowsOnce(t *testing.T) {
	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	expired := now.Add(-3 * time.Hour)
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 26, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "processing-timeout", Status: model.TaskStatusProcessing.Int16(), ProcessingNode: "worker-a", Priority: 10, CreateTime: &expired, UpdateTime: &expired, Deleted: 0},
		{ID: 27, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "stale-queued", Status: model.TaskStatusQueued.Int16(), ProcessingNode: "dispatch-token", Priority: 10, CreateTime: &expired, UpdateTime: &expired, Deleted: 0},
	})

	repo := NewGormImportTaskRepository(db)
	var processingRecovered atomic.Int32
	var queuedRecovered atomic.Int32
	runConcurrently(t, 8, func(worker int) {
		recovered, err := repo.RecoverTimedOutProcessingTasks(context.Background(), []int64{26}, ProcessingTimeoutRecovery{
			TimeoutMinutes: 30,
			TimeoutBefore:  now.Add(-30 * time.Minute),
			ErrorMessage:   "Task processing lease expired, recovered by listing control plane",
			ReasonCode:     "PROCESSING_TIMEOUT",
			Stage:          "processing_timeout_recovery",
		})
		if err != nil {
			t.Errorf("RecoverTimedOutProcessingTasks(worker=%d) error = %v", worker, err)
			return
		}
		processingRecovered.Add(int32(recovered))

		recovered, err = repo.RecoverStaleQueuedTasks(context.Background(), []int64{27}, StaleQueuedRecovery{
			TimeoutMinutes: 120,
			TimeoutBefore:  now.Add(-120 * time.Minute),
			ErrorMessage:   "Task stayed queued too long, recovered by listing control plane",
			ReasonCode:     "STALE_QUEUED",
			Stage:          "queued_timeout_recovery",
		})
		if err != nil {
			t.Errorf("RecoverStaleQueuedTasks(worker=%d) error = %v", worker, err)
			return
		}
		queuedRecovered.Add(int32(recovered))
	})

	if processingRecovered.Load() != 1 || queuedRecovered.Load() != 1 {
		t.Fatalf("recovered processing=%d queued=%d, want 1/1", processingRecovered.Load(), queuedRecovered.Load())
	}
	processingTask, err := repo.GetImportTaskByID(context.Background(), 26)
	if err != nil {
		t.Fatalf("GetImportTaskByID(processing) error = %v", err)
	}
	if processingTask == nil || processingTask.Status != model.TaskStatusPendingRetry.Int16() || processingTask.ReasonCode != "PROCESSING_TIMEOUT" {
		t.Fatalf("processing task = %+v, want one recovered pending_retry row", processingTask)
	}
	queuedTask, err := repo.GetImportTaskByID(context.Background(), 27)
	if err != nil {
		t.Fatalf("GetImportTaskByID(queued) error = %v", err)
	}
	if queuedTask == nil || queuedTask.Status != model.TaskStatusPending.Int16() || queuedTask.ProcessingNode != "" || queuedTask.ReasonCode != "STALE_QUEUED" {
		t.Fatalf("queued task = %+v, want one recovered pending row with cleared node", queuedTask)
	}
}

func TestRollbackDispatchRejectsStaleProcessingNode(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 23, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "reclaimed", Status: model.TaskStatusQueued.Int16(), ProcessingNode: "node-b", ErrorMessage: "new claim", Remark: "new claim", Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})

	repo := NewGormImportTaskRepository(db)
	if err := repo.RollbackDispatch(context.Background(), 23, model.TaskStatusPending.Int16(), "node-a", "stale publish failed"); !errors.Is(err, ErrImportTaskNotFound) {
		t.Fatalf("RollbackDispatch(stale node) error = %v, want ErrImportTaskNotFound", err)
	}

	var row listingProductImportTask
	if err := db.Table("listing_product_import_task").Where("id = ?", int64(23)).Take(&row).Error; err != nil {
		t.Fatalf("load stale rollback row: %v", err)
	}
	if row.Status != model.TaskStatusQueued.Int16() || row.ProcessingNode != "node-b" || row.ErrorMessage != "new claim" || row.Remark != "new claim" {
		t.Fatalf("stale rollback row = %+v, want newer queued claim unchanged", row)
	}
}

func TestRecordDispatchDelayPersistsVisibleReasonWithoutChangingStatus(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 24, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "delayed", Status: model.TaskStatusPending.Int16(), ProcessingNode: "", ErrorMessage: "old error", ReasonCode: "OLD", Stage: "old_stage", Remark: "old remark", Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})

	repo := NewGormImportTaskRepository(db)
	updated, err := repo.RecordDispatchDelay(context.Background(), DispatchDelay{
		TaskID:        24,
		CurrentStatus: model.TaskStatusPending.Int16(),
		ReasonCode:    "no_capacity",
		Stage:         "dispatch",
		ErrorMessage:  "Dispatch delayed: no_capacity",
		Remark:        "Dispatch delayed: no_capacity",
	})
	if err != nil {
		t.Fatalf("RecordDispatchDelay() error = %v", err)
	}
	if !updated {
		t.Fatal("RecordDispatchDelay() = false, want true")
	}

	var row listingProductImportTask
	if err := db.Table("listing_product_import_task").Where("id = ?", int64(24)).Take(&row).Error; err != nil {
		t.Fatalf("load delayed row: %v", err)
	}
	if row.Status != model.TaskStatusPending.Int16() || row.ProcessingNode != "" {
		t.Fatalf("delayed row changed lifecycle fields: %+v", row)
	}
	if row.ReasonCode != "no_capacity" || row.Stage != "dispatch" || row.ErrorMessage != "Dispatch delayed: no_capacity" || row.Remark != "Dispatch delayed: no_capacity" {
		t.Fatalf("delayed row = %+v, want persisted dispatch reason", row)
	}
}

func TestCountDailyDispatchUsageCountsSuccessfulAndInFlightTasks(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	day := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	yesterday := day.AddDate(0, 0, -1)
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 25, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "published", Status: model.TaskStatusPublished.Int16(), Priority: 10, CreateTime: &day, UpdateTime: &day, Deleted: 0},
		{ID: 26, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "draft", Status: model.TaskStatusDraft.Int16(), Priority: 10, CreateTime: &day, UpdateTime: &day, Deleted: 0},
		{ID: 27, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "processing", Status: model.TaskStatusProcessing.Int16(), Priority: 10, CreateTime: &day, UpdateTime: &day, Deleted: 0},
		{ID: 28, TenantID: 10, StoreID: 100, Platform: "legacy", TargetPlatform: "shein", Region: "us", ProductID: "queued", Status: model.TaskStatusQueued.Int16(), Priority: 10, CreateTime: &day, UpdateTime: &day, Deleted: 0},
		{ID: 29, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "old", Status: model.TaskStatusPublished.Int16(), Priority: 10, CreateTime: &yesterday, UpdateTime: &yesterday, Deleted: 0},
		{ID: 30, TenantID: 10, StoreID: 200, Platform: "shein", Region: "us", ProductID: "other-store", Status: model.TaskStatusDraft.Int16(), Priority: 10, CreateTime: &day, UpdateTime: &day, Deleted: 0},
	})

	repo := NewGormImportTaskRepository(db)
	counts, err := repo.CountDailyDispatchUsage(context.Background(), "shein", 10, 100, day)
	if err != nil {
		t.Fatalf("CountDailyDispatchUsage() error = %v", err)
	}
	if counts.Completed != 2 || counts.Processing != 1 || counts.Queued != 1 {
		t.Fatalf("daily usage = %+v, want completed=2 processing=1 queued=1", counts)
	}
}

func TestRecordDispatchEventPersistsAuditFact(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	if err := AutoMigrateImportTaskRepository(db); err != nil {
		t.Fatalf("AutoMigrateImportTaskRepository() error = %v", err)
	}
	repo := NewGormImportTaskRepository(db)
	if err := repo.RecordDispatchEvent(context.Background(), DispatchEvent{
		TaskID:         31,
		TenantID:       10,
		StoreID:        100,
		Platform:       "shein",
		Action:         "skipped",
		ReasonCode:     "daily_limit_exhausted",
		Stage:          "dispatch",
		Capacity:       0,
		Queued:         2,
		Processing:     1,
		CompletedToday: 7,
		DailyLimit:     10,
		OwnerNode:      "node-a",
	}); err != nil {
		t.Fatalf("RecordDispatchEvent() error = %v", err)
	}

	var row listingDispatchEvent
	if err := db.Table("listing_dispatch_event").Where("task_id = ?", int64(31)).Take(&row).Error; err != nil {
		t.Fatalf("load dispatch event: %v", err)
	}
	if row.Action != "skipped" || row.ReasonCode != "daily_limit_exhausted" || row.CompletedToday != 7 || row.Processing != 1 || row.DailyLimit != 10 || row.OwnerNode != "node-a" {
		t.Fatalf("dispatch event = %+v, want persisted audit fact", row)
	}
}

func TestCountQueuedByStoreGroupsAcrossTenantsForPlatform(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 31, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "queued-a", Status: model.TaskStatusQueued.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 32, TenantID: 20, StoreID: 100, Platform: "legacy", TargetPlatform: "shein", Region: "us", ProductID: "queued-b", Status: model.TaskStatusQueued.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 33, TenantID: 10, StoreID: 200, Platform: "shein", Region: "us", ProductID: "queued-c", Status: model.TaskStatusQueued.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 34, TenantID: 10, StoreID: 200, Platform: "shein", Region: "us", ProductID: "pending", Status: model.TaskStatusPending.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 35, TenantID: 10, StoreID: 300, Platform: "amazon", Region: "us", ProductID: "wrong-platform", Status: model.TaskStatusQueued.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 36, TenantID: 10, StoreID: 400, Platform: "shein", TargetPlatform: "", Region: "us", ProductID: "empty-target-platform", Status: model.TaskStatusQueued.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})
	setNullTargetPlatforms(t, db, 31, 33, 34, 35)

	repo := NewGormImportTaskRepository(db)
	counts, err := repo.CountQueuedByStore(context.Background(), "shein", []int64{100, 200, 300, 400})
	if err != nil {
		t.Fatalf("CountQueuedByStore() error = %v", err)
	}
	if counts[100] != 2 || counts[200] != 1 {
		t.Fatalf("counts = %v, want store 100=2 and store 200=1", counts)
	}
	if _, ok := counts[300]; ok {
		t.Fatalf("counts includes store 300: %v, want wrong platform omitted", counts)
	}
	if _, ok := counts[400]; ok {
		t.Fatalf("counts includes store 400: %v, want empty target platform omitted", counts)
	}
}

func TestDispatchQueriesReturnEmptyForBlankPlatform(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 41, TenantID: 10, StoreID: 100, Platform: "", Region: "us", ProductID: "blank-candidate", Status: model.TaskStatusPending.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 42, TenantID: 10, StoreID: 100, Platform: "", Region: "us", ProductID: "blank-queued", Status: model.TaskStatusQueued.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})
	setNullTargetPlatforms(t, db, 41, 42)

	repo := NewGormImportTaskRepository(db)
	candidates, err := repo.ListDispatchCandidatesFair(context.Background(), DispatchCandidateRequest{
		Platform:      "  ",
		Limit:         10,
		PerStoreLimit: 1,
	})
	if err != nil {
		t.Fatalf("ListDispatchCandidatesFair(blank platform) error = %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("ListDispatchCandidatesFair(blank platform) len = %d, want 0", len(candidates))
	}

	counts, err := repo.CountQueuedByStore(context.Background(), "  ", []int64{100})
	if err != nil {
		t.Fatalf("CountQueuedByStore(blank platform) error = %v", err)
	}
	if len(counts) != 0 {
		t.Fatalf("CountQueuedByStore(blank platform) = %v, want empty map", counts)
	}
}

func TestAutoMigrateImportTaskRepositoryEnsuresProcessingNodeColumn(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&legacyImportTaskWithoutProcessingNode{}); err != nil {
		t.Fatalf("migrate legacy import task: %v", err)
	}
	if db.Migrator().HasColumn("listing_product_import_task", "processing_node") {
		t.Fatal("legacy table unexpectedly has processing_node before migration")
	}

	if err := AutoMigrateImportTaskRepository(db); err != nil {
		t.Fatalf("AutoMigrateImportTaskRepository() error = %v", err)
	}
	if !db.Migrator().HasColumn("listing_product_import_task", "processing_node") {
		t.Fatal("processing_node column missing after migration")
	}
	columnTypes, err := db.Migrator().ColumnTypes("listing_product_import_task")
	if err != nil {
		t.Fatalf("load column types: %v", err)
	}
	for _, columnType := range columnTypes {
		if columnType.Name() != "processing_node" {
			continue
		}
		if length, ok := columnType.Length(); !ok || length < 128 {
			t.Fatalf("processing_node length = %d, %v; want at least 128", length, ok)
		}
		return
	}
	t.Fatal("processing_node column type metadata missing after migration")
}

func newImportTaskDispatchTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingProductImportTask{}); err != nil {
		t.Fatalf("migrate import task: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("load sqlite handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	return db
}

func runConcurrently(t *testing.T, workers int, fn func(worker int)) {
	t.Helper()
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(workers)
	for worker := 0; worker < workers; worker++ {
		worker := worker
		go func() {
			defer wg.Done()
			<-start
			fn(worker)
		}()
	}
	close(start)
	wg.Wait()
}

func seedDispatchTasks(t *testing.T, db *gorm.DB, rows []listingProductImportTask) {
	t.Helper()
	for _, row := range rows {
		if err := db.Table("listing_product_import_task").Create(&row).Error; err != nil {
			t.Fatalf("seed row %d: %v", row.ID, err)
		}
	}
}

func setNullTargetPlatforms(t *testing.T, db *gorm.DB, ids ...int64) {
	t.Helper()
	if err := db.Table("listing_product_import_task").Where("id IN ?", ids).Update("target_platform", nil).Error; err != nil {
		t.Fatalf("set null target platforms: %v", err)
	}
}

func taskIDs(tasks []ImportTask) []int64 {
	ids := make([]int64, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	return ids
}

func equalInt64s(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
