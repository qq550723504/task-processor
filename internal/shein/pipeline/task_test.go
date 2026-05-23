package pipeline

import (
	"context"
	"testing"

	"task-processor/internal/listingkit/tenantctx"
	"task-processor/internal/model"
)

func TestInjectTaskTenantContextUsesTaskTenantID(t *testing.T) {
	ctx := injectTaskTenantContext(context.Background(), &model.Task{TenantID: 246})

	got := tenantctx.TenantIDFromContext(ctx)
	if got != "246" {
		t.Fatalf("tenant id = %q, want %q", got, "246")
	}
}

func TestInjectTaskTenantContextLeavesContextUnchangedWithoutTenantID(t *testing.T) {
	ctx := injectTaskTenantContext(context.Background(), &model.Task{})

	got := tenantctx.TenantIDFromContext(ctx)
	if got != tenantctx.DefaultTenantID {
		t.Fatalf("tenant id = %q, want %q", got, tenantctx.DefaultTenantID)
	}
}
