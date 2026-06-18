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

func TestHTTPAPIModulesFileDoesNotOwnListingKitSDSRuntimeSupportHook(t *testing.T) {
	t.Parallel()

	modulesContent := readRetiredModulesFileIfPresent(t)

	require.NotContains(t, modulesContent, `"task-processor/internal/sds/httpbootstrap"`)
	require.NotContains(t, modulesContent, "var newSDSSyncServiceForHTTPAPI")
	require.NotContains(t, modulesContent, "sdsbootstrap.NewSyncService")

	supportSrc, err := os.ReadFile("runtime_support_listingkit.go")
	require.NoError(t, err)
	supportContent := string(supportSrc)

	require.Contains(t, supportContent, `"task-processor/internal/sds/httpbootstrap"`)
	require.Contains(t, supportContent, "var newSDSSyncServiceForHTTPAPI")
	require.Contains(t, supportContent, "sdsbootstrap.NewSyncService")
}
