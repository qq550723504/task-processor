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
	if !productimage.IsTenantModelAccessDenied(err) {
		t.Fatalf("Edit() error = %v, want tenant gate error", err)
	}
	if called {
		t.Fatal("faithful editor was called for a denied tenant")
	}
}

func TestTenantAllowlistedFaithfulEditorAllowsConfiguredTenant(t *testing.T) {
	called := false
	editor := &faithfulEditorCapture{called: &called}
	gated := &tenantAllowlistedFaithfulEditor{inner: editor, allowed: tenantIDSet([]string{"tenant-a"})}
	ctx := productimage.WithAIIdentity(context.Background(), productimage.AIIdentity{TenantID: "tenant-a", UserID: "user-a"})

	if _, err := gated.Edit(ctx, &productimage.FaithfulEditRequest{}); err != nil {
		t.Fatalf("Edit() error = %v", err)
	}
	if !called {
		t.Fatal("faithful editor was not called for an allowlisted tenant")
	}
}

func TestTenantAllowlistedFaithfulEditorRequiresUserIdentity(t *testing.T) {
	called := false
	editor := &faithfulEditorCapture{called: &called}
	gated := &tenantAllowlistedFaithfulEditor{inner: editor, allowed: tenantIDSet([]string{"tenant-a"})}
	ctx := productimage.WithAIIdentity(context.Background(), productimage.AIIdentity{TenantID: "tenant-a"})

	_, err := gated.Edit(ctx, &productimage.FaithfulEditRequest{})
	if !productimage.IsTenantModelAccessDenied(err) {
		t.Fatalf("Edit() error = %v, want tenant gate error", err)
	}
	if called {
		t.Fatal("faithful editor was called without user identity")
	}
}

func TestTenantAllowlistedContextAnalyzerSkipsDeniedProvider(t *testing.T) {
	called := false
	analyzer := &contextAnalyzerCapture{called: &called}
	gated := &tenantAllowlistedContextAnalyzer{inner: analyzer, allowed: tenantIDSet([]string{"tenant-a"})}

	ctx := productimage.WithAIIdentity(context.Background(), productimage.AIIdentity{TenantID: "tenant-b", UserID: "user-b"})
	got, err := gated.Analyze(ctx, &productimage.SourceBundle{})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if got == nil || called {
		t.Fatalf("Analyze() result/call = (%+v, %v), want empty context without provider call", got, called)
	}
}

func TestTenantAllowlistedSceneGeneratorFallsBackBeforeGovernedCall(t *testing.T) {
	called := false
	generator := &sceneGeneratorCapture{called: &called}
	gated := &tenantAllowlistedSceneGenerator{inner: generator, allowed: tenantIDSet([]string{"tenant-a"})}

	_, err := gated.GenerateScene(context.Background(), &productimage.SceneGenerationRequest{})
	if !productimage.IsTenantModelAccessDenied(err) {
		t.Fatalf("GenerateScene() error = %v, want tenant gate error", err)
	}
	if called {
		t.Fatal("scene generator was called for a denied tenant")
	}
}

type faithfulEditorCapture struct {
	called *bool
}

func (e *faithfulEditorCapture) Edit(context.Context, *productimage.FaithfulEditRequest) (*productimage.FaithfulEditResult, error) {
	*e.called = true
	return &productimage.FaithfulEditResult{}, nil
}

func (e *faithfulEditorCapture) EditWithRoute(ctx context.Context, req *productimage.FaithfulEditRequest, _ productimage.FaithfulEditRoute) (*productimage.FaithfulEditResult, error) {
	return e.Edit(ctx, req)
}

type sceneGeneratorCapture struct {
	called *bool
}

type contextAnalyzerCapture struct {
	called *bool
}

func (a *contextAnalyzerCapture) Analyze(context.Context, *productimage.SourceBundle) (*productimage.ProductContext, error) {
	*a.called = true
	return &productimage.ProductContext{ProductType: "should-not-run"}, nil
}

func (g *sceneGeneratorCapture) GenerateScene(context.Context, *productimage.SceneGenerationRequest) (*productimage.SceneGenerationResult, error) {
	*g.called = true
	return &productimage.SceneGenerationResult{}, nil
}
