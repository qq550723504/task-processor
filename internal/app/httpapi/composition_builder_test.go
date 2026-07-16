package httpapi

import (
	"context"
	"reflect"
	"runtime"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	"task-processor/internal/core/config"
	"task-processor/internal/listingkit"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	"task-processor/internal/productenrich"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	"task-processor/internal/productimage"
	productimagedomain "task-processor/internal/productimage/domain"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
	prompt "task-processor/internal/prompt"
	promptmgmtapi "task-processor/internal/promptmgmt/api"
	sdshttpapi "task-processor/internal/sds/httpapi"
	"task-processor/internal/sdslogin"
	sdsloginbootstrap "task-processor/internal/sdslogin/bootstrap"
	sheinloginbootstrap "task-processor/internal/sheinlogin/bootstrap"
	"task-processor/internal/taskrpcapi"
)

func TestNewHTTPFeatureCompositionBuilderUsesFeatureOwnedRuntimeBuilders(t *testing.T) {
	t.Parallel()

	builder := newHTTPFeatureCompositionBuilder()

	require.Equal(t,
		runtime.FuncForPC(reflect.ValueOf(buildProductModuleResult).Pointer()).Name(),
		runtime.FuncForPC(reflect.ValueOf(builder.buildProduct).Pointer()).Name(),
	)
	require.Equal(t,
		runtime.FuncForPC(reflect.ValueOf(buildImageModuleResult).Pointer()).Name(),
		runtime.FuncForPC(reflect.ValueOf(builder.buildImage).Pointer()).Name(),
	)
	require.Equal(t,
		runtime.FuncForPC(reflect.ValueOf(buildAmazonListingModuleResult).Pointer()).Name(),
		runtime.FuncForPC(reflect.ValueOf(builder.buildAmazonListing).Pointer()).Name(),
	)
	require.Equal(t,
		runtime.FuncForPC(reflect.ValueOf(buildListingKitModuleResult).Pointer()).Name(),
		runtime.FuncForPC(reflect.ValueOf(builder.buildListingKit).Pointer()).Name(),
	)
}

func TestHTTPFeatureCompositionBuilderBuildsFeaturesInDependencyOrder(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	deps := &runtimeDeps{
		shared:   &sharedRuntimeDeps{},
		features: &featureRuntimeState{},
	}
	order := make([]string, 0, 9)
	sheinClosed := false
	sdsClosed := false
	productService := &stubCompositionProductService{}
	imageService := &stubCompositionImageService{}
	subjectExtractor := &stubCompositionSubjectExtractor{}
	whiteBgRenderer := &stubCompositionWhiteBackgroundRenderer{}
	sceneRenderer := &stubCompositionSceneRenderer{}
	statusProvider := &stubCompositionSDSStatusProvider{}

	builder := httpFeatureCompositionBuilder{
		buildProduct: func(productenrichhttpapi.RuntimeBuildInput) (*productenrichhttpapi.Module, error) {
			order = append(order, "product")
			return &productenrichhttpapi.Module{
				Service: productService,
				Pool:    stubWorkerPool{},
			}, nil
		},
		buildImage: func(productimagehttpapi.RuntimeBuildInput) (*productimagehttpapi.Module, error) {
			order = append(order, "image")
			return &productimagehttpapi.Module{
				Service:               imageService,
				SubjectExtractor:      subjectExtractor,
				WhiteBackgroundRender: whiteBgRenderer,
				SceneRenderer:         sceneRenderer,
				Pool:                  stubWorkerPool{},
			}, nil
		},
		buildAmazonListing: func(amazonlistinghttpapi.RuntimeBuildInput) (*amazonlistinghttpapi.Module, error) {
			order = append(order, "amazon")
			require.Equal(t, productService, deps.features.productService)
			require.Equal(t, imageService, deps.features.imageService)
			return &amazonlistinghttpapi.Module{}, nil
		},
		buildSheinLogin: func(*runtimeDeps) (*sheinloginbootstrap.BuildResult, func() error, error) {
			order = append(order, "shein-login")
			return &sheinloginbootstrap.BuildResult{}, func() error {
				sheinClosed = true
				return nil
			}, nil
		},
		buildSDSLogin: func(*runtimeDeps) (*sdsloginbootstrap.BuildResult, func() error, error) {
			order = append(order, "sds-login")
			return &sdsloginbootstrap.BuildResult{StatusProvider: statusProvider}, func() error {
				sdsClosed = true
				return nil
			}, nil
		},
		buildListingKit: func(input listingkithttpapi.RuntimeBuildInput) (*listingkithttpapi.Module, error) {
			order = append(order, "listingkit")
			require.Equal(t, subjectExtractor, deps.features.imageSubjectExtractor)
			require.Equal(t, whiteBgRenderer, deps.features.imageWhiteBgRenderer)
			require.Equal(t, sceneRenderer, deps.features.imageSceneRenderer)
			require.NotNil(t, input.Runtime.Support.Repositories.Core.Task)
			require.NotNil(t, input.Runtime.Support.Hooks.ConfigureAuthorization)
			require.Equal(t, statusProvider, input.Runtime.Support.SDSLoginStatusProvider)
			require.Nil(t, input.Runtime.SDSLoginStatusProvider)
			require.Nil(t, input.Runtime.Repositories.Core.Task)
			require.Nil(t, input.Runtime.Hooks.ConfigureAuthorization)
			return &listingkithttpapi.Module{
				TaskLifecycleService: stubCompositionTaskLifecycleService{},
				StoreAccessValidator: stubCompositionStoreAccessValidator{},
				Pool:                 stubWorkerPool{},
			}, nil
		},
		buildPrompt: func(prompt.TenantPromptStore) *promptmgmtapi.BuildResult {
			order = append(order, "prompt")
			return &promptmgmtapi.BuildResult{}
		},
		buildTaskRPC: func(provider taskrpcapi.LocalStatusProvider) (*taskrpcapi.BuildResult, error) {
			order = append(order, "taskrpc")
			require.NotNil(t, provider)
			snapshot := provider()
			require.Equal(t, 3, snapshot["summary"].(map[string]any)["poolCount"])
			require.Contains(t, snapshot["pools"].(map[string]any), "product_enrich")
			require.Contains(t, snapshot["pools"].(map[string]any), "product_image")
			require.Contains(t, snapshot["pools"].(map[string]any), "listing_kit")
			require.NotContains(t, snapshot["pools"].(map[string]any), "amazon_listing")
			return &taskrpcapi.BuildResult{}, nil
		},
		buildSDS: func(*logrus.Logger, *config.Config) *sdshttpapi.BuildResult {
			order = append(order, "sds")
			return &sdshttpapi.BuildResult{}
		},
	}

	composition, err := builder.build(logger, deps)
	require.NoError(t, err)
	require.NotNil(t, composition.productModule)
	require.NotNil(t, composition.imageModule)
	require.NotNil(t, composition.amazonListingModule)
	require.NotNil(t, composition.listingKitModule)
	require.NotNil(t, composition.productSourcingModule)
	require.NotNil(t, composition.promptModule)
	require.NotNil(t, composition.taskRPCResult)
	require.NotNil(t, composition.sdsModule)
	require.Equal(t, []string{
		"product",
		"image",
		"amazon",
		"shein-login",
		"sds-login",
		"listingkit",
		"prompt",
		"taskrpc",
		"sds",
	}, order)
	require.Len(t, deps.shared.closers, 2)
	require.NoError(t, deps.shared.closers[0]())
	require.NoError(t, deps.shared.closers[1]())
	require.True(t, sheinClosed)
	require.True(t, sdsClosed)
}

type stubCompositionTaskLifecycleService struct {
	listingkit.TaskLifecycleService
}

type stubCompositionStoreAccessValidator struct{}

func (stubCompositionStoreAccessValidator) ValidateStoreAccess(context.Context, int64, int64, string) (listingkit.StoreAccess, error) {
	return listingkit.StoreAccess{}, nil
}

type stubCompositionProductService struct{}

func (stubCompositionProductService) CreateGenerateTask(context.Context, *productenrich.GenerateRequest) (*productenrich.Task, error) {
	return nil, nil
}

func (stubCompositionProductService) GetTaskResult(context.Context, string) (*productenrich.TaskResult, error) {
	return nil, nil
}

func (stubCompositionProductService) ProcessProduct(context.Context, *productenrich.Task) (*productenrich.ProductJSON, error) {
	return nil, nil
}

func (stubCompositionProductService) SetTaskSubmitter(productenrich.TaskSubmitter) {}

type stubCompositionImageService struct{}

func (stubCompositionImageService) CreateProcessTask(context.Context, *productimage.ImageProcessRequest) (*productimage.Task, error) {
	return nil, nil
}

func (stubCompositionImageService) GetTaskResult(context.Context, string) (*productimage.TaskResult, error) {
	return nil, nil
}

func (stubCompositionImageService) ReviewTask(context.Context, string, *productimage.ReviewTaskRequest) (*productimage.TaskResult, error) {
	return nil, nil
}

func (stubCompositionImageService) ProcessImages(context.Context, *productimage.Task) (*productimage.ImageProcessResult, error) {
	return nil, nil
}

func (stubCompositionImageService) SetTaskSubmitter(productimage.TaskSubmitter) {}

type stubCompositionSubjectExtractor struct{}

func (stubCompositionSubjectExtractor) Extract(context.Context, string, *productimagedomain.ProductContext) (*productimagedomain.ImageAsset, error) {
	return nil, nil
}

type stubCompositionWhiteBackgroundRenderer struct{}

func (stubCompositionWhiteBackgroundRenderer) Render(context.Context, *productimagedomain.ImageAsset, *productimagedomain.ProductContext) (*productimagedomain.ImageAsset, error) {
	return nil, nil
}

type stubCompositionSceneRenderer struct{}

func (stubCompositionSceneRenderer) Render(context.Context, *productimagedomain.ImageAsset, *productimagedomain.ProductContext) ([]productimagedomain.ImageAsset, error) {
	return nil, nil
}

type stubCompositionSDSStatusProvider struct{}

func (stubCompositionSDSStatusProvider) Status(context.Context) (*sdslogin.Status, error) {
	return nil, nil
}
