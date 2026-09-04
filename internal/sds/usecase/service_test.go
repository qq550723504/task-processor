package usecase

import (
	"context"
	"testing"

	productasset "task-processor/internal/product/asset"
	"task-processor/internal/sds/adapter"
)

type stubAdapter struct {
	lastInput adapter.SyncFromApprovedAssetsInput
	result    *adapter.SyncResult
	err       error
}

func (s *stubAdapter) SyncFromApprovedAssets(_ context.Context, input adapter.SyncFromApprovedAssetsInput) (*adapter.SyncResult, error) {
	s.lastInput = input
	return s.result, s.err
}

func TestSyncFromApprovedAssetsDelegatesScopeToAdapter(t *testing.T) {
	t.Parallel()

	scope := productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"}
	want := &adapter.SyncResult{}
	adp := &stubAdapter{result: want}
	svc := &service{adapter: adp}

	got, err := svc.SyncFromApprovedAssets(context.Background(), ApprovedAssetsInput{
		Sync:  SyncInput{VariantID: 89764},
		Scope: scope,
	})
	if err != nil {
		t.Fatalf("SyncFromApprovedAssets() error = %v", err)
	}
	if got != want {
		t.Fatalf("SyncFromApprovedAssets() result = %p, want %p", got, want)
	}
	if adp.lastInput.Scope != scope {
		t.Fatalf("adapter scope = %+v, want %+v", adp.lastInput.Scope, scope)
	}
	if adp.lastInput.SyncInput.VariantID != 89764 {
		t.Fatalf("adapter variant id = %d, want 89764", adp.lastInput.SyncInput.VariantID)
	}
}

var _ adapterService = (*stubAdapter)(nil)
