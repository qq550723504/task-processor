package listingkit

import (
	"context"
	"testing"

	productasset "task-processor/internal/product/asset"
	sdsadapter "task-processor/internal/sds/adapter"
	sdsusecase "task-processor/internal/sds/usecase"
	sdsworkflow "task-processor/internal/sds/workflow"
)

type recordingApprovedSDSSyncService struct {
	input sdsusecase.ApprovedAssetsInput
}

func (s *recordingApprovedSDSSyncService) SyncFromApprovedAssets(_ context.Context, input sdsusecase.ApprovedAssetsInput) (*sdsadapter.SyncResult, error) {
	s.input = input
	return &sdsadapter.SyncResult{DesignSync: &sdsworkflow.SyncResult{}}, nil
}

func TestPerformSingleSDSApprovedAssetSyncUsesTaskScope(t *testing.T) {
	t.Parallel()

	syncService := &recordingApprovedSDSSyncService{}
	svc := &service{supportDeps: supportDependencies{sdsSyncService: syncService}}
	task := &Task{TenantID: "tenant-a", Request: &GenerateRequest{ProductKey: "product-1"}}

	result, err := svc.performSingleSDSApprovedAssetSync(context.Background(), task, &SDSSyncOptions{VariantID: 89764})
	if err != nil {
		t.Fatalf("performSingleSDSApprovedAssetSync() error = %v", err)
	}
	if result == nil {
		t.Fatal("performSingleSDSApprovedAssetSync() returned nil result")
	}
	wantScope := productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"}
	if syncService.input.Scope != wantScope {
		t.Fatalf("SDS scope = %+v, want %+v", syncService.input.Scope, wantScope)
	}
}

func TestNeedsLocalSDSMockupFallbackWhenSDSReturnsTooFewImages(t *testing.T) {
	t.Parallel()

	summary := &SDSSyncSummary{
		MockupImageURLs: []string{"https://cdn.sdspod.com/out/0/202604/rendered-main.jpg"},
	}
	options := &SDSSyncOptions{
		MockupImageURLs: []string{
			"https://cdn.sdspod.com/images/mockup-1.jpg",
			"https://cdn.sdspod.com/images/mockup-2.jpg",
			"https://cdn.sdspod.com/images/mockup-3.jpg",
		},
	}

	if !needsLocalSDSMockupFallback(summary, options) {
		t.Fatal("expected local fallback when SDS returns fewer images than selected mockups")
	}
}

func TestNeedsLocalSDSMockupFallbackSkipsCompleteSDSSet(t *testing.T) {
	t.Parallel()

	summary := &SDSSyncSummary{
		MockupImageURLs: []string{
			"https://cdn.sdspod.com/out/0/202604/rendered-main.jpg",
			"https://cdn.sdspod.com/out/0/202604/rendered-gallery.jpg",
		},
	}
	options := &SDSSyncOptions{
		MockupImageURLs: []string{
			"https://cdn.sdspod.com/images/mockup-1.jpg",
			"https://cdn.sdspod.com/images/mockup-2.jpg",
		},
	}

	if needsLocalSDSMockupFallback(summary, options) {
		t.Fatal("did not expect local fallback when SDS returns a complete image set")
	}
}
