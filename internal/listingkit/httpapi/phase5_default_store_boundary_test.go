package httpapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRuntimeSupportDoesNotUseRetiredDefaultStoreResolver(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("runtime_support_hooks.go")
	require.NoError(t, err)
	require.NotContains(t, string(src), "DefaultSheinStoreIDResolver")
	require.NotContains(t, string(src), "ResolveDefaultSheinStoreID")
}

func TestHTTPAPIPackageDoesNotOwnDefaultStoreHeuristicFile(t *testing.T) {
	t.Parallel()

	_, err := os.ReadFile("defaults.go")
	require.Error(t, err)
	require.True(t, os.IsNotExist(err), "defaults.go should be removed from httpapi once the heuristic moves back to listingkit")
}
