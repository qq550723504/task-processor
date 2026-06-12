package pipeline

import (
	"context"
	"testing"

	"task-processor/internal/model"
	sharedtenantctx "task-processor/internal/shared/tenantctx"
)

func TestInjectTaskTenantContextUsesTaskTenantID(t *testing.T) {
	ctx := injectTaskTenantContext(context.Background(), &model.Task{TenantID: 246})

	got := sharedtenantctx.TenantIDFromContext(ctx)
	if got != "246" {
		t.Fatalf("tenant id = %q, want %q", got, "246")
	}
}

func TestInjectTaskTenantContextLeavesContextUnchangedWithoutTenantID(t *testing.T) {
	ctx := injectTaskTenantContext(context.Background(), &model.Task{})

	got := sharedtenantctx.TenantIDFromContext(ctx)
	if got != sharedtenantctx.DefaultTenantID {
		t.Fatalf("tenant id = %q, want %q", got, sharedtenantctx.DefaultTenantID)
	}
}
