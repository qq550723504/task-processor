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
