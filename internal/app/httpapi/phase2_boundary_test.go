package httpapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPAPIModulesFileDoesNotOwnFeatureBuildWrappers(t *testing.T) {
	content := readRetiredModulesFileIfPresent(t)
	require.NotContains(t, content, "func buildProductModule(")
	require.NotContains(t, content, "func buildImageModule(")
	require.NotContains(t, content, "func buildAmazonListingModule(")
	require.NotContains(t, content, "func buildListingKitModule(")
}

func TestHTTPAPIModulesFileStaysRetired(t *testing.T) {
	requireModulesFileRetired(t)
}

func TestHTTPAPIAppDoesNotOwnProductImageBuilderShadows(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"modules_productimage_models.go",
		"modules_productimage_components.go",
		"modules_object_storage.go",
	} {
		_, err := os.Stat(name)
		require.True(t, os.IsNotExist(err), "%s should stay retired; ProductImage HTTP builder assembly belongs in internal/productimage/httpapi", name)
	}
}

func TestHTTPAPIAppDoesNotOwnProductEnrichScorerBuilderShadow(t *testing.T) {
	t.Parallel()

	_, err := os.Stat("modules_product_scorer.go")
	require.True(t, os.IsNotExist(err), "modules_product_scorer.go should stay retired; ProductEnrich scorer assembly belongs in internal/productenrich/httpapi")
}

func TestHTTPAPIModulesFileDoesNotOwnBootstrapOrchestration(t *testing.T) {
	modulesContent := readRetiredModulesFileIfPresent(t)

	for _, marker := range []string{
		"func buildBootstrap(",
		"buildRuntimeDeps(",
		"configureSheinLoginAccount(",
		"newHTTPFeatureCompositionBuilder().build(",
		"runtimeBundle.buildServerBundle(",
	} {
		require.NotContains(t, modulesContent, marker)
	}

	bootstrapSrc, err := os.ReadFile("bootstrap.go")
	require.NoError(t, err)
	bootstrapContent := string(bootstrapSrc)
	for _, marker := range []string{
		"func buildBootstrap(",
		"buildRuntimeDeps(",
		"configureSheinLoginAccount(",
		"newHTTPFeatureCompositionBuilder().build(",
		"runtimeBundle.buildServerBundle(",
	} {
		require.Contains(t, bootstrapContent, marker)
	}
}

func TestHTTPAPIModulesFileDoesNotOwnLegacyBuildHandlersFacade(t *testing.T) {
	modulesContent := readRetiredModulesFileIfPresent(t)

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

func readRetiredModulesFileIfPresent(t *testing.T) string {
	t.Helper()

	src, err := os.ReadFile("modules.go")
	if os.IsNotExist(err) {
		return ""
	}
	require.NoError(t, err)
	return string(src)
}

func requireModulesFileRetired(t *testing.T) {
	t.Helper()

	_, err := os.Stat("modules.go")
	require.True(t, os.IsNotExist(err), "modules.go should stay retired; add focused files instead of reviving the historical God file")
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
