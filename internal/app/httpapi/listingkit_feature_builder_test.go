package httpapi

import (
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"task-processor/internal/core/config"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	"task-processor/internal/sheinlogin"
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
		shared: &sharedRuntimeDeps{cfg: &config.Config{}},
		features: &featureRuntimeState{
			productSnapshotReader: stubCompositionProductSnapshotReader{},
			listingKitSupport: &listingKitSupport{
				approvedAssetReader: stubCompositionApprovedAssetReader{},
				sheinCookieStore:    &sheinlogin.RedisStore{},
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

func TestListingKitFeatureBuilderDoesNotRegisterWithoutSheinCookieStore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  *config.Config
	}{
		{name: "cookie store config missing", cfg: &config.Config{}},
		{name: "cookie store initialization fails", cfg: &config.Config{Platforms: config.PlatformsConfig{Shein: config.PlatformConfig{CookieRedis: config.RedisConfig{Host: "127.0.0.1", Port: 1}}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := &runtimeDeps{
				shared: &sharedRuntimeDeps{cfg: tt.cfg},
				features: &featureRuntimeState{
					productSnapshotReader: stubCompositionProductSnapshotReader{},
					listingKitSupport:     &listingKitSupport{approvedAssetReader: stubCompositionApprovedAssetReader{}},
				},
			}
			built := false
			builder := listingKitFeatureBuilder{buildListingKit: func(listingkithttpapi.RuntimeBuildInput) (*listingkithttpapi.Module, error) {
				built = true
				return &listingkithttpapi.Module{Pool: stubWorkerPool{}}, nil
			}}

			module, err := builder.build(logrus.New(), deps)
			if err == nil || !strings.Contains(err.Error(), "cookie store") {
				t.Fatalf("build() module/error = %#v/%v, want explicit cookie store error", module, err)
			}
			if built {
				t.Fatal("ListingKit module builder was called without a usable SHEIN cookie store")
			}
			if module != nil {
				t.Fatalf("build() module = %#v, want nil before module/pool registration", module)
			}
		})
	}
}
