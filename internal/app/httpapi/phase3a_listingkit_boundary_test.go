package httpapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListingKitSupportFileStaysRetired(t *testing.T) {
	t.Parallel()

	_, err := os.Stat("listingkit_support.go")
	require.True(t, os.IsNotExist(err), "listingkit_support.go should stay retired; ListingKit runtime input shaping belongs in feature_builder_listingkit.go")

	featureBuilderSrc, err := os.ReadFile("feature_builder_listingkit.go")
	require.NoError(t, err)
	require.Contains(t, string(featureBuilderSrc), "func newListingKitRuntimeBuildInput(")
	require.Contains(t, string(featureBuilderSrc), "RuntimeSupportInput{")
	require.Contains(t, string(featureBuilderSrc), "BuildRuntimeSupport")
}

func TestListingKitTemporalWorkerUsesRuntimeSupportPath(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("listingkit_temporal_worker.go")
	require.NoError(t, err)
	require.Contains(t, string(src), "Runtime: newListingKitRuntimeBuildInput")
	require.NotContains(t, string(src), "ServiceInput: newListingKitBuildServiceInput")
}
