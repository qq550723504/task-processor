package httpapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPAPITypesDoesNotOwnNamedWorkerPoolMap(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("types.go")
	require.NoError(t, err)

	content := string(src)
	require.NotContains(t, content, "func (c httpFeatureComposition) namedWorkerPools(")
	require.NotContains(t, content, "func (c httpFeatureComposition) workerPools(")
}

func TestHTTPAPITypesDoesNotOwnFeatureCompositionMethods(t *testing.T) {
	t.Parallel()

	typesSrc, err := os.ReadFile("types.go")
	require.NoError(t, err)

	for _, marker := range []string{
		"func (c httpFeatureComposition) runtimeModules(",
		"func (c httpFeatureComposition) routeModules(",
		"func (c httpFeatureComposition) buildRuntimeBundle(",
		"func (c httpFeatureComposition) buildServerBundle(",
		"func (c httpFeatureComposition) productHandler(",
		"func (c httpFeatureComposition) imageHandler(",
		"func (c httpFeatureComposition) taskRPCHTTPModule(",
	} {
		require.NotContains(t, string(typesSrc), marker)
	}

	compositionSrc, err := os.ReadFile("composition_modules.go")
	require.NoError(t, err)
	for _, marker := range []string{
		"func (c httpFeatureComposition) runtimeModules(",
		"func (c httpFeatureComposition) routeModules(",
		"func (c httpFeatureComposition) buildRuntimeBundle(",
		"func (c httpFeatureComposition) buildServerBundle(",
		"func (c httpFeatureComposition) productHandler(",
		"func (c httpFeatureComposition) imageHandler(",
		"func (c httpFeatureComposition) taskRPCHTTPModule(",
	} {
		require.Contains(t, string(compositionSrc), marker)
	}
}

func TestHTTPAPITypesDoesNotOwnRouteHandlerContracts(t *testing.T) {
	t.Parallel()

	typesSrc, err := os.ReadFile("types.go")
	require.NoError(t, err)

	for _, marker := range []string{
		"type productRouteHandler =",
		"type imageRouteHandler =",
		"type amazonListingRouteHandler =",
		"type listingKitRouteHandler =",
		"type sheinLoginRouteHandler interface",
		"type routeDescriptor =",
	} {
		require.NotContains(t, string(typesSrc), marker)
	}

	routeTypesSrc, err := os.ReadFile("route_handler_types.go")
	require.NoError(t, err)
	for _, marker := range []string{
		"type productRouteHandler =",
		"type imageRouteHandler =",
		"type amazonListingRouteHandler =",
		"type listingKitRouteHandler =",
		"type sheinLoginRouteHandler interface",
		"type routeDescriptor =",
	} {
		require.Contains(t, string(routeTypesSrc), marker)
	}
}
