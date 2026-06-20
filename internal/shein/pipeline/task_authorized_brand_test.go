package pipeline

import (
	"context"
	"testing"

	"task-processor/internal/listingruntime"
	"task-processor/internal/model"
	"task-processor/internal/shein/authorizedbrand"
	sheincontext "task-processor/internal/shein/context"
)

func TestApplyAuthorizedBrandConfig_EnabledStoresDefersBrandResolution(t *testing.T) {
	enabled := true
	taskCtx := sheincontext.NewTaskContext(context.Background(), &model.Task{StoreID: 1})
	taskCtx.StoreInfo = &listingruntime.StoreInfo{
		EnableBrandAuthorization: &enabled,
		AuthorizedBrandCode:      "2fd1n",
		AuthorizedBrandName:      "Logitech",
	}

	if err := applyAuthorizedBrandConfig(taskCtx); err != nil {
		t.Fatalf("applyAuthorizedBrandConfig() error = %v", err)
	}
	if taskCtx.AuthorizedBrand != nil {
		t.Fatalf("taskCtx.AuthorizedBrand = %+v, want nil", taskCtx.AuthorizedBrand)
	}
	if _, ok := authorizedbrand.FromContext(taskCtx.Context); ok {
		t.Fatal("authorizedbrand.FromContext() ok = true, want false")
	}
}

func TestApplyAuthorizedBrandConfig_DisabledLeavesContextUnchanged(t *testing.T) {
	originalCtx := context.WithValue(context.Background(), "marker", "keep")
	taskCtx := sheincontext.NewTaskContext(originalCtx, &model.Task{StoreID: 1})

	if err := applyAuthorizedBrandConfig(taskCtx); err != nil {
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
