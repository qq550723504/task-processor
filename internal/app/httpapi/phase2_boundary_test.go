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

func TestHTTPAPIModulesFileDoesNotOwnLegacyBuildHandlersFacade(t *testing.T) {
	modulesSrc, err := os.ReadFile("modules.go")
	require.NoError(t, err)
	modulesContent := string(modulesSrc)

	for _, marker := range []string{
		"func BuildHandlers(",
		`"task-processor/internal/infra/worker"`,
		`"task-processor/internal/productenrich"`,
		`"task-processor/internal/productimage/httpapi"`,
		"productenrich.ProductHandler",
		"productimagehttpapi.RouteHandler",
		"[]worker.WorkerPool",
	} {
		require.NotContains(t, modulesContent, marker)
	}

	facadeSrc, err := os.ReadFile("handlers_legacy.go")
	require.NoError(t, err)
	facadeContent := string(facadeSrc)
	for _, marker := range []string{
		"func BuildHandlers(",
		`"task-processor/internal/infra/worker"`,
		`"task-processor/internal/productenrich"`,
		`"task-processor/internal/productimage/httpapi"`,
		"productenrich.ProductHandler",
		"productimagehttpapi.RouteHandler",
		"[]worker.WorkerPool",
	} {
		require.Contains(t, facadeContent, marker)
	}
}

func TestHTTPAPICompositionBuilderDoesNotOwnProductImageRuntimeInputs(t *testing.T) {
	compositionSrc, err := os.ReadFile("composition_builder.go")
	require.NoError(t, err)
	compositionContent := string(compositionSrc)

	for _, marker := range []string{
		"productenrichhttpapi.RuntimeBuildInput{",
		"productimagehttpapi.RuntimeBuildInput{",
		"deps.attachProductModule(",
		"deps.attachImageModule(",
		"ImageWorkDir:",
	} {
		require.NotContains(t, compositionContent, marker)
	}

	featureBuilderSrc, err := os.ReadFile("feature_builder_listingkit.go")
	require.NoError(t, err)
	featureBuilderContent := string(featureBuilderSrc)
	for _, marker := range []string{
		"productenrichhttpapi.RuntimeBuildInput{",
		"productimagehttpapi.RuntimeBuildInput{",
		"deps.attachProductModule(",
		"deps.attachImageModule(",
		"ImageWorkDir:",
	} {
		require.Contains(t, featureBuilderContent, marker)
	}
}
