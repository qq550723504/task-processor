package httpapi

import (
	"context"
	"testing"

	"task-processor/internal/listingkit"
	sdsadapter "task-processor/internal/sds/adapter"
	sdsdesign "task-processor/internal/sds/design"
	sdstemplate "task-processor/internal/sds/template"
	sdsusecase "task-processor/internal/sds/usecase"
	sdsworkflow "task-processor/internal/sds/workflow"
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

func TestBuildRuntimeModuleAndTemporalRuntimeAcceptRuntimeSupport(t *testing.T) {
	t.Parallel()

	serviceInput := buildSuccessfulServiceInputFixture()
	syncService := stubRuntimeSupportSDSService{}
	statusProvider := stubRuntimeSupportSDSStatusProvider{}
	remoteProvider := stubRuntimeSupportSDSBaselineProvider{}
	runtime := RuntimeDependencies{
		Config:         serviceInput.Config,
		ProductService: serviceInput.ProductService,
		Support: BuildRuntimeSupport(RuntimeSupportInput{
			SDSSyncService:            syncService,
			SDSLoginStatusProvider:    statusProvider,
			SDSBaselineRemoteProvider: remoteProvider,
		}),
	}

	module, err := BuildRuntimeModule(RuntimeBuildInput{
		Logger:  serviceInput.Logger,
		Runtime: runtime,
	})
	if err != nil {
		t.Fatalf("BuildRuntimeModule() error = %v", err)
	}
	if module == nil {
		t.Fatal("expected module")
	}

	result, err := BuildTemporalRuntime(TemporalRuntimeBuildInput{
		Logger:  serviceInput.Logger,
		Runtime: runtime,
	})
	if err != nil {
		t.Fatalf("BuildTemporalRuntime() error = %v", err)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

var _ sdsusecase.Service = stubRuntimeSupportSDSService{}
var _ listingkit.SDSLoginStatusProvider = stubRuntimeSupportSDSStatusProvider{}
var _ listingkit.SDSBaselineRemoteProvider = stubRuntimeSupportSDSBaselineProvider{}

type stubRuntimeSupportSDSService struct{}

func (stubRuntimeSupportSDSService) SyncFromRemoteImage(context.Context, sdsusecase.RemoteImageInput) (*sdsworkflow.SyncResult, error) {
	return nil, nil
}

func (stubRuntimeSupportSDSService) SyncFromLocalFile(context.Context, sdsusecase.LocalFileInput) (*sdsworkflow.SyncResult, error) {
	return nil, nil
}

func (stubRuntimeSupportSDSService) SyncFromImageResult(context.Context, sdsusecase.ImageResultInput) (*sdsadapter.SyncResult, error) {
	return nil, nil
}

func (stubRuntimeSupportSDSService) SyncFromImageRequest(context.Context, sdsusecase.ImageRequestInput) (*sdsadapter.SyncResult, error) {
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
