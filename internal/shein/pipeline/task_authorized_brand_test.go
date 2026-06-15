package pipeline

import (
	"context"
	"errors"
	"testing"

	managementAPI "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	"task-processor/internal/shein/authorizedbrand"
	sheincontext "task-processor/internal/shein/context"
)

type stubAuthorizedBrandResolver struct {
	resolved *authorizedbrand.Resolved
	err      error
}

func (s stubAuthorizedBrandResolver) Resolve(_ context.Context, _ authorizedbrand.Config) (*authorizedbrand.Resolved, error) {
	return s.resolved, s.err
}

func TestApplyAuthorizedBrandConfig_EnabledStoresResolvedBrandAndUpdatesContext(t *testing.T) {
	enabled := true
	taskCtx := sheincontext.NewTaskContext(context.Background(), &model.Task{StoreID: 1})
	taskCtx.StoreInfo = &managementAPI.StoreRespDTO{
		EnableBrandAuthorization: &enabled,
		AuthorizedBrandCode:      "2fd1n",
		AuthorizedBrandName:      "Logitech",
	}
	resolved := &authorizedbrand.Resolved{
		Enabled: true,
		Code:    "2fd1n",
		Name:    "Logitech罗技",
		NameEn:  "Logitech",
	}

	if err := applyAuthorizedBrandConfig(taskCtx, stubAuthorizedBrandResolver{resolved: resolved}); err != nil {
		t.Fatalf("applyAuthorizedBrandConfig() error = %v", err)
	}
	if taskCtx.AuthorizedBrand != resolved {
		t.Fatalf("taskCtx.AuthorizedBrand = %+v, want %+v", taskCtx.AuthorizedBrand, resolved)
	}
	fromCtx, ok := authorizedbrand.FromContext(taskCtx.Context)
	if !ok {
		t.Fatal("authorizedbrand.FromContext() ok = false, want true")
	}
	if fromCtx.Code != "2fd1n" || fromCtx.NameEn != "Logitech" {
		t.Fatalf("authorizedbrand.FromContext() = %+v", fromCtx)
	}
}

func TestApplyAuthorizedBrandConfig_ResolverFailureSurfacesError(t *testing.T) {
	enabled := true
	originalCtx := context.WithValue(context.Background(), "marker", "keep")
	taskCtx := sheincontext.NewTaskContext(originalCtx, &model.Task{StoreID: 1})
	taskCtx.StoreInfo = &managementAPI.StoreRespDTO{
		EnableBrandAuthorization: &enabled,
		AuthorizedBrandCode:      "2fd1n",
	}
	wantErr := errors.New("resolve failed")

	err := applyAuthorizedBrandConfig(taskCtx, stubAuthorizedBrandResolver{err: wantErr})
	if !errors.Is(err, wantErr) {
		t.Fatalf("applyAuthorizedBrandConfig() error = %v, want %v", err, wantErr)
	}
	if taskCtx.AuthorizedBrand != nil {
		t.Fatalf("taskCtx.AuthorizedBrand = %+v, want nil", taskCtx.AuthorizedBrand)
	}
	if taskCtx.Context != originalCtx {
		t.Fatal("taskCtx.Context changed on resolver failure")
	}
}

func TestApplyAuthorizedBrandConfig_DisabledLeavesContextUnchanged(t *testing.T) {
	originalCtx := context.WithValue(context.Background(), "marker", "keep")
	taskCtx := sheincontext.NewTaskContext(originalCtx, &model.Task{StoreID: 1})

	if err := applyAuthorizedBrandConfig(taskCtx, stubAuthorizedBrandResolver{}); err != nil {
		t.Fatalf("applyAuthorizedBrandConfig() error = %v", err)
	}
	if taskCtx.AuthorizedBrand != nil {
		t.Fatalf("taskCtx.AuthorizedBrand = %+v, want nil", taskCtx.AuthorizedBrand)
	}
	if taskCtx.Context != originalCtx {
		t.Fatal("taskCtx.Context changed for disabled config")
	}
	if _, ok := authorizedbrand.FromContext(taskCtx.Context); ok {
		t.Fatal("authorizedbrand.FromContext() ok = true, want false")
	}
}
