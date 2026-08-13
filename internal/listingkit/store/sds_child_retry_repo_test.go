package store

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/listingkit"
	"task-processor/internal/shared/tenantctx"
)

func TestSDSChildRetryRepositoryLocksTaskBeforeSelectingRetryRows(t *testing.T) {
	var logs bytes.Buffer
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{
		Logger: logger.New(log.New(&logs, "", 0), logger.Config{LogLevel: logger.Info}),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}, &listingkit.SDSChildRetryJob{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&listingkit.Task{ID: "task-lock-order", TenantID: "tenant-1"}).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := db.Create(&listingkit.SDSChildRetryJob{
		ID: "job-lock-order", TaskID: "task-lock-order", TenantID: "tenant-1",
		Kind: listingkit.SDSChildRetryKindDesignSync, Status: listingkit.SDSChildRetryJobStatusPending,
		NextRetryAt: time.Now().UTC(),
	}).Error; err != nil {
		t.Fatalf("create retry: %v", err)
	}
	logs.Reset()
	repo := any(NewTaskRepository(db)).(listingkit.SDSChildRetryJobRepository)
	if _, err := repo.ClaimDueSDSChildRetries(context.Background(), time.Now().UTC(), 1, "sweeper", time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("claim due retries: %v", err)
	}
	logText := logs.String()
	taskIndex := strings.Index(logText, "SELECT * FROM `listing_kit_tasks`")
	retryIndex := strings.Index(logText, "SELECT * FROM `listingkit_sds_child_retry_jobs`")
	if taskIndex < 0 || retryIndex < 0 || taskIndex > retryIndex {
		t.Fatalf("SQL lock order = task index %d, retry index %d; logs:\n%s", taskIndex, retryIndex, logText)
	}
}

func TestSDSChildRetryRepositoryLocksOnlyClaimPageTasks(t *testing.T) {
	var logs bytes.Buffer
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{
		Logger: logger.New(log.New(&logs, "", 0), logger.Config{LogLevel: logger.Info}),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}, &listingkit.SDSChildRetryJob{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, taskID := range []string{"task-page-a", "task-page-b"} {
		if err := db.Create(&listingkit.Task{ID: taskID, TenantID: "tenant-1"}).Error; err != nil {
			t.Fatalf("create task %s: %v", taskID, err)
		}
	}
	now := time.Date(2026, 7, 20, 7, 0, 0, 0, time.UTC)
	for _, job := range []listingkit.SDSChildRetryJob{
		{ID: "job-page-a", TaskID: "task-page-a", TenantID: "tenant-1", Kind: listingkit.SDSChildRetryKindDesignSync, Status: listingkit.SDSChildRetryJobStatusPending, NextRetryAt: now},
		{ID: "job-page-b", TaskID: "task-page-b", TenantID: "tenant-1", Kind: listingkit.SDSChildRetryKindDesignSync, Status: listingkit.SDSChildRetryJobStatusPending, NextRetryAt: now.Add(time.Minute)},
	} {
		if err := db.Create(&job).Error; err != nil {
			t.Fatalf("create retry %s: %v", job.ID, err)
		}
	}

	logs.Reset()
	repo := any(NewTaskRepository(db)).(listingkit.SDSChildRetryJobRepository)
	claimed, err := repo.ClaimDueSDSChildRetries(context.Background(), now, 1, "sweeper", now.Add(time.Hour))
	if err != nil {
		t.Fatalf("claim due retries: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != "job-page-a" {
		t.Fatalf("claimed jobs = %#v, want only the first claim-page job", claimed)
	}
	logText := logs.String()
	var parentLock string
	for _, line := range strings.Split(logText, "\n") {
		if strings.Contains(line, "SELECT * FROM") && strings.Contains(line, "listing_kit_tasks") {
			parentLock = line
			break
		}
	}
	if !strings.Contains(parentLock, "LIMIT 1") || !strings.Contains(logText, "listingkit_task_id IN (\"task-page-a\")") || strings.Contains(logText, "listingkit_task_id IN (\"task-page-b\")") {
		t.Fatalf("parent lock or retry page was not limited to the claim page; parent SQL: %s\nlogs:\n%s", parentLock, logText)
	}
}

func TestSDSChildRetryRepositoryPrioritizesEarliestDueTaskPage(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}, &listingkit.SDSChildRetryJob{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, taskID := range []string{"task-page-a", "task-page-z"} {
		if err := db.Create(&listingkit.Task{ID: taskID, TenantID: "tenant-1"}).Error; err != nil {
			t.Fatalf("create task %s: %v", taskID, err)
		}
	}
	now := time.Date(2026, 7, 20, 7, 0, 0, 0, time.UTC)
	for _, job := range []listingkit.SDSChildRetryJob{
		{ID: "job-page-a", TaskID: "task-page-a", TenantID: "tenant-1", Kind: listingkit.SDSChildRetryKindDesignSync, Status: listingkit.SDSChildRetryJobStatusPending, NextRetryAt: now.Add(10 * time.Minute)},
		{ID: "job-page-z", TaskID: "task-page-z", TenantID: "tenant-1", Kind: listingkit.SDSChildRetryKindDesignSync, Status: listingkit.SDSChildRetryJobStatusPending, NextRetryAt: now},
	} {
		if err := db.Create(&job).Error; err != nil {
			t.Fatalf("create retry %s: %v", job.ID, err)
		}
	}

	repo := any(NewTaskRepository(db)).(listingkit.SDSChildRetryJobRepository)
	cutoff := now.Add(20 * time.Minute)
	claimed, err := repo.ClaimDueSDSChildRetries(context.Background(), cutoff, 1, "sweeper", now.Add(time.Hour))
	if err != nil {
		t.Fatalf("claim due retries: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != "job-page-z" {
		t.Fatalf("claimed jobs = %#v, want earliest due job-page-z", claimed)
	}
}

func TestSDSChildRetryRepositorySchedulesOneActiveJobPerTask(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.SDSChildRetryJob{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo, ok := any(NewTaskRepository(db)).(listingkit.SDSChildRetryJobRepository)
	if !ok {
		t.Fatal("task repository does not implement SDSChildRetryJobRepository")
	}

	now := time.Date(2026, 7, 20, 7, 0, 0, 0, time.UTC)
	first, err := repo.ScheduleSDSChildRetry(context.Background(), &listingkit.SDSChildRetryJob{
		TaskID:      "task-1",
		TenantID:    "tenant-1",
		StoreID:     177,
		Kind:        listingkit.SDSChildRetryKindDesignSync,
		Attempt:     1,
		NextRetryAt: now.Add(time.Minute),
		ReasonCode:  "sds_oss_upload_timeout",
		Status:      listingkit.SDSChildRetryJobStatusPending,
	})
	if err != nil {
		t.Fatalf("schedule first retry: %v", err)
	}
	second, err := repo.ScheduleSDSChildRetry(context.Background(), &listingkit.SDSChildRetryJob{
		TaskID:      "task-1",
		TenantID:    "tenant-1",
		StoreID:     177,
		Kind:        listingkit.SDSChildRetryKindDesignSync,
		Attempt:     1,
		NextRetryAt: now.Add(time.Minute),
		ReasonCode:  "sds_oss_upload_timeout",
		Status:      listingkit.SDSChildRetryJobStatusPending,
	})
	if err != nil {
		t.Fatalf("schedule duplicate retry: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("duplicate job ID = %q, want %q", second.ID, first.ID)
	}

	var count int64
	if err := db.Model(&listingkit.SDSChildRetryJob{}).Count(&count).Error; err != nil {
		t.Fatalf("count retry jobs: %v", err)
	}
	if count != 1 {
		t.Fatalf("retry job count = %d, want 1", count)
	}
}

func TestSDSChildRetryRepositoryClaimsDueJobsOnceUntilLeaseExpires(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.SDSChildRetryJob{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo, ok := any(NewTaskRepository(db)).(listingkit.SDSChildRetryJobRepository)
	if !ok {
		t.Fatal("task repository does not implement SDSChildRetryJobRepository")
	}

	now := time.Date(2026, 7, 20, 7, 0, 0, 0, time.UTC)
	job, err := repo.ScheduleSDSChildRetry(context.Background(), &listingkit.SDSChildRetryJob{
		TaskID:      "task-1",
		TenantID:    "tenant-1",
		StoreID:     177,
		Kind:        listingkit.SDSChildRetryKindDesignSync,
		NextRetryAt: now,
		ReasonCode:  "sds_oss_upload_timeout",
		Status:      listingkit.SDSChildRetryJobStatusPending,
	})
	if err != nil {
		t.Fatalf("schedule retry: %v", err)
	}

	leaseUntil := now.Add(10 * time.Minute)
	claimed, err := repo.ClaimDueSDSChildRetries(context.Background(), now, 10, "sweeper-a", leaseUntil)
	if err != nil {
		t.Fatalf("claim due retries: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != job.ID {
		t.Fatalf("claimed jobs = %#v, want job %q", claimed, job.ID)
	}
	if claimed[0].LeaseOwner != "sweeper-a" || claimed[0].LeaseUntil == nil || !claimed[0].LeaseUntil.Equal(leaseUntil) {
		t.Fatalf("claimed lease = owner %q until %v", claimed[0].LeaseOwner, claimed[0].LeaseUntil)
	}

	again, err := repo.ClaimDueSDSChildRetries(context.Background(), now.Add(time.Minute), 10, "sweeper-b", now.Add(11*time.Minute))
	if err != nil {
		t.Fatalf("claim while leased: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("claimed while leased = %#v, want none", again)
	}

	again, err = repo.ClaimDueSDSChildRetries(context.Background(), leaseUntil, 10, "sweeper-b", leaseUntil.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("claim after lease expiry: %v", err)
	}
	if len(again) != 1 || again[0].LeaseOwner != "sweeper-b" {
		t.Fatalf("claimed after lease expiry = %#v", again)
	}
}

func TestSDSChildRetryRepositoryReactivatesTerminalJob(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.SDSChildRetryJob{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo, ok := any(NewTaskRepository(db)).(listingkit.SDSChildRetryJobRepository)
	if !ok {
		t.Fatal("task repository does not implement SDSChildRetryJobRepository")
	}

	first, err := repo.ScheduleSDSChildRetry(context.Background(), &listingkit.SDSChildRetryJob{
		TaskID:      "task-terminal",
		TenantID:    "tenant-1",
		Kind:        listingkit.SDSChildRetryKindDesignSync,
		Attempt:     1,
		NextRetryAt: time.Now().UTC().Add(time.Minute),
		Status:      listingkit.SDSChildRetryJobStatusPending,
	})
	if err != nil {
		t.Fatalf("schedule first retry: %v", err)
	}
	first.Status = listingkit.SDSChildRetryJobStatusExhausted
	first.Attempt = 3
	first.LastError = "old failure"
	if err := repo.SaveSDSChildRetry(context.Background(), first); err != nil {
		t.Fatalf("save exhausted retry: %v", err)
	}

	wantNextRetryAt := time.Now().UTC()
	reactivated, err := repo.ScheduleSDSChildRetry(context.Background(), &listingkit.SDSChildRetryJob{
		TaskID:      "task-terminal",
		TenantID:    "tenant-1",
		Kind:        listingkit.SDSChildRetryKindDesignSync,
		Attempt:     0,
		NextRetryAt: wantNextRetryAt,
		ReasonCode:  "manual_child_task_retry",
		LastError:   "manual retry queued",
		Status:      listingkit.SDSChildRetryJobStatusPending,
	})
	if err != nil {
		t.Fatalf("reactivate terminal retry: %v", err)
	}
	if reactivated.ID != first.ID || reactivated.Status != listingkit.SDSChildRetryJobStatusPending || reactivated.Attempt != 0 {
		t.Fatalf("reactivated job = %+v, want same pending job with reset attempt", reactivated)
	}
	if !reactivated.NextRetryAt.Equal(wantNextRetryAt) || reactivated.ReasonCode != "manual_child_task_retry" {
		t.Fatalf("reactivated scheduling = %+v, want fresh manual scheduling", reactivated)
	}
}

func TestSDSChildRetryRepositoryReactivatesCancelledJob(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.SDSChildRetryJob{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo, ok := any(NewTaskRepository(db)).(listingkit.SDSChildRetryJobRepository)
	if !ok {
		t.Fatal("task repository does not implement SDSChildRetryJobRepository")
	}

	first, err := repo.ScheduleSDSChildRetry(context.Background(), &listingkit.SDSChildRetryJob{
		TaskID: "task-cancelled", TenantID: "tenant-1", Kind: listingkit.SDSChildRetryKindDesignSync,
		Status: listingkit.SDSChildRetryJobStatusPending,
	})
	if err != nil {
		t.Fatalf("schedule first retry: %v", err)
	}
	first.Status = listingkit.SDSChildRetryJobStatusCancelled
	if err := repo.SaveSDSChildRetry(context.Background(), first); err != nil {
		t.Fatalf("save cancelled retry: %v", err)
	}

	reactivated, err := repo.ScheduleSDSChildRetry(context.Background(), &listingkit.SDSChildRetryJob{
		TaskID: "task-cancelled", TenantID: "tenant-1", Kind: listingkit.SDSChildRetryKindDesignSync,
		ReasonCode: "manual_child_task_retry", Status: listingkit.SDSChildRetryJobStatusPending,
	})
	if err != nil {
		t.Fatalf("reactivate cancelled retry: %v", err)
	}
	if reactivated.ID != first.ID || reactivated.Status != listingkit.SDSChildRetryJobStatusPending || reactivated.ReasonCode != "manual_child_task_retry" {
		t.Fatalf("reactivated job = %+v, want same pending job with fresh reason", reactivated)
	}
}

func TestSDSChildRetryRepositoryCoordinatesRepairWithActiveLease(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.SDSChildRetryJob{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := any(NewTaskRepository(db)).(listingkit.SDSChildRetryRepairCoordinator)
	jobRepo := any(NewTaskRepository(db)).(listingkit.SDSChildRetryJobRepository)
	job, err := jobRepo.ScheduleSDSChildRetry(context.Background(), &listingkit.SDSChildRetryJob{
		TaskID: "task-repair-coordination", TenantID: "tenant-1", Kind: listingkit.SDSChildRetryKindDesignSync,
		Status: listingkit.SDSChildRetryJobStatusPending,
	})
	if err != nil {
		t.Fatalf("schedule retry: %v", err)
	}
	leaseUntil := time.Now().UTC().Add(time.Hour)
	if err := db.Model(&listingkit.SDSChildRetryJob{}).Where("id = ?", job.ID).Updates(map[string]any{
		"lease_owner": "sweeper", "lease_until": leaseUntil,
	}).Error; err != nil {
		t.Fatalf("set active lease: %v", err)
	}
	if _, err := repo.BeginSDSChildRetryRepair(context.Background(), job.TaskID, job.Kind); err != listingkit.ErrSDSRepairRetryInProgress {
		t.Fatalf("begin repair with active lease error = %v, want ErrSDSRepairRetryInProgress", err)
	}
	if err := db.Model(&listingkit.SDSChildRetryJob{}).Where("id = ?", job.ID).Updates(map[string]any{
		"lease_until": time.Now().UTC().Add(-time.Minute),
	}).Error; err != nil {
		t.Fatalf("expire lease: %v", err)
	}
	lease, err := repo.BeginSDSChildRetryRepair(context.Background(), job.TaskID, job.Kind)
	if err != nil {
		t.Fatalf("begin repair after lease expiry: %v", err)
	}
	if err := repo.EndSDSChildRetryRepair(context.Background(), lease); err != nil {
		t.Fatalf("end repair after lease expiry: %v", err)
	}
	var after listingkit.SDSChildRetryJob
	if err := db.Where("id = ?", job.ID).First(&after).Error; err != nil {
		t.Fatalf("reload retry: %v", err)
	}
	if after.Status != listingkit.SDSChildRetryJobStatusCancelled || after.LeaseOwner != "" || after.LeaseUntil != nil {
		t.Fatalf("retry after repair preparation = %+v, want cancelled without lease", after)
	}
}

func TestSDSChildRetryRepositoryHoldsRepairLeaseUntilReleased(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}, &listingkit.SDSChildRetryJob{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&listingkit.Task{ID: "task-repair-lease", TenantID: "tenant-1"}).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	repo := any(NewTaskRepository(db)).(listingkit.SDSChildRetryRepairCoordinator)
	jobRepo := any(NewTaskRepository(db)).(listingkit.SDSChildRetryJobRepository)

	lease, err := repo.BeginSDSChildRetryRepair(context.Background(), "task-repair-lease", listingkit.SDSChildRetryKindDesignSync)
	if err != nil {
		t.Fatalf("begin repair: %v", err)
	}
	if lease == nil || lease.JobID == "" {
		t.Fatalf("repair lease = %+v, want durable lease", lease)
	}
	if _, err := jobRepo.ScheduleSDSChildRetry(context.Background(), &listingkit.SDSChildRetryJob{
		TaskID: "task-repair-lease", TenantID: "tenant-1", Kind: listingkit.SDSChildRetryKindCatalogProduct,
		Status: listingkit.SDSChildRetryJobStatusPending,
	}); err != listingkit.ErrSDSRepairRetryInProgress {
		t.Fatalf("schedule during repair error = %v, want ErrSDSRepairRetryInProgress", err)
	}
	if err := repo.EndSDSChildRetryRepair(context.Background(), lease); err != nil {
		t.Fatalf("end repair: %v", err)
	}
	if _, err := jobRepo.ScheduleSDSChildRetry(context.Background(), &listingkit.SDSChildRetryJob{
		TaskID: "task-repair-lease", TenantID: "tenant-1", Kind: listingkit.SDSChildRetryKindCatalogProduct,
		Status: listingkit.SDSChildRetryJobStatusPending,
	}); err != nil {
		t.Fatalf("schedule after repair: %v", err)
	}
}

func TestSDSChildRetryRepositoryRefillsClaimPageAfterActiveSibling(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.SDSChildRetryJob{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := any(NewTaskRepository(db)).(listingkit.SDSChildRetryJobRepository)
	now := time.Now().UTC()
	active, err := repo.ScheduleSDSChildRetry(context.Background(), &listingkit.SDSChildRetryJob{
		TaskID: "task-active-sibling", TenantID: "tenant-1", Kind: listingkit.SDSChildRetryKindCatalogProduct,
		NextRetryAt: now, Status: listingkit.SDSChildRetryJobStatusPending,
	})
	if err != nil {
		t.Fatalf("schedule active sibling: %v", err)
	}
	if _, err := repo.ScheduleSDSChildRetry(context.Background(), &listingkit.SDSChildRetryJob{
		TaskID: "task-active-sibling", TenantID: "tenant-1", Kind: listingkit.SDSChildRetryKindDesignSync,
		NextRetryAt: now, Status: listingkit.SDSChildRetryJobStatusPending,
	}); err != nil {
		t.Fatalf("schedule blocked sibling: %v", err)
	}
	if _, err := repo.ScheduleSDSChildRetry(context.Background(), &listingkit.SDSChildRetryJob{
		TaskID: "task-unrelated", TenantID: "tenant-1", Kind: listingkit.SDSChildRetryKindDesignSync,
		NextRetryAt: now, Status: listingkit.SDSChildRetryJobStatusPending,
	}); err != nil {
		t.Fatalf("schedule unrelated retry: %v", err)
	}
	leaseUntil := now.Add(time.Hour)
	if err := db.Model(&listingkit.SDSChildRetryJob{}).Where("id = ?", active.ID).Updates(map[string]any{
		"lease_owner": "sweeper", "lease_until": leaseUntil,
	}).Error; err != nil {
		t.Fatalf("set active sibling lease: %v", err)
	}
	claimed, err := repo.ClaimDueSDSChildRetries(context.Background(), now, 1, "replacement", now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("claim due retries: %v", err)
	}
	if len(claimed) != 1 || claimed[0].TaskID != "task-unrelated" {
		t.Fatalf("claimed jobs = %#v, want unrelated task to fill page", claimed)
	}
}

func TestSDSChildRetryRepositoryListsLegacyDefaultTenantJobs(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.SDSChildRetryJob{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&listingkit.SDSChildRetryJob{ID: "legacy-default-retry", TaskID: "task-legacy-default", Kind: listingkit.SDSChildRetryKindDesignSync, Status: listingkit.SDSChildRetryJobStatusPending}).Error; err != nil {
		t.Fatalf("create legacy retry: %v", err)
	}
	if err := db.Create(&listingkit.SDSChildRetryJob{ID: "other-tenant-retry", TaskID: "task-other-tenant", TenantID: "tenant-1", Kind: listingkit.SDSChildRetryKindDesignSync, Status: listingkit.SDSChildRetryJobStatusPending}).Error; err != nil {
		t.Fatalf("create other retry: %v", err)
	}
	repo := any(NewTaskRepository(db)).(listingkit.SDSChildRetryJobStatusSource)
	jobs, err := repo.ListSDSChildRetries(tenantctx.WithTenantID(context.Background(), tenantctx.DefaultTenantID), "task-legacy-default")
	if err != nil {
		t.Fatalf("list legacy retries: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != "legacy-default-retry" {
		t.Fatalf("legacy retries = %#v, want legacy default job", jobs)
	}
}

func TestSDSChildRetryRepositoryDoesNotOverwriteActiveLease(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.SDSChildRetryJob{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo, ok := any(NewTaskRepository(db)).(listingkit.SDSChildRetryJobRepository)
	if !ok {
		t.Fatal("task repository does not implement SDSChildRetryJobRepository")
	}
	first, err := repo.ScheduleSDSChildRetry(context.Background(), &listingkit.SDSChildRetryJob{
		TaskID:   "task-race",
		TenantID: "tenant-1",
		Kind:     listingkit.SDSChildRetryKindDesignSync,
		Status:   listingkit.SDSChildRetryJobStatusPending,
	})
	if err != nil {
		t.Fatalf("schedule first retry: %v", err)
	}
	first.Status = listingkit.SDSChildRetryJobStatusExhausted
	if err := repo.SaveSDSChildRetry(context.Background(), first); err != nil {
		t.Fatalf("save exhausted retry: %v", err)
	}

	leaseUntil := time.Now().UTC().Add(10 * time.Minute)
	if err := db.Model(&listingkit.SDSChildRetryJob{}).Where("id = ?", first.ID).Updates(map[string]any{
		"status":      listingkit.SDSChildRetryJobStatusPending,
		"lease_owner": "sweeper",
		"lease_until": leaseUntil,
	}).Error; err != nil {
		t.Fatalf("claim retry job: %v", err)
	}

	got, err := repo.ScheduleSDSChildRetry(context.Background(), &listingkit.SDSChildRetryJob{
		TaskID: "task-race", TenantID: "tenant-1", Kind: listingkit.SDSChildRetryKindDesignSync,
		Status: listingkit.SDSChildRetryJobStatusPending, ReasonCode: "manual_child_task_retry",
	})
	if err != nil {
		t.Fatalf("schedule concurrent retry: %v", err)
	}
	if got.Status != listingkit.SDSChildRetryJobStatusPending || got.LeaseOwner != "sweeper" || got.LeaseUntil == nil {
		t.Fatalf("job after concurrent claim = %+v, want pending job with existing lease", got)
	}
}

func TestMemSDSChildRetryRepositoryReactivatesTerminalJob(t *testing.T) {
	repo, ok := NewMemTaskRepository().(listingkit.SDSChildRetryJobRepository)
	if !ok {
		t.Fatal("memory task repository does not implement SDSChildRetryJobRepository")
	}

	first, err := repo.ScheduleSDSChildRetry(context.Background(), &listingkit.SDSChildRetryJob{
		TaskID:   "mem-task-terminal",
		TenantID: "tenant-1",
		Kind:     listingkit.SDSChildRetryKindDesignSync,
		Status:   listingkit.SDSChildRetryJobStatusPending,
	})
	if err != nil {
		t.Fatalf("schedule first retry: %v", err)
	}
	first.Status = listingkit.SDSChildRetryJobStatusCompleted
	if err := repo.SaveSDSChildRetry(context.Background(), first); err != nil {
		t.Fatalf("save completed retry: %v", err)
	}

	reactivated, err := repo.ScheduleSDSChildRetry(context.Background(), &listingkit.SDSChildRetryJob{
		TaskID:     "mem-task-terminal",
		TenantID:   "tenant-1",
		Kind:       listingkit.SDSChildRetryKindDesignSync,
		ReasonCode: "manual_child_task_retry",
		Status:     listingkit.SDSChildRetryJobStatusPending,
	})
	if err != nil {
		t.Fatalf("reactivate completed retry: %v", err)
	}
	if reactivated.ID != first.ID || reactivated.Status != listingkit.SDSChildRetryJobStatusPending || reactivated.ReasonCode != "manual_child_task_retry" {
		t.Fatalf("reactivated job = %+v, want same pending job with fresh reason", reactivated)
	}
}

func TestMemSDSChildRetryRepositoryClaimsAtMostOneJobPerTask(t *testing.T) {
	repo, ok := NewMemTaskRepository().(listingkit.SDSChildRetryJobRepository)
	if !ok {
		t.Fatal("memory task repository does not implement SDSChildRetryJobRepository")
	}
	now := time.Now().UTC()
	for _, kind := range []listingkit.SDSChildRetryKind{
		listingkit.SDSChildRetryKindDesignSync,
		listingkit.SDSChildRetryKindCatalogProduct,
	} {
		if _, err := repo.ScheduleSDSChildRetry(context.Background(), &listingkit.SDSChildRetryJob{
			TaskID: "mem-task-serialized", TenantID: "tenant-1", Kind: kind,
			NextRetryAt: now, Status: listingkit.SDSChildRetryJobStatusPending,
		}); err != nil {
			t.Fatalf("schedule %s retry: %v", kind, err)
		}
	}

	claimed, err := repo.ClaimDueSDSChildRetries(context.Background(), now, 10, "sweeper-a", now.Add(time.Hour))
	if err != nil {
		t.Fatalf("claim due retries: %v", err)
	}
	if len(claimed) != 1 || claimed[0].TaskID != "mem-task-serialized" {
		t.Fatalf("claimed jobs = %#v, want exactly one job for the parent task", claimed)
	}

	again, err := repo.ClaimDueSDSChildRetries(context.Background(), now, 10, "sweeper-b", now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("claim sibling while first job is leased: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("claimed sibling jobs = %#v, want none while the parent task has an active lease", again)
	}
}
