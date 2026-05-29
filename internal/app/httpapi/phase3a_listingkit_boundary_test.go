package httpapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListingKitSupportFileKeepsFeatureOwnedServiceBundlesOutOfAppLayer(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("listingkit_support.go")
	require.NoError(t, err)
	require.NotContains(t, string(src), "newListingKitBuildServiceRepositories")
	require.NotContains(t, string(src), "newListingKitBuildServiceHooks")
	require.Contains(t, string(src), "BuildRuntimeSupport")
}

func TestListingKitTemporalWorkerUsesRuntimeSupportPath(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("listingkit_temporal_worker.go")
	require.NoError(t, err)
	require.Contains(t, string(src), "Runtime: newListingKitRuntimeBuildInput")
	require.NotContains(t, string(src), "ServiceInput: newListingKitBuildServiceInput")
}
