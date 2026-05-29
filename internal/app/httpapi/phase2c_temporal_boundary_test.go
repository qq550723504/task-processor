package httpapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListingKitTemporalWorkerEntrypointDoesNotOwnDirectServiceStartup(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("listingkit_temporal_worker.go")
	require.NoError(t, err)

	content := string(src)
	require.NotContains(t, content, "BuildService(")
	require.NotContains(t, content, "StartListingKitSheinPublishTemporalWorker(")
}
