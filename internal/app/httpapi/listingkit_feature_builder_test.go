package httpapi

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
)

func TestNewListingKitFeatureBuilderUsesFeatureOwnedRuntimeBuilders(t *testing.T) {
	t.Parallel()

	builder := newListingKitFeatureBuilder()

	require.Equal(t,
		runtime.FuncForPC(reflect.ValueOf(productenrichhttpapi.BuildRuntimeModule).Pointer()).Name(),
		runtime.FuncForPC(reflect.ValueOf(builder.buildProduct).Pointer()).Name(),
	)
	require.Equal(t,
		runtime.FuncForPC(reflect.ValueOf(productimagehttpapi.BuildRuntimeModule).Pointer()).Name(),
		runtime.FuncForPC(reflect.ValueOf(builder.buildImage).Pointer()).Name(),
	)
	require.Equal(t,
		runtime.FuncForPC(reflect.ValueOf(listingkithttpapi.BuildRuntimeModule).Pointer()).Name(),
		runtime.FuncForPC(reflect.ValueOf(builder.buildListingKit).Pointer()).Name(),
	)
}

func TestListingKitFeatureBuilderBuildsRequestedFeatures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		options              listingKitFeatureBuildOptions
		wantOrder            []string
		seedProductFeature   bool
		wantProductModule    bool
		wantImageModule      bool
		wantListingKitModule bool
	}{
		{
			name: "product and listingkit",
			options: listingKitFeatureBuildOptions{
				includeListingKit: true,
			},
			wantOrder:            []string{"product", "listingkit"},
			wantProductModule:    true,
			wantListingKitModule: true,
		},
		{
			name: "product and image",
			options: listingKitFeatureBuildOptions{
				includeImage: true,
			},
			wantOrder:         []string{"product", "image"},
			wantProductModule: true,
			wantImageModule:   true,
		},
		{
			name: "listingkit without rebuilding product",
			options: listingKitFeatureBuildOptions{
				includeListingKit: true,
				skipProduct:       true,
			},
			wantOrder:            []string{"listingkit"},
			seedProductFeature:   true,
			wantListingKitModule: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logrus.New()
			deps := &runtimeDeps{
				shared:   &sharedRuntimeDeps{},
				features: &featureRuntimeState{},
			}
			order := make([]string, 0, 3)
			productService := &stubCompositionProductService{}
			imageService := &stubCompositionImageService{}
			if tt.seedProductFeature {
				deps.features.productService = productService
			}

			builder := listingKitFeatureBuilder{
				buildProduct: func(productenrichhttpapi.RuntimeBuildInput) (*productenrichhttpapi.Module, error) {
					order = append(order, "product")
					return &productenrichhttpapi.Module{
						Service: productService,
						Pool:    stubWorkerPool{},
					}, nil
				},
				buildImage: func(productimagehttpapi.RuntimeBuildInput) (*productimagehttpapi.Module, error) {
					order = append(order, "image")
					require.Equal(t, productService, deps.features.productService)
					return &productimagehttpapi.Module{
						Service: imageService,
						Pool:    stubWorkerPool{},
					}, nil
				},
				buildListingKit: func(input listingkithttpapi.RuntimeBuildInput) (*listingkithttpapi.Module, error) {
					order = append(order, "listingkit")
					require.Equal(t, productService, deps.features.productService)
					require.NotNil(t, input.Runtime.Support.Repositories.Core.Task)
					require.NotNil(t, input.Runtime.Support.Hooks.ConfigureAuthorization)
					require.Nil(t, input.Runtime.Repositories.Core.Task)
					require.Nil(t, input.Runtime.Hooks.ConfigureAuthorization)
					return &listingkithttpapi.Module{
						Pool: stubWorkerPool{},
					}, nil
				},
			}

			features, err := builder.build(logger, deps, tt.options)
			require.NoError(t, err)
			require.Equal(t, tt.wantOrder, order)
			require.Equal(t, tt.wantProductModule, features.productModule != nil)
			require.Equal(t, tt.wantImageModule, features.imageModule != nil)
			require.Equal(t, tt.wantListingKitModule, features.listingKitModule != nil)
		})
	}
}
