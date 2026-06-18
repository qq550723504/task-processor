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

func TestHTTPAPIModulesFileDoesNotOwnWorkerRuntimeSupport(t *testing.T) {
	t.Parallel()

	modulesSrc, err := os.ReadFile("modules.go")
	require.NoError(t, err)
	modulesContent := string(modulesSrc)

	for _, marker := range []string{
		"func newWorkerPool(",
		"func buildLocalTaskHealthProvider(",
		"worker.NewPoolWithConfig(",
		"TaskTimeout:",
		"GetQueueStats()",
		"GetMetrics()",
	} {
		require.NotContains(t, modulesContent, marker)
	}

	workerSupportSrc, err := os.ReadFile("runtime_worker_pools.go")
	require.NoError(t, err)
	workerSupportContent := string(workerSupportSrc)
	for _, marker := range []string{
		"func newWorkerPool(",
		"func buildLocalTaskHealthProvider(",
		"worker.NewPoolWithConfig(",
		"TaskTimeout:",
		"GetQueueStats()",
		"GetMetrics()",
	} {
		require.Contains(t, workerSupportContent, marker)
	}
}

func TestHTTPAPIModulesFileDoesNotOwnLoginRuntimeSupport(t *testing.T) {
	t.Parallel()

	modulesSrc, err := os.ReadFile("modules.go")
	require.NoError(t, err)
	modulesContent := string(modulesSrc)

	for _, marker := range []string{
		`"task-processor/internal/shein/client"`,
		`"task-processor/internal/sheinlogin/bootstrap"`,
		`"task-processor/internal/sdslogin/bootstrap"`,
		"sheinclient.ConfigureLoginAccountFromConfig(",
		"func buildSheinLoginModuleResult(",
		"func buildSDSLoginModuleResult(",
		"sheinloginbootstrap.BuildHandler(",
		"sdsloginbootstrap.BuildHandler(",
	} {
		require.NotContains(t, modulesContent, marker)
	}

	loginSupportSrc, err := os.ReadFile("runtime_login_modules.go")
	require.NoError(t, err)
	loginSupportContent := string(loginSupportSrc)
	for _, marker := range []string{
		`"task-processor/internal/shein/client"`,
		`"task-processor/internal/sheinlogin/bootstrap"`,
		`"task-processor/internal/sdslogin/bootstrap"`,
		"func configureSheinLoginAccount(",
		"func buildSheinLoginModuleResult(",
		"func buildSDSLoginModuleResult(",
		"sheinclient.ConfigureLoginAccountFromConfig(",
		"sheinloginbootstrap.BuildHandler(",
		"sdsloginbootstrap.BuildHandler(",
	} {
		require.Contains(t, loginSupportContent, marker)
	}
}

func TestHTTPAPICompositionBuilderDoesNotOwnLoginBootstrapTypes(t *testing.T) {
	t.Parallel()

	compositionSrc, err := os.ReadFile("composition_builder.go")
	require.NoError(t, err)
	compositionContent := string(compositionSrc)

	for _, marker := range []string{
		`"task-processor/internal/sheinlogin/bootstrap"`,
		`"task-processor/internal/sdslogin/bootstrap"`,
		"*sheinloginbootstrap.BuildResult",
		"*sdsloginbootstrap.BuildResult",
	} {
		require.NotContains(t, compositionContent, marker)
	}

	loginSupportSrc, err := os.ReadFile("runtime_login_modules.go")
	require.NoError(t, err)
	loginSupportContent := string(loginSupportSrc)
	for _, marker := range []string{
		"type sheinLoginModuleResult = sheinloginbootstrap.BuildResult",
		"type sdsLoginModuleResult = sdsloginbootstrap.BuildResult",
		"type sheinLoginModuleBuilder func(",
		"type sdsLoginModuleBuilder func(",
		"*sheinLoginModuleResult",
		"*sdsLoginModuleResult",
	} {
		require.Contains(t, loginSupportContent, marker)
	}
}

func TestHTTPAPICompositionBuilderDoesNotOwnLoginFeatureAssembly(t *testing.T) {
	t.Parallel()

	compositionSrc, err := os.ReadFile("composition_builder.go")
	require.NoError(t, err)
	compositionContent := string(compositionSrc)

	for _, marker := range []string{
		"deps.addClosers(sheinLoginCloser)",
		"deps.addClosers(sdsLoginCloser)",
		"deps.attachSDSLoginResult(",
	} {
		require.NotContains(t, compositionContent, marker)
	}

	loginSupportSrc, err := os.ReadFile("runtime_login_modules.go")
	require.NoError(t, err)
	loginSupportContent := string(loginSupportSrc)
	for _, marker := range []string{
		"type loginFeatureBuilder struct",
		"type loginFeatureSet struct",
		"deps.addClosers(sheinLoginCloser)",
		"deps.addClosers(sdsLoginCloser)",
		"deps.attachSDSLoginResult(",
	} {
		require.Contains(t, loginSupportContent, marker)
	}
}

func TestHTTPAPIRuntimeStateDoesNotOwnLoginBootstrapResultTypes(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"types.go", "runtime_deps_methods.go"} {
		src, err := os.ReadFile(name)
		require.NoError(t, err)
		content := string(src)

		for _, marker := range []string{
			`"task-processor/internal/sheinlogin/bootstrap"`,
			`"task-processor/internal/sdslogin/bootstrap"`,
			"*sheinloginbootstrap.BuildResult",
			"*sdsloginbootstrap.BuildResult",
		} {
			require.NotContains(t, content, marker)
		}
	}

	loginSupportSrc, err := os.ReadFile("runtime_login_modules.go")
	require.NoError(t, err)
	loginSupportContent := string(loginSupportSrc)
	for _, marker := range []string{
		"type sheinLoginModuleResult = sheinloginbootstrap.BuildResult",
		"type sdsLoginModuleResult = sdsloginbootstrap.BuildResult",
		"type sheinLoginModuleBuilder func(",
		"type sdsLoginModuleBuilder func(",
	} {
		require.Contains(t, loginSupportContent, marker)
	}
}

func TestHTTPAPIRuntimeStateDoesNotOwnSupportModuleResultTypes(t *testing.T) {
	t.Parallel()

	typesSrc, err := os.ReadFile("types.go")
	require.NoError(t, err)
	typesContent := string(typesSrc)

	for _, marker := range []string{
		`"task-processor/internal/promptmgmt/api"`,
		`"task-processor/internal/sds/httpapi"`,
		`"task-processor/internal/taskrpcapi"`,
		"*promptmgmtapi.BuildResult",
		"*sdshttpapi.BuildResult",
		"*taskrpcapi.BuildResult",
	} {
		require.NotContains(t, typesContent, marker)
	}

	resultSrc, err := os.ReadFile("runtime_module_results.go")
	require.NoError(t, err)
	resultContent := string(resultSrc)
	for _, marker := range []string{
		"type promptModuleResult = promptmgmtapi.BuildResult",
		"type sdsModuleResult = sdshttpapi.BuildResult",
		"type taskRPCModuleResult = taskrpcapi.BuildResult",
	} {
		require.Contains(t, resultContent, marker)
	}
}

func TestHTTPAPIRuntimeStateDoesNotOwnFeatureHTTPAPIModuleTypes(t *testing.T) {
	t.Parallel()

	typesSrc, err := os.ReadFile("types.go")
	require.NoError(t, err)
	typesContent := string(typesSrc)

	for _, marker := range []string{
		`"task-processor/internal/amazonlisting/httpapi"`,
		`"task-processor/internal/listingkit/httpapi"`,
		`"task-processor/internal/productenrich/httpapi"`,
		`"task-processor/internal/productimage/httpapi"`,
		"*amazonlistinghttpapi.Module",
		"*listingkithttpapi.Module",
		"*productenrichhttpapi.Module",
		"*productimagehttpapi.Module",
		"productimagehttpapi.RouteHandler",
	} {
		require.NotContains(t, typesContent, marker)
	}

	contractSrc, err := os.ReadFile("feature_module_builders.go")
	require.NoError(t, err)
	contractContent := string(contractSrc)
	for _, marker := range []string{
		"type productModuleResult = productenrichhttpapi.Module",
		"type imageModuleResult = productimagehttpapi.Module",
		"type amazonListingModuleResult = amazonlistinghttpapi.Module",
		"type listingKitModuleResult = listingkithttpapi.Module",
	} {
		require.Contains(t, contractContent, marker)
	}
}

func TestHTTPAPICompositionBuilderDoesNotOwnSupportModuleBuilderContracts(t *testing.T) {
	t.Parallel()

	compositionSrc, err := os.ReadFile("composition_builder.go")
	require.NoError(t, err)
	compositionContent := string(compositionSrc)

	for _, marker := range []string{
		`"task-processor/internal/core/config"`,
		`"task-processor/internal/prompt"`,
		`"task-processor/internal/promptmgmt/api"`,
		`"task-processor/internal/sds/httpapi"`,
		`"task-processor/internal/taskrpcapi"`,
		"*promptmgmtapi.BuildResult",
		"*sdshttpapi.BuildResult",
		"*taskrpcapi.BuildResult",
		"taskrpcapi.ClientProvider",
		"taskrpcapi.LocalStatusProvider",
		"prompt.TenantPromptStore",
	} {
		require.NotContains(t, compositionContent, marker)
	}

	resultSrc, err := os.ReadFile("runtime_module_results.go")
	require.NoError(t, err)
	resultContent := string(resultSrc)
	for _, marker := range []string{
		"type promptModuleBuilder func(",
		"type sdsModuleBuilder func(",
		"type taskRPCModuleBuilder func(",
		"func buildPromptModuleResult(",
		"func buildSDSModuleResult(",
		"func buildTaskRPCModuleResult(",
		"*promptModuleResult",
		"*sdsModuleResult",
		"*taskRPCModuleResult",
	} {
		require.Contains(t, resultContent, marker)
	}
}

func TestHTTPAPICompositionBuilderDoesNotOwnSupportFeatureAssembly(t *testing.T) {
	t.Parallel()

	compositionSrc, err := os.ReadFile("composition_builder.go")
	require.NoError(t, err)
	compositionContent := string(compositionSrc)

	for _, marker := range []string{
		"deps.shared.tenantPromptStore",
		"buildRuntimeBundleFromModules(",
		"runtimeBundle.localTaskHealthProvider()",
		"composition.promptModule = b.buildPrompt(",
		"composition.taskRPCResult = taskRPCResult",
		"composition.sdsModule = b.buildSDS(",
	} {
		require.NotContains(t, compositionContent, marker)
	}

	resultSrc, err := os.ReadFile("runtime_module_results.go")
	require.NoError(t, err)
	resultContent := string(resultSrc)
	for _, marker := range []string{
		"type supportFeatureBuilder struct",
		"type supportFeatureSet struct",
		"deps.shared.tenantPromptStore",
		"buildRuntimeBundleFromModules(",
		"runtimeBundle.localTaskHealthProvider()",
	} {
		require.Contains(t, resultContent, marker)
	}
}
