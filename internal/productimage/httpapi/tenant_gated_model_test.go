package httpapi

import (
	"context"
	"testing"

	productimage "task-processor/internal/productimage"
)

func TestTenantAllowlistedFaithfulEditorRejectsBeforeProviderCall(t *testing.T) {
	called := false
	editor := &faithfulEditorCapture{called: &called}
	gated := &tenantAllowlistedFaithfulEditor{inner: editor, allowed: tenantIDSet([]string{"tenant-a"})}

	_, err := gated.Edit(context.Background(), &productimage.FaithfulEditRequest{})
	if err == nil || !productimage.IsNoRetryError(err) {
		t.Fatalf("Edit() error = %v, want no-retry error", err)
	}
	if called {
		t.Fatal("faithful editor was called for a denied tenant")
	}
}

func TestTenantAllowlistedFaithfulEditorAllowsConfiguredTenant(t *testing.T) {
	called := false
	editor := &faithfulEditorCapture{called: &called}
	gated := &tenantAllowlistedFaithfulEditor{inner: editor, allowed: tenantIDSet([]string{"tenant-a"})}
	ctx := productimage.WithAIIdentity(context.Background(), productimage.AIIdentity{TenantID: "tenant-a"})

	if _, err := gated.Edit(ctx, &productimage.FaithfulEditRequest{}); err != nil {
		t.Fatalf("Edit() error = %v", err)
	}
	if !called {
		t.Fatal("faithful editor was not called for an allowlisted tenant")
	}
}

type faithfulEditorCapture struct {
	called *bool
}

func (e *faithfulEditorCapture) Edit(context.Context, *productimage.FaithfulEditRequest) (*productimage.FaithfulEditResult, error) {
	*e.called = true
	return &productimage.FaithfulEditResult{}, nil
}
