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
		"productRouteHandler",
		"imageRouteHandler",
		"[]worker.WorkerPool",
	} {
		require.Contains(t, facadeContent, marker)
	}
	for _, marker := range []string{
		`"task-processor/internal/productenrich"`,
		`"task-processor/internal/productimage/httpapi"`,
		"productenrich.ProductHandler",
		"productimagehttpapi.RouteHandler",
	} {
		require.NotContains(t, facadeContent, marker)
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

func TestHTTPAPICompositionBuilderDoesNotOwnAmazonListingRuntimeInput(t *testing.T) {
	compositionSrc, err := os.ReadFile("composition_builder.go")
	require.NoError(t, err)
	compositionContent := string(compositionSrc)

	for _, marker := range []string{
		"amazonlistinghttpapi.RuntimeBuildInput{",
		"deps.attachAmazonListingModule(",
		"ProductService:",
		"ImageService:",
	} {
		require.NotContains(t, compositionContent, marker)
	}

	featureBuilderSrc, err := os.ReadFile("feature_builder_amazonlisting.go")
	require.NoError(t, err)
	featureBuilderContent := string(featureBuilderSrc)
	for _, marker := range []string{
		"amazonlistinghttpapi.RuntimeBuildInput{",
		"deps.attachAmazonListingModule(",
		"ProductService:",
		"ImageService:",
	} {
		require.Contains(t, featureBuilderContent, marker)
	}
}

func TestHTTPAPICompositionBuilderDoesNotOwnListingKitRuntimeInput(t *testing.T) {
	compositionSrc, err := os.ReadFile("composition_builder.go")
	require.NoError(t, err)
	compositionContent := string(compositionSrc)

	for _, marker := range []string{
		"newListingKitRuntimeBuildInput(",
		"deps.attachListingKitModule(",
	} {
		require.NotContains(t, compositionContent, marker)
	}

	featureBuilderSrc, err := os.ReadFile("feature_builder_listingkit.go")
	require.NoError(t, err)
	featureBuilderContent := string(featureBuilderSrc)
	for _, marker := range []string{
		"newListingKitRuntimeBuildInput(",
		"deps.attachListingKitModule(",
	} {
		require.Contains(t, featureBuilderContent, marker)
	}
}

func TestHTTPAPICompositionBuilderDoesNotOwnFeatureModuleBuilderContracts(t *testing.T) {
	compositionSrc, err := os.ReadFile("composition_builder.go")
	require.NoError(t, err)
	compositionContent := string(compositionSrc)

	for _, marker := range []string{
		`"task-processor/internal/amazonlisting/httpapi"`,
		`"task-processor/internal/listingkit/httpapi"`,
		`"task-processor/internal/productenrich/httpapi"`,
		`"task-processor/internal/productimage/httpapi"`,
		"RuntimeBuildInput",
		"BuildRuntimeModule",
		"*amazonlistinghttpapi.Module",
		"*listingkithttpapi.Module",
		"*productenrichhttpapi.Module",
		"*productimagehttpapi.Module",
	} {
		require.NotContains(t, compositionContent, marker)
	}

	contractSrc, err := os.ReadFile("feature_module_builders.go")
	require.NoError(t, err)
	contractContent := string(contractSrc)
	for _, marker := range []string{
		"type productModuleBuilder func(",
		"type imageModuleBuilder func(",
		"type amazonListingModuleBuilder func(",
		"type listingKitModuleBuilder func(",
		"func buildProductModuleResult(",
		"func buildImageModuleResult(",
		"func buildAmazonListingModuleResult(",
		"func buildListingKitModuleResult(",
		"BuildRuntimeModule",
	} {
		require.Contains(t, contractContent, marker)
	}
}
