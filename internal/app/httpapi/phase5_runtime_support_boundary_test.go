package httpapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListingKitRuntimeSupportPrerequisitesStayOutOfFeatureOwnedBundleAssembly(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("runtime_support_listingkit.go")
	require.NoError(t, err)
	content := string(src)

	require.NotContains(t, content, "BuildServiceRepositories{")
	require.NotContains(t, content, "BuildServiceHooks{")
	require.NotContains(t, content, "newListingKitBuildServiceRepositories")
	require.NotContains(t, content, "newListingKitBuildServiceHooks")
	require.NotContains(t, content, "BuildRuntimeSupport(")

	require.Contains(t, content, "sheinloginbootstrap.BuildRedisStore")
	require.Contains(t, content, "newSDSSyncServiceForHTTPAPI")
	require.Contains(t, content, "sdsbootstrap.NewBaselineRemoteProvider")
}
