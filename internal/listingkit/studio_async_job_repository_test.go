package listingkit

import (
	"context"
	"errors"
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

func TestMemStudioAsyncJobRepositoryRejectsHeartbeatForTerminalJob(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioAsyncJobRepository()
	ctx := WithTenantID(context.Background(), "tenant-terminal-heartbeat")
	now := time.Now().UTC()
	if err := repo.CreateStudioAsyncJob(ctx, &StudioAsyncJobRecord{
		ID: "terminal-heartbeat-job", TenantID: "tenant-terminal-heartbeat", Path: "/studio/product-images",
		Status: StudioAsyncJobStatusFailed, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	if err := repo.HeartbeatStudioAsyncJob(ctx, "terminal-heartbeat-job", now.Add(time.Minute)); !errors.Is(err, ErrStudioAsyncJobLeaseLost) {
		t.Fatalf("heartbeat error = %v, want ErrStudioAsyncJobLeaseLost", err)
	}
}

func TestStudioAsyncJobRepositoryConditionalRecoveryKeepsHeartbeatOwner(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioAsyncJobRepository()
	ctx := WithTenantID(context.Background(), "tenant-conditional-recovery")
	old := time.Now().UTC().Add(-time.Hour)
	if err := repo.CreateStudioAsyncJob(ctx, &StudioAsyncJobRecord{
		ID: "conditional-recovery-job", TenantID: "tenant-conditional-recovery", Path: "/studio/product-images",
		Status: StudioAsyncJobStatusRunning, CreatedAt: old, UpdatedAt: old,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}
	heartbeatAt := old.Add(time.Minute)
	if err := repo.HeartbeatStudioAsyncJob(ctx, "conditional-recovery-job", heartbeatAt); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}

	recovery := &StudioAsyncJobRecord{
		ID: "conditional-recovery-job", TenantID: "tenant-conditional-recovery", Path: "/studio/product-images",
		Status: StudioAsyncJobStatusFailed, Error: "stale", UpstreamStatus: 500,
		UpdatedAt: time.Now().UTC(),
	}
	claimed, err := repo.UpdateStudioAsyncJobIfRunningSinceForTenant(ctx, "tenant-conditional-recovery", "conditional-recovery-job", old, recovery)
	if err != nil {
		t.Fatalf("conditional recovery: %v", err)
	}
	if claimed {
		t.Fatal("conditional recovery claimed a job after its heartbeat changed")
	}
	job, err := repo.GetStudioAsyncJobForTenant(ctx, "tenant-conditional-recovery", "conditional-recovery-job")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if job.Status != StudioAsyncJobStatusRunning || !job.UpdatedAt.Equal(heartbeatAt) {
		t.Fatalf("job = %+v, want running with heartbeat retained", job)
	}
}

func TestStudioAsyncJobRepositoryRejectsSuccessAfterLeaseLoss(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioAsyncJobRepository()
	ctx := WithTenantID(context.Background(), "tenant-success-lease")
	now := time.Now().UTC()
	if err := repo.CreateStudioAsyncJob(ctx, &StudioAsyncJobRecord{
		ID: "success-lease-job", TenantID: "tenant-success-lease", Path: "/studio/product-images",
		Status: StudioAsyncJobStatusFailed, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	err := repo.UpdateStudioAsyncJobIfRunning(ctx, &StudioAsyncJobRecord{
		ID: "success-lease-job", Status: StudioAsyncJobStatusSucceeded, UpdatedAt: now.Add(time.Minute),
	})
	if !errors.Is(err, ErrStudioAsyncJobLeaseLost) {
		t.Fatalf("conditional success error = %v, want ErrStudioAsyncJobLeaseLost", err)
	}
}

func TestGormStudioAsyncJobRepositoryHeartbeatUpdatesRunningJob(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := AutoMigrateStudioAsyncJobRepository(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := NewGormStudioAsyncJobRepository(db)
	ctx := WithTenantID(context.Background(), "tenant-heartbeat")
	createdAt := time.Now().UTC().Add(-time.Minute)
	if err := repo.CreateStudioAsyncJob(ctx, &StudioAsyncJobRecord{
		ID: "heartbeat-job", TenantID: "tenant-heartbeat", Path: "/studio/product-images",
		Status: StudioAsyncJobStatusRunning, CreatedAt: createdAt, UpdatedAt: createdAt,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	updatedAt := time.Now().UTC()
	if err := repo.HeartbeatStudioAsyncJob(ctx, "heartbeat-job", updatedAt); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	job, err := repo.GetStudioAsyncJob(ctx, "heartbeat-job")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if !job.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("UpdatedAt = %s, want %s", job.UpdatedAt, updatedAt)
	}
}

func TestGormStudioAsyncJobRepositoryConditionalRecoveryRequiresObservedHeartbeat(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := AutoMigrateStudioAsyncJobRepository(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := NewGormStudioAsyncJobRepository(db)
	ctx := WithTenantID(context.Background(), "tenant-conditional-gorm")
	old := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	if err := repo.CreateStudioAsyncJob(ctx, &StudioAsyncJobRecord{
		ID: "conditional-gorm-job", TenantID: "tenant-conditional-gorm", Path: "/studio/product-images",
		Status: StudioAsyncJobStatusRunning, CreatedAt: old, UpdatedAt: old,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}
	heartbeatAt := old.Add(time.Minute)
	if err := repo.HeartbeatStudioAsyncJob(ctx, "conditional-gorm-job", heartbeatAt); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}

	claimed, err := repo.UpdateStudioAsyncJobIfRunningSinceForTenant(ctx, "tenant-conditional-gorm", "conditional-gorm-job", old, &StudioAsyncJobRecord{
		ID: "conditional-gorm-job", TenantID: "tenant-conditional-gorm", Path: "/studio/product-images",
		Status: StudioAsyncJobStatusFailed, Error: "stale", UpstreamStatus: 500, UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("conditional recovery: %v", err)
	}
	if claimed {
		t.Fatal("conditional recovery claimed a job after its heartbeat changed")
	}
	job, err := repo.GetStudioAsyncJobForTenant(ctx, "tenant-conditional-gorm", "conditional-gorm-job")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if job.Status != StudioAsyncJobStatusRunning || !job.UpdatedAt.Equal(heartbeatAt) {
		t.Fatalf("job = %+v, want running with heartbeat retained", job)
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

func TestGormStudioAsyncJobRepositoryTenantRecoveryBypassesUserScope(t *testing.T) {
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
	ctxA := WithRequestIdentity(WithTenantID(context.Background(), "tenant-a"), RequestIdentity{TenantID: "tenant-a", UserID: "user-a"})
	ctxB := WithRequestIdentity(WithTenantID(context.Background(), "tenant-a"), RequestIdentity{TenantID: "tenant-a", UserID: "user-b"})
	old := time.Now().UTC().Add(-time.Hour)
	if err := repo.CreateStudioAsyncJob(ctxA, &StudioAsyncJobRecord{
		ID: "job-recovery", TenantID: "tenant-a", UserID: "user-a", Path: "/studio/product-images",
		Status: StudioAsyncJobStatusRunning, CreatedAt: old, UpdatedAt: old,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}
	if _, err := repo.GetStudioAsyncJob(ctxB, "job-recovery"); err == nil {
		t.Fatal("expected owner-scoped lookup to fail for another user")
	}
	recovered, err := repo.GetStudioAsyncJobForTenant(ctxB, "tenant-a", "job-recovery")
	if err != nil {
		t.Fatalf("tenant recovery lookup: %v", err)
	}
	recovered.Status = StudioAsyncJobStatusFailed
	recovered.UpdatedAt = time.Now().UTC()
	if err := repo.UpdateStudioAsyncJobForTenant(ctxB, "tenant-a", recovered); err != nil {
		t.Fatalf("tenant recovery update: %v", err)
	}
	check, err := repo.GetStudioAsyncJobForTenant(ctxB, "tenant-a", "job-recovery")
	if err != nil {
		t.Fatalf("tenant recovery reread: %v", err)
	}
	if check.Status != StudioAsyncJobStatusFailed {
		t.Fatalf("recovered status = %q, want failed", check.Status)
	}
}
