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

func TestListingKitTemporalWorkerEntrypointFileStaysRetired(t *testing.T) {
	t.Parallel()

	_, err := os.Stat("listingkit_temporal_worker.go")
	require.True(t, os.IsNotExist(err), "listingkit_temporal_worker.go should stay retired; Temporal runtime ownership belongs in internal/listingkit/httpapi")
}
