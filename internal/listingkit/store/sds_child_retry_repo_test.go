package store

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
