package adapter

import (
	"context"
	"errors"
	"testing"

	productasset "task-processor/internal/product/asset"
	"task-processor/internal/sds/workflow"
)

type stubApprovedAssetReader struct {
	inventory productasset.ApprovedAssetInventory
	err       error
	lastScope productasset.InventoryScope
}

func (s *stubApprovedAssetReader) GetApprovedInventory(_ context.Context, scope productasset.InventoryScope) (productasset.ApprovedAssetInventory, error) {
	s.lastScope = scope
	return s.inventory, s.err
}

type stubWorkflowService struct {
	lastInput     workflow.SyncInput
	lastInventory productasset.ApprovedAssetInventory
	result        *workflow.SyncResult
	err           error
}

func (s *stubWorkflowService) SyncDesignFromApprovedAssets(_ context.Context, input workflow.SyncInput, inventory productasset.ApprovedAssetInventory) (*workflow.SyncResult, error) {
	s.lastInput = input
	s.lastInventory = inventory
	return s.result, s.err
}

func TestSyncFromApprovedAssetsReadsScopedInventoryAndDelegates(t *testing.T) {
	t.Parallel()

	scope := productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"}
	inventory := productasset.ApprovedAssetInventory{
		Scope: scope,
		Assets: []productasset.ApprovedAsset{
			{ID: "design-1", Role: productasset.RoleDesign, URL: "https://example.com/design.jpg"},
		},
	}
	reader := &stubApprovedAssetReader{inventory: inventory}
	wf := &stubWorkflowService{result: &workflow.SyncResult{}}
	svc := newServiceWithDeps(reader, wf)

	result, err := svc.SyncFromApprovedAssets(context.Background(), SyncFromApprovedAssetsInput{
		SyncInput: workflow.SyncInput{VariantID: 89764},
		Scope:     scope,
	})
	if err != nil {
		t.Fatalf("SyncFromApprovedAssets() error = %v", err)
	}
	if reader.lastScope != scope {
		t.Fatalf("reader scope = %+v, want %+v", reader.lastScope, scope)
	}
	if wf.lastInput.VariantID != 89764 {
		t.Fatalf("workflow variant id = %d, want 89764", wf.lastInput.VariantID)
	}
	if wf.lastInventory.Scope != scope || len(wf.lastInventory.Assets) != 1 || wf.lastInventory.Assets[0].ID != "design-1" {
		t.Fatalf("workflow inventory = %+v, want scoped approved inventory", wf.lastInventory)
	}
	if result == nil || result.DesignSync != wf.result {
		t.Fatalf("sync result = %+v, want workflow result", result)
	}
	if result.ApprovedAssets.Scope != scope || len(result.ApprovedAssets.Assets) != 1 || result.ApprovedAssets.Assets[0].ID != "design-1" {
		t.Fatalf("sync result inventory = %+v, want scoped approved inventory", result.ApprovedAssets)
	}
}

func TestSyncFromApprovedAssetsRejectsReaderScopeMismatch(t *testing.T) {
	t.Parallel()

	requested := productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"}
	reader := &stubApprovedAssetReader{inventory: productasset.ApprovedAssetInventory{
		Scope: productasset.InventoryScope{TenantID: "tenant-b", ProductKey: "product-1"},
		Assets: []productasset.ApprovedAsset{
			{ID: "design-1", Role: productasset.RoleDesign, URL: "https://example.com/design.jpg"},
		},
	}}
	wf := &stubWorkflowService{}
	svc := newServiceWithDeps(reader, wf)

	_, err := svc.SyncFromApprovedAssets(context.Background(), SyncFromApprovedAssetsInput{Scope: requested})
	if !errors.Is(err, productasset.ErrRepositoryStateInvalid) {
		t.Fatalf("SyncFromApprovedAssets() error = %v, want %v", err, productasset.ErrRepositoryStateInvalid)
	}
	if wf.lastInventory.Scope != (productasset.InventoryScope{}) {
		t.Fatalf("workflow called with mismatched inventory: %+v", wf.lastInventory)
	}
}
