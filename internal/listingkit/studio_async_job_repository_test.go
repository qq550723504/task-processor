package listingkit

import (
	"context"
	"testing"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGormStudioAsyncJobRepositoryScopesByTenant(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := AutoMigrateStudioAsyncJobRepository(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := NewGormStudioAsyncJobRepository(db)
	ctxA := WithTenantID(context.Background(), "tenant-a")
	ctxB := WithTenantID(context.Background(), "tenant-b")
	now := time.Now().UTC()

	if err := repo.CreateStudioAsyncJob(ctxA, &StudioAsyncJobRecord{
		ID:        "job-a",
		Path:      "/studio/designs",
		Status:    StudioAsyncJobStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	if _, err := repo.GetStudioAsyncJob(ctxA, "job-a"); err != nil {
		t.Fatalf("get same-tenant job: %v", err)
	}
	if _, err := repo.GetStudioAsyncJob(ctxB, "job-a"); err == nil {
		t.Fatal("expected cross-tenant lookup to fail")
	}
}

func TestGormStudioAsyncJobRepositoryScopesByUserWhenOwnerScopeEnabled(t *testing.T) {
	t.Parallel()

	restore := SetOwnerScopeRequiredForTesting(true)
	defer restore()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := AutoMigrateStudioAsyncJobRepository(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := NewGormStudioAsyncJobRepository(db)
	baseCtx := WithTenantID(context.Background(), "tenant-a")
	ctxUserA := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-a"})
	ctxUserB := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-b"})
	now := time.Now().UTC()

	if err := repo.CreateStudioAsyncJob(ctxUserA, &StudioAsyncJobRecord{
		ID:        "job-a",
		Path:      "/studio/designs",
		Status:    StudioAsyncJobStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	if _, err := repo.GetStudioAsyncJob(ctxUserA, "job-a"); err != nil {
		t.Fatalf("get same-user job: %v", err)
	}
	if _, err := repo.GetStudioAsyncJob(ctxUserB, "job-a"); err == nil {
		t.Fatal("expected cross-user lookup to fail")
	}
}
