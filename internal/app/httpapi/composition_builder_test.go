package httpapi

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	"task-processor/internal/core/config"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
	prompt "task-processor/internal/prompt"
	promptmgmtapi "task-processor/internal/promptmgmt/api"
	sdshttpapi "task-processor/internal/sds/httpapi"
	sdsloginbootstrap "task-processor/internal/sdslogin/bootstrap"
	sheinloginbootstrap "task-processor/internal/sheinlogin/bootstrap"
	"task-processor/internal/taskrpcapi"
)

func TestHTTPFeatureCompositionBuilderBuildsFeaturesInDependencyOrder(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	deps := &runtimeDeps{}
	order := make([]string, 0, 9)
	sheinClosed := false
	sdsClosed := false

	builder := httpFeatureCompositionBuilder{
		buildProduct: func(*logrus.Logger, *runtimeDeps) (*productenrichhttpapi.Module, error) {
			order = append(order, "product")
			return &productenrichhttpapi.Module{
				Pool: stubWorkerPool{},
			}, nil
		},
		buildImage: func(*logrus.Logger, *runtimeDeps) (*productimagehttpapi.Module, error) {
			order = append(order, "image")
			return &productimagehttpapi.Module{
				Pool: stubWorkerPool{},
			}, nil
		},
		buildAmazonListing: func(*logrus.Logger, *runtimeDeps) (*amazonlistinghttpapi.Module, error) {
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
			return &sdsloginbootstrap.BuildResult{}, func() error {
				sdsClosed = true
				return nil
			}, nil
		},
		buildListingKit: func(*logrus.Logger, *runtimeDeps) (*listingkithttpapi.Module, error) {
			order = append(order, "listingkit")
			return &listingkithttpapi.Module{
				Pool: stubWorkerPool{},
			}, nil
		},
		buildPrompt: func(prompt.TenantPromptStore) *promptmgmtapi.BuildResult {
			order = append(order, "prompt")
			return &promptmgmtapi.BuildResult{}
		},
		buildTaskRPC: func(_ taskrpcapi.ClientProvider, provider taskrpcapi.LocalStatusProvider) (*taskrpcapi.BuildResult, error) {
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
	require.Len(t, deps.closers, 2)
	require.NoError(t, deps.closers[0]())
	require.NoError(t, deps.closers[1]())
	require.True(t, sheinClosed)
	require.True(t, sdsClosed)
}
