package httpapi

import (
	"context"
	"testing"

	"task-processor/internal/listingkit"
	productasset "task-processor/internal/product/asset"
	sdsadapter "task-processor/internal/sds/adapter"
	sdsdesign "task-processor/internal/sds/design"
	sdstemplate "task-processor/internal/sds/template"
	sdsusecase "task-processor/internal/sds/usecase"
	"task-processor/internal/sdslogin"
)

func TestBuildRuntimeSupportProvidesRepositoryAndHookBundles(t *testing.T) {
	t.Parallel()

	support := BuildRuntimeSupport(RuntimeSupportInput{})
	if support.Repositories.Core.Task == nil {
		t.Fatal("expected core task repository builder")
	}
	if support.Repositories.Admin.Store == nil {
		t.Fatal("expected admin store repository builder")
	}
	if support.Hooks.SheinPricingPolicyBuilder == nil {
		t.Fatal("expected shein pricing policy builder")
	}
	if support.Repositories.Admin.GenerationTopicPolicy == nil {
		t.Fatal("expected generation topic policy admin repository builder")
	}
	if support.Hooks.ConfigureAuthorization == nil {
		t.Fatal("expected authorization hook")
	}
}

func TestBuildRuntimeSupportUsesProvidedApprovedAssetReader(t *testing.T) {
	t.Parallel()

	reader := &stubRuntimeSupportApprovedAssetReader{}
	support := BuildRuntimeSupport(RuntimeSupportInput{ApprovedAssets: reader})
	built, closers, err := support.Repositories.Core.ApprovedAsset(nil, nil)
	if err != nil {
		t.Fatalf("build approved asset reader: %v", err)
	}
	if built != reader {
		t.Fatalf("approved asset reader = %v, want shared %v", built, reader)
	}
	if len(closers) != 0 {
		t.Fatalf("approved asset reader closers = %d, want 0 because app owns the shared reader", len(closers))
	}
}

func TestBuildRuntimeSupportCarriesSDSCollaborators(t *testing.T) {
	t.Parallel()

	syncService := stubRuntimeSupportSDSService{}
	statusProvider := stubRuntimeSupportSDSStatusProvider{}
	remoteProvider := stubRuntimeSupportSDSBaselineProvider{}

	support := BuildRuntimeSupport(RuntimeSupportInput{
		SDSSyncService:            syncService,
		SDSLoginStatusProvider:    statusProvider,
		SDSBaselineRemoteProvider: remoteProvider,
	})
	if support.SDSSyncService != syncService {
		t.Fatal("expected SDS sync service to be preserved in runtime support")
	}
	if support.SDSLoginStatusProvider != statusProvider {
		t.Fatal("expected SDS login status provider to be preserved in runtime support")
	}
	if support.SDSBaselineRemoteProvider != remoteProvider {
		t.Fatal("expected SDS baseline remote provider to be preserved in runtime support")
	}

	input := buildRuntimeServiceInput(nil, RuntimeDependencies{Support: support})
	if input.SDSSyncService != syncService {
		t.Fatal("expected runtime service input to consume SDS sync service from support")
	}
	if input.SDSLoginStatusProvider != statusProvider {
		t.Fatal("expected runtime service input to consume SDS login status provider from support")
	}
	if input.SDSBaselineRemoteProvider != remoteProvider {
		t.Fatal("expected runtime service input to consume SDS baseline remote provider from support")
	}
}

func TestBuildRuntimeSupportWithoutSDSCollaboratorsDegradesSafely(t *testing.T) {
	t.Parallel()

	input := buildRuntimeServiceInput(nil, RuntimeDependencies{
		Support: BuildRuntimeSupport(RuntimeSupportInput{}),
	})
	if input.SDSSyncService != nil {
		t.Fatal("expected SDS sync service to be nil when runtime support does not provide one")
	}
	if input.SDSLoginStatusProvider != nil {
		t.Fatal("expected SDS login status provider to be nil when runtime support does not provide one")
	}
	if input.SDSBaselineRemoteProvider != nil {
		t.Fatal("expected SDS baseline remote provider to be nil when runtime support does not provide one")
	}
}

var _ sdsusecase.Service = stubRuntimeSupportSDSService{}
var _ listingkit.SDSLoginStatusProvider = stubRuntimeSupportSDSStatusProvider{}
var _ listingkit.SDSBaselineRemoteProvider = stubRuntimeSupportSDSBaselineProvider{}

type stubRuntimeSupportSDSService struct{}

type stubRuntimeSupportApprovedAssetReader struct{}

func (*stubRuntimeSupportApprovedAssetReader) GetApprovedInventory(context.Context, productasset.InventoryScope) (productasset.ApprovedAssetInventory, error) {
	return productasset.ApprovedAssetInventory{}, nil
}

func (stubRuntimeSupportSDSService) SyncFromApprovedAssets(context.Context, sdsusecase.ApprovedAssetsInput) (*sdsadapter.SyncResult, error) {
	return nil, nil
}

type stubRuntimeSupportSDSStatusProvider struct{}

func (stubRuntimeSupportSDSStatusProvider) Status(context.Context) (*sdslogin.Status, error) {
	return &sdslogin.Status{}, nil
}

type stubRuntimeSupportSDSBaselineProvider struct{}

func (stubRuntimeSupportSDSBaselineProvider) GetProductDetail(context.Context, int64) (*sdstemplate.ProductDetail, error) {
	return nil, nil
}

func (stubRuntimeSupportSDSBaselineProvider) GetDesignProduct(context.Context, int64) (*sdsdesign.DesignProductPage, error) {
	return nil, nil
}

func (stubRuntimeSupportSDSBaselineProvider) GetPrototypeGroups(context.Context, int64) ([]sdsdesign.PrototypeGroup, error) {
	return nil, nil
}
