package listingkit

import (
	"context"
	"testing"
)

func TestWithSheinSubmitTaskIdentityRestoresTenantAdminStoreAccess(t *testing.T) {
	task := &Task{
		TenantID: "101",
		UserID:   "user-1",
		SheinStoreResolutionSnapshot: &SheinStoreResolutionSnapshot{
			StoreID:           202,
			TenantAdminAccess: true,
		},
	}

	ctx, err := withSheinSubmitTaskIdentity(context.Background(), task)
	if err != nil {
		t.Fatalf("withSheinSubmitTaskIdentity() error = %v", err)
	}
	if !RequestHasTenantAdminAccess(ctx) {
		t.Fatal("restored task context does not have tenant-admin store access")
	}
}

func TestLoadTaskExecutionContextRestoresTenantAdminStoreAccess(t *testing.T) {
	repo := &stubSubmitRepo{task: &Task{
		ID:       "task-1",
		TenantID: "101",
		UserID:   "user-1",
		Request:  &GenerateRequest{UserID: "user-1"},
		SheinStoreResolutionSnapshot: &SheinStoreResolutionSnapshot{
			StoreID:           202,
			TenantAdminAccess: true,
		},
	}}

	ctx, _, err := (&service{repo: repo}).loadTaskExecutionContext(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("loadTaskExecutionContext() error = %v", err)
	}
	if !RequestHasTenantAdminAccess(ctx) {
		t.Fatal("loaded task context does not have tenant-admin store access")
	}
}
