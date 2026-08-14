package listingkit

import (
	"context"
	"testing"

	studiodomain "task-processor/internal/listing/studio"
)

func TestStudioBatchRunAdapterPersistsTenantAdminAccess(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRunRepository()
	ctx := WithRequestRoles(
		WithRequestIdentity(context.Background(), RequestIdentity{TenantID: "101", UserID: "admin-1"}),
		[]string{"listingkit_admin"},
	)
	adapter := studioBatchRunRepositoryAdapter{repo: repo}
	run := &studiodomain.BatchRunRecord{
		ID:            "run-admin",
		Mode:          string(StudioBatchRunModeCreateTasks),
		FailurePolicy: string(StudioBatchRunFailurePolicyContinueOnError),
		Status:        string(StudioBatchRunStatusPending),
		TotalBatches:  1,
	}

	if err := adapter.CreateBatchRun(ctx, run, nil); err != nil {
		t.Fatalf("CreateBatchRun() error = %v", err)
	}
	stored, err := repo.GetStudioBatchRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetStudioBatchRun() error = %v", err)
	}
	if !stored.TenantAdminAccess {
		t.Fatalf("stored run = %+v, want persisted tenant-admin access", stored)
	}
}

func TestWithStudioBatchRunIdentityRestoresTenantAdminAccess(t *testing.T) {
	t.Parallel()

	ctx := withStudioBatchRunIdentity(context.Background(), &StudioBatchRunRecord{
		TenantID:          "101",
		UserID:            "admin-1",
		TenantAdminAccess: true,
	})
	if !RequestHasTenantAdminAccess(ctx) {
		t.Fatalf("restored roles = %v, want tenant-admin access", RequestRolesFromContext(ctx))
	}
}
