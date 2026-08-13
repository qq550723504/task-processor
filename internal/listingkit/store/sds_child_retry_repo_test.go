≠rá^—f•ñÿ¶{O,y 'v√Æ∂õ≠package store

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

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
	if err := repo.PrepareSDSChildRetryRepair(context.Background(), job.TaskID, job.Kind); err != listingkit.ErrSDSRepairRetryInProgress {
		t.Fatalf("prepare repair with active lease error = %v, want ErrSDSRepairRetryInProgress", err)
	}
	if err := db.Model(&listingkit.SDSChildRetryJob{}).Where("id = ?", job.ID).Updates(map[string]any{
		"lease_until": time.Now().UTC().Add(-time.Minute),
	}).Error; err != nil {
		t.Fatalf("expire lease: %v", err)
	}
	if err := repo.PrepareSDSChildRetryRepair(context.Background(), job.TaskID, job.Kind); err != nil {
		t.Fatalf("prepare repair after lease expiry: %v", err)
	}
	var after listingkit.SDSChildRetryJob
	if err := db.Where("id = ?", job.ID).First(&after).Error; err != nil {
		t.Fatalf("reload retry: %v", err)
	}
	if after.Status != listingkit.SDSChildRetryJobStatusCancelled || after.LeaseOwner != "" || after.LeaseUntil != nil {
		t.Fatalf("retry after repair preparation = %+v, want cancelled without lease", after)
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
}
