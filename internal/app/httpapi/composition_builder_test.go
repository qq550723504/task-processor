package httpapi

import (
	"context"
	"reflect"
	"runtime"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	"task-processor/internal/authidentity"
	"task-processor/internal/core/config"
	"task-processor/internal/imageagent"
	imageagenthttpapi "task-processor/internal/imageagent/httpapi"
	imageagentstore "task-processor/internal/imageagent/store"
	"task-processor/internal/listingkit"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
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
		runtime.FuncForPC(reflect.ValueOf(buildAmazonListingModuleResult).Pointer()).Name(),
		runtime.FuncForPC(reflect.ValueOf(builder.buildAmazonListing).Pointer()).Name(),
	)
	require.Equal(t,
		runtime.FuncForPC(reflect.ValueOf(buildListingKitModuleResult).Pointer()).Name(),
		runtime.FuncForPC(reflect.ValueOf(builder.buildListingKit).Pointer()).Name(),
	)
	require.Equal(t,
		runtime.FuncForPC(reflect.ValueOf(buildImageAgentModuleResult).Pointer()).Name(),
		runtime.FuncForPC(reflect.ValueOf(builder.buildImageAgent).Pointer()).Name(),
	)
}

func TestHTTPFeatureCompositionBuilderBuildsFeaturesInDependencyOrder(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	deps := &runtimeDeps{
		shared: &sharedRuntimeDeps{},
		features: &featureRuntimeState{
			productSnapshotReader: stubCompositionProductSnapshotReader{},
			listingKitSupport: &listingKitSupport{
				approvedAssetReader: stubCompositionApprovedAssetReader{},
			},
		},
	}
	order := make([]string, 0, 9)
	sheinClosed := false
	sdsClosed := false
	statusProvider := &stubCompositionSDSStatusProvider{}

	builder := httpFeatureCompositionBuilder{
		buildAmazonListing: func(amazonlistinghttpapi.RuntimeBuildInput) (*amazonlistinghttpapi.Module, error) {
			order = append(order, "amazon")
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
			require.NotNil(t, input.Runtime.Support.Repositories.Core.Task)
			require.NotNil(t, input.Runtime.Support.Hooks.SheinPricingPolicyBuilder)
			require.Equal(t, statusProvider, input.Runtime.Support.SDSLoginStatusProvider)
			require.Nil(t, input.Runtime.SDSLoginStatusProvider)
			require.Nil(t, input.Runtime.Repositories.Core.Task)
			require.Nil(t, input.Runtime.Hooks.SheinPricingPolicyBuilder)
			return &listingkithttpapi.Module{
				TaskLifecycleService: stubCompositionTaskLifecycleService{},
				StoreAccessValidator: stubCompositionStoreAccessValidator{},
				Pool:                 stubWorkerPool{},
			}, nil
		},
		buildImageAgent: func(*config.Config, *logrus.Logger) (*imageagenthttpapi.BuildResult, error) {
			order = append(order, "image-agent")
			return &imageagenthttpapi.BuildResult{}, nil
		},
		buildPrompt: func(prompt.TenantPromptStore) *promptmgmtapi.BuildResult {
			order = append(order, "prompt")
			return &promptmgmtapi.BuildResult{}
		},
		buildTaskRPC: func(provider taskrpcapi.LocalStatusProvider) (*taskrpcapi.BuildResult, error) {
			order = append(order, "taskrpc")
			require.NotNil(t, provider)
			snapshot := provider()
			require.Equal(t, 1, snapshot["summary"].(map[string]any)["poolCount"])
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
	require.NotNil(t, composition.amazonListingModule)
	require.NotNil(t, composition.listingKitModule)
	require.NotNil(t, composition.imageAgentModule)
	require.NotNil(t, composition.productSourcingModule)
	require.NotNil(t, composition.promptModule)
	require.NotNil(t, composition.taskRPCResult)
	require.NotNil(t, composition.sdsModule)
	require.Equal(t, []string{
		"amazon",
		"shein-login",
		"sds-login",
		"listingkit",
		"image-agent",
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

func TestAmazonListingFeatureBuilderSkipsModuleWithoutProductSnapshotReader(t *testing.T) {
	t.Parallel()

	called := false
	builder := amazonListingFeatureBuilder{
		buildAmazonListing: func(amazonlistinghttpapi.RuntimeBuildInput) (*amazonlistinghttpapi.Module, error) {
			called = true
			return &amazonlistinghttpapi.Module{}, nil
		},
	}
	deps := &runtimeDeps{
		shared: &sharedRuntimeDeps{},
		features: &featureRuntimeState{listingKitSupport: &listingKitSupport{
			approvedAssetReader: stubCompositionApprovedAssetReader{},
		}},
	}

	module, err := builder.build(logrus.New(), deps)

	require.NoError(t, err)
	require.Nil(t, module)
	require.False(t, called)
}

func TestHTTPFeatureCompositionIncludesImageAgentRouteModule(t *testing.T) {
	module := imageagenthttpapi.NewHTTPModule(nil)
	composition := httpFeatureComposition{imageAgentModule: &imageagenthttpapi.BuildResult{Module: module}}
	require.Equal(t, imageagenthttpapi.ModuleName, composition.imageAgentHTTPModule().Name())
}

func TestImageAgentDurableAssetPublicURLResolverUsesPublisherConfiguration(t *testing.T) {
	cfg := &config.Config{}
	cfg.ImageAgent.ArtifactStore.Enabled = true
	cfg.ImageAgent.ArtifactStore.Provider = "s3"
	cfg.ImageAgent.ArtifactStore.PublicBase = "https://cdn.example.test/assets"
	cfg.ImageAgent.ArtifactStore.S3.Bucket = "image-agent-assets"

	resolver := imageAgentDurableAssetPublicURLResolver(cfg)

	require.NotNil(t, resolver)
	require.Equal(t, "https://cdn.example.test/assets/image-agent/public/tenant-a/run-1/result.png", resolver.PublicURL("image-agent/public/tenant-a/run-1/result.png"))
}

func TestNewImageAgentHTTPServiceStartsWithoutRetiredSceneTenantGate(t *testing.T) {
	workflows := &recordingCompositionImageAgentWorkflowClient{}
	service, err := newImageAgentHTTPService(
		imageagentstore.NewMemoryRepository(),
		workflows,
		staticCompositionImageAgentCatalog{catalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{
			ID: "source-1", Type: imageagent.AuthorizedAssetSource, DisplayURL: "https://cdn.example.test/source-1.png", Width: 1200, Height: 900,
		}}}},
	)
	require.NoError(t, err)
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "tenant-not-in-retired-allowlist", UserID: "user-a"})

	err = service.Start(ctx, imageagent.StartRunInput{
		RunID: "run-1", BusinessTaskID: "task-1", TargetPlatform: "shein",
		ImagePolicyContext: imageagent.ImagePolicyContext{Country: "us", Family: "default", SceneCategory: "shoes"},
		Mode:               imageagent.RunModeManual, IdempotencyKey: "run-key-1",
		Plan: imageagent.Plan{
			Revision: 1, IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1"},
			Slots: []imageagent.Slot{{ID: "slot-1", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-key-1", Status: imageagent.SlotStatusPending}},
		},
	})

	require.NoError(t, err)
	require.Equal(t, 1, workflows.starts)
}

type staticCompositionImageAgentCatalog struct{ catalog imageagent.AssetCatalog }

func (catalog staticCompositionImageAgentCatalog) Resolve(context.Context, imageagent.AssetCatalogScope) (imageagent.AssetCatalog, error) {
	return catalog.catalog, nil
}

type recordingCompositionImageAgentWorkflowClient struct {
	imageagent.WorkflowClient
	starts int
}

func (client *recordingCompositionImageAgentWorkflowClient) StartManual(context.Context, imageagent.WorkflowStart) error {
	client.starts++
	return nil
}

type stubCompositionTaskLifecycleService struct {
	listingkit.TaskLifecycleService
}

type stubCompositionStoreAccessValidator struct{}

func (stubCompositionStoreAccessValidator) ValidateStoreAccess(context.Context, int64, int64, string) (listingkit.StoreAccess, error) {
	return listingkit.StoreAccess{}, nil
}

type stubCompositionSDSStatusProvider struct{}

func (stubCompositionSDSStatusProvider) Status(context.Context) (*sdslogin.Status, error) {
	return nil, nil
}

type stubCompositionProductSnapshotReader struct{}

func (stubCompositionProductSnapshotReader) GetProductSnapshot(context.Context, listingkit.ProductSnapshotQuery) (catalog.ProductSnapshot, error) {
	return catalog.ProductSnapshot{}, nil
}

type stubCompositionApprovedAssetReader struct{}

func (stubCompositionApprovedAssetReader) GetApprovedInventory(context.Context, productasset.InventoryScope) (productasset.ApprovedAssetInventory, error) {
	return productasset.ApprovedAssetInventory{}, nil
}
