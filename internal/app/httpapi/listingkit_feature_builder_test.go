package httpapi

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	listingkithttpapi "task-processor/internal/listingkit/httpapi"
)

func TestNewListingKitFeatureBuilderUsesFeatureOwnedRuntimeBuilder(t *testing.T) {
	t.Parallel()

	builder := newListingKitFeatureBuilder()
	require.Equal(t,
		runtime.FuncForPC(reflect.ValueOf(buildListingKitModuleResult).Pointer()).Name(),
		runtime.FuncForPC(reflect.ValueOf(builder.buildListingKit).Pointer()).Name(),
	)
}

func TestListingKitFeatureBuilderUsesCatalogAndApprovedAssetReaders(t *testing.T) {
	t.Parallel()

	deps := &runtimeDeps{
		shared: &sharedRuntimeDeps{},
		features: &featureRuntimeState{
			productSnapshotReader: stubCompositionProductSnapshotReader{},
			listingKitSupport: &listingKitSupport{
				approvedAssetReader: stubCompositionApprovedAssetReader{},
			},
		},
	}
	built := false
	builder := listingKitFeatureBuilder{
		buildListingKit: func(input listingkithttpapi.RuntimeBuildInput) (*listingkithttpapi.Module, error) {
			built = true
			require.NotNil(t, input.Runtime.ProductSnapshotReader)
			require.NotNil(t, input.Runtime.Support.Repositories.Core.ApprovedAsset)
			return &listingkithttpapi.Module{Pool: stubWorkerPool{}}, nil
		},
	}

	module, err := builder.build(logrus.New(), deps)
	require.NoError(t, err)
	require.True(t, built)
	require.NotNil(t, module)
}
