package httpapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPAPIModulesFileDoesNotOwnFeatureBuildWrappers(t *testing.T) {
	src, err := os.ReadFile("modules.go")
	require.NoError(t, err)
	require.NotContains(t, string(src), "func buildProductModule(")
	require.NotContains(t, string(src), "func buildImageModule(")
	require.NotContains(t, string(src), "func buildAmazonListingModule(")
	require.NotContains(t, string(src), "func buildListingKitModule(")
}
