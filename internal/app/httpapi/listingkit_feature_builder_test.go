package httpapi

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
)

func TestListingKitFeatureBuilderBuildsRequestedFeatures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		options              listingKitFeatureBuildOptions
		wantOrder            []string
		wantImageModule      bool
		wantListingKitModule bool
	}{
		{
			name: "product and listingkit",
			options: listingKitFeatureBuildOptions{
				includeListingKit: true,
			},
			wantOrder:            []string{"product", "listingkit"},
			wantListingKitModule: true,
		},
		{
			name: "product and image",
			options: listingKitFeatureBuildOptions{
				includeImage: true,
			},
			wantOrder:       []string{"product", "image"},
			wantImageModule: true,
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

			builder := listingKitFeatureBuilder{
				buildProduct: func(*logrus.Logger, *runtimeDeps) (*productenrichhttpapi.Module, error) {
					order = append(order, "product")
					return &productenrichhttpapi.Module{
						Service: productService,
						Pool:    stubWorkerPool{},
					}, nil
				},
				buildImage: func(*logrus.Logger, *runtimeDeps) (*productimagehttpapi.Module, error) {
					order = append(order, "image")
					require.Equal(t, productService, deps.features.productService)
					return &productimagehttpapi.Module{
						Service: imageService,
						Pool:    stubWorkerPool{},
					}, nil
				},
				buildListingKit: func(*logrus.Logger, *runtimeDeps) (*listingkithttpapi.Module, error) {
					order = append(order, "listingkit")
					require.Equal(t, productService, deps.features.productService)
					return &listingkithttpapi.Module{
						Pool: stubWorkerPool{},
					}, nil
				},
			}

			features, err := builder.build(logger, deps, tt.options)
			require.NoError(t, err)
			require.Equal(t, tt.wantOrder, order)
			require.NotNil(t, features.productModule)
			require.Equal(t, tt.wantImageModule, features.imageModule != nil)
			require.Equal(t, tt.wantListingKitModule, features.listingKitModule != nil)
		})
	}
}
